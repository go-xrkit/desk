// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"fmt"

	"github.com/go-xrkit/xrkit/ribbon"
	"github.com/go-xrkit/xrkit/stereo"
)

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
}

// NewGrid folds n screens of srcW x srcH into a view of viewW x viewH.
func NewGrid(n, srcW, srcH, viewW, viewH, gap int) (*Grid, error) {
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
	for cols := 1; cols <= n; cols++ {
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

// Move walks the selection. Left and right wrap; up and down clamp.
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
		if g.sel/g.cols < (g.n-1)/g.cols {
			g.sel = min(g.sel+g.cols, g.n-1)
		}
	default:
		return fmt.Errorf("%w: %s", ErrScreens, d)
	}
	return nil
}

// Shape is the grid's columns and rows, for a caller that wants to describe it.
func (g *Grid) Shape() (cols, rows int) { return g.cols, g.rows }
