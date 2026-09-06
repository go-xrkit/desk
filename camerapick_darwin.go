// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build darwin

package desk

import "github.com/go-macos/iokit/usb"

// HeadsetCamera is the camera on the same headset as this display, or empty
// for none.
//
// ⛔ A DESK THAT PHOTOGRAPHS THE MAC'S OWN CAMERA PHOTOGRAPHS THE PERSON'S
// FACE. That is what "the first camera the machine lists" means on a laptop,
// and it is what the photograph key did: the flag that chooses one defaults to
// empty, and empty means first. On a headset, a photograph should be of what
// the person is LOOKING AT.
//
// ⭐ AND THE CAMERA IS ORDINARY. Both VITURE headsets carry a plain USB video
// camera -- a Sonix part, 0c45 -- that macOS lists without any driver or
// protocol at all. Nothing here decodes anything; it only picks the right one.
func HeadsetCamera(display string) string {
	devs, err := usb.Devices(usb.Filter{})
	if err != nil {
		return ""
	}
	want := wantedHeadset(display)
	var hub uint32
	var found bool
	type cam struct {
		loc             uint32
		vendor, product uint16
	}
	var cams []cam
	for _, d := range devs {
		i := d.Info()
		d.Close()
		switch {
		case i.VendorID == vitureVendor && i.ProductID == want:
			hub, found = parentHub(i.LocationID), true
		case i.VendorID == cameraVendor:
			cams = append(cams, cam{i.LocationID, i.VendorID, i.ProductID})
		}
	}
	if !found {
		return ""
	}
	for _, c := range cams {
		if parentHub(c.loc) == hub {
			return cameraID(c.loc, c.vendor, c.product)
		}
	}
	return ""
}
