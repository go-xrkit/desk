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
	// ⛔⛔ BY IDENTITY, NOT BY POSITION. This walked the top level and paired
	// rows with menu items by index, and the day rows were grouped into
	// submenus it reported the curve as missing from a menu it was in. A row's
	// ACTION is what does not move when the menu is rearranged or a title is
	// reworded -- and two titles were reworded the same week.
	for i, r := range FlatRows() {
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
	// The separators are a property of the TOP level, where the rules are drawn.
	for i, r := range rows {
		if r.IsSeparator() {
			seps++
			continue
		}
		if r.Action == ActionNone {
			// A submenu row: it opens something rather than doing something.
			if len(r.Rows) == 0 {
				t.Errorf("row %d (%q) has no action and nothing under it", i, r.Title)
			}
			if r.Title == "" {
				t.Errorf("row %d holds %d rows and says nothing", i, len(r.Rows))
			}
		}
	}
	if seps == 0 {
		t.Error("the menu has no separator; the settings row reads as one of the " +
			"gallery rows")
	}
	// Neither the first nor the last row is a separator: a menu that opens or
	// ends with a rule has a gap in it where a person expects a row.
	if rows[0].IsSeparator() || rows[len(rows)-1].IsSeparator() {
		t.Error("the menu begins or ends with a separator")
	}
	// ⭐ AND THE GROUPING ACTUALLY SHORTENED IT. Thirty-one rows was the
	// complaint; a submenu mechanism that nobody used would leave it at
	// thirty-one and pass every other assertion here.
	if len(rows) >= len(FlatRows()) {
		t.Errorf("the top level has %d rows and the whole menu %d: nothing is "+
			"grouped", len(rows), len(FlatRows()))
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

// TestEveryTickIsWhereItCanBeSeen.
//
// ⛔⛔ macOS DRAWS NOTHING FOR A ROW THAT IS OFF. A checkbox that is off is
// pixel-for-pixel an ordinary row, so a binary tick hidden inside a submenu
// answers "is this on?" only for somebody who already went and looked. The
// grouping was built on that rule and then broke it on the row it most
// obviously covers: "Use the glasses" sat under the six groups until somebody
// said "remonte use the glasses avec les autres en haut".
//
// ⭐ A SET OF MUTUALLY EXCLUSIVE TICKS IS DIFFERENT, and that is why this is not
// simply "no toggles in submenus". One of the three tracking rows is ALWAYS
// ticked, so opening the group answers the question completely; it is the lone
// toggle, whose "off" is silence, that has to be visible without opening
// anything.
func TestEveryTickIsWhereItCanBeSeen(t *testing.T) {
	top := map[Action]bool{}
	for _, r := range TrayRows() {
		top[r.Action] = true
	}
	for _, parent := range TrayRows() {
		if len(parent.Rows) == 0 {
			continue
		}
		ticks := 0
		for _, r := range parent.Rows {
			if r.Toggle {
				ticks++
			}
		}
		for _, r := range parent.Rows {
			if r.Toggle && ticks < 2 {
				t.Errorf("%q is the only tick in %q: off draws nothing, so it "+
					"cannot be told from an ordinary row without opening the "+
					"group. Put it at the top level.", r.Title, parent.Title)
			}
		}
	}
	// And the rows the rule was written for are where it says.
	for _, a := range []Action{ActionStereo3D, ActionFollowHead, ActionPause} {
		if !top[a] {
			t.Errorf("%v carries a tick and is not at the top level", a)
		}
	}
}
