// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"strings"
	"testing"
)

// TestTheHeadsetAndItsCameraShareAHub.
//
// ⭐ MEASURED, with both headsets attached at once:
//
//	0x110000  VITURE Luma Ultra    0x120000  its camera
//	0x2130000 VITURE Beast         0x2110000 its camera
//
// A USB location is a path down the tree, one nibble per port. The glasses and
// their camera hang off the SAME hub, one port apart -- so the parent is what
// says which headset a camera belongs to.
func TestTheHeadsetAndItsCameraShareAHub(t *testing.T) {
	const (
		luma     = 0x110000
		lumaCam  = 0x120000
		beast    = 0x2130000
		beastCam = 0x2110000
	)
	if parentHub(luma) != parentHub(lumaCam) {
		t.Errorf("the Luma (%#x) and its camera (%#x) do not share a hub: %#x vs %#x",
			luma, lumaCam, parentHub(luma), parentHub(lumaCam))
	}
	if parentHub(beast) != parentHub(beastCam) {
		t.Errorf("the Beast (%#x) and its camera (%#x) do not share a hub: %#x vs %#x",
			beast, beastCam, parentHub(beast), parentHub(beastCam))
	}
	// ⛔ AND THE TWO HEADSETS MUST NOT SHARE ONE, or the desk photographs
	// through the glasses somebody is not wearing -- which is the same defect
	// as switching the wrong headset into 3D, one floor down.
	if parentHub(luma) == parentHub(beast) {
		t.Errorf("both headsets read as the same hub %#x", parentHub(luma))
	}
	if parentHub(lumaCam) == parentHub(beastCam) {
		t.Error("both cameras read as the same hub")
	}
	if got := parentHub(0); got != 0 {
		t.Errorf("nothing has hub %#x", got)
	}
}

// TestAPrefixComparisonWouldNotHaveWorked, which is why this walks the path.
//
// ⛔ 0x110000 AND 0x120000 AGREE ON EVERY BYTE ABOVE THE ONE THAT MATTERS. A
// comparison of raw values matches nothing and a comparison of the top byte
// matches everything on the same bus -- including the other headset.
func TestAPrefixComparisonWouldNotHaveWorked(t *testing.T) {
	const luma, lumaCam, beast = 0x110000, 0x120000, 0x2130000
	if luma == lumaCam {
		t.Fatal("the test's own premise is wrong")
	}
	if luma>>24 != beast>>24 {
		t.Skip("the two headsets are not on the same bus here, so this says nothing")
	}
	if parentHub(luma) == parentHub(beast) {
		t.Error("walking the path still confuses the two headsets")
	}
}

// TestTheCameraIdIsBuiltTheWayAVFoundationSpellsIt.
//
// ⭐ MEASURED, and it is NOT padded: the id is "0x" then the location, the
// vendor and the product, each in the shortest hex that holds it. Building it
// beats parsing it -- a parser has to guess where the location ends, and the
// answer is "wherever it happens to end".
func TestTheCameraIdIsBuiltTheWayAVFoundationSpellsIt(t *testing.T) {
	if got, want := cameraID(0x2110000, 0x0c45, 0x6368), "0x21100000c456368"; got != want {
		t.Errorf("the Beast's camera is %q, want %q", got, want)
	}
	if got, want := cameraID(0x120000, 0x0c45, 0x636b), "0x1200000c45636b"; got != want {
		t.Errorf("the Luma's camera is %q, want %q", got, want)
	}
}

// TestTheDisplayNameSaysWhichHeadsetToLookFor, the same key as the 3D row.
func TestTheDisplayNameSaysWhichHeadsetToLookFor(t *testing.T) {
	if got := wantedHeadset("VITURE Beast"); got != beastProduct {
		t.Errorf("a Beast display wants %#04x", got)
	}
	if got := wantedHeadset("viture beast xr"); got != beastProduct {
		t.Errorf("case should not matter: %#04x", got)
	}
	if got := wantedHeadset("VITURE"); got != lumaUltraProd {
		t.Errorf("a Luma display wants %#04x", got)
	}
}

// TestRoomCameraRefusesRatherThanShowTheMacsOwn.
//
// ⛔⛔ THE DEFECT THIS EXISTS FOR. Both call sites carried a comment saying
// "the headset's camera, not the Mac's" and then fell through to the empty
// string when the lookup came back empty -- and an empty camera name means "the
// first one the machine lists" to AVFoundation, which on a laptop is the one
// pointing at the person's FACE. So "show the room" would have shown the
// viewer, and the photograph key would have photographed them.
//
// A rule written in a comment that the code does not enforce is not a rule.
func TestRoomCameraRefusesRatherThanShowTheMacsOwn(t *testing.T) {
	restore := headsetCamera
	t.Cleanup(func() { headsetCamera = restore })

	t.Run("a named camera wins, even when the headset has one", func(t *testing.T) {
		headsetCamera = func(string) string { return "0xheadset" }
		got, err := RoomCamera("0xchosen-by-hand", "VITURE Beast")
		if err != nil {
			t.Fatalf("RoomCamera: %v", err)
		}
		// A person who names a camera means that camera -- including the Mac's,
		// if that is what they typed.
		if got != "0xchosen-by-hand" {
			t.Errorf("got %q, want the named one", got)
		}
	})

	t.Run("no name takes the headset's", func(t *testing.T) {
		headsetCamera = func(display string) string {
			if display != "VITURE Beast" {
				t.Errorf("looked up %q, want the display it was given", display)
			}
			return "0xheadset"
		}
		got, err := RoomCamera("", "VITURE Beast")
		if err != nil {
			t.Fatalf("RoomCamera: %v", err)
		}
		if got != "0xheadset" {
			t.Errorf("got %q, want the headset's", got)
		}
	})

	t.Run("no name and no headset camera REFUSES", func(t *testing.T) {
		headsetCamera = func(string) string { return "" }
		got, err := RoomCamera("", "VITURE Beast")
		if err == nil {
			t.Fatalf("got %q and no error; falling back to the Mac's camera is the bug", got)
		}
		if got != "" {
			t.Errorf("got %q alongside an error; a refusal must not also hand one over", got)
		}
		if !errors.Is(err, ErrNoRoomCamera) {
			t.Errorf("error %v does not answer to ErrNoRoomCamera", err)
		}
		// The message has to name the headset, or a person reading a log cannot
		// tell which of two attached headsets could not be asked.
		if !strings.Contains(err.Error(), "VITURE Beast") {
			t.Errorf("error %q does not name the headset", err)
		}
		if !strings.Contains(err.Error(), "-photo-camera") {
			t.Errorf("error %q does not say how to override it", err)
		}
	})

	t.Run("with no display named, it still says what it is talking about", func(t *testing.T) {
		headsetCamera = func(string) string { return "" }
		_, err := RoomCamera("", "")
		if err == nil {
			t.Fatal("want a refusal")
		}
		if !strings.Contains(err.Error(), "the headset") {
			t.Errorf("error %q leaves a blank where the display should be", err)
		}
	})
}
