// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
)

// rect is a display's rectangle in global display space, in points.
type rect struct{ X, Y, W, H float64 }

func (r rect) right() float64  { return r.X + r.W }
func (r rect) bottom() float64 { return r.Y + r.H }

// The seams. The platform answers them and tests replace them, so everything a
// person can SEE about the pointer is decided in this file rather than in a
// binding that only runs on one machine.
var (
	pointerAt   = platformPointerAt
	displayRect = platformDisplayRect
	allDisplays = platformDisplays
	movePointer = platformMovePointer
)

// ErrPointerLost means the window server would not say where the pointer is.
//
// It is not the same thing as ErrNoPointer off darwin, which says the pointer
// cannot be MOVED here: this one is a machine that can move it and would not
// say where it is.
var ErrPointerLost = errors.New("desk: cannot read the pointer's position")

// Fence keeps the pointer on the screen the band is showing.
//
// THE MOUSE DOES NOT CHANGE SCREENS. The keyboard does.
//
// It was the other way round and it did not work. The band followed the
// pointer, the pointer came back at the other end of the band when it was
// pushed off it, and a display nothing was showing was fetched onto the screen
// in front of the viewer when the pointer wandered onto it. Three mechanisms,
// each answering a hole left by the one before, and the report after every one
// of them was the same: "j'ai encore perdu la souris".
//
// The reason is simple enough once it is said. A person wearing display glasses
// sees ONE screen. The desktop under the pointer is several, most of them
// invisible, and a pointer that can leave the visible one is a pointer that can
// be somewhere its owner cannot look. No amount of following fixes that; it
// only decides how far away the mouse gets before something notices.
//
// So the pointer stays. It is put back on the screen the band is showing,
// every frame, and the way to another screen is the key that turns the band --
// which also brings the pointer, because the screen it is held to has changed.
type Fence struct {
	last int  // the position it was holding the pointer to
	had  bool // whether it has held one at all
}

// Step puts the pointer back on the screen at position focus, if it has left
// it. It reports whether it moved it.
//
// showing is the display each ribbon position is showing, in order. A position
// showing nothing, or a display this machine will not measure, is not a fence:
// the pointer is left alone rather than held against a rectangle nobody knows.
func (f *Fence) Step(showing []uint64, focus int) (bool, error) {
	if focus < 0 || focus >= len(showing) || showing[focus] == 0 {
		f.had = false
		return false, nil
	}
	r, ok := displayRect(showing[focus])
	if !ok || r.W <= 0 || r.H <= 0 {
		f.had = false
		return false, nil
	}
	x, y, ok := pointerAt()
	if !ok {
		f.had = false
		return false, ErrPointerLost
	}

	// A screen the band has just turned to takes the pointer to its MIDDLE
	// rather than to its nearest edge. The edge is where the pointer would be
	// dragged to by a clamp, which is nowhere in particular; the middle is
	// where somebody who just asked for this screen is looking.
	arrived := !f.had || f.last != focus
	f.last, f.had = focus, true
	if !arrived && inside(x, y, r) {
		return false, nil
	}
	if !arrived {
		x, y = clamp(x, r.X, r.right()-1), clamp(y, r.Y, r.bottom()-1)
	} else {
		if inside(x, y, r) {
			return false, nil
		}
		x, y = r.X+r.W/2, r.Y+r.H/2
	}
	if err := movePointer(x, y); err != nil {
		return false, err
	}
	return true, nil
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

func inside(x, y float64, r rect) bool {
	return x >= r.X && x < r.right() && y >= r.Y && y < r.bottom()
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Where says which ribbon position the pointer is on and where it is on that
// screen's own source, in pixels.
//
// It is the fence's because the fence already holds the answer: the pointer is
// on the screen the band is showing, and the rectangle it is held to is the one
// this converts through. srcW and srcH are asked for rather than assumed --
// a screen mirroring this Mac's panel is not the shape of the band.
func (f *Fence) Where(showing []uint64, focus int) (pos, px, py int, ok bool) {
	if focus < 0 || focus >= len(showing) || showing[focus] == 0 {
		return 0, 0, 0, false
	}
	r, ok := displayRect(showing[focus])
	if !ok || r.W <= 0 || r.H <= 0 {
		return 0, 0, 0, false
	}
	x, y, ok := pointerAt()
	if !ok || !inside(x, y, r) {
		return 0, 0, 0, false
	}
	// In FRACTIONS of the screen. The source's pixel size is the caller's --
	// a capture is not always the same size as the rectangle it came from.
	return focus, int((x - r.X) / r.W * PointerScale), int((y - r.Y) / r.H * PointerScale), true
}

// PointerScale is the unit [Fence.Where] answers in: the pointer's place across
// the screen as a number out of this. The caller multiplies by its own source
// size, which is the only thing that knows how many pixels that screen has.
const PointerScale = 1 << 16
