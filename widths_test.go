// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"math"
	"testing"

	"github.com/go-xrkit/xrkit/glasses"
)

// ⛔ THE SENTENCE HAS TO SURVIVE SCREENS OF DIFFERENT WIDTHS, because that is
// now the point: a desk can be one wide screen for a spreadsheet. The old line
// named ONE width and was true only while every screen shared it.
func TestDescribeWidthsSaysWhatWasActuallyMade(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name   string
		widths []int
		want   string
	}{
		{"nothing", nil, "no virtual displays"},
		// The common case still reads as one phrase rather than five.
		{"a ribbon of equals", []int{1920, 1920, 1920, 1920, 1920},
			"5 virtual displays of 1920x1080"},
		{"one wide screen", []int{6400}, "1 virtual display of 6400x1080"},
		// ⭐ The case the change exists for, and the one a person needs to read
		// at a glance: which screen is the wide one.
		{"a wide one and a ribbon", []int{6400, 1920, 1920, 1920},
			"4 virtual displays of 6400x1080 and 3 of 1920x1080"},
		// A width nudged by a pixel is exactly what this must not hide.
		{"one nudged by a pixel", []int{1920, 1921, 1920},
			"3 virtual displays of 1920x1080, 1921x1080 and 1920x1080"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			if got := DescribeWidths(c.widths, 1080); got != c.want {
				t.Errorf("DescribeWidths(%v) =\n  %q\nwant\n  %q", c.widths, got, c.want)
			}
		})
	}
}

// ⛔⛔ A SCREEN WIDER THAN THE VIEW IS THE ONE CASE WHERE THE HEAD IS NOT
// OPTIONAL, which is what this condition decides.
//
// Reported: "le suivi de tete n'etait pas activé et je n'avais pas acces à
// l'icon de xrdesk pour l'activer. j'ai debranché les lunettes pour reprendre le
// focus". Head tracking starts off, its tray row is on the Mac's own menu bar,
// and the desk DIMS that panel by default -- so on a wide screen the one setting
// that reaches the rest of the picture sat behind a dark panel. run_display now
// says the key in the PICTURE, but only when there is something out of reach.
//
// ⚠ "ONLY WHEN" IS THE PART WORTH TESTING. A notice on an ordinary desk, where
// every screen fits the view and the head is a convenience, is a notice nobody
// needs -- and a notice nobody needs is how the next one stops being read.
func TestWiderThanTheViewIsWhatMakesTheHeadNecessary(t *testing.T) {
	t.Parallel()

	beast := glasses.Display{Name: "VITURE Beast", Width: 3840, Height: 1080}
	base, err := NewPlan(beast, Options{Screens: 3})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name  string
		plan  Plan
		want  int
		wider bool
	}{
		{"an ordinary desk, where the head is a convenience", base, 0, false},
		{"one screen wider than the view", base.WithScreenWidth(1, 6400), 6400, true},
		// ⛔ THE WIDEST OF THEM, not the first found: the number goes into the
		// sentence a person reads, and naming the narrower of two wide screens
		// would understate what is out of reach.
		{"two of them, the wider named",
			base.WithScreenWidth(0, 3840).WithScreenWidth(2, 6400), 6400, true},
		{"the wider one first, so order cannot decide it",
			base.WithScreenWidth(0, 6400).WithScreenWidth(2, 3840), 6400, true},
		// ⚠ EXACTLY THE VIEW IS NOT WIDER THAN IT. A screen that fits needs no
		// head, and an off-by-one here would put a notice on every ordinary desk.
		{"a screen exactly the width of the view",
			base.WithScreenWidth(1, base.ScreenW), 0, false},
		// A narrow screen is not wide, however unusual its shape.
		{"a screen narrower than the view", base.WithScreenWidth(1, 1280), 0, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			w, wider := widestBeyondTheView(c.plan)
			if wider != c.wider || w != c.want {
				t.Errorf("widestBeyondTheView = %d, %v; want %d, %v",
					w, wider, c.want, c.wider)
			}
		})
	}
}

// ⛔⛔ A WIDE SCREEN HAS A CEILING THE OPTICS DO NOT SET, and nothing was saying
// so. Reported from inside the glasses, of a screen 6400 across: "l'ecran de
// 6400 est trop grand pour le voir d'un bout a l'autre en tournant la tete".
//
// At one source pixel per panel pixel a screen `views` wide subtends
// views*fovDeg, so reaching its end means turning half that, less the half-view
// already in front of you. The numbers in the table below are the report in
// degrees.
//
// ⭐ AND AMPLIFYING THE HEAD IS NOT THE ANSWER, which the comment on HeadYawFor
// says at length: Desk.headToBand exists to make the picture hold still while
// the head moves, and that was a defect somebody reported and somebody fixed.
// So this measures the ceiling in order to TELL it.
func TestHeadYawForIsTheTurnAWideScreenDemands(t *testing.T) {
	t.Parallel()

	// The Beast's own field of view, so these are the numbers a person reads.
	const fov = 51.57
	for _, c := range []struct {
		px  int
		deg float64
	}{
		// A screen the size of the view needs no turn at all: it is already in
		// front of you, which is the whole doctrine of the ordinary desk.
		{1920, 0},
		// ⚠ AND A NARROW ONE NEEDS LESS THAN NONE, which has to come back as no
		// turn rather than a negative angle: the band allows a screen a quarter
		// of its height wide, and the arithmetic goes through zero there.
		{1280, 0},
		{480, 0},
		{2560, 9},
		{3840, 26},
		{5120, 43},
		{6400, 60},
		{8640, 90},
	} {
		t.Run("", func(t *testing.T) {
			t.Parallel()

			views := float64(c.px) / 1920
			if deg := HeadYawFor(views, fov); math.Abs(deg-c.deg) > 1 {
				t.Errorf("a screen %d wide (%.2f views) needs %.0f° of head turn, "+
					"want about %.0f", c.px, views, deg, c.deg)
			}
		})
	}

	// Degenerate shapes answer nothing rather than a negative angle.
	for _, c := range [][2]float64{{0, fov}, {3, 0}, {-1, fov}} {
		if deg := HeadYawFor(c[0], c[1]); deg != 0 {
			t.Errorf("HeadYawFor(%g, %g) = %g, want 0", c[0], c[1], deg)
		}
	}
}

// ⛔⛔ THE REACH IS THE VIRTUAL CURVE'S RADIUS, said in degrees of head turn, and
// it is a SETTING because nobody can measure a feeling for somebody else.
//
// Asked for of the flat band -- "on peut cintrer un tout petit peu plus?" -- and
// then, when I reached for the pixel-bending kind: "je parle toujours de
// cintrage virtuel, pas de deformation". There is no curvature to increase on a
// flat band: every part of it is square on when it is looked at, which is what
// keeps it at one source pixel per panel pixel. What "more curve" means there is
// a TIGHTER CYLINDER -- the same head turn sweeping more screen -- which is the
// gain, and the reach is the number that sets it.
func TestTheReachIsTheVirtualCurvesRadius(t *testing.T) {
	t.Parallel()

	p, err := NewPlan(glasses.Display{Name: "VITURE Beast", Width: 3840, Height: 1080},
		Options{Screens: 1})
	if err != nil {
		t.Fatal(err)
	}

	// Nobody said: the default, which came from three widths worn and refused.
	if got := p.ReachDeg(); got != ComfortableYawDeg {
		t.Errorf("a plan nobody set a reach on answers %g, want %g",
			got, ComfortableYawDeg)
	}
	// And zero or less puts it back, so a caller clearing it gets the default
	// rather than a division by nothing.
	for _, deg := range []float64{0, -1, -1000} {
		if got := p.WithReach(30).WithReach(deg).ReachDeg(); got != ComfortableYawDeg {
			t.Errorf("WithReach(%g) answers %g, want the default %g",
				deg, got, ComfortableYawDeg)
		}
	}
	// ⚠ AND IT IS CLAMPED, NOT REFUSED. Below a degree the gain runs away: a
	// tenth of a degree on a wide screen is a gain of hundreds, and a head that
	// twitches would throw the picture across the band.
	for _, deg := range []float64{MinReachDeg - 0.1, 0.1, 1} {
		if got := p.WithReach(deg).ReachDeg(); got != MinReachDeg {
			t.Errorf("WithReach(%g) answers %g, want the floor %g",
				deg, got, MinReachDeg)
		}
	}
	if got := p.WithReach(30).ReachDeg(); got != 30 {
		t.Errorf("WithReach(30) answers %g", got)
	}

	// ⭐ AND A SMALLER REACH IS A TIGHTER CYLINDER, which is the whole point: the
	// same screen, the same pixels, a shorter turn of the head to cross it. The
	// gain is asserted through a desk rather than arithmetic, because the gain is
	// what the renderer actually asks for.
	wide := WidePlan(p, 6400)
	for _, c := range []struct {
		reach, gain float64
	}{
		{30, 2.01}, {20, 3.01}, {15, 4.01}, {10, 6.01},
	} {
		q := wide.WithReach(c.reach)
		d, err := New(q, []Feed{&shapedFeed{w: q.ScreenWidth(0), h: q.ScreenH}})
		if err != nil {
			t.Fatal(err)
		}
		d.Render() // a plan does not know a screen is wide until a source arrives
		d.mu.Lock()
		got := d.headGain()
		d.mu.Unlock()
		if math.Abs(got-c.gain) > 0.02 {
			t.Errorf("a reach of %g° gives a gain of %.2f, want about %.2f",
				c.reach, got, c.gain)
		}
		d.Close()
	}
}
