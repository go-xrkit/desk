// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"sync"

	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit"
	"github.com/go-xrkit/xrkit/ribbon"
)

// Where the mouse is, in the picture.
//
// MACOS DOES NOT DRAW A CURSOR ON A DISPLAY THIS PROGRAM MADE. That is not a
// guess, it is a measurement: moving the pointer 600 by 400 across a screen and
// comparing two captures of it changes 2003 pixels on this Mac's own panel and
// 14 on one of the desk's own screens. Fourteen is not an arrow, it is a clock
// ticking somewhere.
//
// So somebody wearing the glasses sees their windows, sees them respond, and
// cannot see the mouse -- "je ne voyais pas la souris". The desk draws it.

// CursorAt maps a pixel of one screen's source to a pixel of the picture,
// through the very tables the picture was drawn with.
//
// Not through the geometry again: a second calculation of where a screen is
// would be a second chance to disagree with where it was actually drawn, which
// is how a cursor ends up a screen away from the window it is over. blits and
// slants are what Render just used; one of them is empty.
//
// A screen drawn twice -- one straddling the seam of the band comes back in two
// pieces -- answers with the piece the pointer is actually in.
func CursorAt(blits []ribbon.Blit, slants []Slant, screen, px, py, srcW, srcH int) (x, y int, ok bool) {
	if srcW <= 0 || srcH <= 0 || px < 0 || py < 0 || px >= srcW || py >= srcH {
		return 0, 0, false
	}
	for _, b := range blits {
		if b.Screen != screen || b.Dst.W <= 0 || b.Dst.H <= 0 || b.SrcXStep <= 0 {
			continue
		}
		// Invert what Blit does: source column = (SrcX + dx*SrcXStep) >> 32.
		dx := int((int64(px)<<fracBits - b.SrcX) / b.SrcXStep)
		if dx < 0 || dx >= b.Dst.W {
			continue
		}
		return b.Dst.X + dx, b.Dst.Y + py*b.Dst.H/srcH, true
	}
	for _, s := range slants {
		if s.Screen != screen {
			continue
		}
		// The columns carry their own source column, so the nearest one is the
		// answer -- and a turned panel's columns are not evenly spaced, which is
		// exactly why this is a search and not a division.
		best, bestOff := -1, 1<<30
		for i, c := range s.Cols {
			if c.Src < 0 {
				continue
			}
			off := int(c.Src) - px
			if off < 0 {
				off = -off
			}
			if off < bestOff {
				best, bestOff = i, off
			}
		}
		if best < 0 {
			continue
		}
		c := s.Cols[best]
		if c.Y1 <= c.Y0 {
			continue
		}
		return s.Dst.X + best, int(c.Y0) + py*int(c.Y1-c.Y0)/srcH, true
	}
	return 0, 0, false
}

// CursorPx is how big the drawn pointer is, in canvas pixels.
//
// Larger than a real cursor on purpose. This one is not read at arm's length on
// a panel: it is read through display glasses, on a picture of a screen that
// may be drawn at a fraction of its own size, and a 32-pixel arrow scaled with
// the screen it is on would be a few pixels of nothing.
const CursorPx = 40

// CursorInk is the colour the drawn pointer is filled with.
//
// White, and it is the only thing on the picture that is: a pointer has to be
// findable against a desktop that is mostly not, and the ring round a chosen
// screen already owns the green.
var CursorInk = toolkit.RGB(0xFF, 0xFF, 0xFF)

// drawCursor paints the pointer at x,y of the canvas.
//
// Through the toolkit's Iconoir seam rather than by hand: an arrow drawn here
// would be an arrow to maintain here, and "cursor-pointer" is already drawn by
// people who draw icons.
func drawCursor(c *Canvas, x, y int) {
	if c == nil || c.W <= 0 || c.H <= 0 {
		return
	}
	p := painter.NewPixelPainterBGRA(c.Pix, c.W, c.H)
	// Shifted by the glyph's own margin, so its TIP lands on the pointer rather
	// than its bounding box. A cursor whose point is seven pixels right of where
	// the mouse actually is, is a cursor that lies about which button it is over.
	dx, dy := cursorInset()
	toolkit.DrawIconoir(p, toolkit.Rect{X: x - dx, Y: y - dy, W: CursorPx, H: CursorPx},
		"cursor-pointer", CursorInk)
}

// inset is where the glyph's ink starts inside the box it is drawn in.
var inset struct {
	once   sync.Once
	dx, dy int
}

// cursorInset measures the margin the icon has around its own arrow.
//
// MEASURED rather than written down: it is a property of somebody else's SVG,
// and a number typed here would be right until the pack was updated and wrong
// silently afterwards. Drawn once into a scratch buffer, at the size it is
// actually used at, and the first inked pixel is the answer.
func cursorInset() (int, int) {
	inset.once.Do(func() {
		buf := make([]byte, CursorPx*CursorPx*4)
		p := painter.NewPixelPainterBGRA(buf, CursorPx, CursorPx)
		toolkit.DrawIconoir(p, toolkit.Rect{W: CursorPx, H: CursorPx}, "cursor-pointer", CursorInk)
		inset.dx, inset.dy = inkStart(buf, CursorPx, CursorPx)
	})
	return inset.dx, inset.dy
}

// inkStart is the top left corner of whatever was drawn into a BGRA buffer.
//
// A glyph with nothing in it -- a pack that has no such name, which draws
// nothing and says nothing -- answers 0,0: the cursor is then drawn where it
// was asked for rather than a whole box away from it.
func inkStart(pix []byte, w, h int) (int, int) {
	dx, dy := w, h
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if o := (y*w + x) * 4; o+3 >= len(pix) || pix[o+3] == 0 {
				continue
			}
			if x < dx {
				dx = x
			}
			if y < dy {
				dy = y
			}
		}
	}
	if dx >= w || dy >= h {
		return 0, 0
	}
	return dx, dy
}
