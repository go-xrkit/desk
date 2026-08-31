// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build darwin

package desk

import "testing"

// TestTheLocationIdCountsTheHops uses the real ids off the machine that needed
// this: the glasses one hop from a root port on a controller of their own, and
// a whole dock two, three and four hops away on another.
func TestTheLocationIdCountsTheHops(t *testing.T) {
	for id, want := range map[uint32]int{
		0x00100000: 1, // XREAL 1S, straight into a root port
		0x02110000: 2, // the hub's Billboard
		0x02132200: 4, // a headset's dongle, three hubs deep
		0x02243300: 4, // storage behind the dock's own chain
		0x02000000: 0, // a controller with no port path: nothing to say
	} {
		if got := hopsFromLocation(id); got != want {
			t.Errorf("%08x: %d hops, want %d", id, got, want)
		}
	}

	// Whatever is plugged into THIS machine, a headset that is found is found
	// somewhere: a hop count of zero would mean the id said nothing, and then
	// the advice that depends on it is the generic one rather than a wrong one.
	for _, u := range Peripherals() {
		t.Logf("%04x:%04x %q is %d hop(s) away", u.Vendor, u.Product, u.Name, u.Hops)
	}
}
