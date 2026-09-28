// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"math"
	"testing"

	"github.com/go-xrkit/xrkit/glasses"
)

// ⛔⛔ A FLAT SCREEN IS ONE FACET TURNING BY NOTHING, not a special case the
// caller has to remember. If flat came back as "no facets" every reader of this
// would need a branch, and one of them would forget it.
func TestFlatComesBackAsOneFacetTurningByNothing(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name  string
		views float64
		fov   float64
		bend  float64
	}{
		{"asked to be flat", 3.3, 51.57, FlatBend},
		{"a negative radius", 3.3, 51.57, -1},
		{"an infinite one, which is what flat MEANS geometrically", 3.3, 51.57, math.Inf(1)},
		{"not a number", 3.3, 51.57, math.NaN()},
		{"a screen of no width", 0, 51.57, DefaultBend},
		{"glasses that report no field of view", 3.3, 0, DefaultBend},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			n, deg := bendFacets(c.views, c.fov, c.bend)
			if n != 1 || deg != 0 {
				t.Errorf("bendFacets(%g, %g, %g) = %d facets of %g°, want 1 of 0",
					c.views, c.fov, c.bend, n, deg)
			}
		})
	}
}

// ⛔ THE ARC IS THE SCREEN'S OWN WIDTH, and this is the claim the whole design
// rests on. At one source pixel per panel pixel a screen 3.3 views across
// subtends 3.3 fields of view, and curving it at the viewing distance wraps
// exactly that much around the viewer -- no more, and not a number someone
// liked.
func TestTheArcIsTheScreensOwnWidth(t *testing.T) {
	t.Parallel()

	const fov = 51.57
	for _, c := range []struct {
		views, bend, wantArc float64
	}{
		// 6400 pixels in a 1920 panel: 3.33 views.
		{6400.0 / 1920, DefaultBend, 3.3333 * fov},
		{5120.0 / 1920, DefaultBend, 2.6667 * fov},
		// One view across is one field of view, whatever else is true.
		{1, DefaultBend, fov},
		// ⭐ Twice the radius is half the arc: a flatter screen wraps less.
		{6400.0 / 1920, 2 * DefaultBend, 3.3333 * fov / 2},
	} {
		n, deg := bendFacets(c.views, fov, c.bend)
		if got := float64(n) * deg; math.Abs(got-c.wantArc) > 0.01 {
			t.Errorf("%g views at curve %g wrap %g°, want %g°",
				c.views, c.bend, got, c.wantArc)
		}
		// ⚠ And no facet turns more than the ceiling, which is what keeps the
		// joins from being findable.
		if deg > MaxFacetDeg+1e-9 {
			t.Errorf("%g views at curve %g: a facet turns %g°, more than %g",
				c.views, c.bend, deg, MaxFacetDeg)
		}
		if n < 1 || n > MaxFacets {
			t.Errorf("%g views at curve %g: %d facets", c.views, c.bend, n)
		}
	}
}

// ⚠ AND A NONSENSE CURVE CANNOT ASK FOR THOUSANDS. The ceiling is not there for
// a caller acting in good faith; it is there because this reads a number a
// person typed.
func TestAnAbsurdCurveIsCapped(t *testing.T) {
	t.Parallel()

	n, deg := bendFacets(3.3, 51.57, 1e-6)
	if n != MaxFacets {
		t.Errorf("a radius of a millionth gave %d facets, want the ceiling %d", n, MaxFacets)
	}
	if n < 1 || deg <= 0 {
		t.Errorf("%d facets of %g°", n, deg)
	}
}

// ⛔⛔ A FLAT DESK GIVES BACK THE ARITHMETIC THE CHAIN ALREADY HAD. This is the
// whole licence for wiring facets in: every desk shipped so far walks one facet
// per screen, and if the ring disagrees anywhere with k*splay, every one of them
// moves.
func TestAFlatRingIsTheChainThatWasThere(t *testing.T) {
	t.Parallel()

	const splay, gap, fov, panelW = 20.0, 12.0, 51.57, 1920
	for _, n := range []int{1, 2, 3, 6, 9} {
		r := newFacetRing(n, splay, gap, fov,
			func(int) int { return panelW },
			func(int) float64 { return 0.48 },
			func(int) float64 { return FlatBend },
			panelW)

		if len(r.f) != n {
			t.Fatalf("%d screens flat gave %d facets, want one each", n, len(r.f))
		}
		for k := -3 * n; k <= 3*n; k++ {
			if got, want := r.angleAt(float64(k)), float64(k)*splay; math.Abs(got-want) > 1e-9 {
				t.Errorf("%d screens: angleAt(%d) = %g, want %g", n, k, got, want)
			}
			if got := r.gapAt(k); got != gap {
				t.Errorf("%d screens: gapAt(%d) = %g, want the one gap %g", n, k, got, gap)
			}
			fa := r.at(k)
			if fa.srcX0 != 0 || fa.srcX1 != panelW {
				t.Errorf("%d screens: facet %d shows [%d,%d), want the whole source",
					n, k, fa.srcX0, fa.srcX1)
			}
			if math.Abs(fa.hw-0.48) > 1e-12 {
				t.Errorf("%d screens: facet %d is %g wide, want the screen's 0.48", n, k, fa.hw)
			}
		}
	}
}

// ⛔ AND A CURVED SCREEN CUTS ITS SOURCE UP EXACTLY ONCE, joins the pieces with
// NO gap, and folds by the splay only where it meets its NEIGHBOUR.
func TestACurvedScreenCutsItsSourceOnceAndLeavesNoGapInside(t *testing.T) {
	t.Parallel()

	const splay, gap, fov, panelW = 20.0, 12.0, 51.57, 1920
	const wide = 6400
	// Two screens: an ordinary one and a curved wide one.
	r := newFacetRing(2, splay, gap, fov,
		func(s int) int {
			if s == 1 {
				return wide
			}
			return panelW
		},
		func(int) float64 { return 0.48 },
		func(s int) float64 {
			if s == 1 {
				return DefaultBend
			}
			return FlatBend
		},
		panelW)

	if len(r.f) < 3 {
		t.Fatalf("the curved screen was cut into %d facets in all", len(r.f))
	}
	// The wide screen's facets tile its source, in order, with no overlap and
	// no hole: a gap would be a black line down the middle of a spreadsheet.
	want := 0
	inside := 0
	for i, fa := range r.f {
		if fa.screen != 1 {
			continue
		}
		if fa.srcX0 != want {
			t.Errorf("facet %d of the wide screen starts at %d, want %d", i, fa.srcX0, want)
		}
		want = fa.srcX1
		if inside > 0 {
			if fa.turnBy == splay {
				t.Errorf("facet %d turns by the splay; only the first of a screen does", i)
			}
			if prev := r.f[i-1]; prev.gapAfter != 0 {
				t.Errorf("a gap of %g inside the screen, before facet %d", prev.gapAfter, i)
			}
		} else if fa.turnBy != splay {
			t.Errorf("the screen's first facet turns by %g, want the splay %g", fa.turnBy, splay)
		}
		inside++
	}
	if want != wide {
		t.Errorf("the facets cover %d pixels of a %d-pixel source", want, wide)
	}
	if last := r.f[len(r.f)-1]; last.gapAfter != gap {
		t.Errorf("the screen's last facet is followed by %g, want the gap %g", last.gapAfter, gap)
	}
}

// ⚠ A FAN WITH NO RING STILL ANSWERS, because a Fan built as a literal is a
// reasonable thing to do to a struct and the suite does it. Adding a field and
// letting its absence crash the geometry is how this file learnt that lesson
// once already, with a segfault rather than a wrong picture.
func TestAFanWithNoRingStillMapsPanels(t *testing.T) {
	t.Parallel()

	bare := &Fan{n: 3, srcW: 1920}
	for _, s := range []int{-4, -1, 0, 2, 5} {
		if got := bare.firstFacet(s); got != s {
			t.Errorf("with no ring, firstFacet(%d) = %d, want the screen itself", s, got)
		}
	}
	screen, x0, x1 := bare.faceAt(0, 1)
	if screen != 1 || x0 != 0 || x1 != 1920 {
		t.Errorf("with no ring, panel 1 shows screen %d [%d,%d), want screen 1 and the whole source",
			screen, x0, x1)
	}

	// And WITH a ring, a screen index outside one turn comes back into it: the
	// band is a ring and walking past the last screen arrives at the first.
	withRing := &Fan{n: 3, srcW: 1920, splayDeg: 20, gap: 0.01, fovDeg: 51.57, hw: 0.48}
	withRing.rebuildRing()
	for _, c := range []struct{ s, want int }{{0, 0}, {1, 1}, {2, 2}, {3, 0}, {-1, 2}, {-3, 0}} {
		if got := withRing.firstFacet(c.s); got != c.want {
			t.Errorf("firstFacet(%d) = %d, want %d", c.s, got, c.want)
		}
	}
}

// ⛔⛔ THE WHOLE WIRING, FROM A PLAN TO THE PANELS A FRAME DRAWS. Every piece
// below has its own test; this is the one that says they are connected. A
// curved wide screen has to arrive at the fan as SEVERAL panels sharing one
// source, and an ordinary desk as one panel per screen.
func TestACurvedPlanReachesTheFanAsFacets(t *testing.T) {
	t.Parallel()

	beast := glasses.Display{Name: "VITURE Beast", Width: 3840, Height: 1080}
	p, err := NewPlan(beast, Options{Screens: 2})
	if err != nil {
		t.Fatal(err)
	}
	flat, err := NewFan(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(flat.ring.f); got != 2 {
		t.Errorf("a flat desk of 2 reaches the fan as %d panels, want 2", got)
	}

	// The same desk with screen 1 wide and curved.
	q := p.WithScreenWidth(1, 6400).WithBend(DefaultBend)
	curved, err := NewFan(q)
	if err != nil {
		t.Fatal(err)
	}
	curved.SetSourceWidths([]int{1920, 6400})
	curved.SetBends(q.Bends())

	var facets []facet
	for _, fa := range curved.ring.f {
		if fa.screen == 1 {
			facets = append(facets, fa)
		}
	}
	if len(facets) < 2 {
		t.Fatalf("the curved wide screen reached the fan as %d panel(s); it should be cut up", len(facets))
	}
	// ⛔ They tile its source once, in order, with no gap between them -- a gap
	// inside a screen would be a black line down the middle of a spreadsheet.
	at := 0
	for i, fa := range facets {
		if fa.srcX0 != at {
			t.Errorf("facet %d starts at %d, want %d", i, fa.srcX0, at)
		}
		at = fa.srcX1
		if i < len(facets)-1 && fa.gapAfter != 0 {
			t.Errorf("facet %d is followed by a gap of %g inside the screen", i, fa.gapAfter)
		}
	}
	if at != 6400 {
		t.Errorf("the facets cover %d pixels of the 6400-pixel screen", at)
	}
	// And the screen that is NOT wide stayed one panel, whatever the setting.
	ones := 0
	for _, fa := range curved.ring.f {
		if fa.screen == 0 {
			ones++
		}
	}
	if ones != 1 {
		t.Errorf("the panel-sized screen was cut into %d, want 1 -- only a wide screen curves", ones)
	}
}

// ⛔⛔ Facets IS THE LINE THAT FOUND THE DEFECT, so it is tested like one. A
// curved screen and a flat one draw the same picture at the same speed: the
// first curved run through the glasses looked perfect and proved nothing,
// because nothing could say which of the two it was.
func TestFacetsSaysWhatTheBandIsDrawnFrom(t *testing.T) {
	t.Parallel()

	// A fan with no ring answers with its screens, which is what it drew from
	// before facets existed.
	if got := (&Fan{n: 4, srcW: 1920}).Facets(); got != 4 {
		t.Errorf("with no ring, Facets() = %d, want the 4 screens", got)
	}
	// A desk with no fan is drawn by the strip, which has no facets to count.
	if got := (&Desk{}).Facets(); got != 0 {
		t.Errorf("with no fan, Facets() = %d, want 0", got)
	}
	// And one WITH a fan answers from it.
	if got := (&Desk{fan: &Fan{n: 3, srcW: 1920}}).Facets(); got != 3 {
		t.Errorf("with a fan of 3, Facets() = %d, want 3", got)
	}

	beast := glasses.Display{Name: "VITURE Beast", Width: 3840, Height: 1080}
	p, err := NewPlan(beast, Options{Screens: 2})
	if err != nil {
		t.Fatal(err)
	}
	flat, err := NewFan(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := flat.Facets(); got != 2 {
		t.Errorf("a flat desk of 2 is drawn from %d facets, want 2", got)
	}

	q := p.WithScreenWidth(1, 6400).WithBend(DefaultBend)
	curved, err := NewFan(q)
	if err != nil {
		t.Fatal(err)
	}
	curved.SetSourceWidths([]int{1920, 6400})
	curved.SetBends(q.Bends())
	if got := curved.Facets(); got <= 2 {
		t.Errorf("a curved wide screen is drawn from %d facets, want more than the 2 screens", got)
	}
}

// ⛔⛔ THE BEND IS JUDGED BY LOOKING, so it has to change WHILE the desk runs.
// The curve was rejected in August by wearing it; comparing two builds through
// a restart is not comparing, and a setting that needs a relaunch is a setting
// nobody will weigh twice.
func TestTheWideScreenBendsAndFlattensWhileTheDeskRuns(t *testing.T) {
	// ⚠ A FOLDED band, not testPlan's flat one: a flat band is drawn by the
	// strip, which has no panels to cut into facets. Bending belongs to the fan.
	p, err := NewPlan(glasses.Display{Name: "VITURE Beast", Width: 3840, Height: 1080},
		Options{Screens: 2})
	if err != nil {
		t.Fatal(err)
	}
	p = p.WithScreenWidth(0, p.ScreenW*3)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	if got := d.Plan().BendAsked(); got != FlatBend {
		t.Fatalf("a desk nobody told starts bent at %g, want flat", got)
	}
	flat := d.Facets()

	d.Do(ActionWrapWide)
	if got := d.Plan().BendAsked(); got != DefaultBend {
		t.Errorf("after asking to wrap it, the plan says %g", got)
	}
	bent := d.Facets()
	if bent <= flat {
		t.Errorf("wrapping drew %d facets against %d flat; a bend is more pieces", bent, flat)
	}

	d.Do(ActionFlatWide)
	if got := d.Plan().BendAsked(); got != FlatBend {
		t.Errorf("after asking for it flat, the plan says %g", got)
	}
	if got := d.Facets(); got != flat {
		t.Errorf("flattening drew %d facets, want the %d it started with", got, flat)
	}

	// ⭐ AND ASKING FOR THE SHAPE IT IS ALREADY IN DOES NOTHING. Rebuilding the
	// band, the gallery and the fan to arrive back where it started is work
	// nobody asked for -- once per press, held.
	d.Do(ActionFlatWide)
	if got := d.Facets(); got != flat {
		t.Errorf("asking twice for flat drew %d facets, want %d", got, flat)
	}
}
