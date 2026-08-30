// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"testing"

	"github.com/go-xrkit/desk"
)

// TestRibbonIDsPutsTheMacFirst.
//
// The off-by-one this exists to stop: with screen 1 showing this Mac's own
// display, "the screens this program made" and "the screens on the band" are no
// longer the same list, and everything that turns a POSITION into a display --
// placing an application, taking a screen away, holding the pointer -- reads
// the wrong one for the whole session if it asks the wrong question.
func TestRibbonIDsPutsTheMacFirst(t *testing.T) {
	made := []uint64{101, 102, 103}

	got := ribbonIDs(true, 1, made)
	want := []uint64{1, 101, 102, 103}
	if len(got) != len(want) {
		t.Fatalf("ribbonIDs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ribbonIDs = %v, want %v", got, want)
		}
	}
	// And it does not disturb the slice it was given: appending to a slice with
	// spare capacity would write into the caller's.
	if made[0] != 101 {
		t.Errorf("the screens this program made were rewritten: %v", made)
	}

	// Not mirroring, and mirroring with nothing to mirror, are both the band as
	// this program made it.
	for _, c := range []struct {
		name   string
		mirror bool
		mac    uint64
	}{
		{"mirroring turned off", false, 1},
		{"no main display to show", true, 0},
	} {
		got := ribbonIDs(c.mirror, c.mac, made)
		if len(got) != len(made) || got[0] != made[0] {
			t.Errorf("%s: ribbonIDs = %v, want %v", c.name, got, made)
		}
	}
}

func TestMainDisplayIsTheOneThatSaysSo(t *testing.T) {
	offers := []desk.Offer{
		{ID: "display-101", Name: "XR screen 1", Kind: desk.KindDisplay},
		{ID: "display-2", Name: "display 2", Kind: desk.KindDisplay},
		{ID: "display-1", Name: "display 1 (main)", Kind: desk.KindDisplay, Main: true},
	}
	o, ok := mainDisplay(offers)
	if !ok || o.ID != "display-1" {
		t.Errorf("mainDisplay = %v,%v, want the one marked Main", o, ok)
	}
	// By the PROPERTY, not the label: a name that says "(main)" without the
	// field is a sentence, and reading a sentence back is how this breaks
	// silently the day the sentence changes.
	liar := []desk.Offer{{ID: "display-9", Name: "display 9 (main)", Kind: desk.KindDisplay}}
	if o, ok := mainDisplay(liar); ok {
		t.Errorf("mainDisplay = %v, want nothing: no offer says it is the main one", o)
	}
	if _, ok := mainDisplay(nil); ok {
		t.Error("mainDisplay found a screen among no offers")
	}
	// A panel this program renders onto is not a screen of the machine.
	if _, ok := mainDisplay([]desk.Offer{{ID: "panel-1", Kind: desk.KindPanel, Main: true}}); ok {
		t.Error("mainDisplay took a rendered panel for this Mac's screen")
	}
}
