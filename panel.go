// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import "github.com/go-widgets/toolkit"

// A panel is where one run's composed frames are shown, and the one thing in
// [Run] that is not portable.
//
// ⛔⛔ IT IS NOT A COMPILE-TIME DIFFERENCE, WHICH IS WHY IT WAS MISSED.
// `window.Open` builds perfectly well for `GOOS=android` — go-widgets is pure
// Go — and then fails when it is CALLED, because a phone has no window server:
//
//	desk: listing displays: window: cannot enumerate screens: neither
//	    WAYLAND_DISPLAY nor DISPLAY is set
//
// So the ribbon reached a Pixel 11 Pro Fold, found the glasses, planned six
// screens across 309° — and had nowhere to put them. A seam that only existed
// in the build tags could not have caught that; this one is chosen at run time,
// on the platform's own terms.
//
// ⭐ THE TWO ROADS ARE GENUINELY DIFFERENT, and the interface is the shape they
// share rather than a lowest common denominator:
//
//	desktop  a full-screen window on the chosen display; the toolkit owns the
//	         loop and PULLS a frame when it repaints, and input is real
//	android  a Presentation on the display the glasses provide; we own the loop
//	         and PUSH a frame into a slot the host lends us, and there is no
//	         keyboard on a headset
//
// Both are driven by the same Repaint call from the same ribbon loop, so the
// cadence is the ribbon's on either road.
type panel interface {
	// Size is the framebuffer to compose for, in pixels.
	Size() (int, int)

	// Show presents until the panel closes or the platform's loop ends.
	//
	// frame is the pixels to show right now, as bytes, width and height. in
	// receives input where a platform has any; it may never be called.
	Show(frame func() ([]byte, int, int), in func(toolkit.Event)) error

	// Repaint asks for the newest frame to be shown, from any goroutine.
	//
	// The ribbon advances on a timer rather than on input — a desk full of
	// other people's windows moves whether or not the wearer does — so a panel
	// that could only repaint from its own event loop would show a still
	// picture. Where a back-end cannot do it, this is a no-op and the opening
	// said so.
	Repaint()

	// Close stops the presentation. Show returns once it has.
	Close() error
}

// openPanel opens the panel for the display the run chose, and displayStillThere
// says whether that display is still attached. Both are implemented once per
// road: panel_other.go for the window systems, panel_android.go for the host.
//
// ⚠ displayStillThere must speak the SAME error vocabulary on both roads —
// errDisplaysUnreadable for a list that could not be read, errDamagedName for a
// name corrupted in memory, anything else for a display that really went away.
// The watch in Run quits on that third case, so a platform answering a plain
// error where it meant "I cannot tell" would take a desk away from somebody
// wearing it. That has happened twice for other reasons; see displaylist.go.
