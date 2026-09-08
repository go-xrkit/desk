// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"strconv"
	"strings"
	"time"
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

// damagedNameQuiet is how long the report stays silent after saying itself.
//
// ⛔⛔ TEN MINUTES, AND THE NUMBER CAME FROM A LOG. The desk looks its display up
// once a second, so ONE corruption at startup printed the same 250-character
// sentence 3 939 times in a 66-minute session -- 3 939 lines out of 4 797, which
// is 82% of everything the run had to say. The whole reason this recovery is
// loud is that a rare fault leaves no other evidence; a log nobody can read
// leaves none either, and it buries the OTHER messages that might have named the
// culprit.
const damagedNameQuiet = 10 * time.Minute

// damagedNames says a damaged name once, then holds its tongue.
//
// ⭐ IT STILL SPEAKS AGAIN, and that is deliberate: a session that ends abruptly
// -- which is what this fault used to do -- must still show that the damage was
// STILL THERE at the end, not only that it happened once at the start. So the
// repeat carries the count, and a run of any length says how bad it was.
//
// The zero value is ready to use.
type damagedNames struct {
	seen map[string]int
	said map[string]time.Time
}

// due counts an occurrence of key and says whether it is time to speak, and how
// many there have been. Every occurrence is counted whether or not it is spoken.
func (d *damagedNames) due(now time.Time, key string) (int, bool) {
	if d.seen == nil {
		d.seen, d.said = map[string]int{}, map[string]time.Time{}
	}
	d.seen[key]++
	last, spoken := d.said[key]
	if spoken && now.Sub(last) < damagedNameQuiet {
		return d.seen[key], false
	}
	d.said[key] = now
	return d.seen[key], true
}

// report is what to log for this damage, or "" to say nothing this time.
func (d *damagedNames) report(now time.Time, want, got string) string {
	n, speak := d.due(now, want+"\x00->\x00"+got)
	switch {
	case !speak:
		return ""
	case n == 1:
		return damagedNameReport(want, got)
	}
	return "⛔ the display name is STILL damaged: " + strconv.Quote(want) +
		", seen " + strconv.Itoa(n) + " times now"
}

// reportOnce is the same throttle for a sentence that is already written: it
// the first time, then a short line carrying the count.
//
// ⭐ IT EXISTS BECAUSE A SECOND CASE ARRIVED. An empty display list is a failed
// read rather than an unplugged screen (displaylist.go); it repeats once a
// second exactly as a damaged name does, and it needs the same treatment --
// said, then counted -- rather than a second copy of this logic.
func (d *damagedNames) reportOnce(now time.Time, key, msg string) string {
	n, speak := d.due(now, key)
	switch {
	case !speak:
		return ""
	case n == 1:
		return msg
	}
	return "⛔ " + key + " is STILL happening, seen " + strconv.Itoa(n) + " times now"
}
