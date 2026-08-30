// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build darwin

package desk

import "github.com/go-macos/objc"

// platformPump gives AppKit the main thread for a moment.
//
// It is what makes the menu-bar item REAL while the desk is waiting for a pair
// of glasses. go-macos/statusitem says it plainly: "a status item in a process
// whose main thread never runs a loop is an object with no window, and it is
// indistinguishable -- from Go -- from one that works". That is exactly what
// this was: the item was built, the log said so, and nothing was drawn, because
// the only run loop in this program is the one go-widgets/window starts once a
// display has been chosen.
//
// A pump rather than a run: the wait has to keep polling for the glasses, and
// -[NSApplication run] does not return.
func platformPump() { _ = objc.PumpRunLoop(pumpSlice) }

// pumpSlice is how long AppKit gets each time.
//
// Long enough that a press is delivered and AppKit's own menu tracking takes
// over -- it runs a nested loop of its own from there, so this only has to get
// the press through -- and short enough that the wait still notices the glasses
// being plugged in, and a Quit chosen from that very menu.
const pumpSlice = 0.05
