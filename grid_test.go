// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"testing"

	"github.com/go-xrkit/xrkit/ribbon"
)

func testGrid(t *testing.T, n int) *Grid {
	t.Helper()
	g, err := NewGrid(n, 1920, 1200, 1920, 1200, DefaultGapPx)
	if err != nil {
		t.Fatalf("NewGrid = %v", err)
	}
	return g
}

// TestEveryScreenIsInTheGridAndInsideTheView.
func TestEveryScreenIsInTheGridAndInsideTheView(t *testing.T) {
	for n := 1; n <= 12; n++ {
		g := testGrid(t, n)
		blits := g.Frame(nil)
		if len(blits) != n {
			t.Errorf("%d screens gave %d cells", n, len(blits))
		}
		seen := map[int]bool{}
		for _, b := range blits {
			seen[b.Screen] = true
			if b.Dst.X < 0 || b.Dst.Y < 0 ||
				b.Dst.X+b.Dst.W > 1920 || b.Dst.Y+b.Dst.H > 1200 {
				t.Errorf("%d screens: cell %d is at %+v, outside the view", n, b.Screen, b.Dst)
			}
		}
		if len(seen) != n {
			t.Errorf("%d screens: only %d of them are drawn", n, len(seen))
		}
	}
}

// TestCellsKeepTheScreensOwnShape: a grid that squashed one axis would show a
// picture of a desktop that is not the shape of that desktop.
func TestCellsKeepTheScreensOwnShape(t *testing.T) {
	for n := 1; n <= 9; n++ {
		g := testGrid(t, n)
		cols, rows := g.Shape()
		want := 1920.0 / 1200.0
		got := float64(g.cellW) / float64(g.cellH)
		if diff := got - want; diff > 0.02 || diff < -0.02 {
			t.Errorf("%d screens in %dx%d: cells are %.3f, want the screen's %.3f",
				n, cols, rows, got, want)
		}
	}
}

// TestCellsDoNotOverlap.
func TestCellsDoNotOverlap(t *testing.T) {
	for n := 2; n <= 9; n++ {
		g := testGrid(t, n)
		blits := g.Frame(nil)
		for i := range blits {
			for j := i + 1; j < len(blits); j++ {
				a, b := blits[i].Dst, blits[j].Dst
				if a.X < b.X+b.W && b.X < a.X+a.W && a.Y < b.Y+b.H && b.Y < a.Y+a.H {
					t.Errorf("%d screens: cells %d and %d overlap: %+v %+v",
						n, blits[i].Screen, blits[j].Screen, a, b)
				}
			}
		}
	}
}

// TestLeftAndRightWrapAndUpAndDownClamp is the whole of moving in a grid: the
// band is a circle, so its ends really are next to each other; the fold into
// rows is this type's own invention and has no seam to walk through.
func TestLeftAndRightWrapAndUpAndDownClamp(t *testing.T) {
	g := testGrid(t, 6) // 3x2 on this view
	cols, _ := g.Shape()

	if err := g.Select(0); err != nil {
		t.Fatal(err)
	}
	if err := g.Move(ribbon.Left); err != nil {
		t.Fatal(err)
	}
	if got := g.Selected(); got != 5 {
		t.Errorf("left from the first is %d, want the last", got)
	}
	if err := g.Move(ribbon.Right); err != nil {
		t.Fatal(err)
	}
	if got := g.Selected(); got != 0 {
		t.Errorf("right from the last is %d, want the first", got)
	}

	// Up from the top row stays put, rather than wrapping into a row the
	// viewer would then have to find.
	if err := g.Move(ribbon.Up); err != nil {
		t.Fatal(err)
	}
	if got := g.Selected(); got != 0 {
		t.Errorf("up from the top row moved to %d", got)
	}
	if err := g.Move(ribbon.Down); err != nil {
		t.Fatal(err)
	}
	if got := g.Selected(); got != cols {
		t.Errorf("down from the top row is %d, want %d", got, cols)
	}
	if err := g.Move(ribbon.Down); err != nil {
		t.Fatal(err)
	}
	if got := g.Selected(); got != cols {
		t.Errorf("down from the bottom row moved to %d", got)
	}

	// And up from a lower row really does move, which is what makes the clamp
	// above a clamp rather than a stuck selection.
	if err := g.Move(ribbon.Up); err != nil {
		t.Fatal(err)
	}
	if got := g.Selected(); got != 0 {
		t.Errorf("up from the second row is %d, want 0", got)
	}

	// A ragged last row: down from a column the last row does not have lands on
	// the nearest screen it does, rather than off the end.
	r := testGrid(t, 5) // 3x2, with one screen in the last row
	if err := r.Select(2); err != nil {
		t.Fatal(err)
	}
	if err := r.Move(ribbon.Down); err != nil {
		t.Fatal(err)
	}
	if got := r.Selected(); got != 4 {
		t.Errorf("down from the ragged column is %d, want the last screen 4", got)
	}
}

func TestGridRefusals(t *testing.T) {
	g := testGrid(t, 4)
	if err := g.Select(4); !errors.Is(err, ErrScreens) {
		t.Errorf("Select(4) = %v, want an ErrScreens", err)
	}
	if err := g.Select(-1); !errors.Is(err, ErrScreens) {
		t.Errorf("Select(-1) = %v, want an ErrScreens", err)
	}
	if err := g.Move(ribbon.Direction(99)); !errors.Is(err, ErrScreens) {
		t.Errorf("Move(99) = %v, want an ErrScreens", err)
	}
	if _, _, _, _, ok := g.Cell(4); ok {
		t.Error("Cell(4) answered for a screen there is not")
	}
	if _, _, _, _, ok := g.Cell(-1); ok {
		t.Error("Cell(-1) answered")
	}
	x, y, w, h, ok := g.Cell(0)
	if !ok || w <= 0 || h <= 0 || x < 0 || y < 0 {
		t.Errorf("Cell(0) = %d,%d %dx%d ok=%v", x, y, w, h, ok)
	}

	for name, tc := range map[string]struct {
		n, srcW, srcH, viewW, viewH, gap int
		is                               error
	}{
		"no screens":     {0, 1920, 1200, 1920, 1200, 0, ErrNoScreens},
		"no source":      {2, 0, 1200, 1920, 1200, 0, ErrScreens},
		"no source rows": {2, 1920, 0, 1920, 1200, 0, ErrScreens},
		"no view":        {2, 1920, 1200, 0, 1200, 0, ErrScreens},
		"no view rows":   {2, 1920, 1200, 1920, 0, 0, ErrScreens},
		"a negative gap": {2, 1920, 1200, 1920, 1200, -1, ErrScreens},
		// Enough gap that no shape leaves a cell at all.
		"nothing fits": {4, 1920, 1200, 100, 100, 200, ErrScreens},
	} {
		if _, err := NewGrid(tc.n, tc.srcW, tc.srcH, tc.viewW, tc.viewH, tc.gap); !errors.Is(err, tc.is) {
			t.Errorf("%s: NewGrid = %v, want a %v", name, err, tc.is)
		}
	}
}

// TestAShapeThatWouldLeaveNoCellIsPassedOver.
//
// In a view four pixels wide, four columns leave one pixel each — and a cell
// that keeps a 16:10 screen's shape is then zero pixels tall. That shape is not
// a candidate, and the grid falls back on one that is.
func TestAShapeThatWouldLeaveNoCellIsPassedOver(t *testing.T) {
	g, err := NewGrid(4, 1920, 1200, 4, 1200, 0)
	if err != nil {
		t.Fatalf("NewGrid = %v", err)
	}
	cols, rows := g.Shape()
	if cols != 1 {
		t.Errorf("the grid is %dx%d; four columns of one pixel cannot hold a cell",
			cols, rows)
	}
	if g.cellW <= 0 || g.cellH <= 0 {
		t.Errorf("the chosen cell is %dx%d", g.cellW, g.cellH)
	}
}
