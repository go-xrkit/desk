// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build darwin

package desk

import (
	"fmt"
	"slices"

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

// PointerToHost puts the pointer on a display this desk did not make.
//
// ⛔⛔ THE WAY OUT USED TO BE UNPLUGGING THE HEADSET. The desk's screens are
// real displays to the window server, laid out BESIDE the physical ones in its
// coordinate space, so the pointer can leave past the right-hand edge of the
// last real screen and land on one that only the glasses show. Wearing them,
// that is the whole point and the band follows it. With them on the table it is
// a pointer nobody can see, on a desktop nobody can reach.
//
// ours is every display id the caller is responsible for -- the ones this
// program made, and the headset's own. The first display that is not one of
// them wins: CGGetActiveDisplayList puts the main display first, so on a Mac
// with one screen and a headset that is the screen somebody is sitting at.
//
// ⛔ IT MOVES TO THE MIDDLE OF A DISPLAY rather than restoring a remembered
// position. Where the pointer was before it wandered is not where it is wanted:
// it is wanted somewhere VISIBLE, now, and the middle of the main screen is the
// one place a person cannot fail to find it. [PointerHome] is the other thing,
// for a caller that moved the pointer itself and owes it back.
func PointerToHost(ours []uint64) error {
	ids, err := pointer.Displays()
	if err != nil {
		return fmt.Errorf("desk: cannot list the displays: %w", err)
	}
	for _, id := range ids {
		if slices.Contains(ours, uint64(id)) {
			continue
		}
		if err := pointer.MoveToDisplay(id); err != nil {
			return fmt.Errorf("desk: cannot bring the pointer to display %d: %w",
				id, err)
		}
		return nil
	}
	// Every display attached is one of ours. It is reachable -- a headset with
	// the Mac's own panel asleep is a machine whose only screens this program
	// made -- and it is not a failure of the pointer: there is nowhere else for
	// it to go, and saying so names the situation rather than the call.
	return fmt.Errorf("%w: all %d displays are the desk's own", ErrNoHostDisplay, len(ids))
}
