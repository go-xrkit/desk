// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"strings"
	"testing"
)

// ⭐ EXACTLY ONE OF THE THREE CARRIES THE TICK, and it is the one the HEADSET is
// in -- not the one last clicked. The glasses have their own button, so a menu
// that remembered its own clicks would be wrong from the first press of it.
func TestTheTickIsOnTheModeTheHeadsetIsIn(t *testing.T) {
	rows := []Action{ActionTrackOff, ActionTrackAnchored, ActionTrackSmooth}
	for mode, want := range map[int]Action{0: ActionTrackOff, 1: ActionTrackAnchored, 2: ActionTrackSmooth} {
		ticked := 0
		for _, a := range rows {
			on, why := stateFor(a, Stereo3D{}, true, Tracking{Mode: mode})
			if why != "" {
				t.Errorf("mode %d: %v is unavailable for %q", mode, a, why)
			}
			if on {
				ticked++
				if a != want {
					t.Errorf("mode %d ticks %v, want %v", mode, a, want)
				}
			}
		}
		if ticked != 1 {
			t.Errorf("mode %d ticks %d rows, want exactly one", mode, ticked)
		}
	}
}

// ⛔ THE REASON IS SAID ON EVERY ONE OF THE THREE, because macOS has no notion
// of a disabled GROUP: a row that is grey with nothing beside it is a row
// somebody presses again, and then asks why nothing happened.
func TestWhyItCannotTrackIsSaidOnEveryRow(t *testing.T) {
	const why = "these glasses are passing the picture through"
	for _, a := range []Action{ActionTrackOff, ActionTrackAnchored, ActionTrackSmooth, ActionRecenter} {
		on, got := stateFor(a, Stereo3D{}, true, Tracking{Mode: 1, Why: why})
		if got != why {
			t.Errorf("%v says %q, want %q", a, got, why)
		}
		if on {
			t.Errorf("%v is ticked while it cannot be used", a)
		}
	}
}

// Recentring is something that HAPPENS, so it is never ticked -- a tick on it
// would be one that never turns off.
func TestRecentringIsNeverTicked(t *testing.T) {
	for mode := range 3 {
		if on, _ := stateFor(ActionRecenter, Stereo3D{}, true, Tracking{Mode: mode}); on {
			t.Errorf("recentring is ticked in mode %d", mode)
		}
	}
}

func TestEachTrackingActionAsksForOneMode(t *testing.T) {
	for a, want := range map[Action]int{
		ActionTrackOff: 0, ActionTrackAnchored: 1, ActionTrackSmooth: 2,
	} {
		got, ok := trackingFor(a)
		if !ok {
			t.Errorf("%v is not a tracking action", a)
		}
		if got != want {
			t.Errorf("%v asks for mode %d, want %d", a, got, want)
		}
	}
	// Recentring is not one of the three: it has no mode to ask for.
	if _, ok := trackingFor(ActionRecenter); ok {
		t.Error("recentring was taken for a mode")
	}
	if _, ok := trackingFor(ActionStereo3D); ok {
		t.Error("the 3D row was taken for a tracking mode")
	}
}

// Every one of the four rows says what it does, because that sentence is what a
// notice shows after the key is pressed.
func TestEachRowSaysWhatItDoes(t *testing.T) {
	for a, want := range map[Action]string{
		ActionTrackAnchored: "anchor", ActionTrackSmooth: "smoothly",
		ActionTrackOff: "fix", ActionRecenter: "back in front",
	} {
		if got := a.String(); !strings.Contains(got, want) {
			t.Errorf("%d says %q, want something containing %q", int(a), got, want)
		}
	}
}

// ⛔ THE FOUR ROWS GO THROUGH THE DESK, and these are the paths they take: a
// desk whose glasses do not track, one whose headset refuses, and one that
// accepts. All three end in a sentence on the picture, because a key pressed
// blind that says nothing is indistinguishable from a key that did nothing.
func TestWhatTheDeskDoesWithEachTrackingRow(t *testing.T) {
	newDesk := func(on func(int, bool) error) *Desk {
		d := &Desk{OnTracking: on}
		d.Badge(1, nil, nil)
		return d
	}

	// No glasses that track: said, not silent.
	d := newDesk(nil)
	d.Do(ActionTrackAnchored)
	if !strings.Contains(d.notice.toast.Text, "do not track") {
		t.Errorf("a desk with no tracking said %q", d.notice.toast.Text)
	}

	// A headset that refuses: what it said is what is shown.
	d = newDesk(func(int, bool) error { return errAsked })
	d.Do(ActionTrackSmooth)
	if !strings.Contains(d.notice.toast.Text, "nothing to anchor") {
		t.Errorf("a refusal said %q", d.notice.toast.Text)
	}

	// Accepted: each of the three asks for its own mode.
	for a, want := range map[Action]int{
		ActionTrackOff: 0, ActionTrackAnchored: 1, ActionTrackSmooth: 2,
	} {
		var got int
		var recentre bool
		d = newDesk(func(m int, r bool) error { got, recentre = m, r; return nil })
		d.Do(a)
		if got != want || recentre {
			t.Errorf("%v asked for mode %d recentre=%v, want %d false", a, got, recentre, want)
		}
		if !strings.Contains(d.notice.toast.Text, a.String()) {
			t.Errorf("%v said %q", a, d.notice.toast.Text)
		}
	}

	// Recentring asks for exactly that, and says so in its own words.
	var recentre bool
	d = newDesk(func(_ int, r bool) error { recentre = r; return nil })
	d.Do(ActionRecenter)
	if !recentre {
		t.Error("recentring did not ask to recentre")
	}
	if !strings.Contains(d.notice.toast.Text, "back in front of you") {
		t.Errorf("recentring said %q", d.notice.toast.Text)
	}
}

var errAsked = errTracking("these glasses are passing the picture through, so there is nothing to anchor")

type errTracking string

func (e errTracking) Error() string { return string(e) }
