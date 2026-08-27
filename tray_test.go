// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import "testing"

// TestTrayRows: every row says something, asks for something a desk can do, and
// no two rows claim the same key.
//
// The menu is the one part of this application a person reads rather than
// remembers, so a row that does nothing, or a key that means two things, is a
// defect they meet immediately -- and neither is visible from the code that
// builds the menu, which happily accepts both.
func TestTrayRows(t *testing.T) {
	rows := TrayRows()
	if len(rows) < 3 {
		t.Fatalf("the menu has %d rows", len(rows))
	}

	keys := map[string]string{}
	actions := map[Action]string{}
	seps := 0
	for i, r := range rows {
		if r.Action == ActionNone {
			seps++
			if r.Title != "" || r.Key != "" {
				t.Errorf("row %d is a separator carrying %q/%q", i, r.Title, r.Key)
			}
			continue
		}
		if r.Title == "" {
			t.Errorf("row %d asks for %v and says nothing", i, r.Action)
		}
		// A row's action has to be one the desk answers. ActionNone is the
		// separator, and String() reporting "none" is how an action that is not
		// in the table gives itself away.
		if r.Action.String() == "none" {
			t.Errorf("row %d (%q) asks for an action the desk does not know", i, r.Title)
		}
		if was, seen := actions[r.Action]; seen {
			t.Errorf("row %d (%q) repeats the action of %q", i, r.Title, was)
		}
		actions[r.Action] = r.Title
		if r.Key == "" {
			continue
		}
		// One character: AppKit DRAWS a longer key equivalent in the menu and
		// then never matches a keystroke against it, so the row looks bound and
		// is not.
		if len([]rune(r.Key)) != 1 {
			t.Errorf("row %d (%q) has the key %q, which is not one character",
				i, r.Title, r.Key)
		}
		if was, seen := keys[r.Key]; seen {
			t.Errorf("row %d (%q) claims the key %q, which %q already has",
				i, r.Title, r.Key, was)
		}
		keys[r.Key] = r.Title
	}
	if seps == 0 {
		t.Error("the menu has no separator; the settings row reads as one of the " +
			"gallery rows")
	}
	// Neither the first nor the last row is a separator: a menu that opens or
	// ends with a rule has a gap in it where a person expects a row.
	if rows[0].Action == ActionNone || rows[len(rows)-1].Action == ActionNone {
		t.Error("the menu begins or ends with a separator")
	}

	// The two rows that matter are there, because this is the whole reason the
	// item exists: the settings cannot be reached from inside the glasses, and
	// neither can quitting when the shortcut was refused.
	for _, want := range []Action{ActionSettings, ActionQuit} {
		if _, ok := actions[want]; !ok {
			t.Errorf("the menu cannot ask for %v", want)
		}
	}
}

// TestTheSettingsActionStopsTheDeskAndSaysWhy.
//
// Both flags, and they are different questions: the ribbon has to come down
// either way -- one window at a time, and the ribbon owns a whole display -- and
// only the caller knows whether to put a settings window in its place or to
// stop.
func TestTheSettingsActionStopsTheDeskAndSaysWhy(t *testing.T) {
	p := testPlan(t)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	if d.Quit() || d.WantsSettings() {
		t.Fatal("a fresh desk already wants to stop")
	}
	d.Do(ActionSettings)
	if !d.Quit() {
		t.Error("the desk did not stop, so the settings window cannot open")
	}
	if !d.WantsSettings() {
		t.Error("the desk stopped without saying it was for the settings, so the " +
			"session ends instead")
	}

	// And quitting is NOT asking for the settings: the two must not be confused,
	// or the desk comes back after a person has told it to stop.
	d2, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	defer d2.Close()
	d2.Do(ActionQuit)
	if !d2.Quit() || d2.WantsSettings() {
		t.Errorf("quit gave quit=%v settings=%v", d2.Quit(), d2.WantsSettings())
	}
}

// TestTheSettingsActionWorksFromTheGalleryToo: the menu is chosen blind, and a
// person in the gallery who asks for the settings must not be told nothing.
func TestTheSettingsActionWorksFromTheGalleryToo(t *testing.T) {
	p := testPlan(t)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	d.Do(ActionGalleryOpen)
	d.Do(ActionSettings)
	if !d.Quit() || !d.WantsSettings() {
		t.Errorf("from the gallery: quit=%v settings=%v", d.Quit(), d.WantsSettings())
	}
}
