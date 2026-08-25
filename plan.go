// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

// Package desk shows several captured computer screens floating on a 360°
// ribbon inside AR glasses, scrolled from the keyboard.
package desk

import (
	"errors"
	"fmt"
	"math"

	"github.com/go-xrkit/xrkit/glasses"
	"github.com/go-xrkit/xrkit/projection"
	"github.com/go-xrkit/xrkit/ribbon"
)

// Errors a plan can refuse with.
var (
	// ErrUnknownOptics means the headset was recognised but its field of view is
	// not in the catalogue. Guessing one is not an option: a wrong field of view
	// renders everything, in the wrong place, without any symptom.
	ErrUnknownOptics = errors.New("desk: the field of view of these glasses is not known")
	// ErrScreens means the requested number of screens is not usable.
	ErrScreens = errors.New("desk: unusable number of screens")
	// ErrMargin means the requested margin is not usable.
	ErrMargin = errors.New("desk: unusable margin")
	// ErrFOV means the field of view given is not one.
	ErrFOV = errors.New("desk: unusable field of view")
)

// Options are the choices a person makes.
type Options struct {
	// Screens is how many virtual screens to put on the ribbon. Zero asks for
	// as many as fit round the circle without overlapping.
	Screens int

	// FOVDeg overrides the catalogue's horizontal field of view, in degrees.
	// It exists because the catalogue is honest about what it does not know, and
	// because a person who measures their own optics should be able to say so.
	FOVDeg float64

	// MarginDeg is how much wider than the view the panorama is kept, so that a
	// scroll in progress has pixels to reveal rather than an edge. It is spent
	// on both sides.
	MarginDeg float64
}

// DefaultMarginDeg is a margin wide enough that a scroll never reaches the edge
// of the panorama between two frames at any speed the navigator will produce.
const DefaultMarginDeg = 12

// Plan is everything the renderer needs, worked out from the headset itself.
//
// The rule that shapes all of it is the one the viewer asked for: ONE SCREEN IS
// ONE FULL VIEW. A virtual screen is created at exactly one eye's resolution,
// and given exactly the arc that eye can see, so that looking straight at a
// screen shows it edge to edge, at one source pixel per output pixel.
type Plan struct {
	// Model is the headset this was worked out for.
	Model string

	// ScreenW and ScreenH are the pixel size to create each virtual display at:
	// one eye's viewport, which is the most the glasses can show at once.
	ScreenW, ScreenH int

	// Stereoscopic reports whether the display is in a side-by-side 3D mode, in
	// which case the two eyes get different pixels of the same frame.
	Stereoscopic bool

	// HFOVDeg and VFOVDeg are one eye's field of view, in degrees.
	HFOVDeg, VFOVDeg float64

	// Layout places the screens. DensityDeg is derived so that a screen of the
	// eye's own shape spans exactly HFOVDeg.
	Layout ribbon.Layout

	// Pano is the panorama buffer the screens are composited into, sized so that
	// a screen at rest reads one source pixel per destination pixel.
	Pano ribbon.Pano

	// count is how many screens the ribbon carries.
	count int
}

// String renders the plan the way a person would want it logged.
func (p Plan) String() string {
	return fmt.Sprintf("%s: %d screens of %dx%d, %.2f°x%.2f° each, panorama %dx%d over %.0f°x%.0f°",
		p.Model, p.count, p.ScreenW, p.ScreenH,
		p.HFOVDeg, p.VFOVDeg, p.Pano.W, p.Pano.H,
		p.Pano.Window.HSpanDeg, p.Pano.Window.VSpanDeg)
}

// Screens is the ribbon's screens, all identical because each one is a whole
// view of the glasses.
func (p Plan) Screens() []ribbon.Screen {
	out := make([]ribbon.Screen, p.count)
	for i := range out {
		out[i] = ribbon.Screen{ID: fmt.Sprintf("screen-%d", i+1), W: p.ScreenW, H: p.ScreenH}
	}
	return out
}

// NewPlan works out how to fill these glasses.
func NewPlan(d glasses.Display, opts Options) (Plan, error) {
	if opts.MarginDeg < 0 || opts.MarginDeg >= 90 {
		return Plan{}, fmt.Errorf("%w: %g°", ErrMargin, opts.MarginDeg)
	}
	margin := opts.MarginDeg
	if margin == 0 {
		margin = DefaultMarginDeg
	}
	if opts.Screens < 0 {
		return Plan{}, fmt.Errorf("%w: %d", ErrScreens, opts.Screens)
	}

	stereoscopic, eyeW, eyeH := glasses.StereoMode(d.Width, d.Height)
	if eyeW <= 0 || eyeH <= 0 {
		return Plan{}, fmt.Errorf("%w: %s has no usable size", ErrScreens, d)
	}
	aspect := float64(eyeW) / float64(eyeH)

	model := d.Name
	var h, v float64
	switch p, ok := glasses.Identify(d.Name); {
	case opts.FOVDeg > 0:
		// The viewer's own figure wins, and the vertical follows from the shape
		// of the eye rather than from any catalogue.
		if opts.FOVDeg >= 180 {
			return Plan{}, fmt.Errorf("%w: %g°", ErrFOV, opts.FOVDeg)
		}
		h = opts.FOVDeg
		v = deg(2 * math.Atan(math.Tan(rad(h)/2)/aspect))
		if ok {
			model = p.Model
		}
	case ok && p.Known():
		model = p.Model
		h, v, _ = p.FOV(aspect)
	case ok:
		return Plan{}, fmt.Errorf("%w: %s — pass a field of view to say what it is",
			ErrUnknownOptics, p.Model)
	default:
		return Plan{}, fmt.Errorf("%w: %s is not a headset this knows — pass a field of view",
			ErrUnknownOptics, d)
	}

	plan := Plan{
		Model:        model,
		ScreenW:      eyeW,
		ScreenH:      eyeH,
		Stereoscopic: stereoscopic,
		HFOVDeg:      h,
		VFOVDeg:      v,
	}

	// DensityDeg is the arc for one width of a SQUARE screen, and a wider screen
	// gets proportionally more. So dividing by the aspect is what makes a screen
	// of the eye's own shape span exactly the eye's own field of view.
	plan.Layout = ribbon.Layout{
		DensityDeg:   h / aspect,
		GapDeg:       0,
		FullWidthDeg: h,
		Arrangement:  ribbon.Spread,
	}

	n := opts.Screens
	if n == 0 {
		// At least two always fit, because a field of view is under 180°, so this
		// needs no floor of its own.
		n = int(360 / h)
	}
	plan.count = n

	// The panorama covers the view plus a margin on each side, and carries
	// enough pixels that a screen at rest is neither stretched nor shrunk.
	// The horizontal window needs no clamp: a field of view is under 180° and a
	// margin under 90°, so the span cannot reach a full circle. The vertical one
	// does, because a tall field of view plus a generous margin can exceed the
	// half-turn from pole to pole, and a window past the poles has no meaning.
	hSpan := h + 2*margin
	vSpan := v + 2*margin
	if vSpan > 180 {
		vSpan = 180
	}
	plan.Pano = ribbon.Pano{
		W:      int(math.Ceil(float64(eyeW) * hSpan / h)),
		H:      int(math.Ceil(float64(eyeH) * vSpan / v)),
		Window: projection.Projection{Kind: projection.Equirect, HSpanDeg: hSpan, VSpanDeg: vSpan},
	}
	return plan, nil
}

func rad(d float64) float64 { return d * math.Pi / 180 }
func deg(r float64) float64 { return r * 180 / math.Pi }

// Count is how many screens the ribbon carries.
func (p Plan) Count() int { return p.count }
