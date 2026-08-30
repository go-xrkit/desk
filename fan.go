// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"fmt"
	"math"
)

// A Fan is the band with its screens TURNED: a chain of flat panels hinged edge
// to edge, each angled by the splay from the last, projected for wherever the
// viewer is looking.
//
// It is the second half of being able to push the band back. [Strip] draws the
// band with every screen square on, which is what it looks like from directly in
// front and wrong for the ones off to the side: at a desk of three monitors the
// two beside the middle one are turned towards you. A Fan turns them.
//
// The chain is INFINITE and the screen index wraps, which is how the band stays a
// ring: panel n is screen 0 again, one turn further along the chain. So walking
// right past the last screen arrives at the first, exactly as it did flat, and
// nothing about the navigator or the gallery had to learn what an angle is.
//
// The screens are still FLAT. A curve bows the screen you are reading, which
// argues with the depth the glasses already present -- that was measured, worn,
// and deleted. A rotation leaves every panel flat and only changes which way it
// faces. See [Slant] for what one turned panel projects to, and why that is a
// trapezoid rather than a guess.
type Fan struct {
	n                        int
	splayDeg                 float64
	distance                 float64
	hw, panelH, f            float64
	viewW, viewH, srcW, srcH int
	// srcWidths is the source width of each screen that is not the shape of the
	// band. See [Fan.SetSourceWidths].
	srcWidths []int

	// slots holds one column buffer per panel a frame can show, reused frame to
	// frame. A slant's columns are a slice into one of these, so a caller may
	// hold every slant of a frame at once -- which the drawing loop does.
	slots [][]SlantCol
}

// FanReach is how many panels either side of the middle one a frame considers.
//
// Four, which is more than a rectilinear projection can ever show: at the far end
// of the distance range four screens span the view, so the fifth is off the edge
// whatever the splay. Considering one too many costs a projection that comes back
// refused; considering one too few loses a screen that should be in shot, and
// nothing would say so.
const FanReach = 4

// NewFan prepares the chain for this plan.
//
// It refuses a plan with no screens or no size, and a splay of nothing -- which
// is not a fan at all but the flat band, and [Strip] draws that better: every
// panel square on means every panel is a rectangle, and a rectangle is a run of
// row copies rather than a pixel at a time.
func NewFan(plan Plan) (*Fan, error) {
	switch {
	case plan.Count() <= 0:
		return nil, fmt.Errorf("%w: %d screens", ErrNoScreens, plan.Count())
	case plan.ScreenW <= 0 || plan.ScreenH <= 0:
		return nil, fmt.Errorf("%w: screens of %dx%d",
			ErrScreens, plan.ScreenW, plan.ScreenH)
	case plan.SplayDeg() <= 0:
		return nil, fmt.Errorf("%w: a splay of %g is the flat band; use a Strip",
			ErrScreens, plan.SplayDeg())
	}
	hw, panelH, f := slantOptics(plan.HFOVDeg, plan.ScreenW, plan.ScreenW, plan.ScreenH)
	fan := &Fan{
		n: plan.Count(), splayDeg: plan.SplayDeg(), distance: plan.Distance(),
		hw: hw, panelH: panelH, f: f,
		viewW: plan.ScreenW, viewH: plan.ScreenH,
		srcW: plan.ScreenW, srcH: plan.ScreenH,
		slots: make([][]SlantCol, 2*FanReach+1),
	}
	for i := range fan.slots {
		fan.slots[i] = make([]SlantCol, 0, plan.ScreenW)
	}
	return fan, nil
}

// Angle is where panel j of the chain is, in radians from straight ahead.
//
// It is the angle to the panel's CENTRE, which is what the scroll interpolates
// between: the panels of a chain do not subtend equal angles -- the ones further
// along are further away -- so a position half way between two screens is half
// way between their two angles and not half of a fixed pitch.
func (f *Fan) Angle(j int) float64 {
	lx, lz, rx, rz := slantChain(j, f.splayDeg, f.hw, f.distance, 0)
	return math.Atan2((lx+rx)/2, (lz+rz)/2)
}

// Frame appends the panels in shot to dst: the focused screen and its
// neighbours, for a band that has moved `toward` screens past the focused one
// (see [Strip.Toward]).
//
// The columns of the returned slants live in the Fan's own buffers, so every
// slant of one frame may be held at once (the drawing loop does) and the frame
// after this one overwrites them.
func (f *Fan) Frame(dst []Slant, focus int, toward float64) []Slant {
	// The viewer's rotation: between the angle to the panel in front and the
	// angle to the one they are moving towards. Rotating by it brings that point
	// of the chain to the middle of the view, which is what scrolling means.
	//
	// Panel 0 of the chain is always the FOCUSED screen, so the chain is rebuilt
	// around wherever the navigator says the desk is -- which is why nothing here
	// needs the band to close, or the screens to be in any particular order on it.
	next := 1
	if toward < 0 {
		next = -1
	}
	turn := f.Angle(0) + toward*float64(next)*(f.Angle(next)-f.Angle(0))

	slot := 0
	for j := -FanReach; j <= FanReach; j++ {
		lx, lz, rx, rz := slantChain(j, f.splayDeg, f.hw, f.distance, turn)
		at := f.screenAt(focus + j)
		s, ok := slantOf(f.slots[slot], at, lx, lz, rx, rz,
			f.panelH, f.f, f.viewW, f.viewH, f.sourceWidth(at), f.srcH)
		if !ok {
			continue
		}
		// Keep the buffer this slant was built in: the next panel gets the next
		// one, so nothing is overwritten while the frame is still being drawn.
		f.slots[slot] = s.Cols[:0]
		slot++
		dst = append(dst, s)
	}
	return dst
}

// screenAt is which screen panel j of the chain shows.
//
// The chain runs on for ever and the screens repeat along it, which is what makes
// the band a ring: walking right past the last screen arrives at the first,
// because panel n is screen 0 one turn further out.
func (f *Fan) screenAt(j int) int {
	return ((j % f.n) + f.n) % f.n
}

// SetSourceWidths gives screens their own source widths.
//
// It is not decoration: the columns of a turned panel are gathered one pixel at
// a time out of the source row, so a panel told it is 1920 pixels wide when its
// capture is 1670 reads past the end of the row. That is a PANIC, and it is how
// this was found -- "slice bounds out of range [:6684] with capacity 6680",
// where 6680 is 1670 pixels of BGRA, with the app running and the glasses on.
//
// A nil or short slice, or an entry of zero, leaves that screen on the width
// every other screen has.
func (f *Fan) SetSourceWidths(w []int) {
	f.srcWidths = append(f.srcWidths[:0], w...)
}

// sourceWidth is how wide screen i's pixels are.
func (f *Fan) sourceWidth(i int) int {
	if i < 0 || i >= len(f.srcWidths) || f.srcWidths[i] <= 0 {
		return f.srcW
	}
	return f.srcWidths[i]
}
