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

	// USB is what the headset says about itself over the bus, when the caller
	// has looked. It may be nil.
	//
	// It is STRONGER evidence than the display name and is consulted first. A
	// display name is whatever the panel puts in its EDID, and a dock, a
	// capture card or a KVM in the path can replace it with something generic;
	// a USB product id names a model, and for some brands it is the only thing
	// that does. On these very glasses the bus says "XREAL 1S" while the
	// display is not up at all.
	USB *glasses.USB

	// MarginDeg is how much wider than the view the panorama is kept, so that a
	// scroll in progress has pixels to reveal rather than an edge. It is spent
	// on both sides.
	MarginDeg float64
}

// DefaultScreens is how many virtual screens a desk gets when nobody says.
//
// Six: two rows of three in the gallery, which is a shape a person keeps a map
// of, and enough screens that the band is worth turning. It used to be "as
// many as fit round the circle", which was a real limit when the screens were
// curved and is a fiction now that they are flat.
const DefaultScreens = 6

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

	// How says what named the model, so a caller can show whether the answer
	// came from the bus, from the display, or from the person at the keyboard.
	How glasses.How

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
	p, how := glasses.IdentifyDevice(d.Name, opts.USB)
	ok := how != glasses.NotIdentified
	switch {
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
		How:          how,
		ScreenW:      eyeW,
		ScreenH:      eyeH,
		Stereoscopic: stereoscopic,
		HFOVDeg:      h,
		VFOVDeg:      v,
	}

	n := opts.Screens
	if n == 0 {
		n = DefaultScreens
	}
	plan.count = n

	// The screens are FLAT, so the angles are a scroll coordinate and nothing
	// more — which is what lets there be as many of them as a person wants.
	//
	// A curved band had to fit in 360°, and at one screen per view that is seven
	// of them and no more. Flat, the circle is a fiction: the yaw says how far
	// along the band the viewer is, and the band is however long it needs to be.
	// So n screens are spread over the full turn whatever n is, and three, six or
	// nine of them fold into a gallery a person can actually keep a map of.
	// A hair under the full turn. The screens and their gaps must SUM to less
	// than 360°, and n pitches of exactly 360/n sum to exactly 360 — which the
	// placement refuses, correctly, for a band that is meant to close. A
	// millionth of a degree of slack is 0.00003 pixels across an eleven-thousand
	// pixel band: it settles the comparison and is not a position anyone could
	// measure.
	const turnDeg = 360 - 1e-6
	pitchDeg := turnDeg / float64(n)
	gapDeg := pitchDeg * DefaultGapPx / float64(eyeW+DefaultGapPx)
	plan.Layout = ribbon.Layout{
		// DensityDeg is the arc for one width of a SQUARE screen, and a wider
		// screen gets proportionally more, so dividing by the aspect is what
		// makes a screen of the eye's own shape span the whole pitch bar the gap.
		DensityDeg:   (pitchDeg - gapDeg) / aspect,
		GapDeg:       gapDeg,
		FullWidthDeg: pitchDeg - gapDeg,
		Arrangement:  ribbon.Packed,
	}

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
