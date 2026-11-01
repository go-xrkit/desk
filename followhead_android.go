// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

// FollowHeadByDefault says whether a session should start following the head
// without being asked.
//
// ⛔⛔ TRUE ON ANDROID, AND THE REASON IS THAT NOTHING THERE CAN ASK. Measured
// on a Pixel 11 Pro Fold on 2026-11-01, with the ribbon already drawing on a
// VITURE Beast at 62 frames a second:
//
//	no global shortcut for ⌃⌥⌘F (follow your head): hotkey: unsupported on
//	    this platform (darwin only)
//	no trackpad gestures: multitouch: only macOS has this
//
// All three ways of sending [ActionFollowHead] — a global shortcut, a menu bar
// item, three fingers on a trackpad — are macOS. So the one platform where
// turning your head IS the interface was the one platform that could not turn
// head-following on.
//
// ⭐ AND IT IS WHAT THE PHONE IS FOR. Android's own desktop mode already puts a
// screen on the glasses; what it cannot do is a band wider than the view that
// scrolls as you look along it, which is this ribbon's whole point — six
// screens at 51.57° each, about 309° of band against roughly 50° of view.
// Starting without it would ship the one feature the platform already has.
//
// ⚠ `-head=false` turns it off, and the camera is asked for only when it is on:
// a session that cannot open the camera says so and carries on fixed to the
// glasses, which is [ActionTrackOff] and a perfectly good desk.
const FollowHeadByDefault = true
