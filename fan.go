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
	n                  int
	splayDeg           float64
	distance           float64
	hw, gap, panelH, f float64
	// anchor is what the chain does when the gaze moves. See [Anchoring].
	anchor                   Anchoring
	viewW, viewH, srcW, srcH int
	// srcWidths is the source width of each screen that is not the shape of the
	// band. See [Fan.SetSourceWidths].
	srcWidths []int

	// slots holds one column buffer per panel a frame can show, reused frame to
	// frame. A slant's columns are a slice into one of these, so a caller may
	// hold every slant of a frame at once -- which the drawing loop does.
	slots [][]SlantCol
}

// Anchoring is what the chain does when the viewer looks at a different screen,
// and it is a choice because no arrangement has both halves of it.
//
// ⛔⛔ RE-SQUARING IS A MOVE. The chain built around screen F and the one built
// around F+1 are the same shape in a different PLACE -- a rotation and a
// translation apart. A viewer rotation absorbs the rotation; nothing absorbs the
// translation. So a desk that always presents the screen you are looking at
// square on must shift when you cross the half-way point between two screens:
// measured at 45 pixels, 1.3° of view, in a single frame, on six identical
// screens at twenty degrees.
//
// ⭐ AND A RIGID DESK HAS THE OTHER HALF OF IT. Leave the chain alone and the
// motion is perfectly smooth, but the screens away from the middle are seen
// obliquely -- at twenty degrees the second one along is 40° off square, exactly
// as the far monitors of a real desk are. That is honest and it is not what
// everyone wants to read on.
//
// The only angle with both is [Plan.FacingSplayDeg], where a rigid chain has
// every screen facing the viewer already; it was worn and refused as too deep a
// crease. So this is a setting, and the person wearing the glasses picks.
type Anchoring int

const (
	// AnchorOnGaze rebuilds the chain around whichever screen is being looked
	// at. The screen being read is square on at the set distance whatever the
	// curvature, at the cost of a step each time the gaze crosses a half-way
	// point.
	AnchorOnGaze Anchoring = iota
	// AnchorFixed leaves the chain where it is and only turns the viewer, the
	// way a desk of monitors behaves.
	AnchorFixed
)

// String names an Anchoring for a log line and for a test failure that has to
// be readable without counting iota.
func (a Anchoring) String() string {
	if a == AnchorFixed {
		return "fixed"
	}
	return "on the gaze"
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
	// ⭐ THE GAP IS THE STRIPS OWN, expressed in world units. The flat band
	// leaves DefaultGapPx between screens; the same gap here is what makes the
	// two renderers agree about where the next screen starts, and what lets
	// somebody find the fold at all.
	gap := slantGap(hw, plan.ScreenW)
	fan := &Fan{
		n: plan.Count(), splayDeg: plan.SplayDeg(), distance: plan.Distance(),
		hw: hw, gap: gap, panelH: panelH, f: f,
		viewW: plan.ScreenW, viewH: plan.ScreenH,
		srcW: plan.ScreenW, srcH: plan.ScreenH,
		slots: make([][]SlantCol, 2*FanReach+1),
	}
	for i := range fan.slots {
		fan.slots[i] = make([]SlantCol, 0, plan.ScreenW)
	}
	return fan, nil
}

// hwOf is the half-width of chain panel k, for a chain built around focus.
//
// It goes through [Fan.screenAt] because the chain wraps, and through
// [Fan.sourceWidth] because a screen's panel is as wide as its pixels are: the
// band's own width is the default, and a screen that is not that shape gets its
// own. See the note on [slantChain] for the desk that made this necessary.
func (f *Fan) hwOf(focus int) func(k int) float64 {
	return func(k int) float64 {
		at := f.screenAt(focus + k)
		return f.hw * float64(f.sourceWidth(at)) / float64(f.srcW)
	}
}

// centre is the middle of panel j of a chain built around focus, in camera
// space with the viewer facing straight ahead.
//
// It is the point the scroll interpolates BETWEEN, and it is a POINT rather
// than an angle on purpose: see [Fan.Frame]. There used to be an Angle method
// here returning the arc-tangent of it, and nothing called it once the scroll
// stopped interpolating angles.
//
// ⛔ IT TAKES THE FOCUS because a panel's width is its SCREEN's width, and which
// screen a panel shows depends on where the chain was built from. A chain of
// identical screens does not care and every other one does.
func (f *Fan) centre(focus, j int) (x, z float64) {
	lx, lz, rx, rz := slantChain(j, f.splayDeg, f.hwOf(focus), f.gap, f.distance, 0)
	return (lx + rx) / 2, (lz + rz) / 2
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
	// Panel 0 of the chain is the screen BEING LOOKED AT, so the chain is rebuilt
	// around wherever the navigator says the desk is -- which is why nothing here
	// needs the band to close, or the screens to be in any particular order on it.
	//
	// ⛔⛔ IT USED TO BE THE FOCUSED SCREEN, AND HEAD TRACKING BROKE THAT WITHOUT
	// SAYING SO. It was true while the only thing that moved the band was a
	// deliberate turn: the navigator eased to a screen and `toward` came back to
	// zero. followHead sets the yaw and leaves the focus alone ON PURPOSE, so
	// somebody who turns their head one screen to the right sits at toward = 1
	// for as long as they look there -- and at toward = 1 the chain drew the
	// wrong shape.
	//
	// ⭐ MEASURED, six screens at a splay of twenty degrees: the screen in front
	// was centred but TURNED 23.6° AWAY, so its trapezoid ran x 92..1612 of 1920
	// instead of filling the view, and the hinge to its neighbour landed at
	// x 1612 -- inside the picture rather than at its edge. Reported in exactly
	// those words: "l'angle de cintrage n'est pas au bon endroit et tombe a
	// l'interieur de l'ecran en face". Further out it was worse: at toward = 2
	// the screen was not even centred (x 420), because `turn` below extrapolates
	// the FIRST step linearly and a chain's steps are not equal angles.
	//
	// Taking the whole screens off `toward` and giving them to `focus` leaves a
	// residue in [-0.5, 0.5]: the panel in front is the one being looked at,
	// square on when it is squarely looked at, and the extrapolation never runs
	// beyond half a step. screenAt wraps, so a focus off either end is a screen.
	// ⛔⛔ AND RE-ANCHORING IS A MOVE, WHICH IS WHY IT IS A CHOICE. The two
	// placements -- the chain built around screen F and the one built around
	// F+1 -- are the same shape in a different place: related by a rotation AND
	// A TRANSLATION. A viewer rotation can absorb the rotation and nothing can
	// absorb the translation, so at the half-way point the whole desk shifts.
	// Measured on six identical screens at twenty degrees: 45 pixels, 1.3° of
	// view, in one frame. Reported as "c'est deroutant quand on tourne la tete
	// d'avoir les ecrans qui se replace face a soit d'un coup".
	//
	// ⭐ THERE IS NO ARRANGEMENT THAT HAS BOTH. Either the desk is rigid and the
	// screens away from the middle are seen obliquely, as the far monitors of a
	// real desk are, or it re-squares the one being looked at and therefore
	// moves. See [Anchoring]; the wearer picks.
	base := 0
	if f.anchor == AnchorOnGaze {
		// Taking the whole screens off `toward` and giving them to `focus`
		// leaves a residue in [-0.5, 0.5]: the panel in front is the one being
		// looked at, square on when it is squarely looked at. screenAt wraps, so
		// a focus off either end is still a screen.
		whole := math.Round(toward)
		focus += int(whole)
		toward -= whole
	} else {
		// Rigid: the chain is not moved, so the viewer walks along it. base is
		// the panel the gaze has reached and the residue is the rest of the
		// step, which keeps the interpolation below inside ONE hinge however
		// far along the band somebody has turned -- the extrapolation that put
		// a screen at x 420 at toward = 2 is gone with it.
		base = int(math.Floor(toward))
		toward -= float64(base)
	}

	next := base + 1
	if f.anchor == AnchorOnGaze && toward < 0 {
		next = -1
	}
	// ⛔⛔ INTERPOLATE THE POINT, NOT THE ANGLE. Rotating the view by an angle
	// moves the picture by f*tan of it, which is not linear in the angle -- so a
	// band a quarter of a screen along landed somewhere the flat renderer did
	// not put it. Measured, at a splay of nothing where the two must agree
	// exactly: 362,436 of 8,294,400 bytes different at a quarter screen, 999,606
	// at a half. Interpolating the CENTRES makes it exact, because at a splay of
	// nothing the two centres are a pitch apart on one plane and f*tan of the
	// angle to a point on that plane is the point's own offset.
	//
	// ⭐ AND IT WAS INVISIBLE FOR AS LONG AS THE TEST EXISTED, because
	// TestAFanOfNothingDrawsWhatTheStripDraws was comparing two blank canvases:
	// Desk.sources is empty until a frame has been pulled.
	ax, az := f.centre(focus, base)
	bx, bz := f.centre(focus, next)
	t := toward
	if f.anchor == AnchorOnGaze {
		t = toward * float64(next)
	}
	turn := math.Atan2(ax+t*(bx-ax), az+t*(bz-az))

	// ⛔⛔ AND NEVER MORE THAN HALF WAY ROUND THE RING. The chain is infinite and
	// the screens repeat along it, which is what makes the band close -- but at a
	// strong enough curvature it closes GEOMETRICALLY too: six screens at the
	// maximum sixty degrees is exactly a turn, so panel +4 has come all the way
	// round and lands IN FRONT of the viewer, drawn on top of the screens that
	// are really there. Measured by the sweep in TestTheFoldProtocol: "screen 2
	// runs to x=1920 and screen 6 starts at x=0: they overlap by 1920" -- one
	// screen covering the whole view over another.
	//
	// slantOf cannot catch it: a wrapped panel is in front of the viewer and
	// squarely enough turned to pass every test it makes. Half a turn along the
	// chain is the far side of the desk, and the far side of a desk is not in
	// shot.
	reach := FanReach
	if f.splayDeg > 0 {
		if half := int(180 / f.splayDeg); half < reach {
			reach = half
		}
	}

	// ⛔ THE REACH FOLLOWS THE GAZE, NOT THE ANCHOR. Rigid, the chain stays put
	// and the viewer walks along it, so the panels in shot are the ones around
	// where they are LOOKING -- centred on the anchor instead would leave the
	// half of the view they turned towards empty.
	hwOf := f.hwOf(focus)
	slot := 0
	for j := base - reach; j <= base+reach; j++ {
		lx, lz, rx, rz := slantChain(j, f.splayDeg, hwOf, f.gap, f.distance, turn)
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

// SetAnchoring chooses what the chain does when the gaze moves. See
// [Anchoring].
func (f *Fan) SetAnchoring(a Anchoring) { f.anchor = a }

// said is what to tell the wearer when this anchoring takes effect.
func (a Anchoring) said() string {
	if a == AnchorFixed {
		return "the desk stays where it is"
	}
	return "each screen turns towards you"
}
