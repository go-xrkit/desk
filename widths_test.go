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
func TestHeadYawForSaysWhenAScreenIsWiderThanAHeadCanSweep(t *testing.T) {
	t.Parallel()

	// The Beast's own field of view, so these are the numbers a person reads.
	const fov = 51.57
	for _, c := range []struct {
		px     int
		deg    float64
		beyond bool
	}{
		// A screen the size of the view needs no turn at all: it is already in
		// front of you, which is the whole doctrine of the ordinary desk.
		{1920, 0, false},
		// ⚠ AND A NARROW ONE NEEDS LESS THAN NONE, which has to come back as no
		// turn rather than a negative angle: the band allows a screen a quarter
		// of its height wide, and the arithmetic goes through zero there.
		{1280, 0, false},
		{480, 0, false},
		{3840, 26, false},
		// ⚠ 5120 IS THE EDGE and is deliberately in the table: at 43° it is just
		// past comfortable, so an off-by-one in either direction shows up here
		// rather than as somebody's stiff neck.
		{5120, 43, true},
		{6400, 60, true},
		{8640, 90, true},
	} {
		t.Run("", func(t *testing.T) {
			t.Parallel()

			views := float64(c.px) / 1920
			deg, beyond := HeadYawFor(views, fov)
			if math.Abs(deg-c.deg) > 1 {
				t.Errorf("a screen %d wide (%.2f views) needs %.0f° of head turn, "+
					"want about %.0f", c.px, views, deg, c.deg)
			}
			if beyond != c.beyond {
				t.Errorf("a screen %d wide at %.0f° reads beyondComfort=%v, want "+
					"%v (comfortable is %.0f°)",
					c.px, deg, beyond, c.beyond, ComfortableYawDeg)
			}
		})
	}

	// ⛔ AND THE TWO MUST NOT DRIFT APART. WidthWithinReach is HeadYawFor solved
	// for the width, so a screen exactly that wide has to need exactly a
	// comfortable turn -- and one pixel more has to be beyond it. Two numbers
	// that answer the same question from opposite ends are how a notice comes to
	// name a width that is not the one it checks.
	w := WidthWithinReach(1920, fov)
	if deg, beyond := HeadYawFor(float64(w)/1920, fov); beyond ||
		math.Abs(deg-ComfortableYawDeg) > 1 {
		t.Errorf("WidthWithinReach says %d, and that width needs %.1f° "+
			"(beyond=%v); want about %.0f and within reach",
			w, deg, beyond, ComfortableYawDeg)
	}
	if _, beyond := HeadYawFor(float64(w+200)/1920, fov); !beyond {
		t.Errorf("%d is said to be within reach and %d is not past it",
			w, w+200)
	}

	// Degenerate shapes answer nothing rather than a negative angle.
	for _, c := range [][2]float64{{0, fov}, {3, 0}, {-1, fov}} {
		if deg, beyond := HeadYawFor(c[0], c[1]); deg != 0 || beyond {
			t.Errorf("HeadYawFor(%g, %g) = %g, %v; want 0, false", c[0], c[1], deg, beyond)
		}
	}
	if got := WidthWithinReach(0, fov); got != 0 {
		t.Errorf("WidthWithinReach(0, %g) = %d, want 0", fov, got)
	}
	if got := WidthWithinReach(1920, 0); got != 0 {
		t.Errorf("WidthWithinReach(1920, 0) = %d, want 0", got)
	}
}
