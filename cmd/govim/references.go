package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strings"

	"github.com/myitcv/govim"
	"github.com/myitcv/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/myitcv/govim/cmd/govim/internal/golang_org_x_tools/span"
	"github.com/myitcv/govim/cmd/govim/internal/types"
)

func (v *vimstate) references(flags govim.CommandFlags, args ...string) error {
	v.quickfixIsDiagnostics = false
	b, pos, err := v.cursorPos()
	if err != nil {
		return fmt.Errorf("failed to get current position: %v", err)
	}
	params := &protocol.ReferenceParams{
		Context: protocol.ReferenceContext{
			IncludeDeclaration: true,
		},
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: string(b.URI()),
			},
			Position: pos.ToPosition(),
		},
	}

	// TODO this will become fragile at some point
	cwd := v.ParseString(v.ChannelCall("getcwd"))

	// must be non-nil
	locs := []quickfixEntry{}

	refs, err := v.server.References(context.Background(), params)
	if err != nil {
		return fmt.Errorf("called to gopls.References failed: %v", err)
	}
	if len(refs) == 0 {
		return fmt.Errorf("unexpected zero length of references")
	}
	for _, ref := range refs {
		var buf *types.Buffer
		for _, b := range v.buffers {
			if b.URI() == span.URI(ref.URI) {
				buf = b
			}
		}
		fn := span.URI(ref.URI).Filename()
		v.Logf("fn: %v\n", fn)
		if buf == nil {
			byts, err := ioutil.ReadFile(fn)
			if err != nil {
				v.Logf("references: failed to read contents of %v: %v", fn, err)
				continue
			}
			// create a temp buffer
			buf = types.NewBuffer(-1, fn, byts)
		}
		// make fn relative for reporting purposes
		fn, err := filepath.Rel(cwd, fn)
		if err != nil {
			v.Logf("references: failed to call filepath.Rel(%q, %q): %v", cwd, fn, err)
			continue
		}
		p, err := types.PointFromPosition(buf, ref.Range.Start)
		if err != nil {
			v.Logf("references: failed to resolve position: %v", err)
			continue
		}
		line, err := buf.Line(p.Line())
		if err != nil {
			v.Logf("references: location invalid in buffer: %v", err)
			continue
		}
		locs = append(locs, quickfixEntry{
			Filename: fn,
			Lnum:     p.Line(),
			Col:      p.Col(),
			Text:     line,
		})
	}
	toSort := locs[1:]
	// the first entry will always be the definition
	sort.Slice(toSort, func(i, j int) bool {
		lhs, rhs := toSort[i], toSort[j]
		cmp := strings.Compare(lhs.Filename, rhs.Filename)
		if cmp != 0 {
			if lhs.Filename == locs[0].Filename {
				return true
			} else if rhs.Filename == locs[0].Filename {
				return false
			}
		}
		if cmp == 0 {
			cmp = lhs.Lnum - rhs.Lnum
		}
		if cmp == 0 {
			cmp = lhs.Col - rhs.Col
		}
		return cmp < 0
	})
	v.ChannelCall("setqflist", locs, "r")
	v.ChannelEx("copen")
	return nil
}
