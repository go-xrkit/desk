// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"github.com/go-xrkit/android"
	"github.com/go-xrkit/xrkit/glasses"
)

// ⛔⛔ GOOS=android SATISFIES THE `linux` BUILD TAG, AND THAT IS HOW xrdesk
// REACHED THE PHONE AND THEN ASKED X11 WHERE THE GLASSES WERE.
//
// Measured on a Pixel 11 Pro Fold on 2026-11-01, from the program's own journal
// after it had started, found its host and opened its wall socket:
//
//	the menu bar item is built, with 15 rows
//	no menu bar item: exec: "dbus-launch": executable file not found in $PATH
//	desk: listing displays: window: cannot enumerate screens: neither
//	    WAYLAND_DISPLAY nor DISPLAY is set
//
// Nothing was broken: the binary ran, the host answered, the wall listened. The
// command simply asked the DESKTOP toolkit for the display list, and on a phone
// there is no window server to ask. The platform adapter beside this file knew
// better all along — `platform_linux.go` carries `//go:build linux && !android`
// precisely because of this — but `cmd/xrdesk` called `window.Screens` straight,
// so the one call that had to be per-platform was the one that was not.
//
// ⭐ THE DISPLAY LIST IS NOT A WINDOW-SYSTEM QUESTION ON ANDROID. It is the
// host's: the Java side sees `android.view.Display` for the built-in panel and
// for whatever is on the USB-C port, which is how the glasses announce
// themselves at all. Asking it needs no permission and no consent.

// displaysFromHost turns the Android host's display list into the portable
// shape the chooser reads.
//
// It is kept apart from the call that fetches the list so that the mapping can
// be tested on any machine: `android.Display` is a portable type — it is what
// crosses the socket — while `android.Screens` only exists where there is a
// host to ask.
//
// ⚠ Scale is 1 and that is not a placeholder. Android reports no separate
// logical-point space, so framebuffer pixels per point is exactly one; what a
// scale factor carries elsewhere is carried by DensityDPI, which the chooser
// does not read. Writing 0 here would make a caller dividing by it produce
// infinities on a platform where the answer is known.
func displaysFromHost(ds []android.Display) []glasses.Display {
	out := make([]glasses.Display, len(ds))
	for i, d := range ds {
		out[i] = glasses.Display{
			Name:   d.Name,
			Width:  d.Width,
			Height: d.Height,
			// The built-in panel is display 0, which is what Default reports.
			// "Primary" and "the one the phone is" are the same display here.
			Primary: d.Default(),
			Scale:   1,
		}
	}
	return out
}
