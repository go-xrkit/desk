// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"strings"

	"github.com/go-xrkit/xrkit/glasses"
)

// What the bus calls the headsets this desk knows how to talk to.
const (
	vitureVendor  uint16 = 0x35ca
	beastProduct  uint16 = 0x1201
	lumaUltraProd uint16 = 0x1104
)

// ErrNoSideBySide means these glasses have no side-by-side mode to switch to.
//
// ⛔⛔ IT IS NOT A FAILURE, IT IS A FACT ABOUT THE HEADSET. Measured
// 2026-09-06 while VITURE's own SpaceWalker built a three-screen layout on a
// Luma Ultra: the panel never moved -- one line, start to finish,
// "1920x1200, 81 modes, widest 2048" -- and what appeared were VIRTUAL
// displays on the Mac, which SpaceWalker then composited itself.
//
// ⭐ THAT IS THE OPPOSITE OF THE BEAST, where side-by-side is a real mode: the
// EDID changes, model 0x120 becomes 0x220, and 1920 becomes 3840. Both proven
// on the hardware, in both directions.
var ErrNoSideBySide = errNoSideBySide

// canSwitchToSideBySide reports whether the headset this session is running on
// can be switched, and what to say when it cannot.
//
// ⛔ WHICH HEADSET, NOT WHETHER ONE EXISTS. Both were attached while this was
// written, and an earlier version of it took the first VITURE device on the
// bus -- which is whichever the enumeration happened to list first, not the one
// the desk is showing on. It would have switched a Beast sitting on the desk
// while the person was wearing a Luma.
//
// So the SESSION'S DISPLAY says which headset, and the BUS says whether that
// one is really there. The display name is the right key for the first
// question and the wrong one for the second: a Beast presents a display called
// "VITURE Beast" and a Luma Ultra presents one called just "VITURE", so the
// name distinguishes them but cannot confirm either is on the bus.
func canSwitchToSideBySide(display string, us []glasses.USB) (why string, can bool) {
	onBus := func(product uint16) bool {
		for _, u := range us {
			if u.Vendor == vitureVendor && u.Product == product {
				return true
			}
		}
		return false
	}
	if strings.Contains(strings.ToLower(display), "beast") {
		if !onBus(beastProduct) {
			return "the Beast is not on the bus any more", false
		}
		return "", true
	}
	if onBus(lumaUltraProd) {
		return "these glasses have no side-by-side mode; the desk draws 3D itself", false
	}
	return "no headset here can be switched into side-by-side", false
}
