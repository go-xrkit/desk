// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-xrkit/xrkit/glasses"
)

var (
	busBeast = glasses.USB{Vendor: vitureVendor, Product: beastProduct, Name: "VITURE Beast XR Glasses"}
	busLuma  = glasses.USB{Vendor: vitureVendor, Product: lumaUltraProd, Name: "VITURE Luma Ultra XR GLASSES"}
	// The companion interface a Luma also publishes. It is called "VITURE
	// Microphone" and that name has misled this fleet twice, in both
	// directions -- so a test carries it.
	busMic = glasses.USB{Vendor: vitureVendor, Product: 0x1102, Name: "VITURE Microphone"}
	busHub = glasses.USB{Vendor: 0x05ac, Product: 0x1234, Name: "a hub"}
)

// TestTheSessionsDisplaySaysWHICHHeadset, not the bus.
//
// ⛔⛔ AN EARLIER VERSION OF THIS TOOK THE FIRST VITURE DEVICE ON THE BUS, and
// both were attached while it was written. That is whichever the enumeration
// happened to list first -- not the one the desk is showing on. It would have
// switched a Beast sitting on the desk while the person was wearing a Luma.
func TestTheSessionsDisplaySaysWHICHHeadset(t *testing.T) {
	both := []glasses.USB{busBeast, busLuma, busMic, busHub}

	// On a Beast, with a Luma ALSO attached: the switch is allowed.
	if why, can := canSwitchToSideBySide("VITURE Beast", both); !can {
		t.Errorf("a Beast session was refused its own switch: %q", why)
	}
	// On the Luma, with a Beast ALSO attached: it must NOT be allowed, or the
	// desk switches the headset nobody is wearing.
	why, can := canSwitchToSideBySide("VITURE", both)
	if can {
		t.Fatal("a Luma session was sent to the Beast's switch while both were attached")
	}
	if !strings.Contains(why, "no side-by-side") {
		t.Errorf("the Luma is told %q", why)
	}
	// ⭐ AND IT SAYS WHAT HAPPENS INSTEAD. A fact with no remedy ends the
	// conversation; the desk draws 3D itself, which is what SpaceWalker does
	// on the same hardware.
	if !strings.Contains(why, "draws 3D itself") {
		t.Errorf("the Luma is told %q, leaving nothing to do about it", why)
	}
}

// TestTheBusStillHasToConfirmIt: the display name says which headset, and only
// the bus can say it is still there.
func TestTheBusStillHasToConfirmIt(t *testing.T) {
	// A Beast display with no Beast on the bus -- unplugged mid-session.
	why, can := canSwitchToSideBySide("VITURE Beast", []glasses.USB{busMic, busHub})
	if can {
		t.Fatal("it would have asked a headset that is not there")
	}
	if !strings.Contains(why, "not on the bus") {
		t.Errorf("an unplugged Beast is reported as %q", why)
	}
	// Nothing at all.
	if why, can := canSwitchToSideBySide("", nil); can || !strings.Contains(why, "can be switched") {
		t.Errorf("an empty bus gives %q (%v)", why, can)
	}
	// ⛔ THE MICROPHONE INTERFACE IS NOT A HEADSET. It carries 64-byte reports
	// in both directions, which is why it has been mistaken for the control
	// channel twice; it is not one.
	if _, can := canSwitchToSideBySide("VITURE", []glasses.USB{busMic}); can {
		t.Error("the microphone interface was taken for a switchable headset")
	}
}

// TestTheRefusalIsRecognisableAsOne, so a caller can tell it from a cable.
func TestTheRefusalIsRecognisableAsOne(t *testing.T) {
	if !errors.Is(ErrNoSideBySide, ErrNoGlasses3D) {
		t.Errorf("%v is not recognisable as a 3D refusal", ErrNoSideBySide)
	}
	if !strings.Contains(ErrNoSideBySide.Error(), "side-by-side") {
		t.Errorf("it reads %q", ErrNoSideBySide)
	}
}
