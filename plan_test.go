// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
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
	beast := glasses.Display{Name: "VITURE Beast", Width: 3840, Height: 1080}
	for _, tc := range []struct {
		name string
		d    glasses.Display
		opts Options
		want error
	}{
		{"negative screens", beast, Options{Screens: -1}, ErrScreens},
		{"a display with no size", glasses.Display{Name: "VITURE Beast"}, Options{}, ErrScreens},
		{"a field of view that is not one", beast, Options{FOVDeg: 180}, ErrFOV},
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
