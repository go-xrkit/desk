// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"math"
	"strconv"
	"time"

	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit"
)

// DefaultBadgeSeconds is how long the screen's number stays up after the band
// moves.
//
// Long enough to read without looking for it, short enough that it is gone
// before it becomes part of the picture. Zero seconds turns it off.
const DefaultBadgeSeconds = 1.5

// A badge says which screen the viewer has arrived at, for a moment.
//
// The band moving is silent otherwise: six identical desktops slide past and
// nothing says which one is now in front. That is fine while a screen has an
// application on it and unusable while it does not.
//
// It is a toolkit Toast, not something drawn here. Which means the glyphs, the
// pill, the colours and the fade come from the same place as every other piece
// of interface on the machine, and this file only decides WHEN it is up and
// WHERE it sits.
type badge struct {
	toast   *toolkit.Toast
	theme   *toolkit.Theme
	frames  int
	shownAt int
}

// newBadge prepares the badge. A duration of zero or less turns it off, and
// then nothing is drawn and nothing is ticked.
func newBadge(seconds float64, theme *toolkit.Theme) *badge {
	if !(seconds > 0) {
		return nil
	}
	if theme == nil {
		theme = toolkit.DefaultDark()
	}
	t := toolkit.NewToast("", toolkit.ToastInfo)
	return &badge{
		toast: t,
		theme: theme,
		// In frames, because that is what Toast counts and what the loop has.
		frames:  int(math.Round(seconds / FrameInterval.Seconds())),
		shownAt: -1,
	}
}

// show puts the number up, or leaves it alone when it is already this one.
//
// Re-arming on every frame would mean a badge that never goes away while the
// band is at rest, which is the opposite of the point.
func (b *badge) show(screen, of int) {
	if b == nil || screen == b.shownAt {
		return
	}
	b.shownAt = screen
	// The number and nothing else. It is drawn large enough that "screen 3 of 6"
	// would be a banner across the whole view, and a person turning the band
	// already knows what they are counting.
	b.toast.Text = strconv.Itoa(screen + 1)
	_ = of
	b.toast.Visible().Set(true)
	b.toast.Life().Set(b.frames)
}

// tick counts one frame off.
func (b *badge) tick() {
	if b == nil {
		return
	}
	b.toast.Tick()
}

// up reports whether the badge is currently showing.
func (b *badge) up() bool { return b != nil && b.toast.Visible().Get() }

// draw puts the number on the picture, BIG and in the middle.
//
// It was small and at the bottom, and it was reported as wanting to be a large
// ephemeral overlay of the screen you are on — which is right: this is not a
// label to be read, it is an answer to be caught out of the corner of an eye
// while the band is still moving. So it is one glyph, in the middle of the
// view, at a size that cannot be missed, and gone in a second and a half.
//
// The size comes from the toolkit's own font, scaled: SetFont with a bitmap
// font at a large integer scale, and the Toast then sizes its own pill to it.
// Nothing here rasterises a glyph — the type is the same type as everywhere
// else, just bigger.
func (b *badge) draw(c *Canvas) {
	if !b.up() || c == nil || c.W <= 0 || c.H <= 0 {
		return
	}
	// Restored afterwards, and by a defer: everything else drawn into this
	// picture — the gallery's numbers, its words — expects the font it asked
	// for, and a font left behind would be a five-times-too-large label in the
	// next thing that drew.
	was := toolkit.CurrentFont()
	toolkit.SetFont(toolkit.NewBitmapFont(badgeScale(c.H)))
	defer toolkit.SetFont(was)

	// AnchorIn is what SIZES the pill to its text, and it only docks to an edge
	// — there is no centre corner. So it is anchored to get the size, and then
	// moved into the middle.
	view := toolkit.Rect{X: 0, Y: 0, W: c.W, H: c.H}
	b.toast.AnchorIn(view, toolkit.TopCenter, 0)
	r := b.toast.Bounds()
	b.toast.SetBounds(toolkit.Rect{
		X: (c.W - r.W) / 2,
		Y: (c.H - r.H) / 2,
		W: r.W, H: r.H,
	})
	b.toast.Draw(painter.NewPixelPainterBGRA(c.Pix, c.W, c.H), b.theme)
}

// badgeScale is how many times the built-in font is magnified for the number.
//
// From the height of the picture rather than a constant, so it is the same size
// to look at on a 1080-row panel and on a 1600-row one. A twelfth of the view
// is about a hand's width at the distance these glasses put a screen.
func badgeScale(h int) int {
	const glyph = 7 // the built-in bitmap is 7 rows tall at scale 1
	s := h / 12 / glyph
	if s < 1 {
		s = 1
	}
	return s
}

// badgeFrames is what a duration comes to at the loop's frame rate, exported
// for a caller that wants to say so in a log.
func badgeFrames(seconds float64) int {
	if !(seconds > 0) {
		return 0
	}
	return int(math.Round(seconds / FrameInterval.Seconds()))
}

// BadgeDuration turns a configured number of seconds into a duration, for a
// caller reporting what it will do.
func BadgeDuration(seconds float64) time.Duration {
	return time.Duration(badgeFrames(seconds)) * FrameInterval
}
