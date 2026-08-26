// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
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
	if got := EvidenceFor(monitor, true, bus); got == nil || *got != oneS {
		t.Error("the bus was refused for a display the person named")
	}
	if got := EvidenceFor(glasses.Display{Name: "XREAL 1S"}, true, nil); got != nil {
		t.Error("EvidenceFor invented evidence")
	}
	// End to end: the monitor is refused rather than planned with somebody
	// else's optics.
	if _, err := NewPlan(monitor, Options{USB: EvidenceFor(monitor, false, bus)}); !errors.Is(err, ErrUnknownOptics) {
		t.Errorf("planning the monitor gave %v, want %v", err, ErrUnknownOptics)
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

	// Named by the person, with two attached and nothing to tell them apart:
	// no evidence rather than a coin toss.
	if got := EvidenceFor(glasses.Display{Name: "DisplayPort"}, true, both); got != nil {
		t.Errorf("with two headsets attached the bus still picked %v", got)
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
