// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"strconv"

	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit"
)

// SelectionInk is the colour of the border round the chosen screen.
//
// Orange, and not the theme's accent. The accent is a blue that has to sit
// politely next to the interface it belongs to; this has to be found at a
// glance, in a headset, against six captured desktops whose contents nobody
// chose. Orange is the one strong hue that is in almost no window chrome and in
// very little wallpaper.
var SelectionInk = toolkit.RGB(0xFF, 0x8C, 0x1A)

// SelectionWidth is how thick that border is.
//
// Thick. A one-pixel ring round a tile that subtends a hand's width in a
// headset is a rumour; this is the whole answer to "which one am I about to
// choose", so it is drawn like one.
const SelectionWidth = 8

// marks are what a person picks BY: a border round the chosen screen, a number
// on every screen, and a line at the bottom saying what Enter would do.
//
// Without them the gallery is unusable, and it was: the arrows moved a
// selection that was NOWHERE ON THE PICTURE. Six identical desktops, a
// highlight kept in a variable, and Enter jumping to whichever one the viewer
// had lost count of.
//
// The border came second. The first version marked the selection by colouring
// its number, which was reported straight back as not legible — a small
// coloured pill on one of six tiles is a cue and not an answer.
type marks struct {
	theme *toolkit.Theme
	badge *toolkit.Badge
	says  *toolkit.Toast
}

func newMarks(theme *toolkit.Theme) *marks {
	if theme == nil {
		theme = toolkit.DefaultDark()
	}
	says := toolkit.NewToast("", toolkit.ToastInfo)
	says.Visible = true // for as long as the gallery is
	return &marks{theme: theme, badge: toolkit.NewBadge(""), says: says}
}

// draw numbers each cell of g, borders sel, and says what Enter would do.
func (m *marks) draw(c *Canvas, g *Grid, sel int) {
	if m == nil || c == nil || g == nil || c.W <= 0 || c.H <= 0 {
		return
	}
	p := painter.NewPixelPainter(c.Pix, c.W, c.H)
	for i := 0; i < g.Cells(); i++ {
		// Every index below Cells has a cell, and every cell has a positive
		// size: NewGrid refuses a shape that leaves one without.
		x, y, w, h, _ := g.Cell(i)

		if g.IsAdder(i) {
			// A square the size of a screen with a plus in the middle: there is
			// no captured desktop behind this one, so the cell has to draw its
			// own edges or it is a plus floating in a void — which is what it
			// was the first time.
			p.StrokeRect(painter.Rect{X: x, Y: y, W: w, H: h},
				m.theme.OnSurface, adderWidth)
			m.plus(p, x, y, w, h)
		} else {
			m.badge.Text = strconv.Itoa(i + 1)
			m.badge.Fill = m.theme.Surface
			m.badge.Ink = m.theme.OnSurface
			// Bottom left. The top is where a window's own title bar is and the
			// middle is where its content is; the bottom left of a desktop is
			// the emptiest corner it has.
			m.badge.SetBounds(toolkit.Rect{X: x + markInset, Y: y + h - markInset - markH})
			m.badge.Draw(p, m.theme)
		}

		if i == sel {
			// INSIDE the cell, not around it: a border drawn outside would sit
			// in the gap, where the neighbouring screen's edge is, and on the
			// outermost cells it would fall off the picture altogether.
			p.StrokeRect(painter.Rect{X: x, Y: y, W: w, H: h}, SelectionInk, SelectionWidth)
		}
	}

	// And in words, at the bottom of the VIEW.
	switch {
	case g.IsAdder(sel):
		m.says.Text = "add a screen  (Enter)"
	case sel >= 0 && sel < g.Screens():
		m.says.Text = "screen " + strconv.Itoa(sel+1) + " of " +
			strconv.Itoa(g.Screens()) + "  (Enter to go there)"
	default:
		return
	}
	m.says.AnchorIn(toolkit.Rect{X: 0, Y: 0, W: c.W, H: c.H}, toolkit.BottomCenter, 0)
	m.says.Draw(p, m.theme)
}

// markInset is how far a number sits from its cell's corner, and markH how tall
// the pill is — the toolkit's own glyph height plus its padding, which is what
// the badge would choose for itself.
//
// Small, on purpose. A number is for telling one screen from another once the
// border has already said which is chosen; making it large would put a label
// over the desktop it is labelling. Legibility of the SELECTION is the border's
// job, and a taller pill would not make the number bigger anyway — the glyphs
// are the toolkit's font at the toolkit's size, so a tall pill is a narrow
// capsule with a small number floating in the middle of it.
const (
	markInset = 8
	markH     = 18
)

// adderWidth is how thick the outline of the "add a screen" cell is. Thinner
// than the selection border, so the two are never confused: one says "there
// could be a screen here", the other says "this is the one you are choosing".
const adderWidth = 3

// plus draws the sign in the middle of the cell that adds a screen.
//
// Big, from the toolkit's own font scaled up — the same mechanism the arrival
// number uses. A glyph at the interface's default size, in the middle of a tile
// that subtends a hand's width, is a speck.
func (m *marks) plus(p painter.Painter, x, y, w, h int) {
	was := toolkit.CurrentFont()
	toolkit.SetFont(toolkit.NewBitmapFont(plusScale(h)))
	defer toolkit.SetFont(was)

	m.badge.Text = "+"
	// The accent, and a pill: a Badge always has one — a transparent Fill is
	// read as "unset" and falls back to the accent anyway — so it is used on
	// purpose rather than fought. A round accent-coloured plus is what an "add"
	// looks like everywhere else, and the ORANGE border is what says whether it
	// is the thing about to be chosen, so the two never have to compete.
	m.badge.Fill = toolkit.RGBA{}
	m.badge.Ink = toolkit.RGBA{}

	// Sized from the font rather than by drawing it once to find out. Letting
	// the badge auto-size means DRAWING it, and the first draw lands at
	// whatever bounds it had — which put a stray plus in the corner of the
	// picture, over screen one.
	pw := toolkit.TextWidth("+") + 2*toolkit.BadgePadX
	ph := toolkit.GlyphHeight() + 2*toolkit.BadgePadY
	m.badge.SetBounds(toolkit.Rect{
		X: x + (w-pw)/2, Y: y + (h-ph)/2, W: pw, H: ph,
	})
	m.badge.Draw(p, m.theme)
}

// plusScale is how many times the built-in font is magnified for the plus:
// about a quarter of the cell's height.
func plusScale(h int) int {
	const glyph = 7
	s := h / 4 / glyph
	if s < 1 {
		s = 1
	}
	return s
}
