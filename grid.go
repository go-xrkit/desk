// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"fmt"

	"github.com/go-xrkit/xrkit/ribbon"
	"github.com/go-xrkit/xrkit/stereo"
)

// DefaultGapPx is the band the gallery leaves between two cells, in pixels.
//
// The ribbon needs no such number — the space between two screens is whatever
// the ribbon left there — but a grid is a fold this package invents, so the
// space in it is this package's to choose. Wide enough that two cells read as
// two screens rather than as one wide one.
const DefaultGapPx = 48

// A Grid is every screen at once, in front of the viewer.
//
// It is the flat counterpart of the ribbon's gallery, and it is head-locked on
// purpose: the band may be anywhere, but the grid is always straight ahead, so
// opening it and closing it costs no motion and shows the ribbon exactly as it
// was left.
//
// Left and right WRAP, because the band is a circle and the last screen really
// is next to the first. Up and down CLAMP, because the fold into rows is this
// type's invention and has no seam to walk through.
type Grid struct {
	n          int
	cols, rows int
	viewW      int
	viewH      int
	cellW      int
	cellH      int
	padX, padY int
	gap        int
	srcW, srcH int
	srcY       []int32
	sel        int
	// screens is how many of the cells are screens. The rest — one — is where a
	// screen is added. It is zero for a grid built through NewGridCols, which
	// means every cell is a screen.
	screens int
}

// DefaultColumns is how wide the gallery is when nobody says.
//
// Three, whenever three columns hold every screen in three rows or fewer —
// which is exactly the three, six and nine a desk is usually built from. A
// FIXED width is the point: a screen keeps its place in the grid as others are
// added, so the map a person builds of where things are survives the desk
// growing. Past nine it stops paying, and the shape is chosen by what leaves
// the screens biggest instead.
const DefaultColumns = 3

// NewGrid folds n screens of srcW x srcH into a view of viewW x viewH, in
// [DefaultColumns] where that holds them and in whatever leaves them biggest
// otherwise.
//
// It lays out ONE MORE cell than there are screens: the last one is where a
// screen is added. A gallery is where somebody looks at the desk they have, so
// it is where they will want another one — and the alternative is a settings
// file, which is not where anybody is when they run out of room.
func NewGrid(n, srcW, srcH, viewW, viewH, gap int) (*Grid, error) {
	if n <= 0 {
		return nil, fmt.Errorf("%w: %d screens", ErrNoScreens, n)
	}
	cells := n + 1 // the last one is the "add a screen" cell
	// The column count is chosen from the SCREENS, not the cells. Three, six or
	// nine screens keep three columns whether or not the adder pushes them into
	// another row — a screen holding its COLUMN is what the fixed width is for,
	// and it would be lost if adding one cell could turn nine screens from three
	// columns into four.
	cols := 0
	if n <= DefaultColumns*DefaultColumns {
		cols = min(n, DefaultColumns)
	}
	g, err := NewGridCols(cells, srcW, srcH, viewW, viewH, gap, cols)
	if err != nil {
		return nil, err
	}
	g.screens = n
	return g, nil
}

// NewGridCols folds the screens into exactly cols columns. A cols of zero or
// less asks for the shape that leaves the screens biggest, and so does a cols
// that turns out to leave no cell at all: a person asking for six columns in a
// window too narrow for them should get a gallery, not a refusal.
func NewGridCols(n, srcW, srcH, viewW, viewH, gap, cols int) (*Grid, error) {
	switch {
	case n <= 0:
		return nil, fmt.Errorf("%w: %d screens", ErrNoScreens, n)
	case srcW <= 0 || srcH <= 0:
		return nil, fmt.Errorf("%w: screens of %dx%d", ErrScreens, srcW, srcH)
	case viewW <= 0 || viewH <= 0:
		return nil, fmt.Errorf("%w: a view of %dx%d", ErrScreens, viewW, viewH)
	case gap < 0:
		return nil, fmt.Errorf("%w: a gap of %d", ErrScreens, gap)
	}
	g := &Grid{n: n, viewW: viewW, viewH: viewH, gap: gap, srcW: srcW, srcH: srcH}
	// The shape is chosen by what it leaves the screens: of every column count,
	// the one whose cells come out biggest. Area, not width, because a taller
	// grid that fits is worth more than a wider one that has to shrink to.
	best := -1
	lo, hi := 1, n
	if cols > 0 && cols <= n {
		lo, hi = cols, cols
	}
	for cols := lo; cols <= hi; cols++ {
		rows := (n + cols - 1) / cols
		w := (viewW - (cols-1)*gap) / cols
		h := (viewH - (rows-1)*gap) / rows
		if w <= 0 || h <= 0 {
			continue
		}
		// Each cell keeps the screen's own shape, so the smaller of the two
		// fits decides.
		if w*srcH > h*srcW {
			w = h * srcW / srcH
		} else {
			h = w * srcH / srcW
		}
		if w <= 0 || h <= 0 {
			continue
		}
		if area := w * h; area > best {
			g.cols, g.rows, g.cellW, g.cellH, best = cols, rows, w, h, area
		}
	}
	if best < 0 {
		if lo != 1 || hi != n {
			// The asked-for width does not fit. Fall back on choosing rather than
			// refusing: the number of columns is a preference, and a gallery in
			// the wrong shape beats no gallery at all.
			return NewGridCols(n, srcW, srcH, viewW, viewH, gap, 0)
		}
		return nil, fmt.Errorf("%w: %d screens with a %d-pixel gap in a %dx%d view",
			ErrScreens, n, gap, viewW, viewH)
	}
	gridW := g.cols*g.cellW + (g.cols-1)*gap
	gridH := g.rows*g.cellH + (g.rows-1)*gap
	g.padX, g.padY = (viewW-gridW)/2, (viewH-gridH)/2
	g.srcY = make([]int32, g.cellH)
	for y := range g.srcY {
		g.srcY[y] = int32(int64(y) * int64(srcH) / int64(g.cellH))
	}
	return g, nil
}

// Frame appends the blits for the whole grid and returns the extended slice.
// There is no offset argument, and that is the point: the grid is head-locked.
func (g *Grid) Frame(dst []ribbon.Blit) []ribbon.Blit {
	for i := 0; i < g.n; i++ {
		row, col := i/g.cols, i%g.cols
		// A ragged last row is left-aligned rather than centred: a screen that
		// moves sideways when another one is added is a screen you have to
		// look for.
		x := g.padX + col*(g.cellW+g.gap)
		y := g.padY + row*(g.cellH+g.gap)
		dst = append(dst, ribbon.Blit{
			Screen:   i,
			Dst:      stereo.Rect{X: x, Y: y, W: g.cellW, H: g.cellH},
			SrcX:     0,
			SrcXStep: int64(g.srcW) << fracBits / int64(g.cellW),
			SrcY:     g.srcY,
		})
	}
	return dst
}

// Cell is where screen i sits in the view, for a caller drawing a highlight
// round the selected one without asking this type for pixels.
func (g *Grid) Cell(i int) (x, y, w, h int, ok bool) {
	if i < 0 || i >= g.n {
		return 0, 0, 0, 0, false
	}
	row, col := i/g.cols, i%g.cols
	return g.padX + col*(g.cellW+g.gap), g.padY + row*(g.cellH+g.gap), g.cellW, g.cellH, true
}

// Selected is the highlighted screen.
func (g *Grid) Selected() int { return g.sel }

// Select highlights screen i.
func (g *Grid) Select(i int) error {
	if i < 0 || i >= g.n {
		return fmt.Errorf("%w: screen %d of %d", ErrScreens, i, g.n)
	}
	g.sel = i
	return nil
}

// Move walks the selection. Left and right wrap; up and down stay in their
// column, and do nothing when it has no cell that way.
func (g *Grid) Move(d ribbon.Direction) error {
	switch d {
	case ribbon.Right:
		g.sel = (g.sel + 1) % g.n
	case ribbon.Left:
		g.sel = (g.sel + g.n - 1) % g.n
	case ribbon.Up:
		if g.sel >= g.cols {
			g.sel -= g.cols
		}
	case ribbon.Down:
		// Only when there is a cell DIRECTLY below. Clamping to the last cell
		// instead moved the selection sideways: with six screens the grid is
		// three columns and the last row holds only the cell that adds one, so
		// a person pressing down in the middle column arrived in the first --
		// measured, 4 -> 6, and reported as "un probleme de deplacement
		// colonne". Down is a COLUMN move; a column that ends, ends.
		if g.sel+g.cols < g.n {
			g.sel += g.cols
		}
	default:
		return fmt.Errorf("%w: %s", ErrScreens, d)
	}
	return nil
}

// Len is how many screens the grid holds. It is what makes a Grid a
// [ribbon.Selectable], so the navigator drives it without knowing that these
// screens are flat.
// Len is how many SCREENS this gallery is of, which is what a navigator needs
// of it — not how many cells it draws. The cell that adds a screen is this
// type's own furniture and is none of the navigator's business.
func (g *Grid) Len() int { return g.Screens() }

// Cells is how many cells it draws, screens plus the one that adds one.
func (g *Grid) Cells() int { return g.n }

// Shape is the grid's columns and rows, for a caller that wants to describe it.
func (g *Grid) Shape() (cols, rows int) { return g.cols, g.rows }

// At is the screen whose cell contains the point, in view coordinates.
//
// It is how a mouse chooses a screen. The gaps between cells and the margin
// round the grid belong to nothing: a click there is a click on the background,
// which is a click that should do nothing rather than the nearest thing.
func (g *Grid) At(x, y int) (int, bool) {
	for i := 0; i < g.n; i++ {
		cx, cy, w, h, _ := g.Cell(i)
		if x >= cx && x < cx+w && y >= cy && y < cy+h {
			return i, true
		}
	}
	return 0, false
}

// Screens is how many of the cells are screens.
func (g *Grid) Screens() int {
	if g.screens > 0 {
		return g.screens
	}
	return g.n
}

// Adder is the index of the cell that adds a screen, and false when this grid
// has none.
func (g *Grid) Adder() (int, bool) {
	if g.screens <= 0 || g.screens >= g.n {
		return 0, false
	}
	return g.n - 1, true
}

// IsAdder reports whether i is the cell that adds a screen.
func (g *Grid) IsAdder(i int) bool {
	a, ok := g.Adder()
	return ok && i == a
}
