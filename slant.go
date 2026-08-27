// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"math"

	"github.com/go-xrkit/xrkit/stereo"
)

// A Slant is one screen drawn TURNED TOWARDS THE VIEWER: a flat panel rotated
// about the vertical axis, projected.
//
// It is what a desk of monitors looks like. The one in front is square on; the
// ones beside it are angled in, so their far edge is shorter than their near
// edge and their surface is foreshortened. That is not the curvature this
// package deleted -- a curve bows the screen you are looking AT, which argues
// with the depth the glasses already present. A rotation leaves every screen
// flat and only changes which way it faces, which is the difference between a
// bent screen and a turned one.
//
// The shape is a TRAPEZOID and that is not an approximation: a plane rotated
// about the vertical projects to one exactly. Vertical edges stay vertical --
// the rotation does not move a point up or down -- so a source column lands on
// a destination column, and the only things that vary along the width are how
// tall that column is and which source column it reads. Nothing here needs a
// warp table, a matrix per pixel, or a panorama.
//
// At zero rotation the two vertical edges are the same depth, so the height is
// constant and the source columns are evenly spaced: the trapezoid IS the
// rectangle this package drew before, byte for byte. That is asserted rather
// than asserted-in-a-comment; see TestASlantAtZeroDegreesIsTheOldRectangle.
type Slant struct {
	// Screen is the index of the screen on the band. Like a blit's, it is not
	// unique within a frame: a screen straddling the seam comes back twice.
	Screen int
	// Dst is the bounding box in the canvas. Every column of it is written
	// between its own two rows; the rest of the box is left alone.
	Dst stereo.Rect
	// Cols is one entry per column of Dst, in order.
	Cols []SlantCol
}

// SlantCol is one destination column of a slant: which source column it shows,
// and the destination rows it covers.
//
// Rows rather than a scale, because the scale is what a reader would have to
// work out from them and the rows are what the drawing loop wants. Y0 and Y1 are
// absolute canvas rows, so a column can be clipped at the top or the bottom of
// the canvas without the loop knowing what clipping is.
type SlantCol struct {
	// Src is the source column, or -1 for a column with no source at all --
	// which happens where a rotated panel has turned past the edge of the
	// source. Such a column is left as background rather than stretched.
	Src int32
	// Y0 and Y1 are the destination rows this column covers, [Y0, Y1).
	Y0, Y1 int32
}

// MaxSlantDeg is how far a panel may be turned before it is dropped from the
// frame.
//
// Eighty degrees, not ninety. At ninety a panel is edge-on: it projects to a
// line one column wide, and drawing it is a column of stretched pixels that
// reads as a scratch on the picture. The last ten degrees are worth nothing and
// cost a division by a depth approaching zero.
const MaxSlantDeg = 80

// DefaultFOVDeg is the field of view assumed when the headset's is not known.
//
// The plan reports a field of view and does not require one, on purpose: what
// fills the glasses is one source pixel per panel pixel, which needs a
// resolution and nothing else. But the moment screens are TURNED, the angle
// matters -- it decides how much a panel off to the side is stretched by a flat
// projection -- so a number is needed and this is the one.
//
// Forty-five degrees, which is close to what the headsets in the catalogue
// actually have (44.9° for a Luma Ultra, 45.6° for a Beast). Getting it wrong
// does not move the screen in front of the viewer by a pixel; it makes the ones
// beside it slightly more or slightly less stretched than they should be.
const DefaultFOVDeg = 45

// slantChain is where panel j of the chain is, in camera space.
//
// The arrangement is a CHAIN, which is what a desk of monitors is: each panel
// joined to the last one's edge and turned by splayDeg from it. Panel 0 faces the
// viewer squarely at `distance`; panel 1 is hinged on its right edge and angled
// away; panel -1 is hinged on its left. So a splay of nothing is one flat plane
// -- exactly the band this package drew before -- and that is not a coincidence
// to be grateful for, it is the reason this arrangement was chosen over panels
// tangent to a circle. A circle has no zero.
//
// turn is the viewer's own rotation in radians, which is how the scroll gets in:
// the chain is fixed and the head moves, so everything rotates by -turn about the
// origin. hw is the panel's half-width in world units.
//
// The returned edges are the panel's left and right vertical edges, and panelH
// its world height.
func slantChain(j int, splayDeg, hw, distance, turn float64) (lx, lz, rx, rz float64) {
	// Walk out to panel j along the chain, hinge by hinge. Only |j| steps, and a
	// desk is nine screens: the loop is cheaper than the trigonometry it would
	// take to close the form, and it cannot drift from the definition.
	ax, az := -hw, distance
	step := func(k int) (float64, float64) {
		a := rad(float64(k) * splayDeg)
		return 2 * hw * math.Cos(a), -2 * hw * math.Sin(a)
	}
	switch {
	case j > 0:
		for k := range j {
			dx, dz := step(k)
			ax, az = ax+dx, az+dz
		}
	case j < 0:
		for k := j; k < 0; k++ {
			dx, dz := step(k)
			ax, az = ax-dx, az-dz
		}
	}
	dx, dz := step(j)
	bx, bz := ax+dx, az+dz

	// The viewer's rotation, applied to both edges.
	c, s := math.Cos(turn), math.Sin(turn)
	return ax*c - az*s, ax*s + az*c, bx*c - bz*s, bx*s + bz*c
}

// slantOf projects one panel, given its two vertical edges in camera space.
//
// The projection is RECTILINEAR, and that is not a choice: the glasses show a
// flat panel at a fixed distance subtending a fixed angle, so what is drawn into
// it is a flat projection of the world from that panel's centre. Which has a
// consequence worth stating, because it looks like a defect and is not: a screen
// off to the side comes out STRETCHED, wider than square on and taller on its
// outer edge, the way a monitor at the edge of a wide-angle photograph does. The
// intuition that says the far edge should be shorter belongs to human vision,
// which is not a flat panel; an equal-angle mapping would keystone that way and
// be wrong for this one.
//
// ok is false for a panel behind the viewer, turned past [MaxSlantDeg], or
// landing entirely off the canvas. The Cols slice is appended to scratch, so a
// caller drawing every frame keeps one buffer instead of allocating per panel per
// frame -- which means a slant must be DRAWN before the next one is asked for.
func slantOf(scratch []SlantCol, screen int, lx, lz, rx, rz, panelH, f float64,
	viewW, viewH, srcW, srcH int) (Slant, bool) {

	if viewW <= 0 || viewH <= 0 || srcW <= 0 || srcH <= 0 || panelH <= 0 || f <= 0 {
		return Slant{}, false
	}
	// Behind the viewer, or through them: the divisions below would be
	// meaningless and the shape is not on the canvas in any case.
	if lz <= 0 || rz <= 0 {
		return Slant{}, false
	}
	// Edge on. The angle is read off the two depths rather than passed in, so it
	// is the angle of the panel that is actually being projected.
	if turned := math.Abs(math.Atan2(rz-lz, rx-lx)); turned > rad(MaxSlantDeg) &&
		turned < math.Pi-rad(MaxSlantDeg) {
		return Slant{}, false
	}

	x0 := f*lx/lz + float64(viewW)/2
	x1 := f*rx/rz + float64(viewW)/2
	if x1 <= x0 {
		return Slant{}, false
	}
	left, right := int(math.Floor(x0)), int(math.Ceil(x1))
	if right <= 0 || left >= viewW {
		return Slant{}, false
	}

	// Perspective-correct across the width: 1/z is linear in SCREEN x, and so is
	// u/z. Interpolating u itself is the classic mistake -- it slides the texture
	// along the surface as the angle grows, which reads as the picture sliding
	// under the screen rather than the screen turning.
	invZ0, invZ1 := 1/lz, 1/rz
	uOverZ0, uOverZ1 := 0.0, float64(srcW)*invZ1

	out := scratch[:0]
	first, last := max(left, 0), min(right, viewW)
	topMost, botMost := viewH, 0
	for x := first; x < last; x++ {
		t := (float64(x) + 0.5 - x0) / (x1 - x0)
		invZ := invZ0 + t*(invZ1-invZ0)
		u := (uOverZ0 + t*(uOverZ1-uOverZ0)) / invZ
		col := SlantCol{Src: -1}
		if u >= 0 && u < float64(srcW) {
			col.Src = int32(u)
		}
		h := f * panelH * invZ
		y0 := float64(viewH)/2 - h/2
		// Every panel is at eye level, so its projection straddles the middle row
		// of the canvas whatever its height: a column always covers at least that
		// row, and there is no empty column to guard against. A screen one pixel
		// tall comes out one or two rows tall rather than nothing, which is the
		// right answer for a capture that hands over a status strip.
		col.Y0 = int32(max(int(math.Floor(y0)), 0))
		col.Y1 = int32(min(int(math.Ceil(y0+h)), viewH))
		topMost = min(topMost, int(col.Y0))
		botMost = max(botMost, int(col.Y1))
		out = append(out, col)
	}
	// At least one column, always: the two tests above have already established
	// that the projection overlaps the canvas, and an overlap of a rectangle with
	// a rectangle is at least a pixel wide.
	return Slant{
		Screen: screen,
		Dst:    stereo.Rect{X: first, Y: topMost, W: last - first, H: botMost - topMost},
		Cols:   out,
	}, true
}

// slantOptics is the half-width, world height and focal length of a chain of
// screens of this shape seen through a field of view of fovDeg.
//
// The pin is that ONE SCREEN FILLS THE VIEW AT DISTANCE ONE, which is the
// doctrine this package started from and the thing that must not change when the
// screens start turning. Everything else follows from it: the half-width is the
// tangent of half the field of view, the focal length is what maps that to half
// the canvas, and the height is the width times the source's own shape -- so a
// 16:10 screen stays 16:10 and nothing here has to be told an aspect ratio.
func slantOptics(fovDeg float64, viewW, srcW, srcH int) (hw, panelH, f float64) {
	if fovDeg <= 0 || fovDeg >= 180 {
		fovDeg = DefaultFOVDeg
	}
	hw = math.Tan(rad(fovDeg) / 2)
	f = float64(viewW) / 2 / hw
	panelH = 2 * hw * float64(srcH) / float64(srcW)
	return hw, panelH, f
}
