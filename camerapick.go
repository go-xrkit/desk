// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"fmt"
	"strings"
)

// A USB location id is a path down the tree, one nibble per port.
//
// ⭐ MEASURED, with both headsets attached. The glasses and their own camera
// hang off the SAME hub, one port apart:
//
//	0x110000  VITURE Luma Ultra      0x120000  its camera
//	0x2130000 VITURE Beast           0x2110000 its camera
//
// So the headset a camera belongs to is the one that shares its PARENT: strip
// the trailing zero nibbles to get the path, then drop the last port.
const cameraVendor uint16 = 0x0c45

// parentHub is the location path with its last port removed.
//
// ⛔ A PREFIX COMPARISON WOULD NOT DO. 0x110000 and 0x120000 differ in the
// nibble that matters and agree on everything above it; comparing raw values,
// or the top bytes, either matches everything or nothing. What identifies a
// headset is the PATH to its hub.
func parentHub(loc uint32) uint32 {
	if loc == 0 {
		return 0
	}
	for loc&0xF == 0 {
		loc >>= 4
	}
	return loc >> 4
}

// cameraID builds the name AVFoundation gives a USB camera.
//
// ⭐ MEASURED, and it is not padded: the id is "0x" then the location, the
// vendor and the product -- the location in the shortest hex that holds it,
// the vendor and product padded to four digits each.
//
//	0x2110000 + 0c45 + 6368  ->  "0x21100000c456368"
//	0x120000  + 0c45 + 636b  ->  "0x1200000c45636b"
//
// ⛔ I READ IT AS UNPADDED FIRST and the test, built from the two ids the
// machine actually reports, said no. A format guessed from one example is a
// format guessed.
//
// Building it beats parsing it: a caller that parses has to guess where the
// location ends, and the answer is "wherever it happens to end".
func cameraID(loc uint32, vendor, product uint16) string {
	return fmt.Sprintf("0x%x%04x%04x", loc, vendor, product)
}

// wantedHeadset is the USB product id of the headset a display belongs to.
//
// The same key as [canSwitchToSideBySide], for the same reason: a Beast
// presents a display called "VITURE Beast" and a Luma Ultra presents one
// called just "VITURE".
func wantedHeadset(display string) uint16 {
	if strings.Contains(strings.ToLower(display), "beast") {
		return beastProduct
	}
	return lumaUltraProd
}

// ErrNoRoomCamera is returned when the room cannot be shown or photographed
// because the headset's own camera was not found.
var ErrNoRoomCamera = errors.New("desk: the headset's camera was not found")

// headsetCamera is [HeadsetCamera], named so a test can stand in for it. The
// real one reads the USB tree, which a test cannot arrange.
var headsetCamera = HeadsetCamera

// RoomCamera is the camera that sees what the person is looking at.
//
// ⛔⛔ IT REFUSES RATHER THAN FALL BACK TO THE MAC'S OWN, and that is the whole
// point of it. An empty camera name means "the first one the machine lists" to
// AVFoundation, which on a laptop is the one pointing at the person's FACE. So
// a passthrough that failed to find the headset's camera would put the viewer's
// face on a ribbon screen where they asked for the room, and the photograph key
// would photograph them instead of what they were looking at.
//
// ⛔ AND THAT IS EXACTLY WHAT HAPPENED. Both call sites carried a comment
// saying "the headset's camera, not the Mac's" -- and then fell through to the
// empty string when the lookup came back empty. A rule written in a comment
// that the code does not enforce is not a rule. Measured 2026-09-06: with the
// Beast attached the lookup works and passthrough delivers 1920x1080 from the
// glasses; the silent fallback was one unplugged headset away.
//
// named wins whenever it is set: a person who names a camera means that camera,
// including the Mac's if that is what they typed. display is the headset's
// display name, which is how [HeadsetCamera] tells one headset from another.
func RoomCamera(named, display string) (string, error) {
	if named != "" {
		return named, nil
	}
	if c := headsetCamera(display); c != "" {
		return c, nil
	}
	which := display
	if which == "" {
		which = "the headset"
	}
	return "", fmt.Errorf("%w: %s has none. The camera hangs off the same USB hub as "+
		"the glasses, so a headset attached for its picture only -- over a display "+
		"cable -- does not present one. Name a camera with -photo-camera to override",
		ErrNoRoomCamera, which)
}
