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
		// One more than the screens: the last cell is where a screen is added.
		if len(blits) != n+1 {
			t.Errorf("%d screens gave %d cells, want %d", n, len(blits), n+1)
		}
		seen := map[int]bool{}
		for _, b := range blits {
			seen[b.Screen] = true
			if b.Dst.X < 0 || b.Dst.Y < 0 ||
				b.Dst.X+b.Dst.W > 1920 || b.Dst.Y+b.Dst.H > 1200 {
				t.Errorf("%d screens: cell %d is at %+v, outside the view", n, b.Screen, b.Dst)
			}
		}
		if len(seen) != n+1 {
			t.Errorf("%d screens: %d cells are drawn, want %d", n, len(seen), n+1)
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
	// Six screens and the cell that adds one: seven cells, 3x3.
	g := testGrid(t, 6)
	cols, _ := g.Shape()
	last := g.Cells() - 1

	if err := g.Select(0); err != nil {
		t.Fatal(err)
	}
	if err := g.Move(ribbon.Left); err != nil {
		t.Fatal(err)
	}
	if got := g.Selected(); got != last {
		t.Errorf("left from the first is %d, want the last cell %d", got, last)
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
	// Down again reaches the third row, which is where the cell that adds a
	// screen lives when there are six.
	if err := g.Move(ribbon.Down); err != nil {
		t.Fatal(err)
	}
	if got := g.Selected(); got != last {
		t.Errorf("down from the middle row is %d, want the adder %d", got, last)
	}
	if err := g.Move(ribbon.Down); err != nil {
		t.Fatal(err)
	}
	if got := g.Selected(); got != last {
		t.Errorf("down from the bottom row moved to %d", got)
	}

	// And up from a lower row really does move, which is what makes the clamp
	// above a clamp rather than a stuck selection.
	if err := g.Move(ribbon.Up); err != nil {
		t.Fatal(err)
	}
	if got := g.Selected(); got != cols {
		t.Errorf("up from the adder is %d, want %d", got, cols)
	}

	// A ragged last row: down from a column the last row does not have lands on
	// the nearest cell it does, rather than off the end.
	r := testGrid(t, 5) // five screens and an adder: 3x2
	if err := r.Select(2); err != nil {
		t.Fatal(err)
	}
	if err := r.Move(ribbon.Down); err != nil {
		t.Fatal(err)
	}
	if got, want := r.Selected(), r.Cells()-1; got != want {
		t.Errorf("down from the ragged column is %d, want the last cell %d", got, want)
	}
}

func TestGridRefusals(t *testing.T) {
	g := testGrid(t, 4)
	past := g.Cells() // one past the last cell, adder included
	if err := g.Select(past); !errors.Is(err, ErrScreens) {
		t.Errorf("Select(%d) = %v, want an ErrScreens", past, err)
	}
	if err := g.Select(-1); !errors.Is(err, ErrScreens) {
		t.Errorf("Select(-1) = %v, want an ErrScreens", err)
	}
	if err := g.Move(ribbon.Direction(99)); !errors.Is(err, ErrScreens) {
		t.Errorf("Move(99) = %v, want an ErrScreens", err)
	}
	if _, _, _, _, ok := g.Cell(past); ok {
		t.Errorf("Cell(%d) answered for a cell there is not", past)
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

// TestAScreenKeepsItsPlaceAsTheDeskGrows is what a fixed width is FOR.
//
// Three, six and nine screens are the counts a desk is usually built from, and
// in three columns each of them is the one before it with a row added. A screen
// that moved when another was created would cost the viewer the map they had
// built of where things are.
func TestAScreenKeepsItsPlaceAsTheDeskGrows(t *testing.T) {
	var was []int
	for _, n := range []int{3, 6, 9} {
		g := testGrid(t, n)
		// Three columns at every one of these counts. The ROWS grow, and the
		// cell that adds a screen takes one more slot, but the column a screen
		// is in is what a person remembers.
		cols, rows := g.Shape()
		if cols != 3 {
			t.Errorf("%d screens fold into %dx%d, want three columns", n, cols, rows)
		}
		// The COLUMN a screen is in, not its pixel position: the cells shrink as
		// rows are added, so everything shifts a little, but screen 4 is under
		// screen 1 whether the desk has six screens or nine.
		var xs []int
		for i := 0; i < n; i++ {
			xs = append(xs, i%3)
		}
		if was != nil {
			for i := range was {
				if xs[i] != was[i] {
					t.Errorf("%d screens: screen %d moved from column %d to column %d",
						n, i, was[i], xs[i])
				}
			}
		}
		was = xs
	}
}

// TestAWidthThatDoesNotFitIsAPreferenceAndNotARefusal.
func TestAWidthThatDoesNotFitIsAPreferenceAndNotARefusal(t *testing.T) {
	// Eight columns of a 300-pixel view, with a 48-pixel gap between each pair,
	// wants more gap than there is view.
	g, err := NewGridCols(8, 1920, 1200, 300, 300, DefaultGapPx, 8)
	if err != nil {
		t.Fatalf("NewGridCols = %v", err)
	}
	if cols, _ := g.Shape(); cols == 8 {
		t.Error("eight columns were used in a view too narrow for them")
	}
	if g.cellW <= 0 || g.cellH <= 0 {
		t.Errorf("the chosen cell is %dx%d", g.cellW, g.cellH)
	}
	// And asking for more columns than there are screens is simply choosing.
	if _, err := NewGridCols(2, 1920, 1200, 1920, 1200, 0, 9); err != nil {
		t.Errorf("NewGridCols with more columns than screens = %v", err)
	}
}

// TestPastNineTheShapeIsChosen: a fixed three columns stops paying when it
// would make fourteen rows of them.
func TestPastNineTheShapeIsChosen(t *testing.T) {
	g := testGrid(t, 12)
	if cols, _ := g.Shape(); cols == 3 {
		t.Error("twelve screens were folded into three columns; past nine the " +
			"shape should be chosen by what leaves the screens biggest")
	}
}
