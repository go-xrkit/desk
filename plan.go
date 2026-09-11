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
	"strconv"
	"strings"

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

	// DamagedName is the name this display ARRIVED with, when it arrived
	// damaged and the catalogue entry had to be recovered from what survived.
	// Empty in every ordinary case.
	//
	// ⛔⛔ THE DAMAGE USED TO COST THE OPTICS, SILENTLY. A long-lived Go string
	// in this program loses its first four bytes now and then -- eight times
	// since 2026-09-06, roughly four in a hundred runs, and nothing measurable
	// separates a run that corrupts from one that does not (see
	// damagedname.go). When the victim is the display's name, the catalogue no
	// longer recognises it, HFOVDeg stays zero, and [slantOptics] quietly
	// substitutes [DefaultFOVDeg]:
	//
	//	before: VITURE Beast: ... 50.35°x32.74° each, curved 60.0° at 3.00x
	//	after:      RE Beast: ... field of view not known, curved 60.0° at 3.00x
	//
	// The field of view fixes hw = tan(fov/2), so 45° in place of 50.35° stretches
	// EVERYTHING off-axis by the wrong factor -- the neighbouring panels and the
	// folds between them. A rendering defect whose only other symptom is half a
	// line in a log that `open` throws away.
	//
	// ⭐ SO IT IS RECOVERED AND SAID, never one without the other. Recovering in
	// silence would turn a visible fault into an invisible one and nobody would
	// collect another instance; [Plan.String] carries this into the note beside
	// every capture, which is a timestamped trace nobody has to be present for.
	DamagedName string

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
	// ⛔⛔ AND IT SAYS ITS SHAPE. This used to name the model, the count, the
	// size and the optics -- everything except the two numbers that decide
	// what the picture LOOKS like. So "the angle is still wrong", reported
	// from the glasses, could not be answered without rebuilding the
	// program: nothing anywhere said which curvature was in force. A line
	// that describes an arrangement has to describe the arrangement.
	shape := fmt.Sprintf(", curved %.1f° at %.2fx", p.SplayDeg(), p.Distance())
	if p.SplayDeg() <= 0 {
		shape = fmt.Sprintf(", flat at %.2fx", p.Distance())
	}
	// ⛔⛔ AND IT NAMES THE SCREENS THAT ARE NOT THAT SIZE. "6 screens of
	// 1920x1080" was written beside a photograph of a desk with a 3840 on it --
	// an Odyssey G95NC mirrored onto position 1 -- and three defects lived
	// behind that sentence for as long as it was believed. A description that
	// reports the NOMINAL shape of a mixed desk is not a description of it.
	var odd []string
	for i := range p.count {
		if w := p.ScreenWidth(i); w != p.ScreenW {
			odd = append(odd, fmt.Sprintf("%d is %dx%d", i+1, w, p.ScreenH))
		}
	}
	if len(odd) > 0 {
		shape += " (screen " + strings.Join(odd, ", screen ") + ")"
	}
	// ⛔⛔ AND IT SAYS WHEN THE NAME ARRIVED DAMAGED. Recovering in silence would
	// turn a visible fault into an invisible one, and this line is written into
	// the note beside every capture -- a timestamped trace of a fault that
	// happens about four times in a hundred runs and that nobody has ever been
	// present for. See [Plan.DamagedName].
	if p.DamagedName != "" {
		shape += ", name arrived DAMAGED as " + strconv.Quote(p.DamagedName)
	}
	return fmt.Sprintf("%s: %d screens of %dx%d, %s%s",
		p.Model, p.count, p.ScreenW, p.ScreenH, optics, shape)
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
	// ⛔⛔ AND THE BAND IS RE-DERIVED, or it goes on believing every screen is
	// the shape of the eye. Without this a wide panel took more arc than the
	// layout had budgeted and ribbon.Place refused the whole desk:
	// "418.5° of screens and gaps". See withBandLayout.
	return p.withBandLayout()
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

	// ⛔⛔ A NAME WITH A NUL IN IT IS NOT A NAME, AND IT COST THE OPTICS. See
	// [Plan.DamagedName]: the same heap corruption that damagedname.go recovers
	// from when LOOKING FOR the display also reaches the name the plan is built
	// from, and there it went unnoticed -- the catalogue missed, the field of
	// view stayed unknown, and the geometry silently used 45°.
	//
	// ⭐ THE SAME PREDICATE AND THE SAME STRICTNESS, against the catalogue
	// instead of the attached displays: undamage repairs only a name carrying a
	// NUL, and only when exactly one candidate of the same length ends the same
	// way. Two candidates is a guess, and a guess here is a headset's optics
	// wrapped around somebody's desk.
	//
	// ⛔ IT DOES NOT TOUCH THE USB EVIDENCE. What is repaired is the STRING; the
	// repaired name then goes through IdentifyDevice like any other. The bus
	// still says which headsets are attached and not which display is which one
	// -- the distinction EvidenceFor exists to keep, and which must not be
	// traded away to work around a rare fault.
	var damaged string
	if !ok {
		if models := glasses.Models(); len(models) > 0 {
			if i, repaired := undamage(d.Name, models); repaired {
				if q, qhow := glasses.IdentifyDevice(models[i], opts.USB); qhow != glasses.NotIdentified {
					damaged, model = d.Name, models[i]
					p, how, ok = q, qhow, true
				}
			}
		}
	}
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
		DamagedName:  damaged,
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
		// ⛔⛔ THE DERIVED ANGLE WAS TRIED HERE AND TAKEN BACK OUT, at the request
		// of the person wearing it: "en gros il faut remettre ce qu'on avait avant
		// que je signale ce problème".
		//
		// [Plan.FacingSplayDeg] is the angle at which each neighbour EXACTLY faces
		// the viewer -- 51.6° on a Beast -- and it is therefore the STRONGEST
		// curvature that has a geometric meaning, not the most comfortable one.
		// Worn, it reads as a deep crease with heavily stretched outer edges,
		// because a rectilinear projection magnifies everything off-axis: a screen
		// 1920 pixels wide square-on came out 917.
		//
		// So the default is a chosen twenty degrees again, and the derivation
		// stays available as a method for anybody who wants the strong version.
		// A number that is principled is not thereby pleasant, and the person
		// looking through the glasses is the only instrument for that.
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
	return p.withBandLayout()
}

// withBandLayout is the plan with its [ribbon.Layout] derived from the screens
// it ACTUALLY has, so the band closes whatever shapes are on it.
//
// ⛔⛔ THE DENSITY USED TO COME FROM THE GLASSES' OWN ASPECT, AND A DESK IN USE
// PROVED THAT WRONG. [ribbon.Place] gives screen i an arc of DensityDeg times
// ITS aspect — which is right, and is what keeps the pixels square — so the sum
// only comes to a turn when every screen is the shape of the eye. Mirror a wide
// panel onto a ribbon position, which [Desk.fit] does through
// [Plan.WithScreenWidth], and the band overflows:
//
//	six 1920x1080              360.0°   fits
//	one 2560 among five 1920   379.5°   ribbon: screens do not fit in 360°
//	one 3440 among five 1920   406.3°
//	one 3840 among five 1920   418.5°   ← reported from a real desk
//	one 5120 among five 1920   457.6°
//
// The 418.5° was reported as a CURVATURE failure, because pressing "more
// curved" is what rebuilt the band and surfaced it. Curvature had nothing to do
// with it: the screen was an Odyssey G95NC at 3840x1080.
//
// ⭐ SO THE ARC IS SHARED OUT AMONG THE SHAPES THERE ARE. A screen twice as wide
// takes twice the arc — which is what square pixels mean — and the others give
// it up, rather than the band refusing to exist. For screens that are all the
// eye's shape this is the old formula exactly: the sum is n times the one
// aspect, and (turn − n·gap)/(n·aspect) is (pitch − gap)/aspect.
func (p Plan) withBandLayout() Plan {
	n := p.count
	if n < 1 || p.ScreenW <= 0 || p.ScreenH <= 0 {
		return p
	}
	// A hair under the full turn. The screens and their gaps must SUM to less
	// than 360°, and n pitches of exactly 360/n sum to exactly 360 — which the
	// placement refuses, correctly, for a band that is meant to close. A
	// millionth of a degree of slack is 0.00003 pixels across an eleven-thousand
	// pixel band: it settles the comparison and is not a position anyone could
	// measure.
	const turnDeg = 360 - 1e-6
	pitchDeg := turnDeg / float64(n)
	// The shapes actually on the band, which is the sum ribbon.Place will take.
	var shapes float64
	for i := range n {
		shapes += float64(p.ScreenWidth(i)) / float64(p.ScreenH)
	}
	// ⛔⛔ AND THE GAP IS AN ARC OF THE BAND THERE IS, NOT OF THE ONE THERE WOULD
	// BE IF EVERY SCREEN WERE THE EYE'S SHAPE. It used to be
	// pitchDeg*gap/(ScreenW+gap), which is this expression with
	// n*(ScreenW+gap) substituted for the band -- true only when every screen is
	// nominal. With an Odyssey G95NC on the band (3840 among three 1920) the
	// arcs still summed to a turn, so ribbon.Place shared a band 20% too short
	// out among them and EVERY screen came out 1536 pixels wide instead of 1920.
	// The wide one was not the only casualty; it was just the cause.
	//
	// Written this way, screen i's width comes back as exactly ScreenWidth(i)
	// and every gap as exactly DefaultGapPx, whatever the mix. See
	// TestTheBandIsAsLongAsItsScreens.
	gapDeg := turnDeg * float64(DefaultGapPx) / float64(p.BandPx())
	p.Layout = ribbon.Layout{
		// DensityDeg is the arc for one width of a SQUARE screen, and a wider
		// screen gets proportionally more.
		DensityDeg:   (turnDeg - gapDeg*float64(n)) / shapes,
		GapDeg:       gapDeg,
		FullWidthDeg: pitchDeg - gapDeg,
		Arrangement:  ribbon.Packed,
	}
	return p
}

// BandPx is how long the whole band is in pixels at distance 1: every screen's
// own width, plus one gap each.
//
// ⛔ ONE GAP PER SCREEN, NOT ONE PER PAIR, because the band closes: the extra
// gap is the one across the seam, and [ribbon.Place] counts it the same way.
//
// It is a method rather than an expression in two places because the layout and
// the flat renderer must agree about it exactly. They did not: the layout
// derived its gap from a nominal band and [Strip] was built with a nominal
// length, so a desk with one wide screen scaled every screen down.
func (p Plan) BandPx() int {
	band := 0
	for i := range p.count {
		band += p.ScreenWidth(i) + DefaultGapPx
	}
	return band
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

// DefaultSplayDeg is the angle a desk gets when its optics are unknown: twenty
// degrees.
//
// ⛔⛔ ITS OWN DOCUMENTATION USED TO CLAIM "the neighbours face you", AND THEY DO
// NOT. Measured on a VITURE Beast, six screens, 51.57° per eye: at twenty
// degrees the neighbour misses facing the viewer by 28.3°. It does not look
// turned towards anybody -- it leans away, which is what "l'angle de cintrage
// n'est pas au bon endroit" turned out to mean once there was a picture to look
// at rather than a question to answer.
//
//	splay  the neighbour misses facing you by
//	    0                             -44.0°
//	   20 (this constant)             -28.3°
//	   40                             -11.1°
//	 51.6                               0.0°   ← [Plan.FacingSplayDeg]
//	   60                              +8.8°
//
// So it is a FALLBACK now and not a default: it is what a plan gets when there
// is no field of view to derive from. See [Plan.FacingSplayDeg], which is what a
// plan with known optics gets instead.
const DefaultSplayDeg = 20.0

// FacingSplayDeg is the angle at which the neighbouring screens FACE the viewer
// -- a desk of monitors, each turned towards the chair.
//
// ⛔⛔ NOT ALL AT ONE DISTANCE, which is what this line used to say and what
// three days of hunting were spent believing. Measured here, six screens at
// 51.57° per eye: even at this angle the panel edges sit between 0.667 and
// 1.110, a 66% spread. The panels tile edge to edge, so the chain is a polygon
// sized by how wide a screen is, and the viewer is not at its centre.
//
// ⭐ WITH TILING PANELS YOU MAY HAVE ANY TWO OF {a free curvature dial, tiling,
// equal distance} AND NEVER ALL THREE. The dial and the tiling are the two
// kept -- decided once the spread had been measured rather than assumed -- so
// this angle is where each screen FACES you, not where they are equidistant.
//
// ⭐ IT IS A FIXED POINT, and that is the whole derivation: a panel faces the
// viewer when the angle it is turned by equals the angle it SITS at. Walking
// s ← angle-to-panel-1(s) converges in a handful of steps, because the angle a
// panel sits at barely moves with the splay (44° to 51.5° across the whole
// range) while the splay it is compared against sweeps sixty degrees.
//
// ⛔ AND IT DEPENDS ON THE OPTICS, which is why it could never be a constant. It
// is a function of the field of view and of how far the band is pushed back, so
// a headset with a wider eye and a desk pushed further away want different
// numbers. Twenty degrees was chosen once and applied to everything.
//
// A plan with no field of view returns [DefaultSplayDeg]: there is nothing to
// derive from, and a fallback that is honest about being one is better than a
// number invented from a default FOV.
func (p Plan) FacingSplayDeg() float64 {
	if p.HFOVDeg <= 0 || p.ScreenW <= 0 || p.ScreenH <= 0 {
		return DefaultSplayDeg
	}
	hw, _, _ := slantOptics(p.HFOVDeg, p.ScreenW, p.ScreenW, p.ScreenH)
	gap := slantGap(hw, p.ScreenW)
	d := p.Distance()
	s := 0.0
	for range 24 {
		// Where the centre of panel 1 is: the fold, then the panel's own extent
		// along its own direction, with half the gap in each panel's plane --
		// which is exactly what [slantChain] walks. ⛔ THE GAP BELONGS IN THE
		// FIXED POINT TOO: left out, the derived angle missed facing the viewer
		// by 0.84°, and only the test that pins the derivation caught it.
		a := rad(s)
		next := deg(math.Atan2(
			hw*(1+math.Cos(a))+gap*math.Cos(a/2),
			d-hw*math.Sin(a)-gap*math.Sin(a/2)))
		if math.Abs(next-s) < 1e-9 {
			s = next
			break
		}
		s = next
	}
	// ⛔ ONLY THE CEILING IS GUARDED, because only the ceiling can happen: the
	// numerator is hw(1+cos s), which is never negative, so the arc-tangent lands
	// in [0°, 180°] and a floor of zero would be a branch no test could reach. A
	// wide eye at a close distance really does ask for more than MaxSplayDeg, and
	// gets it clamped like every other angle in this package.
	if s > MaxSplayDeg {
		return MaxSplayDeg
	}
	return s
}

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
