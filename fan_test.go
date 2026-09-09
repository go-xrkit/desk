// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"math"
	"testing"

	"github.com/go-xrkit/xrkit/glasses"
)

// fanPlan is a band of n screens turned by splay degrees, seen from distance d.
func fanPlan(t *testing.T, n int, splay, d float64) Plan {
	t.Helper()
	p, err := NewPlan(glasses.Display{Name: "VITURE Beast", Width: 3840, Height: 1080},
		Options{Screens: n, SplayDeg: splay, Distance: d})
	if err != nil {
		t.Fatalf("NewPlan = %v", err)
	}
	if splay > 0 && p.SplayDeg() != splay {
		t.Fatalf("the plan is splayed %g, want %g", p.SplayDeg(), splay)
	}
	return p
}

// TestAFanOfNothingDrawsWhatTheStripDraws.
//
// The barrier the whole arrangement was chosen for. A splay of nothing is one
// flat plane, so the turned renderer and the flat one are drawing the same
// screens in the same places -- and they have to produce the same PICTURE, not
// merely the same rectangles.
//
// It is compared pixel for pixel because that is the only comparison that means
// anything: the two paths compute their source columns differently (a
// fixed-point stepper against a projective divide), so agreeing on the geometry
// and disagreeing on the pixels is exactly the failure this catches.
func TestAFanOfNothingDrawsWhatTheStripDraws(t *testing.T) {
	flat := fanPlan(t, 4, -1, 1) // negative asks for the flat band
	if flat.SplayDeg() != 0 {
		t.Fatalf("the flat plan is splayed by %g", flat.SplayDeg())
	}
	strip, err := New(flat, feedsFor(flat))
	if err != nil {
		t.Fatal(err)
	}
	defer strip.Close()
	if strip.fan != nil {
		t.Fatal("a flat plan built a fan")
	}

	// The same plan, with a fan forced on it at a splay of nothing: NewFan
	// refuses that, on purpose, so the fan is built by hand from the same
	// numbers. Refusing is right for production -- the strip is faster -- and
	// wrong for a test that has to compare the two.
	hw, panelH, f := slantOptics(flat.HFOVDeg, flat.ScreenW, flat.ScreenW, flat.ScreenH)
	fan := &Fan{
		n: flat.Count(), splayDeg: 0, distance: flat.Distance(),
		hw: hw, panelH: panelH, f: f,
		viewW: flat.ScreenW, viewH: flat.ScreenH,
		srcW: flat.ScreenW, srcH: flat.ScreenH,
		slots: make([][]SlantCol, 2*FanReach+1),
	}
	for i := range fan.slots {
		fan.slots[i] = make([]SlantCol, 0, flat.ScreenW)
	}

	// ⛔⛔ ONE RENDER FIRST, OR BOTH CANVASES ARE BACKGROUND. Desk.sources is
	// empty until a frame has been pulled, and Compose on an empty source draws
	// nothing -- so this comparison held two blank pictures against each other
	// and passed. Found when a deliberately broken chain was run against it and
	// the pixels still matched; see TestAWideScreenIsWideInTheChainToo, which is
	// the test that could not fail.
	strip.Render()
	if len(strip.sources) == 0 || strip.sources[0].W == 0 {
		t.Fatal("the sources are still empty after a render, so nothing is compared")
	}

	// ⭐ SQUARELY FACING A SCREEN, THE TWO ARE ONE PICTURE, byte for byte. Every
	// screen, because a ribbon does not put screen zero anywhere in particular
	// and the seam has to be crossed.
	for focus := range flat.Count() {
		if differ, total := disagree(strip, fan, focus, 0); differ != 0 {
			t.Errorf("screen %d square on: the turned renderer and the flat one "+
				"differ in %d of %d bytes", focus, differ, total)
		}
	}

	// ⛔⛔ AND BETWEEN TWO SCREENS THEY PART COMPANY, ON PURPOSE. This used to
	// demand the same equality here, and got it -- from two blank canvases.
	//
	// The two are not the same MOTION. [Strip] slides a band of pixels in front
	// of a fixed window, so every screen stays square on however far it has
	// slid. A chain is a place: the panels stay where they are and the VIEWER
	// turns, so a flat wall foreshortens as you look along it, exactly as a real
	// one does. The two coincide when you are square on, which is the anchor the
	// arrangement was chosen for, and nowhere else.
	//
	// The chain is the one that has to be right, because it is the one that runs
	// when the panels are turned and when the head is tracked. The strip is the
	// cheaper approximation, and it is what the flat band uses.
	//
	// Measured here rather than waved at, so neither side can drift: 6.2% of the
	// bytes at a quarter screen and 10.0% at a half, on a four-screen desk.
	for focus := range flat.Count() {
		for _, toward := range []float64{0.25, -0.25, 0.5} {
			differ, total := disagree(strip, fan, focus, toward)
			// ⛔ AND IT MUST DIFFER. If these two ever came out identical, one of
			// them would have stopped doing what its own doc comment says, and
			// the paragraph above would be describing code that is not there.
			if differ == 0 {
				t.Errorf("screen %d, %g along: the sliding band and the turning "+
					"viewer drew the same picture, which neither of them should",
					focus, toward)
			}
			if frac := float64(differ) / float64(total); frac > 0.15 {
				t.Errorf("screen %d, %g along: they differ in %.1f%% of the bytes, "+
					"which is more than turning a flat wall accounts for",
					focus, toward, 100*frac)
			}
		}
	}
}

// disagree draws one state with both renderers and counts the bytes that differ.
//
// The offset is walked through the NEIGHBOUR'S own step rather than an average
// one, which is how [Strip.Toward] reads it back: on a band with one wide screen
// the step towards it and the step away from it are different lengths.
func disagree(d *Desk, fan *Fan, focus int, toward float64) (differ, total int) {
	s := d.strip
	off := s.centre[focus]
	if toward != 0 {
		step := 1
		if toward < 0 {
			step = -1
		}
		next := ((focus+step)%s.n + s.n) % s.n
		span := s.short(s.centre[next] - s.centre[focus])
		off += int(math.Round(math.Abs(toward) * float64(span)))
	}

	want := NewCanvas(s.viewW, s.viewH)
	want.Compose(s.Frame(nil, off), d.sources, DefaultBackground)

	got := NewCanvas(s.viewW, s.viewH)
	got.ComposeSlants(fan.Frame(nil, focus, toward), d.sources, DefaultBackground)

	for i := range got.Pix {
		if got.Pix[i] != want.Pix[i] {
			differ++
		}
	}
	return differ, len(got.Pix)
}

// TestAFanShowsTheNeighboursTurned: pulled back, the frame is made of several
// panels, the middle one square on and the others keystoned.
func TestAFanShowsTheNeighboursTurned(t *testing.T) {
	p := fanPlan(t, 6, 20, 3)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if d.fan == nil {
		t.Fatal("a splayed plan built no fan")
	}

	d.Render()
	if len(d.blits) != 0 {
		t.Errorf("the turned band emitted %d rectangles", len(d.blits))
	}
	if len(d.slants) < 3 {
		t.Fatalf("the frame is %d panels; pulled back to three screens across "+
			"the view it should be at least three", len(d.slants))
	}

	// The middle one is square on -- it is the one being read -- and at least one
	// other is not.
	//
	// Within a pixel, because the two edges of a square-on panel are the same
	// height in real numbers and a row is an integer: a rounding of one row is
	// not a keystone, and asking for exact equality made this fail on the panel
	// that was in fact perfectly square.
	square, turned := 0, 0
	for _, s := range d.slants {
		first := int(s.Cols[0].Y1 - s.Cols[0].Y0)
		last := int(s.Cols[len(s.Cols)-1].Y1 - s.Cols[len(s.Cols)-1].Y0)
		if abs(first-last) <= 1 {
			square++
			continue
		}
		// And a keystone worth the name rather than a rounding: five rows at
		// least, which at this splay and distance is what twenty degrees of turn
		// comes to. The measured figure is nine per cent of the height; the
		// threshold is well under it on purpose, because what is being asserted
		// is "turned, not rounded" and not a particular angle.
		if abs(first-last) < 5 {
			t.Errorf("screen %d is keystoned by %d rows out of %d, which is neither "+
				"square nor turned", s.Screen, abs(first-last), max(first, last))
		}
		turned++
	}
	if square == 0 {
		t.Error("no panel is square on, so nothing is straight ahead")
	}
	if turned == 0 {
		t.Error("no panel is keystoned, so nothing was turned")
	}

	// And the screens are the band's, in order, wrapping round: the chain runs on
	// for ever and the screens repeat along it.
	for _, s := range d.slants {
		if s.Screen < 0 || s.Screen >= p.Count() {
			t.Errorf("the frame shows screen %d of %d", s.Screen, p.Count())
		}
	}
}

// TestTheFanWrapsTheBand: panel n of the chain is screen 0 again, which is what
// keeps the band a ring while the arrangement is an open chain.
func TestTheFanWrapsTheBand(t *testing.T) {
	p := fanPlan(t, 3, 20, 2)
	fan, err := NewFan(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ j, want int }{
		{0, 0}, {1, 1}, {2, 2}, {3, 0}, {4, 1}, {-1, 2}, {-2, 1}, {-3, 0}, {-4, 2},
	} {
		if got := fan.screenAt(c.j); got != c.want {
			t.Errorf("panel %d shows screen %d, want %d", c.j, got, c.want)
		}
	}
}

// TestNewFanRefusesWhatIsNotAFan: no screens, no size, and the flat band -- which
// is not a fan and has a faster renderer.
func TestNewFanRefusesWhatIsNotAFan(t *testing.T) {
	good := fanPlan(t, 4, 20, 2)
	for _, c := range []struct {
		what string
		plan Plan
	}{
		{"no screens", Plan{ScreenW: 100, ScreenH: 100}},
		{"no width", func() Plan { p := good; p.ScreenW = 0; return p }()},
		{"no height", func() Plan { p := good; p.ScreenH = 0; return p }()},
		{"the flat band", good.WithSplay(0)},
	} {
		if _, err := NewFan(c.plan); err == nil {
			t.Errorf("%s: built a fan anyway", c.what)
		}
	}
}

// TestTheSplayStopsAtBothEnds, and a press at an end is nothing at all.
func TestTheSplayStopsAtBothEnds(t *testing.T) {
	p := fanPlan(t, 6, DefaultSplayDeg, 2)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	for range int(MaxSplayDeg/SplayStep) + 4 {
		d.Do(ActionRounder)
	}
	if got := d.Plan().SplayDeg(); got != MaxSplayDeg {
		t.Errorf("pressing rounder past the end gave %g, want %g", got, MaxSplayDeg)
	}
	for range int(MaxSplayDeg/SplayStep) + 4 {
		d.Do(ActionFlatter)
	}
	if got := d.Plan().SplayDeg(); got != 0 {
		t.Errorf("pressing flatter past the end gave %g", got)
	}
	// Flat means the flat renderer, which is the point of going flat.
	if d.fan != nil {
		t.Error("at a splay of nothing the desk still has a fan")
	}
	d.Render()
	if len(d.blits) == 0 {
		t.Error("flat, the frame is made of no rectangles at all")
	}
	// And back again builds one.
	d.Do(ActionRounder)
	if d.fan == nil {
		t.Error("turning the screens again built no fan")
	}
	if err := d.Err(); err != nil {
		t.Errorf("walking both ends left an error: %v", err)
	}
}

// TestTheSplayKeepsTheFocusedScreen: the rebuild is a new fan over a new ribbon,
// so keeping the position is deliberate rather than lucky.
func TestTheSplayKeepsTheFocusedScreen(t *testing.T) {
	p := fanPlan(t, 6, DefaultSplayDeg, 2)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	for range 3 {
		d.Do(ActionNext)
	}
	was := d.Nav().Focus()
	if was == 0 {
		t.Fatal("the band did not move")
	}
	d.Do(ActionRounder)
	if got := d.Nav().Focus(); got != was {
		t.Errorf("turning the screens moved the desk to screen %d from %d", got, was)
	}
	d.Do(ActionFlatter)
	if got := d.Nav().Focus(); got != was {
		t.Errorf("flattening moved the desk to screen %d from %d", got, was)
	}
}

// TestTheSplayDoesNothingInTheGallery: it is head-locked and shows every screen
// at a readable size, so there is no angle to change.
func TestTheSplayDoesNothingInTheGallery(t *testing.T) {
	p := fanPlan(t, 6, DefaultSplayDeg, 2)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	d.Do(ActionGalleryOpen)
	was := d.Plan().SplayDeg()
	d.Do(ActionRounder)
	d.Do(ActionFlatter)
	if got := d.Plan().SplayDeg(); got != was {
		t.Errorf("the gallery changed the splay to %g from %g", got, was)
	}
	if err := d.Err(); err != nil {
		t.Errorf("pressing it in the gallery reported %v", err)
	}
}

// TestTheBandHasOnePositionForBothRenderers.
//
// Where the band IS comes from the strip, which takes each screen's place from
// the ribbon's own arrangement. It matters more than it looks: guessing that the
// screens are evenly spread put the band half a screen out of step with the
// navigator once, and that is invisible until somebody wears it. A fan doing its
// own arithmetic from the yaw would be the same mistake made twice -- and it was,
// for an afternoon: the desk opened at "screen three and a half" because the two
// disagreed about where screen zero starts.
func TestTheBandHasOnePositionForBothRenderers(t *testing.T) {
	p := fanPlan(t, 6, DefaultSplayDeg, 2)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	// Arrived on a screen, the band is ON that screen: nought screens past it,
	// whichever one it is and however it got there.
	for i := range p.Count() {
		if i > 0 {
			d.Do(ActionNext)
			d.Advance(largeEnoughToArrive)
		}
		focus := d.Nav().Focus()
		if focus != i {
			t.Fatalf("after %d steps the navigator is on screen %d", i, focus)
		}
		if got := d.strip.Toward(d.Nav().Yaw(), focus); got < -0.01 || got > 0.01 {
			t.Errorf("on screen %d the band is %g screens past it", focus, got)
		}
	}
	// Round the seam, which is where an absolute position along the band would
	// report most of a band instead of a little of one.
	d.Do(ActionNext)
	d.Advance(largeEnoughToArrive)
	if got := d.Nav().Focus(); got != 0 {
		t.Errorf("past the last screen the navigator is on %d", got)
	}
	if got := d.strip.Toward(d.Nav().Yaw(), 0); got < -0.01 || got > 0.01 {
		t.Errorf("past the last screen the band is %g screens past screen 0", got)
	}
	// Half way between two screens, both ways round, and never more than half a
	// screen from the one the navigator names.
	d.Do(ActionNext)
	for range 20 {
		d.Advance(0.02)
		focus := d.Nav().Focus()
		// Mid-turn the navigator names the screen it is going TO, so the band can
		// be most of a screen away from it -- but never more than one, and never
		// the long way round, which is what the seam wrapping is for.
		if got := d.strip.Toward(d.Nav().Yaw(), focus); got < -1.05 || got > 1.05 {
			t.Fatalf("mid-turn the band is %g screens from screen %d", got, focus)
		}
	}
}

// TestAValidPlanAlwaysBuildsItsFan.
//
// build drops the error from NewFan, and this is why it may. NewFan refuses three
// things -- no screens, no size, and a splay of nothing -- and by the time build
// asks, all three are impossible: the first two would have stopped NewStrip, and
// the third is the branch the call sits inside.
//
// Asserted over the whole range rather than argued in a comment, because the
// argument is only as good as the next change to NewFan.
func TestAValidPlanAlwaysBuildsItsFan(t *testing.T) {
	for n := 1; n <= MaxScreens; n++ {
		for _, splay := range []float64{SplayStep, DefaultSplayDeg, MaxSplayDeg} {
			for _, dist := range []float64{1, 2, MaxDistance} {
				p := fanPlan(t, n, splay, dist)
				_, _, _, fan, err := build(p)
				if err != nil {
					t.Fatalf("%d screens, splay %g, distance %g: build = %v",
						n, splay, dist, err)
				}
				if fan == nil {
					t.Errorf("%d screens, splay %g, distance %g: no fan",
						n, splay, dist)
				}
			}
		}
	}
	// And a plan with no angle builds no fan, which is the other half of the
	// same branch.
	flat := fanPlan(t, 4, -1, 1)
	if _, _, _, fan, err := build(flat); err != nil || fan != nil {
		t.Errorf("the flat band built fan=%v, err=%v", fan != nil, err)
	}
}

// TestComposeSlantsIgnoresAScreenItHasNoSourceFor.
//
// A frame is built from the ribbon and drawn against a list of sources, and the
// two are the caller's to keep in step. When they are not -- a screen added while
// a frame was in flight -- the choice is to skip that panel or to read past the
// end of a slice, and only one of those is a choice.
func TestComposeSlantsIgnoresAScreenItHasNoSourceFor(t *testing.T) {
	c := NewCanvas(200, 100)
	src := Source{Pix: make([]byte, 20*10*4), W: 20, H: 10, Stride: 20 * 4}
	for i := range src.Pix {
		src.Pix[i] = 0xFF
	}
	cols := []SlantCol{{Src: 0, Y0: 10, Y1: 30}, {Src: 1, Y0: 10, Y1: 30}}
	c.ComposeSlants([]Slant{
		{Screen: -1, Dst: rectOf(0, 10, 2, 20), Cols: cols},
		{Screen: 7, Dst: rectOf(4, 10, 2, 20), Cols: cols},
	}, []Source{src}, DefaultBackground)

	// Nothing but the background: both panels named a screen that is not there.
	for i := 0; i+3 < len(c.Pix); i += 4 {
		if c.Pix[i] != DefaultBackground[0] {
			t.Fatalf("byte %d is %d, not the background", i, c.Pix[i])
		}
	}
	// And the one that names a real screen is drawn, or this proves nothing.
	c2 := NewCanvas(200, 100)
	c2.ComposeSlants([]Slant{{Screen: 0, Dst: rectOf(0, 10, 2, 20), Cols: cols}},
		[]Source{src}, DefaultBackground)
	painted := false
	for i := 0; i+3 < len(c2.Pix); i += 4 {
		if c2.Pix[i] == 0xFF {
			painted = true
			break
		}
	}
	if !painted {
		t.Error("the panel naming a real screen was not drawn either")
	}
}

// TestTowardRefusesToGuess: a focus that is not a screen, and a band of one.
//
// Both are nought rather than an error, and for the same reason: this answers a
// question about where the band is, and when there is no sensible answer the band
// has not moved. A caller cannot act on an error here -- it is called once per
// frame from a draw loop.
func TestTowardRefusesToGuess(t *testing.T) {
	p := fanPlan(t, 4, -1, 1)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	for _, focus := range []int{-1, 4, 99} {
		if got := d.strip.Toward(d.Nav().Yaw(), focus); got != 0 {
			t.Errorf("focus %d gave %g", focus, got)
		}
	}

	// One screen: there is nowhere else for the band to be.
	one := fanPlan(t, 1, -1, 1)
	solo, err := New(one, feedsFor(one))
	if err != nil {
		t.Fatal(err)
	}
	defer solo.Close()
	if got := solo.strip.Toward(solo.Nav().Yaw(), 0); got != 0 {
		t.Errorf("a band of one screen is %g screens past it", got)
	}
}

// TestTowardTakesTheShortWayRoundBothWays.
//
// The seam, from either side. An offset a whisker past the last screen's centre
// is a whisker PAST it, not most of a band before it -- and the same going the
// other way, which is a separate branch and was written as one.
func TestTowardTakesTheShortWayRoundBothWays(t *testing.T) {
	p := fanPlan(t, 6, -1, 1)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	s := d.strip
	slot := float64(s.total) / float64(p.Count())

	// Asked about a screen most of a band away: the answer is the short way
	// round. On screen 0 and asked about the last screen, the band is ONE screen
	// back -- not five forward, which is the same place and a useless thing to be
	// told, and what an unwrapped subtraction says.
	// Asked about EVERY screen from one position: the answer is never more than
	// half a band, whichever way round it lies.
	//
	// This is the property rather than a number, and it is the one that matters:
	// two screens on a closed band are always within half a band of each other,
	// so an answer bigger than that is the long way round -- the same place said
	// uselessly. A ribbon does not put screen zero at zero degrees (this one
	// starts at 210), so which screens need the wrap is not something to write
	// down; that they all obey it is.
	half := float64(p.Count()) / 2
	sawForward, sawBack := false, false
	// From every screen, and either side of each, because which pairs need the
	// wrap depends on where the ribbon put screen zero -- and that is not this
	// test's business to know.
	for from := range p.Count() {
		for _, nudge := range []float64{-0.4, 0, 0.4} {
			at := float64(s.centre[from]) + nudge*slot
			yaw := at / float64(s.total) * 2 * math.Pi
			for k := range p.Count() {
				got := s.Toward(yaw, k)
				if got > half+0.01 || got < -half-0.01 {
					t.Errorf("%g past screen %d, screen %d is %g screens away in "+
						"a band of %d", nudge, from, k, got, p.Count())
				}
				if got > 0.5 {
					sawForward = true
				}
				if got < -0.5 {
					sawBack = true
				}
			}
		}
	}
	if !sawForward || !sawBack {
		t.Errorf("every screen came out on the same side (forward %v, back %v), so "+
			"the wrapping was never exercised in both directions",
			sawForward, sawBack)
	}

	// Just before and just after each screen's centre, asked about that screen.
	for focus := range p.Count() {
		for _, delta := range []float64{-0.4, -0.1, 0.1, 0.4} {
			off := s.centre[focus] + int(delta*slot)
			// Toward works from a yaw, so go the other way through Offset: the
			// yaw whose offset is this. The band is a full turn, so that is a
			// simple proportion.
			yaw := float64(off) / float64(s.total) * 2 * math.Pi
			got := s.Toward(yaw, focus)
			if got < delta-0.02 || got > delta+0.02 {
				t.Errorf("screen %d, %g of a screen along: Toward said %g",
					focus, delta, got)
			}
		}
	}
}

// TestTheScreenBeingLookedAtIsTheOneInFront pins the fix for a fold that landed
// inside the picture.
//
// ⛔⛔ THE CHAIN USED TO BE BUILT AROUND THE FOCUSED SCREEN, and head tracking
// decoupled the focus from where somebody is looking: followHead sets the yaw
// and leaves the focus alone, so a head turned one screen to the right sits at
// toward = 1 indefinitely. There the old chain drew the screen in front TURNED
// 23.6° away -- x 92..1612 of 1920 rather than the whole view -- which put its
// hinge at x 1612, inside the picture. Reported from the glasses as "l'angle de
// cintrage n'est pas au bon endroit et tombe a l'interieur de l'ecran en face".
//
// ⭐ SO IT IS THE FULL WIDTH THAT IS ASSERTED, not merely which screen is drawn:
// the old code centred the right screen and still drew it wrong, so a test that
// only asked "which screen" would have passed throughout.
func TestTheScreenBeingLookedAtIsTheOneInFront(t *testing.T) {
	p := Plan{ScreenW: 1920, ScreenH: 1200, HFOVDeg: 45.6, VFOVDeg: 29.2}
	p = p.WithScreens(6).WithSplay(20)
	f, err := NewFan(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		toward float64
		want   int
	}{{-2, 4}, {-1, 5}, {0, 0}, {1, 1}, {2, 2}, {6, 0}} {
		var got *Slant
		for _, s := range f.Frame(nil, 0, c.toward) {
			if s.Dst.W > 1 {
				got = &s
			}
		}
		if got == nil {
			t.Fatalf("toward %+.0f: nothing drawn", c.toward)
		}
		if got.Screen != c.want {
			t.Errorf("toward %+.0f: screen %d in front, want %d",
				c.toward, got.Screen, c.want)
		}
		if got.Dst.X != 0 || got.Dst.W != p.ScreenW {
			t.Errorf("toward %+.0f: screen %d spans x %d..%d, want the whole view 0..%d"+
				" -- a screen squarely looked at is a screen squarely faced",
				c.toward, got.Screen, got.Dst.X, got.Dst.X+got.Dst.W, p.ScreenW)
		}
	}
}

// TestTwoScreensSideBySideLeaveASeam.
//
// ⛔⛔ THE FOLD WAS INVISIBLE, AND THAT IS WHAT KEPT BEING REPORTED. The panels
// were hinged edge to edge with NOTHING between them, so the last column of one
// screen and the first column of the next were neighbours -- measured from the
// glasses at "screen 6 at x 0..917, screen 1 at x 916..1920", which overlap by a
// pixel. Three reports of "l'angle est toujours mauvais" were partly this: there
// was no line to look at, so there was no way to say where the fold was.
//
// Asked for in those terms: "il faut donc pouvoir detecter la fin d'un ecran sur
// le coté et le debut d'un autre", then "pas de probleme pour avoir un leger
// espace entre chaque ecran".
//
// The gap is the flat band's own DefaultGapPx, which is what makes the two
// renderers agree about where the next screen starts; see
// TestASplayOfNothingIsTheFlatBand for the other half of that.
func TestTwoScreensSideBySideLeaveASeam(t *testing.T) {
	for _, dist := range []float64{2, 3, 4} {
		p := fanPlan(t, 6, DefaultSplayDeg, dist)
		f, err := NewFan(p)
		if err != nil {
			t.Fatalf("distance %g: NewFan = %v", dist, err)
		}
		panels := f.Frame(nil, 0, 0)

		compared := 0
		for i := 1; i < len(panels); i++ {
			prev, cur := panels[i-1], panels[i]
			// The two edges that FORM the seam have to be the panels' own, not
			// the canvas's: a panel cut off on the right ends where the canvas
			// does. Its other end can be clipped and the seam still means
			// something, which at distance 2 is the only way to measure one at
			// all -- the screen in front fills most of the view and both its
			// neighbours run off an edge.
			if prev.Dst.X+prev.Dst.W >= p.ScreenW || cur.Dst.X <= 0 {
				continue
			}
			compared++
			if seam := cur.Dst.X - (prev.Dst.X + prev.Dst.W); seam < 1 {
				t.Errorf("distance %g: screen %d ends at x=%d and screen %d starts "+
					"at x=%d -- a seam of %d",
					dist, prev.Screen, prev.Dst.X+prev.Dst.W,
					cur.Screen, cur.Dst.X, seam)
			}
		}
		// ⛔ AND SOMETHING HAD TO BE COMPARED. Every pair being skipped as
		// clipped would leave a green test that looked at nothing, which is the
		// shape of failure this package keeps meeting.
		if compared == 0 {
			t.Fatalf("distance %g: of %d panels no pair showed both its own "+
				"edges, so no seam was measured", dist, len(panels))
		}
	}
}

// TestAWideScreenIsWideInTheChainToo.
//
// ⛔⛔ THE CHAIN GAVE EVERY PANEL THE SAME WIDTH, AND ONE DESK HERE IS NOT LIKE
// THAT. An Odyssey G95NC is mirrored onto a position: 7680x2160 native,
// captured at the band's height, so 3840x1080 -- TWICE every other screen.
// [Strip] has placed it as 3840 on the band since the day widths became
// per-screen; the chain squeezed it into a 1920 panel, so it was compressed by
// half AND the fold to its neighbour landed where a 1920 screen would have
// ended.
//
// Reported from the glasses as "on a toujours un probleme d'écran dont la
// cassure n'est pas au bon endroit" -- three times, across two other causes --
// and then diagnosed from the same chair: "ce peut il que ce soit du a la
// taille de l'odyssey que tu redimensionne en largeur pour faire tenir dans
// l'ecran des lunettes?".
//
// ⭐ AND THE OLD SUITE WAS GREEN THROUGHOUT, because every plan it built had
// screens of one shape. A bench run is what settled it: at widths of 1920, 3440
// and 5120 for one screen, not one panel edge moved.
//
// The comparison is the one TestAFanOfNothingDrawsWhatTheStripDraws makes --
// pixel for pixel against the flat renderer, at a splay of nothing -- because
// the flat renderer has always known each screen's own width. A mixed band is
// exactly where the two disagreed.
func TestAWideScreenIsWideInTheChainToo(t *testing.T) {
	flat := fanPlan(t, 4, -1, 1) // negative asks for the flat band
	const wide = 1
	flat = flat.WithScreenWidth(wide, 2*flat.ScreenW)
	if flat.ScreenWidth(wide) != 2*flat.ScreenW {
		t.Fatalf("screen %d is %d wide, want %d -- the plan refused the shape "+
			"and there is nothing mixed to test",
			wide, flat.ScreenWidth(wide), 2*flat.ScreenW)
	}

	feeds := make([]Feed, flat.Count())
	widths := make([]int, flat.Count())
	for i := range feeds {
		widths[i] = flat.ScreenWidth(i)
		feeds[i] = newStripedFeed(widths[i], flat.ScreenH, byte(10*(i+1)))
	}
	strip, err := New(flat, feeds)
	if err != nil {
		t.Fatal(err)
	}
	defer strip.Close()

	// Built by hand, because NewFan refuses a splay of nothing on purpose.
	hw, panelH, f := slantOptics(flat.HFOVDeg, flat.ScreenW, flat.ScreenW, flat.ScreenH)
	fan := &Fan{
		n: flat.Count(), splayDeg: 0, distance: flat.Distance(),
		hw: hw, gap: slantGap(hw, flat.ScreenW), panelH: panelH, f: f,
		viewW: flat.ScreenW, viewH: flat.ScreenH,
		srcW: flat.ScreenW, srcH: flat.ScreenH,
		slots: make([][]SlantCol, 2*FanReach+1),
	}
	for i := range fan.slots {
		fan.slots[i] = make([]SlantCol, 0, flat.ScreenW)
	}
	fan.SetSourceWidths(widths)

	// ⭐ THE PANEL IS AS WIDE AS THE SCREEN, in the units the chain works in:
	// f*2*hw_i is the screen's own pixel width, because f*2*hw is the view and
	// hw_i is hw times the screen's share of it. Asserted before any picture,
	// so a failure says which of the two things is wrong.
	hwOf := fan.hwOf(0)
	for i := range flat.Count() {
		if got, want := f*2*hwOf(i), float64(flat.ScreenWidth(i)); math.Abs(got-want) > 1e-6 {
			t.Errorf("panel %d projects %g pixels wide at distance 1, want %d",
				i, got, flat.ScreenWidth(i))
		}
	}

	// One render, or both canvases are background and nothing is compared.
	strip.Render()
	if len(strip.sources) == 0 || strip.sources[wide].W != flat.ScreenWidth(wide) {
		t.Fatalf("after a render the wide source is %v, so nothing is compared",
			strip.sources)
	}

	// ⭐ AND THE BAND IS AS LONG AS ITS SCREENS, so each keeps its own pixels.
	// Before, the length was Count()*(ScreenW+gap) -- as if every screen were
	// nominal -- while ribbon.Place shared the arcs out by shape, so the wide
	// screen took its share out of the others and EVERY screen came out 1536
	// pixels wide instead of 1920. See Plan.BandPx.
	if got, want := strip.strip.total, flat.BandPx(); got != want {
		t.Errorf("the band is %d pixels for screens totalling %d", got, want)
	}
	for i := range flat.Count() {
		if got, want := strip.strip.width[i], flat.ScreenWidth(i); got != want {
			t.Errorf("screen %d is %d pixels on the band, want its own %d",
				i, got, want)
		}
	}

	// Square on, the two renderers are one picture, with a screen of another
	// shape on the band. This is the assertion the whole change is for.
	for focus := range flat.Count() {
		if differ, total := disagree(strip, fan, focus, 0); differ != 0 {
			t.Errorf("screen %d square on, with a %d-wide screen on the band: the "+
				"turned renderer and the flat one differ in %d of %d bytes",
				focus, flat.ScreenWidth(wide), differ, total)
		}
	}
}

// newStripedFeed is a source whose colour changes along x.
//
// ⛔⛔ THE FLAT FEEDS COULD NOT SEE THE BUG ABOVE. newFakeFeed paints one colour
// over a whole screen, so squeezing that screen to half its width leaves every
// pixel exactly the colour it was: run against the single-width chain this test
// exists to refuse, the pixel comparison passed, and only the assertion on the
// panel widths failed. A comparison that cannot come out different has measured
// nothing -- so the source is striped, and a compression moves the stripes.
func newStripedFeed(w, h int, tag byte) *fakeFeed {
	const stripe = 16
	pix := make([]byte, w*h*4)
	for y := range h {
		for x := range w {
			v := tag
			if (x/stripe)%2 == 1 {
				v = 255 - tag
			}
			i := (y*w + x) * 4
			pix[i], pix[i+1], pix[i+2], pix[i+3] = v, v, v, 255
		}
	}
	return &fakeFeed{src: Source{Pix: pix, W: w, H: h, Stride: w * 4}, fresh: true}
}
