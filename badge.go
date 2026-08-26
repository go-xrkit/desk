// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"fmt"
	"math"
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
	b.toast.Text = fmt.Sprintf("screen %d of %d", screen+1, of)
	b.toast.Visible = true
	b.toast.Life = b.frames
}

// tick counts one frame off.
func (b *badge) tick() {
	if b == nil {
		return
	}
	b.toast.Tick()
}

// up reports whether the badge is currently showing.
func (b *badge) up() bool { return b != nil && b.toast.Visible }

// draw puts the badge on the picture, low and centred: at the top it would sit
// under whatever the captured desktop has there, which is its own menu bar.
func (b *badge) draw(c *Canvas) {
	if !b.up() || c == nil || c.W <= 0 || c.H <= 0 {
		return
	}
	b.toast.AnchorIn(toolkit.Rect{X: 0, Y: 0, W: c.W, H: c.H}, toolkit.BottomCenter, 0)
	b.toast.Draw(painter.NewPixelPainter(c.Pix, c.W, c.H), b.theme)
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
