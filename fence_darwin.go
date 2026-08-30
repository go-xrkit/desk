// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build darwin

package desk

import "github.com/go-macos/pointer"

func platformPointerAt() (x, y float64, ok bool) {
	at, err := pointer.Position()
	if err != nil {
		return 0, 0, false
	}
	return at.X, at.Y, true
}

func platformDisplayRect(id uint64) (rect, bool) {
	b, err := pointer.Bounds(uint32(id))
	if err != nil {
		return rect{}, false
	}
	return rect{X: b.X, Y: b.Y, W: b.W, H: b.H}, true
}

func platformDisplays() []uint64 {
	ids, err := pointer.Displays()
	if err != nil {
		return nil
	}
	out := make([]uint64, len(ids))
	for i, id := range ids {
		out[i] = uint64(id)
	}
	return out
}

// platformMovePointer moves the pointer and presses nothing: CGWarpMouseCursor-
// Position is not an event.
func platformMovePointer(x, y float64) error { return pointer.MoveTo(pointer.Point{X: x, Y: y}) }
