// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/go-xrkit/xrkit/glasses"
	"github.com/go-xrkit/xrkit/ribbon"
)

// TestOneScreenIsOneView is the requirement the whole plan exists to satisfy:
// looking straight at a screen must show it edge to edge, filling the glasses.
//
// It is checked by PLACING the screens with the layout the plan produced and
// measuring the arc a screen actually got — not by re-deriving the arithmetic,
// which would only prove the test and the code agree.
func TestOneScreenIsOneView(t *testing.T) {
	for _, d := range []glasses.Display{
		{Name: "VITURE Beast", Width: 3840, Height: 1080},
		{Name: "VITURE Beast", Width: 1920, Height: 1200},
		{Name: "VITURE Luma Ultra", Width: 3840, Height: 1080},
		{Name: "XREAL 1S", Width: 3840, Height: 1080},
		{Name: "XREAL 1S", Width: 1920, Height: 1200},
	} {
		p, err := NewPlan(d, Options{})
		if err != nil {
			t.Fatalf("%s: NewPlan = %v", d, err)
		}
		r, err := ribbon.Place(p.Screens(), p.Layout)
		if err != nil {
			t.Fatalf("%s: Place = %v", d, err)
		}
		// One screen is one full view, in PIXELS. It used to be stated in
		// degrees — a screen's arc equalling the eye's field of view — and that
		// was the right way to say it while the screens were curved. Flat, the
		// angles are a scroll coordinate: what makes a screen fill the glasses
		// is that it is drawn at one source pixel per panel pixel, and the arc
		// only has to be the same for every screen so the band is even.
		strip, err := NewStrip(placedOf(r), p.Count()*(p.ScreenW+DefaultGapPx),
			p.ScreenW, p.ScreenH, p.ScreenW, p.ScreenH)
		if err != nil {
			t.Fatalf("%s: NewStrip = %v", d, err)
		}
		for i := 0; i < p.Count(); i++ {
			if got := strip.width[i]; got != p.ScreenW {
				t.Errorf("%s: screen %d is %d pixels wide on the band, want the view's %d",
					d, i, got, p.ScreenW)
			}
		}
		// And the screen created must be exactly one eye's worth of pixels,
		// which is what "the most the glasses can show" means.
		_, eyeW, eyeH := glasses.StereoMode(d.Width, d.Height)
		if p.ScreenW != eyeW || p.ScreenH != eyeH {
			t.Errorf("%s: screens are %dx%d, want one eye's %dx%d",
				d, p.ScreenW, p.ScreenH, eyeW, eyeH)
		}
	}
}

func TestScreenCount(t *testing.T) {
	d := glasses.Display{Name: "VITURE Beast", Width: 3840, Height: 1080}

	// Asking for none means "as many as fit", which for a 51.57° view is six.
	p, err := NewPlan(d, Options{})
	if err != nil {
		t.Fatalf("NewPlan = %v", err)
	}
	if p.Count() != 6 {
		t.Errorf("Count() = %d, want 6 screens of 51.57° round a circle", p.Count())
	}
	if len(p.Screens()) != p.Count() {
		t.Errorf("Screens() gave %d, Count() says %d", len(p.Screens()), p.Count())
	}
	// They must all be distinguishable and all the same size.
	seen := map[string]bool{}
	for _, s := range p.Screens() {
		if seen[s.ID] {
			t.Errorf("duplicate screen id %q", s.ID)
		}
		seen[s.ID] = true
		if s.W != p.ScreenW || s.H != p.ScreenH {
			t.Errorf("screen %q is %dx%d, want %dx%d", s.ID, s.W, s.H, p.ScreenW, p.ScreenH)
		}
	}

	// And an explicit count is honoured.
	p, err = NewPlan(d, Options{Screens: 3})
	if err != nil {
		t.Fatalf("NewPlan = %v", err)
	}
	if p.Count() != 3 {
		t.Errorf("Count() = %d, want the 3 that were asked for", p.Count())
	}
}

// TestWideOpticsStillGetTwoScreens pins down why the automatic count needs no
// floor: a field of view is under 180°, so even the widest optics leave room
// for two screens round the circle.
func TestWideOpticsStillGetTwoScreens(t *testing.T) {
	p, err := NewPlan(glasses.Display{Name: "VITURE Beast", Width: 1920, Height: 1080},
		Options{FOVDeg: 179.9})
	if err != nil {
		t.Fatalf("NewPlan = %v", err)
	}
	if p.Count() < 2 {
		t.Errorf("Count() = %d; two screens always fit", p.Count())
	}
}

// TestOverrideWins: the catalogue is honest about what it does not know, so a
// person who measured their own optics must be able to say so — including for
// glasses the catalogue has never heard of.
func TestOverrideWins(t *testing.T) {
	p, err := NewPlan(glasses.Display{Name: "Some Prototype", Width: 3840, Height: 1080},
		Options{FOVDeg: 40})
	if err != nil {
		t.Fatalf("NewPlan = %v", err)
	}
	if math.Abs(p.HFOVDeg-40) > 1e-9 {
		t.Errorf("HFOVDeg = %g, want the 40° that was given", p.HFOVDeg)
	}
	// The vertical must follow from the eye's shape, not from a catalogue.
	want := deg(2 * math.Atan(math.Tan(rad(40)/2)/(1920.0/1080)))
	if math.Abs(p.VFOVDeg-want) > 1e-9 {
		t.Errorf("VFOVDeg = %g, want %g derived from the eye's aspect", p.VFOVDeg, want)
	}
	if p.Model != "Some Prototype" {
		t.Errorf("Model = %q, want the display's own name", p.Model)
	}

	// On a headset the catalogue DOES know, the override still wins but the
	// model keeps its proper name.
	p, err = NewPlan(glasses.Display{Name: "VITURE Beast", Width: 3840, Height: 1080},
		Options{FOVDeg: 40})
	if err != nil {
		t.Fatalf("NewPlan = %v", err)
	}
	if math.Abs(p.HFOVDeg-40) > 1e-9 {
		t.Errorf("HFOVDeg = %g, want the override to win over the catalogue", p.HFOVDeg)
	}
	if p.Model != "VITURE Beast" {
		t.Errorf("Model = %q, want the catalogue's name", p.Model)
	}
}

// TestUnknownOpticsAreNoLongerAnObstacle.
//
// This used to be the refusal that mattered most: a wrong field of view
// renders everything, in the wrong place, with no symptom, so a headset nobody
// had measured could not be planned for at all.
//
// Flat screens took the field of view out of the geometry. What makes a screen
// fill the glasses is that it is drawn at one source pixel per panel pixel,
// which needs the panel's RESOLUTION and nothing else. So these now work — and
// they say so honestly: a model where the catalogue knows one, the display's
// own name where it does not, and no field of view either way, meaning "not
// known" rather than "zero degrees".
func TestUnknownOpticsAreNoLongerAnObstacle(t *testing.T) {
	for _, tc := range []struct {
		name      string
		d         glasses.Display
		wantModel string
	}{
		{"a headset with no published figure",
			glasses.Display{Name: "VITURE Pro 2", Width: 3840, Height: 1080}, "VITURE Pro 2"},
		{"not a headset at all",
			glasses.Display{Name: "Odyssey G95NC", Width: 7680, Height: 2160}, "Odyssey G95NC"},
		{"a display that names only a brand",
			glasses.Display{Name: "VITURE", Width: 1920, Height: 1200}, "VITURE glasses"},
	} {
		p, err := NewPlan(tc.d, Options{})
		if err != nil {
			t.Errorf("%s: NewPlan = %v", tc.name, err)
			continue
		}
		if p.Model != tc.wantModel {
			t.Errorf("%s: model is %q, want %q", tc.name, p.Model, tc.wantModel)
		}
		if p.HFOVDeg != 0 {
			t.Errorf("%s: a field of view of %g° was invented", tc.name, p.HFOVDeg)
		}
		// And it draws: every screen exactly one view wide, which is the rule
		// that replaced the one the field of view used to carry.
		d, err := New(p, feedsFor(p))
		if err != nil {
			t.Errorf("%s: New = %v", tc.name, err)
			continue
		}
		for i := 0; i < p.Count(); i++ {
			if got := d.strip.width[i]; got != p.ScreenW {
				t.Errorf("%s: screen %d is %d wide, want the view's %d",
					tc.name, i, got, p.ScreenW)
			}
		}
		d.Close()
	}
}

func TestRefusesNonsense(t *testing.T) {
	beastUSB := glasses.Display{Name: "VITURE Beast", Width: 3840, Height: 1080}
	for _, tc := range []struct {
		name string
		d    glasses.Display
		opts Options
		want error
	}{
		{"negative screens", beastUSB, Options{Screens: -1}, ErrScreens},
		{"a display with no size", glasses.Display{Name: "VITURE Beast"}, Options{}, ErrScreens},
		{"a field of view that is not one", beastUSB, Options{FOVDeg: 180}, ErrFOV},
	} {
		_, err := NewPlan(tc.d, tc.opts)
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", tc.name, err, tc.want)
		}
	}
}

func TestPlanString(t *testing.T) {
	p, err := NewPlan(glasses.Display{Name: "VITURE Beast", Width: 3840, Height: 1080}, Options{})
	if err != nil {
		t.Fatalf("NewPlan = %v", err)
	}
	s := p.String()
	for _, want := range []string{"VITURE Beast", "6 screens", "1920x1080", "51.57"} {
		if !strings.Contains(s, want) {
			t.Errorf("String() = %q, missing %q", s, want)
		}
	}
}

func TestStereoscopicIsReported(t *testing.T) {
	p, _ := NewPlan(glasses.Display{Name: "VITURE Beast", Width: 3840, Height: 1080}, Options{})
	if !p.Stereoscopic {
		t.Error("a 3840x1080 mode is side-by-side 3D and must be reported as such")
	}
	p, _ = NewPlan(glasses.Display{Name: "VITURE Beast", Width: 1920, Height: 1200}, Options{})
	if p.Stereoscopic {
		t.Error("a 1920x1200 mode is one eye, not two")
	}
}

// TestTheBusNamesTheModelWhenTheDisplayDoesNot.
//
// A display name is whatever the panel put in its EDID, and a dock, a capture
// card or a KVM in the path can replace it with something generic. A USB
// product id names a model. These are the real numbers off a pair of XREAL 1S:
// vendor 0x3318, product 0x043e, product string "XREAL 1S", read from the bus
// while the video link was down.
func TestTheBusNamesTheModelWhenTheDisplayDoesNot(t *testing.T) {
	generic := glasses.Display{Name: "DisplayPort", Width: 1920, Height: 1200}
	real := &glasses.USB{Vendor: 0x3318, Product: 0x043e, Name: "XREAL 1S"}

	// Without the bus, the display names nothing: the desk still runs, and says
	// so — the model is the string the panel gave and the field of view is not
	// known.
	blind, err := NewPlan(generic, Options{})
	if err != nil {
		t.Fatalf("without the bus: %v", err)
	}
	if blind.Model != "DisplayPort" || blind.HFOVDeg != 0 {
		t.Errorf("without the bus: model %q, %g°; want the panel's own name and no figure",
			blind.Model, blind.HFOVDeg)
	}

	p, err := NewPlan(generic, Options{USB: real})
	if err != nil {
		t.Fatalf("with the bus: %v", err)
	}
	if p.Model != "XREAL 1S" {
		t.Errorf("model is %q, want the one the bus named", p.Model)
	}
	if p.How != glasses.ByUSBProduct {
		t.Errorf("How is %v, want %v", p.How, glasses.ByUSBProduct)
	}
	// 52° on the diagonal of a 16:10 eye is 44.94° across. If this ever reads
	// 52, something has started treating a diagonal as a horizontal.
	if got := p.HFOVDeg; math.Abs(got-44.94) > 0.01 {
		t.Errorf("horizontal field of view is %.2f°, want 44.94°", got)
	}
}

// TestTheBusOutranksTheDisplayName: both are present and they disagree. The
// product id names one model; the display name is a different, real headset.
func TestTheBusOutranksTheDisplayName(t *testing.T) {
	p, err := NewPlan(
		glasses.Display{Name: "XREAL Air 2", Width: 1920, Height: 1200},
		Options{USB: &glasses.USB{Vendor: 0x3318, Product: 0x043e, Name: "XREAL 1S"}})
	if err != nil {
		t.Fatalf("NewPlan = %v", err)
	}
	if p.Model != "XREAL 1S" || p.How != glasses.ByUSBProduct {
		t.Errorf("got %q by %v; the bus should outrank the display name", p.Model, p.How)
	}
}

// TestTheViewersOwnFigureStillWins: -fov is a person saying what they measured,
// and no amount of evidence about the model overrides it.
func TestTheViewersOwnFigureStillWins(t *testing.T) {
	p, err := NewPlan(
		glasses.Display{Name: "DisplayPort", Width: 1920, Height: 1200},
		Options{FOVDeg: 40, USB: &glasses.USB{Vendor: 0x3318, Product: 0x043e, Name: "XREAL 1S"}})
	if err != nil {
		t.Fatalf("NewPlan = %v", err)
	}
	if p.HFOVDeg != 40 {
		t.Errorf("horizontal field of view is %v, want the 40° that was asked for", p.HFOVDeg)
	}
	// The model still comes from the bus: the person gave an angle, not a name.
	if p.Model != "XREAL 1S" {
		t.Errorf("model is %q, want the one the bus named", p.Model)
	}
}

// placedOf is the ribbon's screens, as the strip wants them.
func placedOf(r *ribbon.Ribbon) []ribbon.Placed {
	out := make([]ribbon.Placed, r.Len())
	for i := range out {
		out[i] = r.At(i)
	}
	return out
}

// TestPlanDistance: what "further away" is, and both ends of it.
//
// The plan is where the distance lives because it is the pixel SCALE of the
// band and nothing else -- the screens, their number and their resolution are
// untouched, so the navigator, the gallery and the captures know nothing about
// it. Which is also why one multiplication in build() is the whole of the
// feature at this stage.
func TestPlanDistance(t *testing.T) {
	p, err := NewPlan(glasses.Display{Name: "VITURE Beast", Width: 3840, Height: 1080},
		Options{Screens: 6})
	if err != nil {
		t.Fatal(err)
	}
	// The near end by default: one screen fills the view, which is where this
	// started and what the doctrine says.
	if got := p.Distance(); got != 1 {
		t.Errorf("a fresh plan is at %g", got)
	}

	for _, c := range []struct {
		asked, want float64
	}{
		{0, 1}, {-3, 1}, {0.5, 1}, // below the near end is the near end
		{1, 1}, {2, 2}, {MaxDistance, MaxDistance},
		{MaxDistance + 1, MaxDistance}, {1000, MaxDistance},
	} {
		if got := p.WithDistance(c.asked).Distance(); got != c.want {
			t.Errorf("WithDistance(%g) = %g, want %g", c.asked, got, c.want)
		}
	}

	// Nothing else about the plan moves: same screens, same size, same count.
	far := p.WithDistance(MaxDistance)
	if far.Count() != p.Count() || far.ScreenW != p.ScreenW || far.ScreenH != p.ScreenH {
		t.Errorf("the distance changed the screens: %d of %dx%d became %d of %dx%d",
			p.Count(), p.ScreenW, p.ScreenH, far.Count(), far.ScreenW, far.ScreenH)
	}
	if far.Layout != p.Layout {
		t.Error("the distance changed where the screens are on the band")
	}

	// Options carries it in, so a caller that composes a plan once gets the
	// distance a person chose without a second call.
	q, err := NewPlan(glasses.Display{Name: "VITURE Beast", Width: 3840, Height: 1080},
		Options{Screens: 6, Distance: 2})
	if err != nil {
		t.Fatal(err)
	}
	if got := q.Distance(); got != 2 {
		t.Errorf("Options.Distance gave %g", got)
	}

	// The step divides the range: a person can reach the far end and stop
	// anywhere on the way, which is what makes it usable from a key.
	if steps := (MaxDistance - 1) / DistanceStep; steps != float64(int(steps)) {
		t.Errorf("%g steps of %g do not land on the far end %g",
			steps, DistanceStep, MaxDistance)
	}
}

// TestPlanSplay: the angle between neighbours, both ends of it, and what a
// caller who has not thought about it gets.
//
// The last part is the interesting one. Zero from Options means "I did not say",
// because zero is what a zero value is, and the useful answer to that is the
// default rather than the flat band. Asking for flat is therefore a NEGATIVE,
// which is odd to look at and unambiguous to write -- and the alternative was a
// pointer in a struct that is otherwise all values.
func TestPlanSplay(t *testing.T) {
	p, err := NewPlan(glasses.Display{Name: "VITURE Beast", Width: 3840, Height: 1080},
		Options{Screens: 6})
	if err != nil {
		t.Fatal(err)
	}
	// ⛔⛔ THIS HAS BEEN BOTH WAYS AND THE WEARER SETTLED IT. It demanded
	// DefaultSplayDeg, then for a while the DERIVED angle -- Plan.FacingSplayDeg,
	// the curvature at which each neighbour exactly faces you, 51.6° on these
	// optics. That is the strongest curvature with a geometric meaning, which is
	// not the same as a comfortable one: worn, it reads as a deep crease, and a
	// screen 1920 pixels wide square-on came out 917. Reported as "en gros il
	// faut remettre ce qu'on avait avant que je signale ce problème".
	//
	// So a plan nobody splayed is twenty degrees, a chosen number, and the
	// derivation stays a method for whoever wants the strong version.
	if p.SplayDeg() != DefaultSplayDeg {
		t.Errorf("a plan nobody splayed is %g, want %g", p.SplayDeg(), DefaultSplayDeg)
	}
	// ⭐ AND THE DERIVATION IS STILL NOT THE FALLBACK IN DISGUISE: it is exercised
	// by TestTheDerivedSplayMakesTheNeighbourFaceYou, and a derivation that
	// quietly gave up and returned the constant would make that test pass having
	// proved nothing.
	if p.FacingSplayDeg() == DefaultSplayDeg {
		t.Errorf("the derived angle is exactly the fallback %g; nothing was derived",
			DefaultSplayDeg)
	}

	for _, c := range []struct {
		asked, want float64
	}{
		{-1, 0}, {-90, 0}, {0, 0},
		{SplayStep, SplayStep}, {DefaultSplayDeg, DefaultSplayDeg},
		{MaxSplayDeg, MaxSplayDeg}, {MaxSplayDeg + 1, MaxSplayDeg}, {1000, MaxSplayDeg},
	} {
		if got := p.WithSplay(c.asked).SplayDeg(); got != c.want {
			t.Errorf("WithSplay(%g) = %g, want %g", c.asked, got, c.want)
		}
	}

	// A negative asked for through Options is the flat band; zero is the default.
	flat, err := NewPlan(glasses.Display{Name: "VITURE Beast", Width: 3840, Height: 1080},
		Options{Screens: 6, SplayDeg: -1})
	if err != nil {
		t.Fatal(err)
	}
	if got := flat.SplayDeg(); got != 0 {
		t.Errorf("Options.SplayDeg = -1 gave %g, want the flat band", got)
	}

	// Nothing else about the plan moves: the angle is which way the screens face
	// and not what they are.
	turned := p.WithSplay(MaxSplayDeg)
	if turned.Count() != p.Count() || turned.ScreenW != p.ScreenW ||
		turned.ScreenH != p.ScreenH || turned.Layout != p.Layout ||
		turned.Distance() != p.Distance() {
		t.Error("the splay changed something other than the angle")
	}

	// The step divides the range, so a person can reach both ends and stop
	// anywhere between them.
	if steps := MaxSplayDeg / SplayStep; steps != float64(int(steps)) {
		t.Errorf("%g steps of %g do not land on %g", steps, SplayStep, MaxSplayDeg)
	}
}

// TestAWidePanelDoesNotBreakTheBand.
//
// ⛔⛔ THIS IS THE 418.5°. ribbon.Place gives screen i an arc of DensityDeg
// times ITS aspect, and the density was derived from the GLASSES' aspect — so
// the sum only came to a turn when every screen was the shape of the eye.
// Mirror a wide panel onto a ribbon position, which Desk.fit does through
// WithScreenWidth, and the whole desk was refused:
//
//	desk: placing 6 screens: ribbon: screens do not fit in 360°:
//	418.5° of screens and gaps
//
// Reported as a CURVATURE failure, because pressing "more curved" is what
// rebuilt the band and surfaced it. The screen was an Odyssey G95NC at
// 3840x1080, and 3840 is the width that lands on 418.5° exactly.
func TestAWidePanelDoesNotBreakTheBand(t *testing.T) {
	p := Plan{ScreenW: 1920, ScreenH: 1080}.WithScreens(6)
	if _, err := ribbon.Place(p.Screens(), p.Layout); err != nil {
		t.Fatalf("six screens of the eye's own shape: %v", err)
	}
	// Every shape WithScreenWidth accepts, from a panel on its side to the
	// widest ultrawide, on a band that is already full.
	for _, w := range []int{270, 1920, 2560, 3440, 3840, 5120, 8640} {
		q := p.WithScreenWidth(2, w)
		if got := q.ScreenWidth(2); got != w {
			t.Fatalf("WithScreenWidth(2, %d) left it %d; the case is not being tested", w, got)
		}
		r, err := ribbon.Place(q.Screens(), q.Layout)
		if err != nil {
			t.Errorf("one screen %d wide among five 1920: %v", w, err)
			continue
		}
		if r.Len() != q.Count() {
			t.Errorf("one screen %d wide: %d placed, want %d", w, r.Len(), q.Count())
		}
	}
}

// ⭐ AND THE UNIFORM CASE IS UNCHANGED, to the last decimal. Sharing the arc
// among the shapes there are reduces to the old formula when every shape is the
// same: the sum is n times one aspect, and (turn − n·gap)/(n·aspect) is
// (pitch − gap)/aspect. A regression here would move every screen on every
// desk, which is not a thing to discover from a photograph.
func TestTheUniformBandIsUnmoved(t *testing.T) {
	for _, n := range []int{1, 2, 4, 6, 9} {
		p := Plan{ScreenW: 1920, ScreenH: 1080}.WithScreens(n)
		const turnDeg = 360 - 1e-6
		pitch := turnDeg / float64(n)
		gap := pitch * DefaultGapPx / float64(1920+DefaultGapPx)
		want := (pitch - gap) / (1920.0 / 1080.0)
		if got := p.Layout.DensityDeg; math.Abs(got-want) > 1e-9 {
			t.Errorf("%d screens: DensityDeg %.12f, want %.12f", n, got, want)
		}
		if got := p.Layout.GapDeg; math.Abs(got-gap) > 1e-9 {
			t.Errorf("%d screens: GapDeg %.12f, want %.12f", n, got, gap)
		}
	}
}

// TestTheDerivedSplayMakesTheNeighbourFaceYou.
//
// ⛔⛔ THE OLD DEFAULT DID NOT, AND ITS OWN DOCUMENTATION CLAIMED IT DID:
// "Enough to read as turned -- the keystone is visible, the neighbours face
// you". Measured on a VITURE Beast, six screens, 51.57° per eye, the neighbour
// misses facing the viewer by 28.3° at twenty degrees. Reported from the glasses
// as "le pli n'est toujours pas bien situé", and settled by rendering the fold
// with synthetic screens and looking at it: at 20° the two panels lean the SAME
// way, at the derived angle they make the symmetric V a desk of monitors makes.
//
// ⭐ FACING IS A GEOMETRIC TEST, NOT A JUDGEMENT: a panel faces the viewer when
// its surface is perpendicular to the line of sight to its own centre. That is
// what this measures, and it is why the fix needed nobody to squint at a
// headset.
func TestTheDerivedSplayMakesTheNeighbourFaceYou(t *testing.T) {
	for _, c := range []struct {
		fov  float64
		n    int
		dist float64
	}{
		{51.57, 6, 1}, {51.57, 6, 2}, {45.6, 6, 1}, {51.57, 9, 1}, {40, 4, 1.5},
	} {
		p := Plan{ScreenW: 1920, ScreenH: 1080, HFOVDeg: c.fov}.
			WithScreens(c.n).WithDistance(c.dist).WithSplay(0)
		s := p.FacingSplayDeg()
		p = p.WithSplay(s)
		hw, _, _ := slantOptics(p.HFOVDeg, p.ScreenW, p.ScreenW, p.ScreenH)
		lx, lz, rx, rz := slantChain(1, p.SplayDeg(), sameWidth(hw), gapOf(p), p.Distance(), 0)
		sight := math.Atan2((lx+rx)/2, (lz+rz)/2)
		surf := math.Atan2(rx-lx, rz-lz)
		if miss := deg(surf-sight) - 90; math.Abs(miss) > 0.05 {
			t.Errorf("fov %g, %d screens, distance %g: at the derived %.2f° the "+
				"neighbour misses facing you by %+.2f°", c.fov, c.n, c.dist, s, miss)
		}
		// ⛔ AND THE OLD CONSTANT MISSES BADLY, which is what makes the assertion
		// above worth making rather than a tautology.
		q := p.WithSplay(DefaultSplayDeg)
		lx, lz, rx, rz = slantChain(1, q.SplayDeg(), sameWidth(hw), gapOf(q), q.Distance(), 0)
		sight = math.Atan2((lx+rx)/2, (lz+rz)/2)
		surf = math.Atan2(rx-lx, rz-lz)
		if miss := deg(surf-sight) - 90; math.Abs(miss) < 5 {
			t.Errorf("fov %g: the chosen %g° already faces the viewer (%+.2f°), so "+
				"this test proves nothing", c.fov, DefaultSplayDeg, miss)
		}
	}
}

// ⭐ AT DISTANCE ONE THE ANSWER IS THE FIELD OF VIEW, PLUS THE GAP, in closed
// form -- and the closed form is an independent check on the iteration rather
// than a restatement of it.
//
// The reason there is one at all: when every panel faces the viewer, the viewer
// is on each panel's perpendicular bisector, so both of its corners are the same
// distance away -- and since consecutive panels share a fold, EVERY corner of
// the chain is on one circle, of radius sec(fov/2) at distance one. The panels
// are then equal chords of that circle, each subtending the field of view
// exactly, and the gaps are the chords between them.
//
// ⭐⭐ WHICH IS ALSO WHY THE GAP GOES ON THE BISECTOR. A chord leaving a point of
// a circle makes the same angle with the chord arriving there as half its own
// arc, so the chord of the gap lies at half the splay from the panel -- the
// bisector of the fold, which is where [slantChain] puts it, arrived at
// independently from a demand for left-right symmetry.
//
// So the splay is one panel's arc plus one gap's arc: fov + 2·asin(g/2R). It was
// the fov exactly while the panels were jointive, and that identity is the
// ancestor of this one.
func TestTheDerivedSplayAtDistanceOneIsTheFieldOfView(t *testing.T) {
	for _, fov := range []float64{30, 40, 45.6, 51.57, 55} {
		p := Plan{ScreenW: 1920, ScreenH: 1080, HFOVDeg: fov}.WithScreens(6)
		hw := math.Tan(rad(fov) / 2)
		r := math.Hypot(hw, 1)
		g := 2 * hw * float64(DefaultGapPx) / 1920
		want := fov + 2*deg(math.Asin(g/(2*r)))
		if got := p.FacingSplayDeg(); math.Abs(got-want) > 1e-9 {
			t.Errorf("fov %g at distance 1: derived %g, want %g (the fov plus %g "+
				"for the gap)", fov, got, want, want-fov)
		}
		// And the gap really is what separates the two, rather than a term small
		// enough to hide a mistake in: it is worth about a degree here.
		if want-fov < 0.5 || want-fov > 2 {
			t.Errorf("fov %g: the gap is worth %g°, which is not a gap", fov, want-fov)
		}
	}
	// Pushed back, the neighbour subtends less and wants less turning.
	p := Plan{ScreenW: 1920, ScreenH: 1080, HFOVDeg: 51.57}.WithScreens(6).WithDistance(2)
	if got := p.FacingSplayDeg(); got >= 51.57 || got <= 0 {
		t.Errorf("at distance 2 the derived splay is %g, want less than the fov", got)
	}
}

// ⛔ AND A PLAN WITH NO OPTICS SAYS SO by falling back to the constant, rather
// than deriving a number from a field of view it does not have.
func TestAPlanWithNoOpticsFallsBackToTheChosenAngle(t *testing.T) {
	for _, p := range []Plan{
		{ScreenW: 1920, ScreenH: 1080},
		{ScreenW: 1920, HFOVDeg: 50},
		{ScreenH: 1080, HFOVDeg: 50},
	} {
		if got := p.WithScreens(6).FacingSplayDeg(); got != DefaultSplayDeg {
			t.Errorf("a plan of %dx%d fov %g derived %g, want the fallback %g",
				p.ScreenW, p.ScreenH, p.HFOVDeg, got, DefaultSplayDeg)
		}
	}
}

// ⛔ A WIDE EYE CLOSE UP ASKS FOR MORE THAN THE PACKAGE ALLOWS, and gets it
// clamped like every other angle here. Past MaxSplayDeg a chain wraps round past
// the viewer's own shoulders, which is not a desk whatever the arithmetic wants.
func TestTheDerivedSplayStopsAtTheCeiling(t *testing.T) {
	p := Plan{ScreenW: 1920, ScreenH: 1080, HFOVDeg: 120}.WithScreens(3)
	if got := p.FacingSplayDeg(); got != MaxSplayDeg {
		t.Errorf("a 120° eye derived %g, want it clamped to %g", got, MaxSplayDeg)
	}
	// And just under the ceiling it is NOT clamped, or the test above would pass
	// for a function that always returned the ceiling.
	q := Plan{ScreenW: 1920, ScreenH: 1080, HFOVDeg: 45}.WithScreens(6)
	if got := q.FacingSplayDeg(); got >= MaxSplayDeg {
		t.Errorf("a 45° eye derived %g, which is at the ceiling", got)
	}
}

// ⛔⛔ A LINE THAT DESCRIBES AN ARRANGEMENT HAS TO DESCRIBE THE ARRANGEMENT. The
// description named the model, the count, the size and the optics -- everything
// except the two numbers that decide what the picture looks like. So "l'angle
// est toujours mauvais", reported from the glasses, could not be answered
// without rebuilding the program: nothing said which curvature was in force.
func TestThePlanSaysItsShape(t *testing.T) {
	p := Plan{ScreenW: 1920, ScreenH: 1080, HFOVDeg: 51.57}.WithScreens(6)
	p = p.WithSplay(p.FacingSplayDeg())
	s := p.String()
	// ⛔ THE ANGLE IS ASKED OF THE PLAN, NOT WRITTEN DOWN HERE. A literal was,
	// and the geometry moved under it twice: what this test is for is that the
	// line NAMES the curvature to one decimal, not that the curvature is any
	// particular number.
	for _, want := range []string{"curved", fmt.Sprintf("%.1f°", p.SplayDeg()), "1.00x"} {
		if !strings.Contains(s, want) {
			t.Errorf("%q does not contain %q", s, want)
		}
	}
	// The flat band is named rather than described as "curved 0°", which reads
	// like a curvature that happens to be zero instead of the choice it is.
	if got := p.WithSplay(-1).String(); !strings.Contains(got, "flat") ||
		strings.Contains(got, "curved") {
		t.Errorf("the flat band describes itself as %q", got)
	}
	// And the distance is in there, because it is the other half of the shape.
	if got := p.WithDistance(2).String(); !strings.Contains(got, "2.00x") {
		t.Errorf("%q does not say how far back the band is", got)
	}
}

// TestTheDistancesAreNotEqualAndTheDocumentationSaysSo.
//
// ⛔⛔ THE COMMENT CLAIMED "all at one distance" AND THREE DAYS WERE SPENT
// BELIEVING IT. "l'angle est toujours mauvais" was reported three times; each
// time the expectation being tested against was a promise the geometry never
// kept. Measuring the spread is what ended it, and pinning the measurement is
// what stops it coming back as a bug report.
//
// ⭐ IT IS NOT A DEFECT, IT IS A CHOICE, made after the spread was measured:
// with panels tiling edge to edge you may have any two of {a free curvature
// dial, tiling, equal distance} and never all three. The dial and the tiling
// were kept.
func TestTheDistancesAreNotEqualAndTheDocumentationSaysSo(t *testing.T) {
	p := Plan{ScreenW: 1920, ScreenH: 1080, HFOVDeg: 51.57}.WithScreens(6)
	p = p.WithSplay(p.FacingSplayDeg())
	hw, _, _ := slantOptics(p.HFOVDeg, p.ScreenW, p.ScreenW, p.ScreenH)

	lo, hi := math.Inf(1), math.Inf(-1)
	for j := -1; j <= 1; j++ {
		_, lz, _, rz := slantChain(j, p.SplayDeg(), sameWidth(hw), gapOf(p), p.Distance(), 0)
		for _, z := range []float64{lz, rz} {
			// A panel behind the viewer has no depth worth comparing; slantOf
			// refuses it, and the band never shows it.
			if z <= 0 {
				continue
			}
			lo, hi = math.Min(lo, z), math.Max(hi, z)
		}
	}
	if hi/lo < 1.5 {
		t.Errorf("the depths run %.3f..%.3f, a spread of %.0f%%: the geometry now "+
			"keeps the screens near enough to one distance that the comment "+
			"saying it does NOT should be revisited", lo, hi, 100*(hi/lo-1))
	}
	// ⭐ AND THE FLAT BAND REALLY IS FLAT, which is the one state where the two
	// geometries must agree and the only anchor that makes either verifiable.
	_, lz, _, rz := slantChain(0, 0, sameWidth(hw), gapOf(p), p.Distance(), 0)
	if math.Abs(lz-rz) > 1e-12 {
		t.Errorf("at a splay of nothing the edges are at %.12f and %.12f, want "+
			"the same depth: the flat band is what the whole chain is checked against",
			lz, rz)
	}
}

// TestThePlanNamesTheScreenThatIsNotLikeTheOthers.
//
// ⛔⛔ "6 SCREENS OF 1920x1080" WAS WRITTEN BESIDE A PHOTOGRAPH OF A DESK WITH A
// 3840 ON IT. An Odyssey G95NC is mirrored onto position 1 here, and three
// separate defects lived behind that sentence for as long as it was believed:
// the chain, the band's length and Strip.Toward all assumed the nominal width,
// and the one line that could have said otherwise reported the nominal width.
func TestThePlanNamesTheScreenThatIsNotLikeTheOthers(t *testing.T) {
	p := Plan{ScreenW: 1920, ScreenH: 1080, HFOVDeg: 51.57}.WithScreens(6)
	if s := p.String(); strings.Contains(s, "screen ") {
		t.Errorf("%q names an odd screen on a desk that has none", s)
	}
	p = p.WithScreenWidth(0, 3840)
	s := p.String()
	if !strings.Contains(s, "screen 1 is 3840x1080") {
		t.Errorf("%q does not say that screen 1 is 3840 wide", s)
	}
	// And every one of them, not just the first: a desk may mirror more than one
	// panel, and a line that stops at the first is a line that hides the second.
	p = p.WithScreenWidth(4, 1280)
	if s := p.String(); !strings.Contains(s, "screen 1 is 3840x1080") ||
		!strings.Contains(s, "screen 5 is 1280x1080") {
		t.Errorf("%q does not name both odd screens", s)
	}
}
