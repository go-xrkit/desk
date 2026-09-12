// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
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
			on, why := stateFor(a, Stereo3D{}, true, Tracking{Mode: mode}, false)
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
		on, got := stateFor(a, Stereo3D{}, true, Tracking{Mode: 1, Why: why}, false)
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
		if on, _ := stateFor(ActionRecenter, Stereo3D{}, true, Tracking{Mode: mode}, false); on {
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

// TestTheFollowMyHeadRowCarriesATick.
//
// ⛔⛔ IT NEVER DID. The row is declared a toggle, with an open eye and a filled
// one for its two states, and stateFor had no case for it -- so it fell through
// to the default and answered "off" for the life of the session. macOS draws
// NOTHING for an unticked row, so a feature that was running looked exactly like
// one that was not: "lorsque follow my head est activé il faut avoir un coche
// sur le menu".
func TestTheFollowMyHeadRowCarriesATick(t *testing.T) {
	if on, why := stateFor(ActionFollowHead, Stereo3D{}, true, Tracking{}, true); !on || why != "" {
		t.Errorf("following a head: on=%v why=%q", on, why)
	}
	if on, _ := stateFor(ActionFollowHead, Stereo3D{}, true, Tracking{}, false); on {
		t.Error("not following a head, and the row is ticked")
	}
	// ⛔ AND IT IS NOT GREYED OUT BY THE HEADSET'S OWN TRACKING. The three
	// tracking rows go grey while the glasses are passing the picture through;
	// this one opens a camera on the Mac's side and has nothing to do with them.
	if _, why := stateFor(ActionFollowHead, Stereo3D{},
		true, Tracking{Why: "passing through"}, true); why != "" {
		t.Errorf("the row was disabled because the headset said %q", why)
	}
}

// TestTheMenuIsToldWhatHappenedAndNotWhatWasAsked.
//
// ⛔ ASKING TO FOLLOW A HEAD OPENS A CAMERA, AND IT CAN REFUSE -- a headset
// plugged in for its picture only presents none. A tick that moved because a row
// was clicked would then say the desk is following a head it cannot see. So the
// seam is told on EVERY path, with what the desk actually ended up doing.
func TestTheMenuIsToldWhatHappenedAndNotWhatWasAsked(t *testing.T) {
	d, h := deskForHead(t)
	var said []bool
	d.OnFollowingHead = func(on bool) { said = append(said, on) }

	d.Do(ActionFollowHead)
	if len(said) != 1 || !said[0] {
		t.Fatalf("switching on told the menu %v", said)
	}
	d.Do(ActionFollowHead)
	if len(said) != 2 || said[1] {
		t.Fatalf("switching off told the menu %v", said)
	}
	_ = h

	// A desk with no camera to open: the attempt is refused, and the menu is
	// told the truth rather than left showing the row as pressed.
	//
	// ⛔ RESTORED AFTERWARDS. openCameraHead is package state, and a test that
	// leaves it holding a stub hands every later test a machine with no camera
	// -- a failure that appears in a file nobody touched.
	was := openCameraHead
	t.Cleanup(func() { openCameraHead = was })
	d.SetHeadSource(nil)
	openCameraHead = func(string) (HeadSource, func() error, error) {
		return nil, nil, errNoCameraHere
	}
	d.Do(ActionFollowHead)
	if len(said) != 3 || said[2] {
		t.Fatalf("a refused camera told the menu %v", said)
	}
}

var errNoCameraHere = errors.New("these glasses have no camera")
