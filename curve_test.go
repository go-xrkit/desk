// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"fmt"
	"math"
	"testing"

	"github.com/go-xrkit/xrkit/glasses"
)

// ⛔⛔ A FLAT SCREEN IS ONE FACET TURNING BY NOTHING, not a special case the
// caller has to remember. If flat came back as "no facets" every reader of this
// would need a branch, and one of them would forget it.
//
// ⚠ AND THE "ONE" IS KNOWN TO BE WRONG FOR A SCREEN WIDER THAN THE VIEW, which
// is recorded here rather than left for the next reader to rediscover. Frame
// walks the chain in PANELS and `toward` is a fraction of the step from one
// panel to the next, so a screen left as a single panel has nowhere for the gaze
// to step: a quarter step goes a quarter of the way to the screen's own next
// copy, a whole splay round the band. Measured, one flat screen of 6400 at a
// splay of twenty: at toward 0 the panel filled all 1920 columns, at toward 0.25
// it painted 382 and left 1538 dark.
//
// Cutting a flat screen into one panel per view fixes that and costs something
// else -- it inflates the half-turn reach cap, which counts panels and assumes
// they all turn -- so the flat fold protocol went from 0 complaints to 217.
// Deriving the cap from the cumulative angle instead recovers most of it but not
// all. See the skipped cases in TestAFrameCoversTheWholeView.
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

// ⛔⛔ THE BAND HAS TO REACH BOTH EDGES OF THE VIEW, and no test asked that until
// a defect arrived from the glasses in three words: "tres etroit".
//
// FanReach is four PANELS either side of the middle, chosen when a panel was a
// screen. Cut one into eighty-six facets and four panels either side is a tenth
// of it: the desk drew a correct picture of a fragment, sharp and sixty-two
// frames a second, and every test passed. They all ask what a panel projects to;
// none asked how many there should be.
//
// So this asks the only question that would have caught it: given a frame, is
// any column of the view left unpainted?
func TestAFrameCoversTheWholeView(t *testing.T) {
	t.Parallel()

	beast := glasses.Display{Name: "VITURE Beast", Width: 3840, Height: 1080}
	for _, c := range []struct {
		name string
		wide int
		bend float64
	}{
		{"an ordinary desk", 0, FlatBend},
		{"one wide screen, flat", 6400, FlatBend},
		// ⭐ The case that was broken. 6400 pixels curved at the viewing
		// distance is 86 facets, and the old reach drew nine of them.
		{"one wide screen, bent", 6400, DefaultBend},
		{"a wider one still", 10240, DefaultBend},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			// ⛔⛔ A KNOWN DEFECT, SKIPPED AND NAMED RATHER THAN QUIETLY DROPPED.
			// A FLAT screen wider than the view is one panel on the chain, and
			// Frame steps in panels: `toward` a quarter of the way to the next
			// one goes a quarter of the way to this screen's own next COPY, a
			// whole splay round the band, which lands in the void beside it.
			// Measured here, one flat screen of 6400 at a splay of twenty:
			//
			//	toward 0.00   0 of 1920 columns unpainted
			//	toward 0.25   1538 unpainted
			//	toward 0.50   727 unpainted
			//	toward 0.75   1538 unpainted
			//
			// ⛔ AND IT IS REACHED BY THE ONE GESTURE WIDE MODE EXISTS FOR.
			// headToBand returns 1 when the desk holds fewer than two screens,
			// so on a single wide screen a head turn goes into the ribbon's yaw
			// unconverted and Desk.draw hands Frame exactly these values:
			// turning to read the right-hand end of a spreadsheet empties the
			// view. The BENT case below is correct and is not skipped, so a wide
			// screen is usable today by curving it.
			//
			// Cutting a flat screen into one panel per view fixes this and
			// inflates the half-turn reach cap, which counts panels and assumes
			// every panel turns: the flat fold protocol went from 0 complaints to
			// 217. Deriving that cap from the cumulative angle instead recovers
			// most of them. Neither half is landed because together they are not
			// yet green, and a renderer that is nearly right is worse than one
			// whose limits are written down.
			// ⛔⛔ CHARACTERISED, NOT SKIPPED, and the difference is not cosmetic.
			// A t.Skip here stopped the frame being DRAWN at all, and the 100%
			// coverage gate caught it at once: Frame fell to 95.0% and reachIn to
			// 88.2%, because the flat wide screen is the only case in the suite
			// that walks those branches. A skip that stops exercising the code is
			// how a known defect becomes an unknown one.
			//
			// ⭐ SO IT FAILS WHEN THE DEFECT IS FIXED. The frame is drawn, the
			// blank columns are counted, and a case marked known must STILL come
			// back blank -- otherwise this says so and asks for the marker to be
			// taken out. A characterisation test that cannot notice being fixed
			// is a comment with a test's name on it.
			known := c.wide > 0 && c.bend == FlatBend
			worstKnown := 0

			// ⛔ WIDE MODE IS ONE SCREEN, so that is the desk this asks about. A
			// wide screen BESIDE an ordinary one is an arrangement the command no
			// longer builds -- "on ne veux que un ecran" -- and asserting coverage
			// for it would be holding the code to a shape nobody can ask for.
			screens := 2
			if c.wide > 0 {
				screens = 1
			}
			p, err := NewPlan(beast, Options{Screens: screens})
			if err != nil {
				t.Fatal(err)
			}
			if c.wide > 0 {
				// ⛔ THROUGH WidePlan, which is the path a PERSON's -wide takes
				// and the only one that clamps. WithScreenWidth is the
				// MEASUREMENT path: it refuses a width outside the band's aspect
				// range, because there the number is a captured source's shape
				// and one out of range is a capture that has gone wrong. Calling
				// it directly with 10240 on a 1080-high band silently produced an
				// ordinary 1920 flat screen -- this subtest failed with 1341 of
				// 1920 columns unpainted, which was the refusal and not the
				// geometry.
				p = WidePlan(p, c.wide).WithBend(c.bend)
			}
			f, err := NewFan(p)
			if err != nil {
				t.Fatal(err)
			}
			if c.wide > 0 {
				// ⛔ THE PLAN'S WIDTH, NOT THE ONE ASKED FOR. WidePlan clamps to
				// the band's aspect range, so 10240 on a 1080-high band is 8640
				// -- and a fan told 10240 while the plan holds 8640 is the two
				// halves of the renderer disagreeing about where the pixels are.
				f.SetSourceWidths([]int{p.ScreenWidth(0)})
				f.SetBends(p.Bends())
			}

			// Every focus and a few positions between, because the gap that
			// "tres etroit" describes moves with the gaze.
			for focus := range p.Count() {
				for _, toward := range []float64{0, 0.25, 0.5, 0.75} {
					painted := make([]bool, p.ScreenW)
					for _, s := range f.Frame(nil, focus, toward) {
						for x := s.Dst.X; x < s.Dst.X+s.Dst.W && x < len(painted); x++ {
							if x >= 0 {
								painted[x] = true
							}
						}
					}
					blank := 0
					for _, ok := range painted {
						if !ok {
							blank++
						}
					}
					// ⚠ A few columns may legitimately be background -- the gap
					// between two screens crosses the view. A TENTH of the view
					// painted is the defect; nine tenths blank is not a gap.
					if known {
						if blank > worstKnown {
							worstKnown = blank
						}
						continue
					}
					if blank > len(painted)/2 {
						t.Errorf("focus %d, %.2f along: %d of %d columns are unpainted",
							focus, toward, blank, len(painted))
					}
				}
			}
			// ⭐ AND THE KNOWN DEFECT HAS TO STILL BE THERE. Measured at 1538 of
			// 1920 when this was written; anything above half the view is the
			// same defect. If it comes back under that, somebody has fixed it --
			// which is good news and means this marker and the one in
			// TestFlatComesBackAsOneFacetTurningByNothing should go, and the
			// stashed facet split should be re-measured against the fold
			// protocol rather than left in a stash.
			if known && worstKnown <= p.ScreenW/2 {
				t.Errorf("a flat screen %d wide now leaves at worst %d of %d "+
					"columns unpainted, where it left 1538: the defect this case "+
					"characterises is fixed, so take the marker out and assert "+
					"coverage like the others", c.wide, worstKnown, p.ScreenW)
			}
		})
	}
}

// ⛔⛔ A SCREEN READS LEFT TO RIGHT, AND NOTHING ASKED THAT ACROSS ITS PANELS.
// The fold protocol checks that the source runs forwards WITHIN one panel -- "a
// source that runs the wrong way at one end is how a mirrored panel would look"
// -- and the facet-join check added with it only runs when consecutive panels
// show the same screen, which on a flat desk never happens. So a screen drawn in
// pieces could have its pieces in any order at all and every test stayed green.
//
// ⭐ IT WAS NOT HYPOTHETICAL. Panel 0 of the chain showed a screen's FIRST facet,
// and slantChain puts panel 0 square on at the viewing distance -- so the eye was
// aimed at the screen's left edge and the whole rest of it wrapped round behind
// and came back on the other side. Reported from inside the glasses in exactly
// these terms: "dans les lunettes je vois la gauche d'un ecran a droite et la
// droite a gauche". Measured on the configuration that was running, one screen
// of 6400 curved at the viewing distance: the view's left edge showed source
// column 5608 and its right edge 933.
//
// ⚠ AND TestAFrameCoversTheWholeView COULD NOT SEE IT, which is the lesson
// worth more than the fix. It counts how many columns are PAINTED and never asks
// WHICH source lands in them, so a frame with every column filled from the wrong
// places scores perfectly. A coverage measure is blind to content by
// construction; this is the test that reads it.
func TestAScreenReadsLeftToRight(t *testing.T) {
	t.Parallel()

	beast := glasses.Display{Name: "VITURE Beast", Width: 3840, Height: 1080}
	for _, c := range []struct {
		name    string
		screens int
		wide    int
		bend    float64
	}{
		{"one wide screen, bent, which is what was reported", 1, 6400, DefaultBend},
		{"a wider one still", 1, 10240, DefaultBend},
		{"a gentler curve", 1, 6400, 2 * DefaultBend},
		{"an ordinary desk, where this has always held", 6, 0, FlatBend},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			p, err := NewPlan(beast, Options{Screens: c.screens})
			if err != nil {
				t.Fatal(err)
			}
			if c.wide > 0 {
				p = WidePlan(p, c.wide).WithBend(c.bend)
			}
			f, err := NewFan(p)
			if err != nil {
				t.Fatal(err)
			}
			widths := make([]int, p.Count())
			for i := range widths {
				widths[i] = p.ScreenWidth(i)
			}
			f.SetSourceWidths(widths)
			f.SetBends(p.Bends())

			for focus := range p.Count() {
				for _, toward := range []float64{0, 0.25, 0.5} {
					// The panels of ONE screen, in the order the view puts them.
					// Panels of other screens are skipped rather than compared:
					// two different screens may sit either way round on the band.
					last := map[int]int32{}
					lastAt := map[int]int{}
					for _, s := range f.Frame(nil, focus, toward) {
						if len(s.Cols) == 0 {
							continue
						}
						first := s.Cols[0].Src
						if prev, seen := last[s.Screen]; seen && first < prev {
							t.Errorf("focus %d, %.2f along: screen %d shows source "+
								"column %d at x=%d after column %d at x=%d -- it "+
								"reads right to left, which is the screen wrapped "+
								"round the viewer rather than laid out in front",
								focus, toward, s.Screen+1, first, s.Dst.X,
								prev, lastAt[s.Screen])
						}
						last[s.Screen] = s.Cols[len(s.Cols)-1].Src
						lastAt[s.Screen] = s.Dst.X
					}
				}
			}

			// ⭐ AND THE SCREEN BEING LOOKED AT IS CENTRED ON ITS MIDDLE, which is
			// the property the fix rests on: panel 0 is square on at the viewing
			// distance, so whichever facet it shows is what the eye is aimed at.
			// Before, that was the screen's first facet -- its left edge.
			if c.wide == 0 {
				return
			}
			mid, half := p.ScreenW/2, p.ScreenWidth(0)/2
			for _, s := range f.Frame(nil, 0, 0) {
				if s.Dst.X <= mid && mid < s.Dst.X+s.Dst.W && len(s.Cols) > 0 {
					got := int(s.Cols[len(s.Cols)/2].Src)
					// Within one facet of the middle: 86 facets of a 6400-wide
					// screen is 74 columns each, and the centre column of the
					// view falls inside one of them rather than on its centre.
					if tol := p.ScreenWidth(0)/len(f.ring.f) + 1; got < half-tol || got > half+tol {
						t.Errorf("the middle of the view shows source column %d "+
							"of a screen %d wide: the eye is aimed %d columns "+
							"from its centre", got, p.ScreenWidth(0), got-half)
					}
					return
				}
			}
			t.Error("no panel covers the middle of the view")
		})
	}
}

// ⛔⛔ HALF A TURN IS A PROPERTY OF THE FOCUS, NOT OF THE RING, and a single
// number for the whole band was wrong for every screen but one.
//
// reachIn caps the reach at len(f)*180/total: how many facets make half a turn
// IF they all turned by the same amount. On a desk with one curved screen among
// ordinary ones they are nothing like uniform -- the curved screen's facets turn
// by MaxFacetDeg, two degrees, while the fold between two ordinary screens is
// the whole splay.
//
// ⭐ MEASURED, six screens at a splay of sixty with one of them 3840 wide,
// curved at the viewing distance: 57 facets over 461.2°, so the uniform cap
// allows 22. From the wide screen those 22 facets walk 44°; from the screen
// beside it, 334° -- very nearly a whole turn -- and from the one after, 218°.
// The far side of the band was therefore drawn OVER the near side, which the
// fold protocol reports as "screen 7 runs to x=1920 and screen 3 starts at x=0:
// they overlap by 1920" -- one screen covering the whole view over another.
//
// ⚠ AND IT WAS THE MAJORITY OF WHAT THE CURVED SWEEP FOUND. Counted by family
// over the whole protocol, both anchorings, seven splays, five distances:
//
//	family                  before  after
//	overlap by                1653      0
//	no seam                    552      0
//	the wrong screen centred    40      0
//	a screen cut short        3710   2790
//
// Nothing was traded for anything: every family fell.
func TestTheBandNeverComesRoundOnItself(t *testing.T) {
	t.Parallel()

	beast := glasses.Display{Name: "VITURE Beast", Width: 3840, Height: 1080}
	// The corner the overlaps lived in -- the widest splays and the furthest
	// distances, where the band closes geometrically -- and a gentle desk as the
	// control, since a cap that fires everywhere would be hiding the band rather
	// than bounding it.
	for _, splay := range []float64{20, 51.6, MaxSplayDeg} {
		for _, dist := range []float64{1, 2, MaxDistance} {
			p, err := NewPlan(beast, Options{Screens: 6})
			if err != nil {
				t.Fatal(err)
			}
			p = p.WithScreenWidth(0, 3840).WithSplay(splay).
				WithDistance(dist).WithBend(DefaultBend)
			widths := make([]int, p.Count())
			for i := range widths {
				widths[i] = p.ScreenWidth(i)
			}
			f, err := NewFan(p)
			if err != nil {
				t.Fatal(err)
			}
			f.SetSourceWidths(widths)
			f.SetBends(p.Bends())

			for focus := range p.Count() {
				for _, toward := range []float64{-0.5, 0, 0.5} {
					where := fmt.Sprintf("%g°, %gx, screen %d, %+.1f along",
						splay, dist, focus+1, toward)
					ss := f.Frame(nil, focus, toward)

					// ⛔ NO PANEL MAY OVERLAP THE ONE BEFORE IT. This is the
					// signature of the wrap: the panels are in chain order, so a
					// panel starting left of where the last one ended is the far
					// side of the band arriving on top of the near side.
					for i := 1; i < len(ss); i++ {
						end := ss[i-1].Dst.X + ss[i-1].Dst.W
						if ss[i].Dst.X < end {
							t.Errorf("%s: screen %d runs to x=%d and screen %d "+
								"starts at x=%d: they overlap by %d",
								where, ss[i-1].Screen+1, end, ss[i].Screen+1,
								ss[i].Dst.X, end-ss[i].Dst.X)
						}
					}

					// ⛔ AND THE CHAIN ITSELF MUST ADMIT A BOUND. A frame can come
					// out looking right because slantOf happened to refuse the
					// wrapped panels, which is luck and not a cap -- so the step to
					// the very first panel is asserted on the chain, where a desk
					// whose every step is already past half a turn would leave no
					// reach for Frame to trim to.
					angleAt, _ := f.chainOf(focus)
					if a := math.Abs(angleAt(1)); a > 180 {
						t.Errorf("%s: one panel along the chain is already %.0f° "+
							"round, so no reach can be inside half a turn", where, a)
					}
				}
			}
		}
	}
}
