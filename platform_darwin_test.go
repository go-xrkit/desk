// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import "testing"

// TestARefusedWidthIsNotAskedForForever.
//
// ⛔⛔ THE DEFECT THIS EXISTS FOR. The loop asked for the SAME width three
// times, five seconds apart, on the belief that "never became active within 5s"
// meant a busy window server. Measured 2026-09-07 that belief is only half
// true: 3840 wide is refused at every height tried, while 3839, 3841 and even
// 5120 and 6400 -- far wider -- open in half a second. A width that is refused
// does not become acceptable by asking for it again, so the third ask has to be
// for something else.
func TestARefusedWidthIsNotAskedForForever(t *testing.T) {
	const planned = 3840
	got := widthsToTry(planned)

	if len(got) < displayTries+1 {
		t.Fatalf("widthsToTry(%d) = %v: too few attempts to ever leave the planned width",
			planned, got)
	}
	for i := 0; i < displayTries; i++ {
		if got[i] != planned {
			t.Errorf("attempt %d is %d, want the planned %d: a busy window server is "+
				"real and is the cheap cause to rule out first", i+1, got[i], planned)
		}
	}
	// After that, every attempt must be a DIFFERENT width, or it is five
	// seconds spent on an answer already given.
	seen := map[int]bool{planned: true}
	for _, w := range got[displayTries:] {
		if seen[w] {
			t.Errorf("width %d is asked for twice in %v: a refusal already answered that",
				w, got)
		}
		seen[w] = true
		if d := w - planned; d != -1 && d != 1 {
			t.Errorf("width %d is %d away from the planned %d: the layout is built to "+
				"the planned width, and a headset's size follows its optics -- one pixel "+
				"is the most that may be taken", w, d, planned)
		}
	}
	// Smaller before wider: a screen wider than planned would not fit the ribbon.
	if got[displayTries] != planned-1 {
		t.Errorf("first nudge is %d, want %d: smaller has to be tried first",
			got[displayTries], planned-1)
	}
}

// A one-pixel screen has no smaller neighbour, and asking for a zero-wide
// display is worse than not asking.
func TestTheNudgeNeverReachesZero(t *testing.T) {
	for _, planned := range []int{1, 2} {
		for _, w := range widthsToTry(planned) {
			if w < 1 {
				t.Errorf("widthsToTry(%d) offers %d", planned, w)
			}
		}
	}
}
