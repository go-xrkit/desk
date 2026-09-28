// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"math"
	"testing"
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
		curve float64
	}{
		{"asked to be flat", 3.3, 51.57, FlatCurve},
		{"a negative radius", 3.3, 51.57, -1},
		{"an infinite one, which is what flat MEANS geometrically", 3.3, 51.57, math.Inf(1)},
		{"not a number", 3.3, 51.57, math.NaN()},
		{"a screen of no width", 0, 51.57, DefaultCurve},
		{"glasses that report no field of view", 3.3, 0, DefaultCurve},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			n, deg := curveFacets(c.views, c.fov, c.curve)
			if n != 1 || deg != 0 {
				t.Errorf("curveFacets(%g, %g, %g) = %d facets of %g°, want 1 of 0",
					c.views, c.fov, c.curve, n, deg)
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
		views, curve, wantArc float64
	}{
		// 6400 pixels in a 1920 panel: 3.33 views.
		{6400.0 / 1920, DefaultCurve, 3.3333 * fov},
		{5120.0 / 1920, DefaultCurve, 2.6667 * fov},
		// One view across is one field of view, whatever else is true.
		{1, DefaultCurve, fov},
		// ⭐ Twice the radius is half the arc: a flatter screen wraps less.
		{6400.0 / 1920, 2 * DefaultCurve, 3.3333 * fov / 2},
	} {
		n, deg := curveFacets(c.views, fov, c.curve)
		if got := float64(n) * deg; math.Abs(got-c.wantArc) > 0.01 {
			t.Errorf("%g views at curve %g wrap %g°, want %g°",
				c.views, c.curve, got, c.wantArc)
		}
		// ⚠ And no facet turns more than the ceiling, which is what keeps the
		// joins from being findable.
		if deg > MaxFacetDeg+1e-9 {
			t.Errorf("%g views at curve %g: a facet turns %g°, more than %g",
				c.views, c.curve, deg, MaxFacetDeg)
		}
		if n < 1 || n > MaxFacets {
			t.Errorf("%g views at curve %g: %d facets", c.views, c.curve, n)
		}
	}
}

// ⚠ AND A NONSENSE CURVE CANNOT ASK FOR THOUSANDS. The ceiling is not there for
// a caller acting in good faith; it is there because this reads a number a
// person typed.
func TestAnAbsurdCurveIsCapped(t *testing.T) {
	t.Parallel()

	n, deg := curveFacets(3.3, 51.57, 1e-6)
	if n != MaxFacets {
		t.Errorf("a radius of a millionth gave %d facets, want the ceiling %d", n, MaxFacets)
	}
	if n < 1 || deg <= 0 {
		t.Errorf("%d facets of %g°", n, deg)
	}
}
