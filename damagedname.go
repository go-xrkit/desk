// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"strconv"
	"strings"
)

// ⛔⛔ A DISPLAY NAME WITH A NUL BYTE IN IT IS A BUG, NOT A DISPLAY NAME.
//
// Measured four times on 2026-09-06, in four different runs: a Go string that
// had been read correctly lost exactly its first four bytes while the program
// ran.
//
//	"VITURE Beast" -> "\x00\x00\x00\x00RE Beast"
//	"Thunderbird"  -> "\x00\x00\x00\x00derbird"
//
// A Go string cannot be assigned into, so something wrote through a pointer:
// heap corruption, in a program with no cgo. What has been ruled out, each by a
// measurement rather than an argument: a use-after-free READ (GODEBUG=
// clobberfree left the bytes zero rather than clobbered), every unsafe.Pointer
// conversion in every package (-gcflags=all=-d=checkptr stayed silent through a
// corruption), go-macos/virtualdisplay (5 displays created, 4000 listings),
// go-macos/screencapture (7 streams, 2769 frames), go-macos/accessibility (544
// full enumerations), and reading the settings file (2000 loads). What is left
// is a C callee writing through a pointer Go handed it, which neither
// instrument can see.
//
// ⛔ THE COST WAS THE WHOLE SESSION. The desk looks its display up by name, so a
// damaged name made it announce that the headset had been unplugged -- while
// the very same message listed that headset among the displays attached -- and
// stop, putting everything back. A person wearing the glasses had their desk
// taken away by a bug in a string.
//
// ⚠ AND THIS DOES NOT HIDE IT. Recovering silently would turn a session-ending
// fault into an invisible one, which is worse: nobody would ever collect
// another instance. It recovers AND says exactly what was damaged, so the next
// occurrence is still evidence.

// undamage reports the screen a damaged name was meant to be, when exactly one
// attached screen fits.
//
// It is deliberately strict. A name is only treated as damaged when it carries
// a NUL, which no real display name does, and the recovery must be
// UNAMBIGUOUS: the surviving tail has to match exactly one attached name, in
// the same place. Two candidates is not a recovery, it is a guess.
func undamage(want string, attached []string) (int, bool) {
	if !strings.ContainsRune(want, 0) {
		return 0, false
	}
	// What survived: everything after the last NUL. The damage seen was always
	// at the head, and a tail is what is left to recognise a name by.
	tail := want[strings.LastIndexByte(want, 0)+1:]
	if tail == "" {
		return 0, false
	}
	found, n := 0, 0
	for i, name := range attached {
		// Same length AND the same tail: a display whose name merely ends the
		// same way -- "XR desk 1" against "desk 1" -- is not this one.
		if len(name) == len(want) && strings.HasSuffix(name, tail) {
			found, n = i, n+1
		}
	}
	if n != 1 {
		return 0, false
	}
	return found, true
}

// damagedNameReport is what to say when a name arrived damaged: loudly, with
// both strings, because this is the only evidence a rare fault leaves.
func damagedNameReport(want, got string) string {
	return "⛔ the display name was DAMAGED in memory: asked for " +
		strconv.Quote(want) + ", which is " + strconv.Quote(got) +
		" with its first bytes overwritten. Carrying on with that display. " +
		"This is a real defect -- a Go string changed under the program -- and " +
		"the run that produced it is worth keeping"
}
