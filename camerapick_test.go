// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import "testing"

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
