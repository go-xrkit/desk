// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"strconv"

	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit"
)

// marks put a number on every cell of the gallery and light up the chosen one.
//
// Without them the gallery is unusable, and it was: the arrows moved a selection
// that was NOWHERE ON THE PICTURE. Six identical desktops, a highlight kept in a
// variable, and Enter jumping to whichever one the viewer had lost count of. It
// was reported the first time anyone tried to pick a screen with it.
//
// A number rather than a ring, and on every cell rather than only the chosen
// one. In a headset a thin outline round one of six tiles is easy to miss and
// impossible to count from; the number says which screen this IS, which is the
// question in front of somebody deciding where to go. The chosen one is the
// accent colour and the rest are not.
//
// Both are toolkit Badges. Nothing here paints a pixel: the pill, the glyphs and
// the two colours all come from the same widget and the same theme as every
// other piece of interface.
type marks struct {
	theme *toolkit.Theme
	badge *toolkit.Badge
	// says which one is chosen, in words, because a small accent-coloured pill
	// on one of six tiles is a cue and not an answer.
	says *toolkit.Toast
}

func newMarks(theme *toolkit.Theme) *marks {
	if theme == nil {
		theme = toolkit.DefaultDark()
	}
	says := toolkit.NewToast("", toolkit.ToastInfo)
	says.Visible = true // for as long as the gallery is
	return &marks{theme: theme, badge: toolkit.NewBadge(""), says: says}
}

// draw puts a number on each cell of g, with sel lit.
func (m *marks) draw(c *Canvas, g *Grid, sel int) {
	if m == nil || c == nil || g == nil || c.W <= 0 || c.H <= 0 {
		return
	}
	p := painter.NewPixelPainter(c.Pix, c.W, c.H)
	for i := 0; i < g.n; i++ {
		// Every index below n has a cell, and every cell has a positive size:
		// NewGrid refuses a shape that leaves one without. So there is nothing
		// to guard here, and no branch to leave untested.
		x, y, _, h, _ := g.Cell(i)
		m.badge.Text = strconv.Itoa(i + 1)
		if i == sel {
			// The theme's own accent, which is what it is for.
			m.badge.Fill = toolkit.RGBA{}
			m.badge.Ink = toolkit.RGBA{}
		} else {
			// Legible and quiet: the surface colour a card would have, with
			// the ink that goes on it.
			m.badge.Fill = m.theme.Surface
			m.badge.Ink = m.theme.OnSurface
		}
		// Bounds of ZERO, both ways, so the badge sizes its own pill to its
		// digits.
		//
		// Forcing a taller pill does NOT make the number bigger: the glyphs
		// are the toolkit's font at the toolkit's size, so a tall pill is a
		// narrow capsule with a small number floating in the middle of it,
		// which is what it looked like when it was tried. Legibility does not
		// come from the pill, so it is not asked of it.
		m.badge.SetBounds(toolkit.Rect{X: x + markInset, Y: y + h - markInset - markH})
		m.badge.Draw(p, m.theme)
	}

	// And in words, at the bottom of the VIEW. The numbers say which cell is
	// which; this says which one Enter would take, which is the question in
	// front of somebody deciding.
	if sel >= 0 && sel < g.n {
		m.says.Text = "screen " + strconv.Itoa(sel+1) + " of " + strconv.Itoa(g.n) +
			"  (Enter to go there)"
		m.says.AnchorIn(toolkit.Rect{X: 0, Y: 0, W: c.W, H: c.H}, toolkit.BottomCenter, 0)
		m.says.Draw(p, m.theme)
	}
}

// markInset is how far a number sits from its cell's corner, and markH how tall
// the pill is — the toolkit's own glyph height plus its padding, which is what
// the badge would choose for itself.
const (
	markInset = 8
	markH     = 18
)
