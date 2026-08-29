// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

// Where the pointer is, in ribbon positions.
//
// This is the piece that makes a desk of captured screens usable rather than
// something to look at. The band shows ONE screen; the pointer lives on the
// desktop, which is several. A person who moves the mouse off the right-hand
// edge of the screen in front of them has, on a real desk, arrived at the next
// monitor — and here they had simply lost the pointer somewhere they could not
// see, with nothing on the picture to say so.
//
// So the band FOLLOWS it: the pointer moves onto another of the desk's screens,
// and the ribbon turns to that screen. Nothing is warped, nothing is
// synthesised, and the pointer stays exactly where the person put it. It is
// their own mouse; the picture catches up.

// PositionOf returns the ribbon position the pointer is on, and whether it is on
// one of the desk's screens at all.
//
// ids is the desk's displays in ribbon order — the same slice [Send] takes — so
// the answer is a position on the band and not a CGDirectDisplayID. A pointer on
// the machine's own display, or on a display this desk is not showing, is not on
// the band: that is the false, and it is a perfectly ordinary state.
// underPointer is the seam: the platform answers it, and a test replaces it.
// Everything a person can see about following -- which screen, and whether the
// pointer is on the desk at all -- is decided here rather than in the binding.
var underPointer = displayUnderPointer

func PositionOf(ids []uint64) (int, bool) {
	id, ok := underPointer()
	if !ok {
		return 0, false
	}
	for i, want := range ids {
		if uint32(want) == id {
			return i, true
		}
	}
	return 0, false
}
