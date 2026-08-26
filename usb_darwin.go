// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"github.com/go-macos/iokit/usb"
	"github.com/go-xrkit/xrkit/glasses"
)

// Peripherals lists every headset on the USB bus.
//
// It exists because a display name is the weaker evidence and is not always
// there at all. These glasses announce themselves on the bus as "XREAL 1S", by
// a product id that names exactly one model, while the video link may be down
// — plugged into a port with no DisplayPort lane, or into one whose bandwidth
// a large monitor has already taken. Knowing which headset is attached before
// there is a picture is what lets the application say so.
//
// Nothing is opened: this reads what the kernel cached when it enumerated the
// device, so it needs no permission and cannot disturb whatever driver holds
// the glasses.
func Peripherals() []glasses.USB {
	devs, err := usb.Devices(usb.Filter{})
	if err != nil {
		return nil
	}
	var out []glasses.USB
	for _, d := range devs {
		i := d.Info()
		d.Close()
		u := glasses.USB{Vendor: i.VendorID, Product: i.ProductID, Name: i.Product}
		// Only a product match is taken. A vendor match alone names a brand and
		// not a model, and a brand cannot give a field of view; taking it here
		// would put a guess where the catalogue deliberately refuses one.
		if _, how := glasses.IdentifyDevice("", &u); how == glasses.ByUSBProduct {
			out = append(out, u)
		}
	}
	return out
}
