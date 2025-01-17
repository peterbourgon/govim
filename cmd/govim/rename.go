package main

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/myitcv/govim"
	"github.com/myitcv/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
)

func (v *vimstate) rename(flags govim.CommandFlags, args ...string) error {
	b, pos, err := v.cursorPos()
	if err != nil {
		return fmt.Errorf("failed to get current position: %v", err)
	}
	var renameTo string
	if len(args) == 1 {
		renameTo = args[0]
	} else {
		curr := v.ParseString(v.ChannelExprf(`expand("<cword>")`))
		renameTo = v.ParseString(v.ChannelExprf(`input("govim: rename '%v' to: ", %q)`, curr, curr))
	}
	params := &protocol.RenameParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: string(b.URI()),
		},
		Position: pos.ToPosition(),
		NewName:  renameTo,
	}
	res, err := v.server.Rename(context.Background(), params)
	if err != nil {
		return fmt.Errorf("called to gopls.Rename failed: %v", err)
	}
	var allChanges map[string][]protocol.TextEdit
	if res.Changes != nil {
		allChanges = *res.Changes
	}
	if len(allChanges) == 0 {
		v.Logf("No changes to apply for rename")
		return nil
	}
	// TODO it feels like we need a new config variable for the strategy to use
	// when making edits of this sort (to multiple files). It doesn't feel right
	// to use the value of &switchbuf because there might be multiple changes
	// (as opposed to jumping to a single definition/location).
	//
	// For now we hardcode a split.
	vp := v.Viewport()
	bufNrs := make(map[string]int)
	var fps []string
	for filepath := range allChanges {
		fps = append(fps, filepath)
	}
	sort.Strings(fps)

	for _, filepath := range fps {
		tf := strings.TrimPrefix(filepath, "file://")
		var bufinfo []struct {
			BufNr   int   `json:"bufnr"`
			Windows []int `json:"windows"`
		}
		v.Parse(v.ChannelCall("getbufinfo", tf), &bufinfo)
		switch len(bufinfo) {
		case 0:
		case 1:
			bufNrs[filepath] = bufinfo[0].BufNr
			if len(bufinfo[0].Windows) > 0 {
				continue
			}
		default:
			return fmt.Errorf("got back multiple buffers searching for %v", tf)
		}
		// Hard code split for now
		v.ChannelExf("%v split %v", flags.Mods, tf)
		bufNrs[filepath] = v.ParseInt(v.ChannelCall("bufnr", tf))
	}
	v.ChannelCall("win_gotoid", vp.Current.WinID)

	for _, filepath := range fps {
		changes := allChanges[filepath]
		if len(changes) == 0 {
			continue
		}
		tf := strings.TrimPrefix(filepath, "file://")
		bufnr := bufNrs[filepath]
		b, ok := v.buffers[bufnr]
		if !ok {
			return fmt.Errorf("expected to have a buffer for %v; did not", tf)
		}
		if err := v.applyProtocolTextEdits(b, changes); err != nil {
			return fmt.Errorf("failed to apply edits for %v: %v", tf, err)
		}
	}
	return nil
}
