// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"time"
)

// DefaultHold is how long the pointer must be pushed into the end of the
// desktop before it comes back at the other end.
//
// It is not zero, and that is the whole design. The left-hand column of pixels
// on the first screen is a place a person legitimately goes — a close button
// lives there — so a wrap that fired on merely TOUCHING the edge would put that
// column out of reach. A push that is still there a third of a second later is
// somebody asking for something.
const DefaultHold = 300 * time.Millisecond

// edgeSlack is how close to the end counts as being at it, in points. The
// window server clamps a pushed pointer to the last pixel, so this only has to
// absorb the difference between a rectangle's edge and the last point inside it.
const edgeSlack = 2

// rect is a display's rectangle in global display space, in points.
type rect struct{ X, Y, W, H float64 }

func (r rect) right() float64 { return r.X + r.W }

// The seams. The platform answers them and tests replace them, so everything a
// person can SEE about wrapping is decided in this file rather than in a
// binding that only runs on one machine.
var (
	pointerAt   = platformPointerAt
	displayRect = platformDisplayRect
	allDisplays = platformDisplays
	movePointer = platformMovePointer
)

// ErrPointerLost means the window server would not say where the pointer is.
//
// It is not the same thing as go-xrkit's ErrNoPointer off darwin, which says
// the pointer cannot be MOVED here: this one is a machine that can move it and
// would not say where it is.
var ErrPointerLost = errors.New("desk: cannot read the pointer's position")

// Edges brings the pointer back at the other end of the ribbon when it is
// pushed off the end of the desktop.
//
// The ribbon is a circle and the desktop is not. Pushing the mouse to the left
// of the first screen puts it against a wall: the window server clamps it
// there, the band cannot follow it anywhere, and the way to the LAST screen is
// all the way back across every screen in between. Wrapping is what makes the
// two agree.
//
// It only happens where the desktop really ends. A screen with another display
// beside it — this Mac's own panel, most often, sitting immediately to the
// right of the screens this program made — keeps its edge, because that edge is
// the way to that display and trapping the pointer inside the ribbon would take
// the machine's real screen away.
type Edges struct {
	// Hold is how long the push must last. Zero means [DefaultHold].
	Hold time.Duration

	// Own is the display the desk's own window is on -- the glasses. Zero
	// means it is not known.
	//
	// It is the difference between the rule as written and the rule as it
	// behaves on a real machine. MEASURED, with six screens up: the glasses
	// sit at x=-13440 and the ribbon runs -11520..0, so the glasses are
	// IMMEDIATELY to the left of the first screen. A pointer pushed off that
	// edge is therefore never clamped anywhere -- it walks straight onto the
	// glasses' own display, which is showing this program's window. It is a
	// pointer on a picture: it cannot click anything, and nothing in the view
	// says where it went.
	//
	// So a pointer that lands there is brought back at once, with no hold: it
	// is not somewhere a person can have meant to go.
	Own uint64

	side  int       // -1 pushing left, +1 pushing right, 0 not at an end
	since time.Time // when this push started
}

// Step is one look at the pointer. It reports whether it moved it.
//
// ids is the desk's displays in ribbon order, the same slice [PositionOf]
// takes. now is passed in rather than read so that a test can push time along
// without sleeping.
func (e *Edges) Step(now time.Time, ids []uint64) (bool, error) {
	x, y, ok := pointerAt()
	if !ok {
		e.side = 0
		return false, ErrPointerLost
	}
	ends, ok := ribbonEnds(ids)
	if !ok {
		e.side = 0
		return false, nil
	}
	left, right := ends[0], ends[1]

	// Lost on the desk's own screen: brought back at once, from whichever
	// side it went out. No hold, because nobody meant to put it there.
	if e.Own != 0 {
		if own, ok := displayRect(e.Own); ok && inside(x, y, own) {
			e.side = 0
			if x < left.X {
				// It went out to the LEFT, so it comes back on the right.
				return e.jump(-1, left, right, y)
			}
			return e.jump(+1, left, right, y)
		}
	}

	side := 0
	switch {
	case x <= left.X+edgeSlack && inRows(y, left):
		side = -1
	case x >= left.right()-edgeSlack && sameRect(left, right) && inRows(y, left):
		// One screen is both ends of the ribbon. Reading the left first would
		// hide the right-hand edge of a one-screen desk.
		side = +1
	case x >= right.right()-edgeSlack && inRows(y, right):
		side = +1
	}
	if side == 0 || !e.endOfTheDesktop(side, left, right) {
		e.side = 0
		return false, nil
	}
	if e.side != side {
		e.side, e.since = side, now
		return false, nil
	}
	if now.Sub(e.since) < e.hold() {
		return false, nil
	}
	moved, err := e.jump(side, left, right, y)
	if err != nil {
		return false, err
	}
	e.side = 0
	return moved, nil
}

// jump crosses to the far edge of the screen at the other end, at the same
// height -- clamped into that screen, which may not span the same rows.
func (e *Edges) jump(side int, left, right rect, y float64) (bool, error) {
	to, at := right, right.right()-edgeSlack-1
	if side > 0 {
		to, at = left, left.X+edgeSlack+1
	}
	if err := movePointer(at, clamp(y, to.Y, to.Y+to.H-1)); err != nil {
		return false, err
	}
	return true, nil
}

func inside(x, y float64, r rect) bool {
	return x >= r.X && x < r.right() && inRows(y, r)
}

func inRows(y float64, r rect) bool { return y >= r.Y && y < r.Y+r.H }

func (e *Edges) hold() time.Duration {
	if e.Hold > 0 {
		return e.Hold
	}
	return DefaultHold
}

// ribbonEnds returns the leftmost and rightmost of the desk's screens.
func ribbonEnds(ids []uint64) ([2]rect, bool) {
	var out [2]rect
	found := false
	for _, id := range ids {
		r, ok := displayRect(id)
		if !ok {
			continue
		}
		if !found {
			out[0], out[1], found = r, r, true
			continue
		}
		if r.X < out[0].X {
			out[0] = r
		}
		if r.right() > out[1].right() {
			out[1] = r
		}
	}
	return out, found
}

// endOfTheDesktop says whether there is really nothing beyond that edge.
//
// A display that merely OVERLAPS the far side does not count: what matters is
// whether the pointer could keep going, and it can only do that onto a display
// that starts beyond the edge it is pushed against.
func (e *Edges) endOfTheDesktop(side int, left, right rect) bool {
	ids := allDisplays()
	for _, id := range ids {
		r, ok := displayRect(id)
		if !ok || id == e.Own {
			// The desk's own screen is not somewhere the pointer can go:
			// it is showing this program's window.
			continue
		}
		if side < 0 && r.X < left.X {
			return false
		}
		if side > 0 && r.right() > right.right() {
			return false
		}
	}
	return true
}

func sameRect(a, b rect) bool { return a == b }

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// DisplayAt returns the display whose rectangle is exactly this one.
//
// A rectangle identifies a display where a size does not: two identical
// monitors are the same size and are not in the same place. It is how the desk
// turns the screen it was given -- which it knows by name and geometry -- into
// the display id everything else here speaks in.
func DisplayAt(x, y, w, h float64) (uint64, bool) {
	want := rect{X: x, Y: y, W: w, H: h}
	for _, id := range allDisplays() {
		if r, ok := displayRect(id); ok && r == want {
			return id, true
		}
	}
	return 0, false
}

// LostHold is how long the pointer must sit on a display no ribbon position is
// showing before the desk goes and gets that display.
//
// It is not zero because crossing one screen to reach another is ordinary, and
// a desk that grabbed a display the moment a pointer passed over it would
// rearrange the ribbon under somebody walking the mouse across their desktop.
const LostHold = 700 * time.Millisecond

// Lost watches for a pointer that has gone somewhere nothing is showing.
//
// This is the other half of following. The band follows the pointer onto any
// screen a position is showing; a pointer that leaves ALL of them -- onto this
// Mac's own panel, most often, which sits right beside the ribbon -- is
// invisible to somebody wearing the glasses. They have not lost the mouse
// exactly: they have lost the screen it is on.
type Lost struct {
	// Hold is how long it must stay there. Zero means [LostHold].
	Hold time.Duration

	on    uint64
	since time.Time
	told  uint64 // the display already reported, so it is said once
}

// Step reports the display to go and get, or 0.
//
// where is the display the pointer is on and ok says whether it is on one at
// all; shown is what the ribbon positions are showing, and own is the desk's
// own screen, which is never worth fetching -- it is showing this program.
func (l *Lost) Step(now time.Time, where uint64, ok bool, shown []uint64, own uint64) uint64 {
	if !ok || where == 0 || where == own {
		l.on, l.told = 0, 0
		return 0
	}
	for _, id := range shown {
		if id == where {
			// On the band. Nothing is lost, and the next trip off it is a new
			// one worth reporting.
			l.on, l.told = 0, 0
			return 0
		}
	}
	if l.on != where {
		l.on, l.since = where, now
		return 0
	}
	if l.told == where || now.Sub(l.since) < l.hold() {
		return 0
	}
	l.told = where
	return where
}

func (l *Lost) hold() time.Duration {
	if l.Hold > 0 {
		return l.Hold
	}
	return LostHold
}
