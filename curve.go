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

// facet is one flat piece of a screen on the chain.
type facet struct {
	screen       int     // which screen it shows
	srcX0, srcX1 int     // the slice of that screen's source it shows
	hw           float64 // its own half-width, in the chain's world units
	turnBy       float64 // degrees it turns from the facet before it
	gapAfter     float64 // the gap at the hinge AFTER it
}

// facetRing is one turn of the band, cut into facets.
//
// ⭐ A FLAT DESK IS ONE FACET PER SCREEN, and then every number below is the one
// the chain used before facets existed: prefix[j] is j*splay, total is n*splay,
// and the gap is the same at every hinge. That is not a coincidence to be
// grateful for -- it is the property that lets the fold protocol prove this
// change inert.
type facetRing struct {
	f       []facet
	prefix  []float64 // prefix[j] is the turn from facet 0 to facet j
	total   float64   // the turn all the way round
	firstOf []int     // firstOf[s] is the index of screen s's first facet
}

// newFacetRing cuts n screens into facets.
//
// widthOf gives screen s its source width in pixels; hwOf its half-width in
// world units; curveOf the radius it is curved at, as a multiple of the viewing
// distance, or [FlatCurve].
func newFacetRing(n int, splayDeg, gap, fovDeg float64,
	widthOf func(s int) int, hwOf func(s int) float64, curveOf func(s int) float64,
	panelW int) *facetRing {

	r := &facetRing{firstOf: make([]int, n)}
	for s := range n {
		r.firstOf[s] = len(r.f)
		w, hw := widthOf(s), hwOf(s)
		views := 1.0
		if panelW > 0 {
			views = float64(w) / float64(panelW)
		}
		count, deg := curveFacets(views, fovDeg, curveOf(s))
		for i := range count {
			// ⛔ THE FIRST FACET OF A SCREEN TURNS BY THE SPLAY, the rest by the
			// facet angle. The fold between two SCREENS is a different thing
			// from the fold inside one, and collapsing them would make a curved
			// screen sit at the wrong angle to its neighbour.
			turn := deg
			if i == 0 {
				turn = splayDeg
			}
			// ⛔ AND NO GAP INSIDE A SCREEN. The gap is the dark seam between
			// two screens; a facet join is the same picture continuing, and a
			// gap there would be a black line down the middle of a spreadsheet.
			after := 0.0
			if i == count-1 {
				after = gap
			}
			r.f = append(r.f, facet{
				screen: s,
				srcX0:  w * i / count,
				srcX1:  w * (i + 1) / count,
				hw:     hw / float64(count),
				turnBy: turn, gapAfter: after,
			})
		}
	}
	// ⚠ PANEL k IS THE SUM OF THE TURNS FROM 1 TO k, not from 0. Panel 0 sits
	// at angle zero by construction -- slantChain starts there -- so its own
	// turnBy is the fold between it and the panel BEFORE it, which belongs to
	// the turn that closes the ring and not to the walk out from zero. Getting
	// this backwards put every panel one fold short, which the flat-ring test
	// caught as angleAt(1) = 0 where 20 was wanted.
	r.prefix = make([]float64, len(r.f))
	for i := 1; i < len(r.f); i++ {
		r.prefix[i] = r.prefix[i-1] + r.f[i].turnBy
	}
	// The turn all the way round closes on the first facet's own fold.
	r.total = r.prefix[len(r.f)-1] + r.f[0].turnBy
	return r
}

// at is facet k of the chain, which runs on for ever in both directions.
func (r *facetRing) at(k int) facet { return r.f[r.mod(k)] }

// mod is k brought into one turn.
func (r *facetRing) mod(k int) int {
	n := len(r.f)
	return ((k % n) + n) % n
}

// angleAt is how far facet k has turned from facet 0.
//
// ⚠ It takes a float because slantChain's signature does; every value passed is
// whole, the bisector being an average of two of these rather than a half index.
func (r *facetRing) angleAt(k float64) float64 {
	i := int(k)
	n := len(r.f)
	turns := i / n
	if i < 0 && i%n != 0 {
		turns--
	}
	return float64(turns)*r.total + r.prefix[i-turns*n]
}

// gapAt is the gap at the hinge after facet k.
func (r *facetRing) gapAt(k int) float64 { return r.at(k).gapAfter }
