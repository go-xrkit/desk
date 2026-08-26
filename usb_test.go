// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"testing"

	"github.com/go-xrkit/xrkit/glasses"
)

// The two pairs of glasses that were on the desk the day this was written,
// with the numbers their own buses reported.
var (
	oneS = glasses.USB{Vendor: 0x3318, Product: 0x043e, Name: "XREAL 1S"}
	luma = glasses.USB{Vendor: 0x35ca, Product: 0x1104, Name: "VITURE Luma Ultra XR GLASSES"}
)

// TestTheBusDoesNotLendItsOpticsToAMonitor.
//
// The bus says which headsets are attached. It does not say which display is
// which headset. Taking the model from one and the pixels from the other
// produced a plan for "XREAL 1S: 7 screens of 3840x2160" against a desktop
// monitor — a headset's optics wrapped round somebody's desk.
func TestTheBusDoesNotLendItsOpticsToAMonitor(t *testing.T) {
	monitor := glasses.Display{Name: "Odyssey G95NC", Width: 7680, Height: 2160, Primary: true}
	bus := []glasses.USB{oneS}

	if got := EvidenceFor(monitor, false, bus); got != nil {
		t.Error("the bus lent its optics to a monitor nobody said was the glasses")
	}
	// Not even when the person named that display by hand. The shortcut that
	// allowed it earned its keep while a field of view decided the geometry;
	// what is left of it is a guess at a name, printed as a model and a figure,
	// as fact.
	if got := EvidenceFor(monitor, true, bus); got != nil {
		t.Errorf("naming the display by hand handed it %v", got)
	}
	if got := EvidenceFor(glasses.Display{Name: "XREAL 1S"}, true, nil); got != nil {
		t.Error("EvidenceFor invented evidence")
	}
	// End to end: the monitor is refused rather than planned with somebody
	// else's optics.
	// End to end: the monitor is planned as ITSELF, with no figure, rather than
	// with a headset's optics. That mattered more when a field of view decided
	// the geometry; it still matters, because the model is what a person reads
	// to know the right thing is being driven.
	q, err := NewPlan(monitor, Options{USB: EvidenceFor(monitor, false, bus)})
	if err != nil {
		t.Fatalf("planning the monitor: %v", err)
	}
	if q.Model != "Odyssey G95NC" || q.HFOVDeg != 0 {
		t.Errorf("the monitor was planned as %q at %g°", q.Model, q.HFOVDeg)
	}
}

// TestTwoHeadsetsOnOneDesk is not a strange case: it is what a desk looks like
// when somebody is comparing two pairs, which is exactly when this code is
// being used. Picking the first of two and saying nothing would put a guess
// where the catalogue refuses one.
func TestTwoHeadsetsOnOneDesk(t *testing.T) {
	both := []glasses.USB{oneS, luma}

	// The display names one of them, so the bus only says which entry it is —
	// and it must be the RIGHT one, not the first.
	got := EvidenceFor(glasses.Display{Name: "VITURE Luma Ultra", Width: 1920, Height: 1080}, false, both)
	if got == nil || *got != luma {
		t.Errorf("got %v, want the Luma Ultra's own entry", got)
	}
	got = EvidenceFor(glasses.Display{Name: "XREAL 1S", Width: 1920, Height: 1200}, false, both)
	if got == nil || *got != oneS {
		t.Errorf("got %v, want the One S's own entry", got)
	}

	// A display that names a headset NEITHER of them is: the bus has nothing to
	// say about it, and must not offer the nearest thing it has.
	if got := EvidenceFor(glasses.Display{Name: "Rokid Max"}, false, both); got != nil {
		t.Errorf("the bus offered %v for a headset that is not on it", got)
	}

	// EvidenceFor is exported and takes whatever a caller hands it, which need
	// not have come from Peripherals. An entry that names only a BRAND must be
	// stepped over rather than used: a brand cannot give a field of view.
	brandOnly := glasses.USB{Vendor: 0x3318, Product: 0xffff, Name: "XREAL Something"}
	if _, how := glasses.IdentifyDevice("", &brandOnly); how != glasses.ByUSBVendor {
		t.Fatalf("the brand-only entry identifies as %v; the premise is wrong", how)
	}
	got = EvidenceFor(glasses.Display{Name: "XREAL 1S", Width: 1920, Height: 1200},
		false, []glasses.USB{brandOnly, oneS})
	if got == nil || *got != oneS {
		t.Errorf("got %v, want the entry a PRODUCT id names", got)
	}

	// Named by the person, with two attached and nothing to tell them apart:
	// no evidence rather than a coin toss.
	if got := EvidenceFor(glasses.Display{Name: "DisplayPort"}, true, both); got != nil {
		t.Errorf("with two headsets attached the bus still picked %v", got)
	}
	if got := EvidenceFor(glasses.Display{Name: "DisplayPort"}, true, both[:1]); got != nil {
		t.Errorf("with one headset attached the bus guessed %v for a display that "+
			"names nothing", got)
	}
}

// TestPeripheralsOnlyEverNamesAModel reads the REAL bus. On a machine with no
// glasses it asserts nothing and costs nothing; on this desk it ran with two
// pairs attached, and then one. What it insists on either way is that nothing
// comes back that a product id does not name — never a brand, because a brand
// cannot give a field of view.
func TestPeripheralsOnlyEverNamesAModel(t *testing.T) {
	all := Peripherals()
	for i := range all {
		if _, how := glasses.IdentifyDevice("", &all[i]); how != glasses.ByUSBProduct {
			t.Errorf("Peripherals()[%d] = %v, which no product id names", i, all[i])
		}
	}
	t.Logf("%d headset(s) on this bus: %v", len(all), all)
}

// TestTheBusRefinesABrandIntoAModel is what today's pair of Luma Ultra needed,
// and what an equality test refused.
//
// Their EDID says the bare word "VITURE". The catalogue holds that as a BRAND,
// deliberately carrying no optics at all — a brand cannot say what to render —
// so the display alone cannot be planned for. The bus says "VITURE Luma Ultra",
// which can. Requiring the two names to be equal refused the one refinement
// that matters.
func TestTheBusRefinesABrandIntoAModel(t *testing.T) {
	brandOnly := glasses.Display{Name: "VITURE", Width: 1920, Height: 1200}

	// The premise: the display's own name identifies a brand and nothing more.
	p, ok := glasses.Identify(brandOnly.Name)
	if !ok || p.Known() {
		t.Fatalf("%q identifies as %q known=%v; the premise of this test is wrong",
			brandOnly.Name, p.Model, p.Known())
	}
	bare, err := NewPlan(brandOnly, Options{})
	if err != nil {
		t.Fatalf("a brand-only display: %v", err)
	}
	if bare.HFOVDeg != 0 {
		t.Errorf("a brand gave a field of view of %g°", bare.HFOVDeg)
	}

	got := EvidenceFor(brandOnly, false, []glasses.USB{oneS, luma})
	if got == nil || *got != luma {
		t.Fatalf("EvidenceFor = %v, want the Luma Ultra's entry", got)
	}
	plan, err := NewPlan(brandOnly, Options{USB: got})
	if err != nil {
		t.Fatalf("with the bus: %v", err)
	}
	if plan.Model != "VITURE Luma Ultra" {
		t.Errorf("model is %q, want the one the bus named", plan.Model)
	}

	// And the brand has to be POSITIVE evidence. A display naming a brand the
	// bus does not carry gets nothing — it must not be handed the only headset
	// there happens to be.
	rokid := glasses.Display{Name: "Rokid", Width: 1920, Height: 1200}
	if p, ok := glasses.Identify(rokid.Name); ok && !p.Known() {
		if got := EvidenceFor(rokid, false, []glasses.USB{luma}); got != nil {
			t.Errorf("a display naming another brand was handed %v", got)
		}
	}
}
