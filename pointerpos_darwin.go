// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build darwin

package desk

import "github.com/go-macos/pointer"

// displayUnderPointer is the display the pointer is on, from CoreGraphics.
//
// It asks for the position and the bounds rather than for "the display with the
// mouse", because CGGetDisplaysWithPoint answers with a LIST — a point on a
// shared edge belongs to two displays — and a ribbon position has to be one
// screen. Containment with the top and left edges inside settles it the same
// way go-macos/accessibility settles which display a window is on.
func displayUnderPointer() (uint32, bool) {
	at, err := pointer.Position()
	if err != nil {
		return 0, false
	}
	ids, err := pointer.Displays()
	if err != nil {
		return 0, false
	}
	for _, id := range ids {
		b, err := pointer.Bounds(id)
		if err != nil {
			continue
		}
		if at.X >= b.X && at.X < b.X+b.W && at.Y >= b.Y && at.Y < b.Y+b.H {
			return id, true
		}
	}
	return 0, false
}
