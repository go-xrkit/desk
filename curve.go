// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import "math"

// DefaultCurve is the radius a curved screen takes when none is named: the
// viewing distance itself, which puts every pixel the same distance from the eye.
//
// ⭐ IT IS THE PHYSICAL CASE, not a number chosen to look good. A 49-inch
// ultrawide is about 1200 mm across and sold as 1000R; watched from the 800 mm a
// desk allows, its arc (1200/1000 = 1.2 rad) and the angle it subtends
// (2·atan(600/800) ≈ 1.29 rad) are the same to within a few degrees. Curvature
// radius ≈ viewing distance is what "curved monitor" has always meant.
const DefaultCurve = 1.0

// FlatCurve is what asks for no curve at all, and it is a NAME rather than a
// magic zero because a person reading `curve = 0` in a settings file has no way
// to tell "flat" from "unset".
const FlatCurve = 0.0

// MaxFacetDeg is the most one facet may turn.
//
// ⚠ CHOSEN, like MaxScreens and MaxDistance, and worth saying so. Below about
// two degrees the joins stop being findable on a 1920-pixel panel: a facet
// turning by 2° at the near distance moves its far edge by under a pixel against
// its neighbour's near one. Smaller would be free of complaint and not free of
// cost -- every facet is a panel the fan walks, projects and clips.
const MaxFacetDeg = 2.0

// MaxFacets is the ceiling on how many a screen is cut into.
//
// A screen 3.3 views across curved at the viewing distance subtends about 172°,
// which is 86 facets at [MaxFacetDeg]. The ceiling is well above that and exists
// so a nonsense curve cannot ask for thousands.
const MaxFacets = 256

// curveFacets is how a curved screen is cut: how many facets, and how far each
// turns from the last.
//
// ⛔ THE ARC COMES FROM THE SCREEN'S OWN WIDTH, not from a number in the
// settings. A screen that is 3.3 views across IS 3.3 views across -- at one
// source pixel per panel pixel it subtends 3.3 times the field of view, and
// curving it at the viewing distance wraps exactly that much around the viewer.
// Anything else would be a curve that lies about where the pixels are.
//
// views is the screen's width as a multiple of the view's; fovDeg is what one
// view subtends; curve is the radius as a multiple of the viewing distance, so
// [DefaultCurve] is "every pixel equidistant" and larger is flatter.
//
// ⚠ A FLAT SCREEN COMES BACK AS ONE FACET TURNING BY NOTHING, rather than as a
// special case the caller has to remember. One facet of the whole source at zero
// degrees IS the flat screen, and saying it this way means the flat path and the
// curved one are the same code.
func curveFacets(views, fovDeg, curve float64) (n int, degPerFacet float64) {
	if views <= 0 || fovDeg <= 0 || curve <= FlatCurve || math.IsInf(curve, 0) || math.IsNaN(curve) {
		return 1, 0
	}
	// The whole arc, in degrees: the angle the screen subtends, divided by the
	// radius in units of the viewing distance. At curve 1 they are equal.
	arc := views * fovDeg / curve
	// ⚠ No floor at one: the guard above leaves views, fovDeg and curve all
	// positive, so arc is positive and its ceiling over two is at least one. A
	// guard here would be a branch no test could reach honestly -- a hole in the
	// coverage gate rather than safety.
	n = int(math.Ceil(arc / MaxFacetDeg))
	if n > MaxFacets {
		n = MaxFacets
	}
	return n, arc / float64(n)
}
