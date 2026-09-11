// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"testing"

	"github.com/go-xrkit/desk"
	"github.com/go-xrkit/xrkit/glasses"
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

// TestThePlanCarriesTheDistanceAndTheSplay.
//
// ⛔⛔ FOR THREE WEEKS NEITHER OF THEM REACHED THE BAND. -distance and -splay
// were parsed, fetched from the settings when the flag was zero, threaded
// through four call sites and the session's own parameter list -- and left out
// of the Options the plan was built from. Go says nothing about a parameter
// nobody reads: `dist` and `splay` appeared EXACTLY ONCE each in a 677-line
// function, in its signature, and the build, the vet and the whole suite were
// green.
//
// It was found by using the flag as an instrument rather than trusting it: a
// capture taken with "-distance 2" came back with the note "curved 20.0° at
// 1.00x" beside it.
//
// So this asserts the one thing nobody was asserting -- that a number given on
// the command line is a number the band has.
func TestThePlanCarriesTheDistanceAndTheSplay(t *testing.T) {
	beast := glasses.Display{Name: "VITURE Beast", Width: 3840, Height: 1080}

	p, err := planFor(beast, 6, 2.5, 40, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Distance(); got != 2.5 {
		t.Errorf("the band sits at %g, want the 2.5 that was asked for", got)
	}
	if got := p.SplayDeg(); got != 40 {
		t.Errorf("the screens are splayed %g°, want the 40 that was asked for", got)
	}
	if got := p.Count(); got != 6 {
		t.Errorf("the band has %d screens, want 6", got)
	}

	// ⭐ AND ZERO STILL MEANS "NOBODY SAID", which is the convention the flags
	// rely on: it is what lets an unset flag fall through to the settings and
	// an unset setting fall through to the plan's own default. A test that only
	// checked the numbers above would pass on a planFor that ignored zero and
	// forced one.
	p, err = planFor(beast, 6, 0, 0, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Distance(); got != 1 {
		t.Errorf("a band nobody placed sits at %g, want 1", got)
	}
	if got := p.SplayDeg(); got != desk.DefaultSplayDeg {
		t.Errorf("a band nobody splayed is %g°, want %g", got, desk.DefaultSplayDeg)
	}

	// And a negative splay is the flat band, which is the other half of that
	// convention and the only way to ask for no curvature at all.
	p, err = planFor(beast, 6, 0, -1, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.SplayDeg(); got != 0 {
		t.Errorf("a band asked to be flat is splayed %g°", got)
	}
}
