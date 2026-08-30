// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

// Command deskchaos runs the desk over and over, breaks it on purpose, and
// checks what it leaves behind.
//
// # Why it exists
//
// Every defect this program has had in a week was found by a person wearing
// the glasses and saying so: a mouse that could not be found, a picture that
// slid away, a menu bar item that was built and never drawn, a screen showing
// itself. None of them were found by its tests, because all of them are about
// what a SESSION does to a machine, and its tests are about what functions do
// to values.
//
// This is the other half. It starts real sessions, with real displays, and
// asserts the things that can only be checked from outside a session:
//
//   - nothing is left behind. No virtual display outlives the process that
//     made it, and no backlight is left where the desk put it.
//   - nothing is shown that cannot be. The desk's own screen is never a screen
//     of its own band; every position draws the display its inventory names.
//   - nothing lies. A line saying the menu bar carries an item is only written
//     when AppKit agrees.
//
// # No glasses needed
//
// A virtual display named from the catalogue IS a headset as far as this
// program can tell -- glasses.Names offers those names, and glasses.Identify
// recognises what comes back. So the bench makes its own headset, and the only
// thing it cannot exercise is the optics.
//
// # It takes the machine
//
// Sessions warp the pointer sixty times a second, turn a panel's backlight
// off, create and destroy displays, and move applications between them; and
// this bench kills them halfway through on purpose. Nobody can use a machine
// while it runs, which is why it refuses to start without -take-the-machine.
package main

// # What it found on its first afternoon
//
//	── killed outright, the way a crash does
//	   ✗ display 1 was left at 0.00, from 0.79
//
// A session that dies without running its deferred calls leaves the Mac's
// panel dark, which is what a person met that morning after a panic. The bench
// puts it back; the program cannot, and that is the finding.
//
// It also found a defect in ITSELF twice over, which is worth writing down
// because both are the same mistake: measuring a machine that has not finished
// changing. Three displays were "still attached" the moment a session ended and
// gone a second later, and a backlight read 0.68 against 0.70 on a session that
// never touched it. An invariant that fires on a machine mid-change is an
// invariant nobody will believe the third time.

// # A night of it
//
// The bench is meant to run where nobody is: a Mac kept for it, with a
// self-hosted runner labelled xrdesk-bench and the Screen Recording and
// Accessibility grants. .github/workflows/nightly-bench.yml is that job.
//
//	deskchaos -take-the-machine -rounds 20 -budget 90m -report bench.json
//
// -budget gives the night an end, because stopping on time beats a run
// somebody has to kill in the morning. -report writes down the seed, and every
// round's fault, headset and arrangement: a defect that needs four screens, a
// mirror and a particular headset to appear is a defect nobody reproduces from
// "something went wrong".
