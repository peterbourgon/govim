// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package source

import (
	"fmt"
	"go/ast"

	"github.com/myitcv/govim/cmd/govim/internal/golang_org_x_tools/lsp/snippet"
)

// structFieldSnippets calculates the plain and placeholder snippets for struct literal field names.
func (c *completer) structFieldSnippets(label, detail string) (*snippet.Builder, *snippet.Builder) {
	if !c.wantStructFieldCompletions() {
		return nil, nil
	}

	// If we are in a deep completion then we can't be completing a field
	// name (e.g. "Foo{f<>}" completing to "Foo{f.Bar}" should not generate
	// a snippet).
	if c.inDeepCompletion() {
		return nil, nil
	}

	clInfo := c.enclosingCompositeLiteral

	// If we are already in a key-value expression, we don't want a snippet.
	if clInfo.kv != nil {
		return nil, nil
	}

	plain, placeholder := &snippet.Builder{}, &snippet.Builder{}
	label = fmt.Sprintf("%s: ", label)

	// A plain snippet turns "Foo{Ba<>" into "Foo{Bar: <>".
	plain.WriteText(label)
	plain.WritePlaceholder(nil)

	// A placeholder snippet turns "Foo{Ba<>" into "Foo{Bar: <*int*>".
	placeholder.WriteText(label)
	placeholder.WritePlaceholder(func(b *snippet.Builder) {
		b.WriteText(detail)
	})

	// If the cursor position is on a different line from the literal's opening brace,
	// we are in a multiline literal.
	if c.view.Session().Cache().FileSet().Position(c.pos).Line != c.view.Session().Cache().FileSet().Position(clInfo.cl.Lbrace).Line {
		plain.WriteText(",")
		placeholder.WriteText(",")
	}

	return plain, placeholder
}

// functionCallSnippets calculates the plain and placeholder snippets for function calls.
func (c *completer) functionCallSnippets(name string, params []string) (*snippet.Builder, *snippet.Builder) {
	// If we are the left side (i.e. "Fun") part of a call expression,
	// we don't want a snippet since there are already parens present.
	if len(c.path) > 1 {
		switch n := c.path[1].(type) {
		case *ast.CallExpr:
			if n.Fun == c.path[0] {
				return nil, nil
			}
		case *ast.SelectorExpr:
			if len(c.path) > 2 {
				if call, ok := c.path[2].(*ast.CallExpr); ok && call.Fun == c.path[1] {
					return nil, nil
				}
			}
		}
	}
	plain, placeholder := &snippet.Builder{}, &snippet.Builder{}
	label := fmt.Sprintf("%s(", name)

	// A plain snippet turns "someFun<>" into "someFunc(<>)".
	plain.WriteText(label)
	if len(params) > 0 {
		plain.WritePlaceholder(nil)
	}
	plain.WriteText(")")

	// A placeholder snippet turns "someFun<>" into "someFunc(<*i int*>, *s string*)".
	placeholder.WriteText(label)
	for i, p := range params {
		if i > 0 {
			placeholder.WriteText(", ")
		}
		placeholder.WritePlaceholder(func(b *snippet.Builder) {
			b.WriteText(p)
		})
	}
	placeholder.WriteText(")")

	return plain, placeholder
}
