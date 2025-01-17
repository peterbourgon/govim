// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lsp

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/myitcv/govim/cmd/govim/internal/golang_org_x_tools/lsp/protocol"
	"github.com/myitcv/govim/cmd/govim/internal/golang_org_x_tools/lsp/source"
	"github.com/myitcv/govim/cmd/govim/internal/golang_org_x_tools/lsp/telemetry/log"
	"github.com/myitcv/govim/cmd/govim/internal/golang_org_x_tools/lsp/telemetry/tag"
	"github.com/myitcv/govim/cmd/govim/internal/golang_org_x_tools/span"
)

func (s *Server) completion(ctx context.Context, params *protocol.CompletionParams) (*protocol.CompletionList, error) {
	uri := span.NewURI(params.TextDocument.URI)
	view := s.session.ViewOf(uri)
	f, m, err := getGoFile(ctx, view, uri)
	if err != nil {
		return nil, err
	}
	spn, err := m.PointSpan(params.Position)
	if err != nil {
		return nil, err
	}
	rng, err := spn.Range(m.Converter)
	if err != nil {
		return nil, err
	}
	candidates, surrounding, err := source.Completion(ctx, view, f, rng.Start, source.CompletionOptions{
		DeepComplete:     s.useDeepCompletions,
		WantDocumentaton: s.wantCompletionDocumentation,
	})
	if err != nil {
		log.Print(ctx, "no completions found", tag.Of("At", rng), tag.Of("Failure", err))
	}
	return &protocol.CompletionList{
		IsIncomplete: false,
		Items:        s.toProtocolCompletionItems(ctx, view, m, candidates, params.Position, surrounding),
	}, nil
}

// Limit deep completion results because in some cases there are too many
// to be useful.
const maxDeepCompletions = 3

func (s *Server) toProtocolCompletionItems(ctx context.Context, view source.View, m *protocol.ColumnMapper, candidates []source.CompletionItem, pos protocol.Position, surrounding *source.Selection) []protocol.CompletionItem {
	// Sort the candidates by score, since that is not supported by LSP yet.
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})
	// We might need to adjust the position to account for the prefix.
	insertionRange := protocol.Range{
		Start: pos,
		End:   pos,
	}
	var prefix string
	if surrounding != nil {
		prefix = strings.ToLower(surrounding.Prefix())
		spn, err := surrounding.Range.Span()
		if err != nil {
			log.Print(ctx, "failed to get span for surrounding position: %s:%v:%v: %v", tag.Of("Position", pos), tag.Of("Failure", err))
		} else {
			rng, err := m.Range(spn)
			if err != nil {
				log.Print(ctx, "failed to convert surrounding position", tag.Of("Position", pos), tag.Of("Failure", err))
			} else {
				insertionRange = rng
			}
		}
	}

	var numDeepCompletionsSeen int

	items := make([]protocol.CompletionItem, 0, len(candidates))
	for i, candidate := range candidates {
		// Match against the label (case-insensitive).
		if !strings.HasPrefix(strings.ToLower(candidate.Label), prefix) {
			continue
		}
		// Limit the number of deep completions to not overwhelm the user in cases
		// with dozens of deep completion matches.
		if candidate.Depth > 0 {
			if !s.useDeepCompletions {
				continue
			}
			if numDeepCompletionsSeen >= maxDeepCompletions {
				continue
			}
			numDeepCompletionsSeen++
		}
		insertText := candidate.InsertText
		if s.insertTextFormat == protocol.SnippetTextFormat {
			insertText = candidate.Snippet(s.usePlaceholders)
		}
		item := protocol.CompletionItem{
			Label:  candidate.Label,
			Detail: candidate.Detail,
			Kind:   toProtocolCompletionItemKind(candidate.Kind),
			TextEdit: &protocol.TextEdit{
				NewText: insertText,
				Range:   insertionRange,
			},
			InsertTextFormat: s.insertTextFormat,
			// This is a hack so that the client sorts completion results in the order
			// according to their score. This can be removed upon the resolution of
			// https://github.com/Microsoft/language-server-protocol/issues/348.
			SortText:      fmt.Sprintf("%05d", i),
			FilterText:    candidate.InsertText,
			Preselect:     i == 0,
			Documentation: candidate.Documentation,
		}
		// Trigger signature help for any function or method completion.
		// This is helpful even if a function does not have parameters,
		// since we show return types as well.
		switch item.Kind {
		case protocol.FunctionCompletion, protocol.MethodCompletion:
			item.Command = &protocol.Command{
				Command: "editor.action.triggerParameterHints",
			}
		}
		items = append(items, item)
	}
	return items
}

func toProtocolCompletionItemKind(kind source.CompletionItemKind) protocol.CompletionItemKind {
	switch kind {
	case source.InterfaceCompletionItem:
		return protocol.InterfaceCompletion
	case source.StructCompletionItem:
		return protocol.StructCompletion
	case source.TypeCompletionItem:
		return protocol.TypeParameterCompletion // ??
	case source.ConstantCompletionItem:
		return protocol.ConstantCompletion
	case source.FieldCompletionItem:
		return protocol.FieldCompletion
	case source.ParameterCompletionItem, source.VariableCompletionItem:
		return protocol.VariableCompletion
	case source.FunctionCompletionItem:
		return protocol.FunctionCompletion
	case source.MethodCompletionItem:
		return protocol.MethodCompletion
	case source.PackageCompletionItem:
		return protocol.ModuleCompletion // ??
	default:
		return protocol.TextCompletion
	}
}
