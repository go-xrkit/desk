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
	"github.com/go-xrkit/xrkit/ribbon"
)

// Errors a plan can refuse with.
var (
	// ErrScreens means the requested number of screens is not usable.
	ErrScreens = errors.New("desk: unusable number of screens")
	// ErrFOV means the field of view given is not one.
	ErrFOV = errors.New("desk: unusable field of view")
)

// Options are the choices a person makes.
type Options struct {
	// Screens is how many virtual screens to put on the ribbon. Zero asks for
	// as many as fit round the circle without overlapping.
	Screens int

	// Distance is how far the band sits from the viewer, as a multiple of the
	// distance at which one screen fills the view. Zero, like one, is the near
	// end. See [Plan.Distance].
	Distance float64

	// SplayDeg is the angle between one screen and the next, in degrees. A
	// NEGATIVE value asks for the flat band; zero asks for [DefaultSplayDeg],
	// because zero is what a caller that has not thought about it passes.
	SplayDeg float64

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
}

// DefaultScreens is how many virtual screens a desk gets when nobody says.
//
// Six: two rows of three in the gallery, which is a shape a person keeps a map
// of, and enough screens that the band is worth turning. It used to be "as
// many as fit round the circle", which was a real limit when the screens were
// curved and is a fiction now that they are flat.
const DefaultScreens = 6

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

	// count is how many screens the ribbon carries.
	count int

	// distance is how far the band sits from the viewer, as a multiple of the
	// distance at which one screen fills the view. See [Plan.Distance].
	distance float64

	// splayDeg is the angle between one screen and the next. See [Plan.SplayDeg].
	splayDeg float64

	// widths is the pixel WIDTH of each screen, when one of them is not the
	// shape of the glasses. Nil, and every entry of zero, mean ScreenW.
	//
	// A screen of the band is a whole view of the glasses, so they are all one
	// shape -- until a position MIRRORS a display this program did not make.
	// This Mac's own panel is 2056x1329, a ratio of 1.547 against the band's
	// 1.778, and it has no 16:9 mode to be put into: its 60 modes are 1.547 and
	// 1.600 and nothing else, which was measured before this field existed. Fit
	// it into a screen of the band's shape and there is an empty band 124 pixels
	// wide down each side of it.
	//
	// So the SCREEN takes the shape instead. Same height as its neighbours,
	// narrower, and the ribbon already knew how to place that: a screen's span
	// on the circle comes from its aspect ratio, which is what keeps the pixels
	// square.
	widths []int
}

// String renders the plan the way a person would want it logged.
//
// A field of view of zero is not printed as "0.00°": it means NOT KNOWN, and a
// number is a claim.
func (p Plan) String() string {
	optics := "field of view not known"
	if p.HFOVDeg > 0 {
		optics = fmt.Sprintf("%.2f°x%.2f° each", p.HFOVDeg, p.VFOVDeg)
	}
	return fmt.Sprintf("%s: %d screens of %dx%d, %s",
		p.Model, p.count, p.ScreenW, p.ScreenH, optics)
}

// Screens is the ribbon's screens: each one a whole view of the glasses, except
// any that has been given a shape of its own by [Plan.WithScreenWidth].
func (p Plan) Screens() []ribbon.Screen {
	out := make([]ribbon.Screen, p.count)
	for i := range out {
		out[i] = ribbon.Screen{ID: fmt.Sprintf("screen-%d", i+1), W: p.ScreenWidth(i), H: p.ScreenH}
	}
	return out
}

// ScreenWidth is how wide screen i is, which is [Plan.ScreenW] unless it has
// been given a shape of its own.
func (p Plan) ScreenWidth(i int) int {
	if i < 0 || i >= len(p.widths) || p.widths[i] <= 0 {
		return p.ScreenW
	}
	return p.widths[i]
}

// WithScreenWidth returns the plan with screen i that many pixels wide, at the
// same height as every other screen. A width of zero or less puts it back to
// the shape of the glasses.
//
// The plan is copied, widths and all: a plan handed out and then changed under
// its holder is a band and a navigator disagreeing about where a screen is.
func (p Plan) WithScreenWidth(i, w int) Plan {
	if i < 0 || i >= p.count {
		return p
	}
	widths := make([]int, p.count)
	copy(widths, p.widths)
	if w >= p.ScreenH*MinAspectNum/MinAspectDen && w <= p.ScreenH*MaxAspectNum/MaxAspectDen {
		widths[i] = w
	} else {
		widths[i] = 0
	}
	p.widths = widths
	return p
}

// The shapes a screen is allowed to take, as fractions of its height: from a
// tall panel on its side to an ultrawide. A source outside that is not a shape
// somebody chose, it is a capture that has gone wrong, and giving the band a
// screen a hundred times too wide would leave nothing else visible.
const (
	MinAspectNum, MinAspectDen = 1, 4
	MaxAspectNum, MaxAspectDen = 8, 1
)

// NewPlan works out how to fill these glasses.
func NewPlan(d glasses.Display, opts Options) (Plan, error) {
	if opts.Screens < 0 {
		return Plan{}, fmt.Errorf("%w: %d", ErrScreens, opts.Screens)
	}
	if opts.Screens > MaxScreens {
		// Clamped, not refused: a caller composing a plan in code asked for a
		// desk, and the nearest legal desk is one. A number that came from a
		// PERSON is refused instead, where their settings are read, so the
		// ceiling is named to whoever chose it.
		opts.Screens = MaxScreens
	}

	stereoscopic, eyeW, eyeH := glasses.StereoMode(d.Width, d.Height)
	if eyeW <= 0 || eyeH <= 0 {
		return Plan{}, fmt.Errorf("%w: %s has no usable size", ErrScreens, d)
	}
	aspect := float64(eyeW) / float64(eyeH)

	// The field of view is REPORTED, not required.
	//
	// It used to decide the layout: a screen was given the arc the eye could
	// see, so a headset whose optics nobody had measured could not be planned
	// for at all, and this refused rather than guess. Flat, the arc is a scroll
	// coordinate and what makes a screen fill the glasses is that it is drawn at
	// one source pixel per panel pixel — which needs the panel's RESOLUTION and
	// nothing else. So an unknown headset is now a headset with an unknown
	// field of view, and it works.
	//
	// The catalogue still earns its place: it names the model, which is what a
	// person needs to see to know the right thing is being driven, and the
	// figure is worth printing where it is known. It simply no longer decides
	// whether the application runs.
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
		model = p.Model
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
	plan = plan.WithScreens(n)
	if opts.Distance > 0 {
		plan = plan.WithDistance(opts.Distance)
	}
	switch {
	case opts.SplayDeg < 0:
		plan = plan.WithSplay(0)
	default:
		plan = plan.WithSplay(opts.SplayDeg)
	case opts.SplayDeg == 0:
		plan = plan.WithSplay(DefaultSplayDeg)
	}

	// There is no panorama any more, and so nothing here to size. The screens
	// are composited straight into the view: the buffer they go into is the
	// picture, and the picture is one screen.
	return plan, nil
}

func rad(d float64) float64 { return d * math.Pi / 180 }
func deg(r float64) float64 { return r * 180 / math.Pi }

// Count is how many screens the ribbon carries.
func (p Plan) Count() int { return p.count }

// WithScreens is this plan with a different number of screens on the band,
// clamped to 1..[MaxScreens].
//
// The screens are FLAT, so the angles are a scroll coordinate and nothing more,
// which is what lets one be added while the desk is running.
//
// A curved band had to fit in 360 degrees, and at one screen per view that was
// seven of them and no more. Flat, the circle is a fiction: the yaw says how far
// along the band the viewer is, and the band is however long it needs to be. So
// n screens are spread over the full turn whatever n is -- and the ceiling is
// therefore a DECISION, not a consequence. See [MaxScreens].
func (p Plan) WithScreens(n int) Plan {
	if n < 1 {
		n = 1
	}
	if n > MaxScreens {
		n = MaxScreens
	}
	p.count = n
	aspect := float64(p.ScreenW) / float64(p.ScreenH)

	// A hair under the full turn. The screens and their gaps must SUM to less
	// than 360°, and n pitches of exactly 360/n sum to exactly 360 — which the
	// placement refuses, correctly, for a band that is meant to close. A
	// millionth of a degree of slack is 0.00003 pixels across an eleven-thousand
	// pixel band: it settles the comparison and is not a position anyone could
	// measure.
	const turnDeg = 360 - 1e-6
	pitchDeg := turnDeg / float64(n)
	gapDeg := pitchDeg * DefaultGapPx / float64(p.ScreenW+DefaultGapPx)
	p.Layout = ribbon.Layout{
		// DensityDeg is the arc for one width of a SQUARE screen, and a wider
		// screen gets proportionally more, so dividing by the aspect is what
		// makes a screen of the eye's own shape span the whole pitch bar the gap.
		DensityDeg:   (pitchDeg - gapDeg) / aspect,
		GapDeg:       gapDeg,
		FullWidthDeg: pitchDeg - gapDeg,
		Arrangement:  ribbon.Packed,
	}
	return p
}

// MaxScreens is the most screens a desk carries: nine.
//
// It is an ARBITRARY ceiling, chosen rather than derived, and that is worth
// saying plainly because nothing in this package supplies one. The screens are
// flat, so the band is as long as it needs to be; the plan would spread forty of
// them over the turn without complaint, and a test used to.
//
// Nine is where three things happen to agree:
//
//   - the gallery is [DefaultColumns] wide, so nine fills three rows of three
//     exactly, with nothing ragged and every screen keeping its column as the
//     desk grows;
//   - a screen costs a display to create and a stream to capture, and that cost
//     is linear — on macOS it is a CGVirtualDisplay each;
//   - past nine, a person stops holding a map of where things are, which is the
//     whole point of a fixed arrangement.
//
// A number asked for above it is CLAMPED rather than refused when it comes from
// the geometry ([Plan.WithScreens]), and REFUSED with the ceiling named when it
// comes from a person's settings file — a clamp is right for a program composing
// a plan and wrong for a line somebody wrote and expects to mean something.
const MaxScreens = 9

// Distance is how far the band sits from the viewer, as a multiple of the
// distance at which one screen fills the view exactly.
//
// One is the near end and the doctrine this started from: a screen is the whole
// view, at one source pixel per panel pixel, and its neighbours are off to the
// sides where the head has to turn to reach them. Two puts each screen in half
// the width, so the two beside it are visible without turning. That is the whole
// of what "further away" means here -- a screen does not move, it takes up less
// room, which is what moving a monitor back does.
//
// It is deliberately not allowed below one. Closer than filling the view means
// seeing PART of a screen, and a desk whose middle screen is cropped is not a
// desk anyone asked for.
func (p Plan) Distance() float64 {
	if p.distance < 1 {
		return 1
	}
	return p.distance
}

// MinDistance is the near end: one screen filling the view exactly, at one
// source pixel per output pixel.
//
// It is the largest a screen can be shown in a given pair of glasses, which is
// what [ActionFit] returns to. Closer than this means seeing part of a screen,
// so the scale stops here rather than continuing into a crop.
const MinDistance = 1.0

// MaxDistance is the far end: four screens across the view.
//
// A float, because it is compared with and assigned to a distance and an
// untyped integer constant next to a float64 is a conversion waiting to be
// forgotten.
//
// Chosen, like [MaxScreens], and for the same kind of reason rather than a
// geometric one. At four, a 1920-pixel screen is 480 pixels of a 1920-pixel
// view: text on it is a texture, not words. Somebody who wants the whole desk at
// once has the gallery, which draws every screen at a readable size instead of
// pretending nine of them fit in one view.
const MaxDistance = 4.0

// WithDistance is this plan seen from a different distance, clamped to
// 1..[MaxDistance].
//
// Nothing about the screens changes: the band is the same ring of the same
// panels at the same resolution. What changes is the number of pixels each one
// occupies in the view, which is the pixel scale of the whole band -- so this is
// one multiplication in one place, and the navigator, the gallery and the
// captures know nothing about it.
func (p Plan) WithDistance(d float64) Plan {
	switch {
	case d < 1:
		d = 1
	case d > MaxDistance:
		d = MaxDistance
	}
	p.distance = d
	return p
}

// DistanceStep is how much one press moves the band.
//
// A quarter, so the near end and the far end are twelve presses apart: enough
// that a person can stop where they want, few enough that they can get there.
const DistanceStep = 0.25

// SplayDeg is the angle between one screen and the next, in degrees.
//
// Zero is the flat band: every screen in one plane, square on, which is what
// this package drew before there was an angle at all and still the right answer
// for somebody who wants a single wide surface. Anything more turns each screen
// towards the viewer, the way the two beside the middle one on a desk of three
// monitors are turned.
//
// It is an angle between NEIGHBOURS and not a total arc, so it means the same
// thing whatever the screen count -- which is the point: three screens at twenty
// degrees and nine at twenty degrees have the same feel in front of you and
// differ in how far round the rest of them go.
//
// The field cannot be negative -- [Plan.WithSplay] clamps at nothing and it is
// the only way in -- so there is no guard here to read past. A plan's zero value
// is the flat band, which is the right thing for it to be.
func (p Plan) SplayDeg() float64 { return p.splayDeg }

// MaxSplayDeg is the widest angle between neighbours.
//
// Sixty. Past it a chain of nine screens wraps round past the viewer's own
// shoulders and the far ones come back into shot from behind, which is not a desk
// -- and a panel turned that far is mostly edge, so its pixels are a smear
// whatever else is true. Chosen, like every other end in this package, and said
// so rather than derived.
const MaxSplayDeg = 60.0

// DefaultSplayDeg is the angle a desk gets when nobody says: twenty degrees.
//
// Enough to read as turned -- the keystone is visible, the neighbours face you --
// and shallow enough that a screen two along is still squarely in front of you
// when you turn to it. It is also close to what people set real monitors to,
// which is the only evidence available for a number like this.
const DefaultSplayDeg = 20.0

// SplayStep is how much one press changes the angle: five degrees, so the whole
// range is twelve presses and each one is visible.
const SplayStep = 5.0

// WithSplay is this plan with a different angle between neighbours, clamped to
// 0..[MaxSplayDeg].
//
// Like the distance, it changes nothing about the screens themselves -- same
// count, same resolution, same order. It changes which way each one faces.
func (p Plan) WithSplay(deg float64) Plan {
	switch {
	case deg < 0:
		deg = 0
	case deg > MaxSplayDeg:
		deg = MaxSplayDeg
	}
	p.splayDeg = deg
	return p
}

// sameShapes says whether two plans give their screens the same widths.
func (p Plan) sameShapes(q Plan) bool {
	if p.count != q.count || p.ScreenW != q.ScreenW {
		return false
	}
	for i := 0; i < p.count; i++ {
		if p.ScreenWidth(i) != q.ScreenWidth(i) {
			return false
		}
	}
	return true
}
