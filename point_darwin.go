// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build darwin

package desk

import (
	"fmt"

	"github.com/go-macos/pointer"
)

// BringPointer puts the mouse pointer in the middle of one of the desk's
// displays.
//
// It is what [Desk.OnPoint] is wired to, and it is the difference between a desk
// a person can look at and one they can use. The applications are on displays the
// window server knows about; the picture in the glasses is a capture of them. So
// the pointer has to be MOVED to the display whose picture somebody is looking
// at -- dragging it there means dragging it across displays whose contents are
// captures of somewhere else, which is dragging it blind.
//
// The screen index is the ribbon's; ids are the displays in the same order, as
// Provide returns them.
func BringPointer(ids []uint64, screen int) error {
	if screen < 0 || screen >= len(ids) {
		return fmt.Errorf("%w: screen %d of %d", ErrScreens, screen, len(ids))
	}
	if err := pointer.MoveToDisplay(uint32(ids[screen])); err != nil {
		return fmt.Errorf("desk: cannot bring the pointer to screen %d: %w",
			screen+1, err)
	}
	return nil
}

// PointerHome remembers where the pointer is, so a caller can put it back.
//
// A desk that moves the pointer and cannot undo it has taken something. The
// returned function is safe to call more than once and does nothing if the
// position could not be read in the first place.
func PointerHome() func() {
	was, err := pointer.Position()
	if err != nil {
		return func() {}
	}
	return func() { _ = pointer.MoveTo(was) }
}
