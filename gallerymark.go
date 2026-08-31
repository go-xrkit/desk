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
// APPLE GREEN, asked for from the glasses: "instead of orange it would be
// gentler on the eye to have an apple green or a sky blue". Orange was chosen
// for being the one strong hue that is in almost no window chrome and in very
// little wallpaper — findable at a glance — and it is also the hue a headset
// pushes hardest at a person's eye across a whole session.
//
// Green keeps what mattered and drops what did not. It is still nothing any
// window chrome uses, it is still unmistakable against six captured desktops,
// and it is not the theme's accent — which is a blue that has to sit politely
// beside the interface it belongs to, where this has to be FOUND.
//
// Sky blue was the other suggestion and is one line away: RGB(0x7F, 0xC8, 0xF8).
// It is not the default only because it is a near neighbour of the accent, and
// two blues that mean different things is how a person stops trusting either.
var SelectionInk = toolkit.RGB(0x7B, 0xD8, 0x8F)

// SelectionWidth is how thick that border is, in logical pixels.
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
	// chosen borders the selected cell, and offer outlines the cell that adds a
	// screen. Both are toolkit widgets: this package draws no pixel of its own.
	chosen *toolkit.SelectionBox
	offer  *toolkit.SelectionBox
}

func newMarks(theme *toolkit.Theme) *marks {
	if theme == nil {
		theme = toolkit.DefaultDark()
	}
	// Everything this file draws answers ONE question -- "which cell is the one
	// about to be taken" -- so it answers in one colour. A Badge and a Toast
	// colour themselves from the theme ACCENT, which is a blue, so the ring said
	// it in green and the sentence under it said it in blue. A copy of the theme
	// with the accent set to the same ink is the whole fix; nothing else about
	// the theme changes, and no widget is drawn by hand to get it.
	mine := *theme
	mine.Accent = SelectionInk
	theme = &mine
	says := toolkit.NewToast("", toolkit.ToastInfo)
	says.Visible().Set(true) // for as long as the gallery is
	offer := toolkit.NewSelectionBox(theme.OnSurface)
	offer.Weight = adderWidth
	return &marks{
		theme:  theme,
		badge:  toolkit.NewBadge(""),
		says:   says,
		chosen: toolkit.NewSelectionBox(SelectionInk),
		offer:  offer,
	}
}

// draw numbers each cell of g, borders sel, and says what Enter would do.
func (m *marks) draw(c *Canvas, g *Grid, sel int) {
	if m == nil || c == nil || g == nil || c.W <= 0 || c.H <= 0 {
		return
	}
	p := painter.NewPixelPainterBGRA(c.Pix, c.W, c.H)
	for i := 0; i < g.Cells(); i++ {
		// Every index below Cells has a cell, and every cell has a positive
		// size: NewGrid refuses a shape that leaves one without.
		x, y, w, h, _ := g.Cell(i)

		if g.IsAdder(i) {
			// A square the size of a screen with a plus in the middle: there is
			// no captured desktop behind this one, so the cell has to draw its
			// own edges or it is a plus floating in a void — which is what it
			// was the first time.
			m.offer.SetBounds(toolkit.Rect{X: x, Y: y, W: w, H: h})
			m.offer.Draw(p, m.theme)
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
			m.chosen.Weight = SelectionWidth
			// What a screen reader is told about the selection. The border has
			// no name of its own; what is chosen is the desktop underneath it.
			m.chosen.Label = m.saying(g, sel)
			m.chosen.SetBounds(toolkit.Rect{X: x, Y: y, W: w, H: h})
			m.chosen.Draw(p, m.theme)
		}
	}

	// And in words, at the bottom of the VIEW.
	m.says.Text = m.saying(g, sel)
	if m.says.Text == "" {
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
// plus draws the "add a screen" mark.
//
// The plus is [toolkit.DrawIconPlus], not a "+" TYPESET from the font. A glyph
// has side bearings and a baseline, so it sits left of centre in its own box
// with unequal arms — invisible in a menu, plain at the size a headset needs,
// and reported from a pair of glasses as "not properly symmetric". It was.
//
// It is SelectionInk, the same orange as the ring round the chosen cell, so the
// gallery answers "this is the one" in one colour rather than two. And it is
// the plus ALONE: the disc it used to sit on was a painter primitive drawn by
// hand, which this package does not do, and it said nothing the ring was not
// already saying.
func (m *marks) plus(p painter.Painter, x, y, w, h int) {
	side := h / 4
	if side < 8 {
		side = 8
	}
	cx, cy := x+w/2, y+h/2
	toolkit.DrawIconPlus(p, toolkit.Rect{
		X: cx - side/2, Y: cy - side/2, W: side, H: side,
	}, SelectionInk)
}

// saying is what the selection is, in words: shown at the bottom of the view,
// and handed to the border as the name a screen reader announces.
//
// One function for both, because they must never disagree — a line saying
// "screen 3" while the accessibility tree says "add a screen" is worse than
// either alone.
func (m *marks) saying(g *Grid, sel int) string {
	switch {
	case g.IsAdder(sel):
		return "add a screen  (Enter)"
	case sel >= 0 && sel < g.Screens():
		return "screen " + strconv.Itoa(sel+1) + " of " +
			strconv.Itoa(g.Screens()) + "  (Enter to go there)"
	}
	return ""
}
