// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

// Two barriers against the mistake this window was made of twice.
//
// The settings window was a stack of items whose heights this package computed:
// toolkit.FormFieldLabelH() plus a slack constant plus n times a row constant,
// summed again in a second place to size the window. Every widget in it was a
// toolkit widget and every one of them was in a box, so nothing about "use the
// toolkit" or "use a layout" would have caught it. What was hand-made was the
// ARITHMETIC.
//
// It cannot be caught by a type either: an int is an int. So it is caught here,
// by reading this package's own source and by checking a laid-out tree.
package desk

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-widgets/toolkit"
	"github.com/go-xrkit/xrkit/glasses"
)

// TestNoWidgetIsGivenAHandComputedSize.
//
// A box item with neither Flex nor Size takes the widget's own measured size
// (toolkit v0.268.0), so an explicit Size is only ever right for something that
// CANNOT measure itself -- a band, a spacer, a fixed slot. Anything else is this
// package deciding how tall a widget is, which is how the window came to be
// right at exactly one size.
//
// The rule is therefore about the ARGUMENT: a Size may be a constant of this
// package's own (a band height, a button width), and may not be arithmetic over
// the toolkit's metrics. FormFieldLabelH() + slack + n*rowH() is the shape that
// went wrong, and it is the shape this refuses.
func TestNoWidgetIsGivenAHandComputedSize(t *testing.T) {
	for _, file := range packageFiles(t) {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			if sel, ok := lit.Type.(*ast.SelectorExpr); !ok || sel.Sel.Name != "Item" {
				return true
			}
			for _, elt := range lit.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				key, ok := kv.Key.(*ast.Ident)
				if !ok || key.Name != "Size" {
					continue
				}
				if bad, why := handComputed(kv.Value); bad {
					t.Errorf("%s: an Item.Size %s. A widget that can measure "+
						"itself needs no Size at all; one that cannot takes a "+
						"constant of this package, not arithmetic over the "+
						"toolkit's metrics", fset.Position(kv.Pos()), why)
				}
			}
			return true
		})
	}
}

// handComputed reports whether an Item.Size expression is this package doing a
// widget's sizing for it: arithmetic, or a call to one of the toolkit's metric
// accessors. toolkit.Scaled(SomeConstant) is fine -- that is the one documented
// way to turn a logical constant into device pixels.
func handComputed(e ast.Expr) (bool, string) {
	switch v := e.(type) {
	case *ast.BinaryExpr:
		return true, "is arithmetic"
	case *ast.CallExpr:
		sel, ok := v.Fun.(*ast.SelectorExpr)
		if !ok {
			return false, ""
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || pkg.Name != "toolkit" {
			return false, ""
		}
		if sel.Sel.Name == "Scaled" {
			// Scaling a constant is the documented way; scaling arithmetic is
			// the same defect one level down.
			for _, a := range v.Args {
				if bad, why := handComputed(a); bad {
					return true, "scales something that " + why
				}
			}
			return false, ""
		}
		return true, "calls toolkit." + sel.Sel.Name + ", which is a widget's own metric"
	}
	return false, ""
}

// packageFiles is every non-test Go file of this package.
func packageFiles(t *testing.T) []string {
	t.Helper()
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, n := range names {
		if strings.HasSuffix(n, "_test.go") {
			continue
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		t.Fatal("no source files found; the barrier is reading nothing")
	}
	return out
}

// TestTheBarrierWouldHaveCaughtTheOldWindow is the negative control: the shape
// that shipped must be the shape the check refuses. A barrier that has never
// rejected anything is a comment.
func TestTheBarrierWouldHaveCaughtTheOldWindow(t *testing.T) {
	fset := token.NewFileSet()
	// Exactly what this package used to write, in both of the ways it wrote it.
	src := `package p
import "github.com/go-widgets/toolkit"
func f(box *toolkit.Container, n int) {
	box.Add(toolkit.Item{Size: fieldH(n), Widget: nil})
	box.Add(toolkit.Item{Size: toolkit.FormFieldLabelH() + n*20, Widget: nil})
	box.Add(toolkit.Item{Size: toolkit.Scaled(ButtonBarH), Widget: nil})
	box.Add(toolkit.Item{Size: toolkit.Scaled(4 * 12), Widget: nil})
}`
	f, err := parser.ParseFile(fset, "old.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var verdicts []string
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		if sel, ok := lit.Type.(*ast.SelectorExpr); !ok || sel.Sel.Name != "Item" {
			return true
		}
		for _, elt := range lit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			if key, ok := kv.Key.(*ast.Ident); !ok || key.Name != "Size" {
				continue
			}
			bad, _ := handComputed(kv.Value)
			verdicts = append(verdicts, map[bool]string{true: "refused", false: "allowed"}[bad])
		}
		return true
	})
	want := []string{"allowed", "refused", "allowed", "refused"}
	if len(verdicts) != len(want) {
		t.Fatalf("read %d sizes, want %d: %v", len(verdicts), len(want), verdicts)
	}
	for i := range want {
		if verdicts[i] != want[i] {
			t.Errorf("size %d was %s, want %s", i, verdicts[i], want[i])
		}
	}
	// The first one -- a call to a helper of this package, fieldH(n) -- is
	// ALLOWED by this check, and it is what actually shipped. Reading through a
	// local helper is a job for a type checker, not a parser, so the second
	// barrier below is what covers it: whatever the arithmetic, the tree it
	// produces has to be right.
}

// TestTheSettingsWindowHasNothingOutsideItsParent, and nothing unplaced.
//
// This is the barrier that does not care how a size was arrived at: it checks
// the RESULT. toolkit.LayoutProblems walks the laid-out tree and reports a child
// outside its parent -- drawing over a sibling or off the surface -- and a child
// nobody placed, which draws nothing and hit-tests nothing, silently.
func TestTheSettingsWindowHasNothingOutsideItsParent(t *testing.T) {
	cfg := &Config{}
	attached := []glasses.USB{oneS, luma}
	root, _ := settingsRoot(cfg, attached, 0, nil, func() {})
	w, h := settingsSize(*cfg, attached)
	root.SetBounds(toolkit.Rect{X: 0, Y: 0, W: w, H: h})

	for _, p := range toolkit.LayoutProblems(root) {
		switch p.Kind {
		case toolkit.LayoutOutside:
			t.Errorf("%v", p)
		case toolkit.LayoutUnplaced:
			// Every control in this window is meant to be placed. There is no
			// CardLayout here and nothing is hidden, so an unplaced widget is a
			// widget somebody forgot -- report it as one.
			t.Errorf("%v", p)
		}
	}

	// The negative control, on this very tree: displace one card and the check
	// has to say so. A check that has only ever returned nothing is a comment.
	cards := found[*toolkit.SettingsGroup](root)
	if len(cards) == 0 {
		t.Fatal("the window has no cards to displace")
	}
	cards[0].SetBounds(toolkit.Rect{X: w + 100, Y: 0, W: 200, H: 100})
	var sawOutside bool
	for _, p := range toolkit.LayoutProblems(root) {
		if p.Kind == toolkit.LayoutOutside {
			sawOutside = true
		}
	}
	if !sawOutside {
		t.Error("a card moved right out of the window was not reported")
	}
}
