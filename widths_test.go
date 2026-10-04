// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
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
