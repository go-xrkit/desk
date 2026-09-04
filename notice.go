// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"math"

	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit"
)

// DefaultNoticeSeconds is how long a notice stays up when nothing takes it
// down. Long enough to read a short sentence, short enough not to become part
// of the picture.
const DefaultNoticeSeconds = 2.5

// A notice is a SENTENCE the desk puts on the picture for a moment.
//
// ⛔ It is what a global shortcut has instead of a button. A key pressed blind
// while another application has the keyboard produces one of three things: a
// change the viewer can see, a change they cannot, or a wait -- and the last
// two are indistinguishable from a key that did nothing. Both were reported the
// same afternoon: "le raccourci pour fit ne semble pas fonctionnel" (it fired,
// and the band was already as close as these glasses go, so nothing moved), and
// "on a l'impression que ca ne fonctionne pas le temps que ca arrive" (the
// application list takes seconds to enumerate and the gallery cannot be drawn
// until it is back).
//
// So it is the badge's machinery with a different job. The badge answers "which
// screen is this" in one glyph the size of a hand; this answers "what did that
// key just do" in a line of ordinary type, and it sits at the BOTTOM where it
// does not cover what the person is reading.
//
// A notice that is WAITING has no life: it stays until whatever it is waiting
// for calls clear, because a placeholder that expires before its work finishes
// is worse than none -- it says the thing failed.
type notice struct {
	toast  *toolkit.Toast
	theme  *toolkit.Theme
	frames int
}

// newNotice prepares it. A nil theme is the dark one, like everywhere else.
func newNotice(seconds float64, theme *toolkit.Theme) *notice {
	if !(seconds > 0) {
		seconds = DefaultNoticeSeconds
	}
	if theme == nil {
		theme = toolkit.DefaultDark()
	}
	return &notice{
		toast:  toolkit.NewToast("", toolkit.ToastInfo),
		theme:  theme,
		frames: int(math.Round(seconds / FrameInterval.Seconds())),
	}
}

// say puts a sentence up for the usual moment.
//
// The nil check is HERE and not only in put: n.frames would be read to make the
// call, and a Desk that was never given a theme has no notice at all.
func (n *notice) say(text string) {
	if n == nil {
		return
	}
	n.put(text, n.frames)
}

// waiting puts a sentence up and LEAVES it there until clear.
//
// Not a long life: a wait is over when the work is done and not a moment
// sooner, and a placeholder that vanishes first says the work failed.
func (n *notice) waiting(text string) { n.put(text, 0) }

func (n *notice) put(text string, life int) {
	if n == nil {
		return
	}
	n.toast.Text = text
	n.toast.Visible().Set(true)
	n.toast.Life().Set(life)
}

// clear takes it down, whether it was waiting or counting.
func (n *notice) clear() {
	if n == nil {
		return
	}
	n.toast.Visible().Set(false)
	n.toast.Life().Set(0)
}

// tick counts one frame off a notice that has a life. One with none is left
// alone -- Toast counts down and hides at zero, which is exactly what a
// placeholder must not do.
func (n *notice) tick() {
	if n == nil || n.toast.Life().Get() <= 0 {
		return
	}
	n.toast.Tick()
}

// up reports whether anything is showing.
func (n *notice) up() bool { return n != nil && n.toast.Visible().Get() && n.toast.Text != "" }

// draw puts the sentence at the bottom of the view.
//
// At the bottom because it is a sentence about what just happened, not the
// thing being looked at -- and the middle is where the badge and the galleries
// are. Small type for the same reason: this is read once and then ignored.
func (n *notice) draw(c *Canvas) {
	if !n.up() || c == nil || c.W <= 0 || c.H <= 0 {
		return
	}
	was := toolkit.CurrentFont()
	toolkit.SetFont(toolkit.NewBitmapFont(noticeScale(c.H)))
	defer toolkit.SetFont(was)

	// AnchorIn's third argument is a STACK INDEX and not an inset: it is how
	// many pills are already at that corner, so passing pixels there put this
	// one thirty pills up and off the top of the view. It is anchored to be
	// SIZED and docked, and then lifted by hand, which is what the badge does
	// too and for the same reason -- there is no corner for "above the edge".
	view := toolkit.Rect{X: 0, Y: 0, W: c.W, H: c.H}
	n.toast.AnchorIn(view, toolkit.BottomCenter, 0)
	r := n.toast.Bounds()
	n.toast.SetBounds(toolkit.Rect{X: r.X, Y: r.Y - noticeInset(c.H), W: r.W, H: r.H})
	n.toast.Draw(painter.NewPixelPainterBGRA(c.Pix, c.W, c.H), n.theme)
}

// noticeScale magnifies the built-in font for a sentence.
//
// A THIRD of the badge, from the height of the picture for the same reason: the
// badge is one glyph a twelfth of the view tall, and a line of type at that
// size would be a banner across the whole of it.
func noticeScale(h int) int {
	const glyph = 7 // the built-in bitmap is 7 rows tall at scale 1
	s := h / 36 / glyph
	if s < 1 {
		s = 1
	}
	return s
}

// noticeInset is how far off the bottom edge it sits: a twentieth of the view,
// so it clears the panel's own edge on every size of screen.
func noticeInset(h int) int {
	if i := h / 20; i > 0 {
		return i
	}
	return 1
}
