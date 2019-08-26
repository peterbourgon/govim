package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/myitcv/govim"
	"github.com/myitcv/govim/cmd/govim/config"
	"github.com/myitcv/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/myitcv/govim/cmd/govim/internal/stringfns"
	"github.com/myitcv/govim/cmd/govim/internal/types"
)

func (v *vimstate) stringfns(flags govim.CommandFlags, args ...string) error {
	var transFns []string
	for _, fp := range args {
		if _, ok := stringfns.Functions[fp]; !ok {
			return fmt.Errorf("failed to resolve transformation function %q", fp)
		}
		transFns = append(transFns, fp)
	}
	var err error
	var start, end types.Point
	var b *types.Buffer
	switch *flags.Range {
	case 2:
		// we have a range
		var pos struct {
			BuffNr int    `json:"buffnr"`
			Mode   string `json:"mode"`
			Start  []int  `json:"start"`
			End    []int  `json:"end"`
		}
		v.Parse(v.ChannelExpr(`{"buffnr": bufnr(""), "mode": visualmode(), "start": getpos("'<"), "end": getpos("'>")}`), &pos)

		if pos.Mode == "\x16" {
			return fmt.Errorf("cannot use %v in visual block mode", config.CommandStringFn)
		}

		var ok bool
		b, ok = v.buffers[pos.BuffNr]
		if !ok {
			return fmt.Errorf("failed to derive buffer")
		}

		start, err = types.PointFromVim(b, pos.Start[1], pos.Start[2])
		if err != nil {
			return fmt.Errorf("failed to get start position of range: %v", err)
		}
		if pos.Mode == "V" {
			// we have already parsed start so we can mutate here
			pos.End = pos.Start
			pos.End[1]++
		}
		end, err = types.PointFromVim(b, pos.End[1], pos.End[2])
		if err != nil {
			return fmt.Errorf("failed to get end position of range: %v", err)
		}
		if pos.Mode == "v" {
			// we need to move past the end of the selection
			rem := b.Contents()[end.Offset():]
			if len(rem) > 0 {
				_, adj := utf8.DecodeRune(rem)
				end, err = types.PointFromVim(b, pos.End[1], pos.End[2]+adj)
				if err != nil {
					return fmt.Errorf("failed to get adjusted end position: %v", err)
				}
			}
		}
	case 0:
		// current line
		b, _, err = v.cursorPos()
		if err != nil {
			return fmt.Errorf("failed to get cursor position for line range")
		}
		start, err = types.PointFromVim(b, *flags.Line1, 1)
		if err != nil {
			return fmt.Errorf("failed to get start position of range: %v", err)
		}
		end, err = types.PointFromVim(b, *flags.Line1+1, 1)
		if err != nil {
			return fmt.Errorf("failed to get end position of range: %v", err)
		}
	default:
		return fmt.Errorf("unknown range indicator %v", *flags.Range)
	}

	endOffset := end.Offset()
	if *flags.Range == 0 {
		endOffset--
	}
	newText := string(b.Contents()[start.Offset():endOffset])
	for _, fp := range transFns {
		fn := stringfns.Functions[fp]
		newText, err = fn(string(newText))
		if err != nil {
			return fmt.Errorf("failed to apply ")
		}
	}
	if *flags.Range == 0 {
		newText += "\n"
	}

	edit := protocol.TextEdit{
		Range: protocol.Range{
			Start: start.ToPosition(),
			End:   end.ToPosition(),
		},
		NewText: newText,
	}
	return v.applyProtocolTextEdits(b, []protocol.TextEdit{edit})
}

func (v *vimstate) stringfncomplete(args ...json.RawMessage) (interface{}, error) {
	lead := v.ParseString(args[0])
	var results []string
	for k := range stringfns.Functions {
		if strings.HasPrefix(k, lead) {
			results = append(results, k)
		}
	}
	sort.Strings(results)
	return results, nil
}
