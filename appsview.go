// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit"
	"github.com/go-xrkit/xrkit/ribbon"
)

// AppIconPx is the SMALLEST an application tile's icon may be, in pixels.
//
// The size actually used comes from the height of the picture — see appsIconPx —
// because a tile read at arm's length through a pair of glasses is not a tile
// read on a monitor. This is the floor under that, for a picture small enough
// that a twelfth of it would be nothing.
const AppIconPx = 48

// An appsView is the gallery of running applications.
//
// It is a toolkit [toolkit.IconGrid] and nothing else: the cells, the labels,
// the selection field, the empty state and the hit testing are the toolkit's,
// and this file only decides what the cells SAY and where the grid sits. That is
// the same division as the badge and the gallery's marks — this package draws no
// pixel of its own.
//
// The selection moves the way the screen gallery's does, because a person should
// not have to learn two galleries: left and right WRAP, up and down CLAMP. Up
// and down need the grid's column count, which is why the toolkit exports it.
type appsView struct {
	grid  *toolkit.IconGrid
	theme *toolkit.Theme
	apps  []App
}

// newAppsView prepares an empty gallery.
func newAppsView(theme *toolkit.Theme) *appsView {
	if theme == nil {
		theme = toolkit.DefaultDark()
	}
	// The two galleries agree on what "chosen" looks like.
	//
	// The screen gallery rings the chosen cell in [SelectionInk] -- orange,
	// picked because it is the one thing on the picture that says which screen
	// Enter would take. An IconGrid fields its selection in the theme ACCENT,
	// which is blue, so the two galleries would answer the same question in two
	// colours. A copy of the theme, with the accent set to the same ink, is the
	// whole fix: everything else about the theme is untouched, and no widget is
	// drawn by hand to get it.
	mine := *theme
	mine.Accent = SelectionInk
	theme = &mine
	g := toolkit.NewIconGrid()
	g.SetIconSize(AppIconPx)
	g.Empty = "nothing with a window to show — every application AX will describe is already here"
	return &appsView{grid: g, theme: theme}
}

// set replaces what the gallery shows, keeping the selection on the same
// APPLICATION where it still exists rather than on the same index.
//
// A person looking at a gallery has their eye on a name; the list underneath is
// re-read every time it is opened and an application that quit shifts every
// index after it. Keeping the index would move the highlight to a neighbour
// under their eye.
func (v *appsView) set(apps []App) {
	was, _ := v.app()
	v.apps = apps
	cells := make([]toolkit.IconCell, len(apps))
	for i, a := range apps {
		cell := toolkit.IconCell{Label: a.String()}
		// The application's OWN icon when somebody looked it up, and a drawn
		// window when not. An icon is how a person recognises a program -- the
		// Dock, Command-Tab and Mission Control all show one -- and the glyph
		// is the honest fallback rather than the intent.
		if ic := a.Icon; ic != nil && len(ic.Pix) == ic.W*ic.H*4 && ic.W > 0 {
			cell.Image = toolkit.NewImageFit(ic.Pix, ic.W, ic.H)
			cell.Image.Alt = a.Name
		} else {
			cell.Icon = toolkit.DrawIconApp
		}
		cells[i] = cell
	}
	v.grid.Cells = cells
	sel := 0
	for i, a := range apps {
		if a.Name == was.Name {
			sel = i
			break
		}
	}
	if len(apps) == 0 {
		sel = -1
	}
	v.grid.SetSelected(sel)
}

// app is the highlighted application.
func (v *appsView) app() (App, bool) {
	i := v.grid.Selected().Get()
	if i < 0 || i >= len(v.apps) {
		return App{}, false
	}
	return v.apps[i], true
}

// selected is the highlighted index, or -1.
func (v *appsView) selected() int { return v.grid.Selected().Get() }

// move walks the selection: left and right wrap, up and down clamp.
func (v *appsView) move(d ribbon.Direction) error {
	n := len(v.apps)
	if n == 0 {
		return ErrNoApps
	}
	i := v.grid.Selected().Get()
	if i < 0 {
		i = 0
	}
	cols := v.grid.Columns()
	switch d {
	case ribbon.Left:
		i = (i - 1 + n) % n
	case ribbon.Right:
		i = (i + 1) % n
	case ribbon.Up:
		if i-cols >= 0 {
			i -= cols
		}
	case ribbon.Down:
		if i+cols < n {
			i += cols
		}
	}
	v.grid.SetSelected(i)
	return nil
}

// at is the application under a point of the picture, for a click.
func (v *appsView) at(x, y int) (int, bool) {
	i := v.grid.IndexAt(x, y)
	if i < 0 || i >= len(v.apps) {
		return 0, false
	}
	return i, true
}

// draw paints the gallery over the whole picture.
//
// Head-locked and full-view, like the screen gallery: it is a fold this package
// invents, so it is always straight ahead and costs no motion to open or close.
func (v *appsView) draw(c *Canvas) {
	if v == nil || c == nil || c.W <= 0 || c.H <= 0 {
		return
	}
	// The font is restored by a defer for the same reason the badge restores it:
	// whatever draws next asked for its own.
	was := toolkit.CurrentFont()
	toolkit.SetFont(toolkit.NewBitmapFont(appsScale(c.H)))
	defer toolkit.SetFont(was)

	// The tile is sized FROM THE PICTURE, not from a constant, and this is the
	// difference between a gallery and a toolbar. Rendered at 1920x1200 with
	// fixed metrics, eight applications came out as one row of ten 190-pixel
	// cells across the top with their names cut to "Code (1" and "Termina",
	// under a blank two thirds of the view.
	//
	// A third of the width each, which is [DefaultColumns] — the same three
	// columns the screen gallery folds into, so the two galleries look like the
	// same place — and a label that has room for what a tile exists to say.
	// A margin, because the edge of this picture is the edge of a PANEL: a label
	// that ends at the last column of pixels reads as cut off whether or not it
	// is, and the longest one — "Firefox (2 windows) on screens 2, 4" — did
	// exactly that against the right edge.
	pad := c.W / 64
	w, h := c.W-2*pad, c.H-2*pad
	v.grid.MinCellW = w / DefaultColumns
	v.grid.SetIconSize(appsIconPx(h, len(v.apps)))
	v.grid.SetBounds(toolkit.Rect{X: pad, Y: pad, W: w, H: h})
	v.grid.Draw(painter.NewPixelPainterBGRA(c.Pix, c.W, c.H), v.theme)
}

// appsIconPx is the icon square: enough for the rows this many applications
// need to FILL the view, within reason.
//
// A gallery is the whole picture, not a strip along the top of it. Eight
// applications at a fixed twelfth of the height came out as three tight rows
// over two thirds of nothing — which is what a toolbar looks like, and the
// screen gallery beside it fills the view.
//
// So the icon is a row's worth of height less its label band, which is about a
// quarter of a cell. Bounded at both ends: never below [AppIconPx], where it
// stops being a picture, and never above a quarter of the view, so three
// applications are three tiles and not three posters.
func appsIconPx(h, apps int) int {
	rows := (apps + DefaultColumns - 1) / DefaultColumns
	if rows < 1 {
		rows = 1
	}
	px := h / rows * 3 / 4
	if max := h / 4; px > max {
		px = max
	}
	if px < AppIconPx {
		px = AppIconPx
	}
	return px
}

// appsScale magnifies the built-in font for a gallery read at arm's length
// through a pair of glasses, from the height of the picture rather than a
// constant — the same rule as the badge, and a STEP SMALLER because a tile holds
// a sentence and not a digit.
//
// Measured on the rendering: at a six-hundredth the longest label a tile can
// carry -- "Firefox (2 windows) on screens 2, 4", thirty-five characters -- fits
// a third of a 1920-wide view. A step larger and the toolkit elided it to
// "on screens", dropping the two numbers the tile exists to give.
func appsScale(h int) int {
	s := h / 600
	if s < 1 {
		s = 1
	}
	if s > 4 {
		s = 4
	}
	return s
}
