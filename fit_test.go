// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-macos/hotkey"
)

// TestFitIsTheLargestOneScreenCanBeShown.
//
// Not a computed best and not "all of them at once": the near end of the
// distance scale is already defined as one screen filling the view exactly, so
// fitting is returning to it.
func TestFitIsTheLargestOneScreenCanBeShown(t *testing.T) {
	p := testPlan(t)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	// Push the band away first, so the return is not a no-op that would pass
	// whatever Fit did.
	for i := 0; i < 20; i++ {
		d.Do(ActionFurther)
	}
	if got := d.Plan().Distance(); got <= MinDistance {
		t.Fatalf("the band did not move away: distance %v", got)
	}
	away := d.Plan().Distance()

	d.Do(ActionFit)
	if got := d.Plan().Distance(); got != MinDistance {
		t.Errorf("after fit the distance is %v, want %v", got, MinDistance)
	}
	if err := d.Err(); err != nil {
		t.Errorf("fit: %v", err)
	}

	// And again from the far end, to show it does not depend on how far it had
	// wandered.
	for i := 0; i < 40; i++ {
		d.Do(ActionFurther)
	}
	if got := d.Plan().Distance(); got != MaxDistance {
		t.Fatalf("the band stopped at %v rather than the far end %v", got, MaxDistance)
	}
	d.Do(ActionFit)
	if got := d.Plan().Distance(); got != MinDistance {
		t.Errorf("from %v, fit gave %v, want %v", away, got, MinDistance)
	}
}

// TestFitIsIdempotent: pressing it at the near end changes nothing and refuses
// nothing. A key pressed blind must be safe to press twice.
func TestFitIsIdempotent(t *testing.T) {
	p := testPlan(t)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	d.Do(ActionFit)
	d.Do(ActionFit)
	if got := d.Plan().Distance(); got != MinDistance {
		t.Errorf("distance %v", got)
	}
	if err := d.Err(); err != nil {
		t.Errorf("pressing fit twice: %v", err)
	}
}

// TestFitIsClaimedOnZero, after the nine screen digits: the digits say "this
// screen", and the one that is not a screen number says "the screen, whole".
func TestFitIsClaimedOnZero(t *testing.T) {
	for _, s := range DefaultShortcuts() {
		if s.Does == ActionFit {
			if s.Want.Key != hotkey.KeyN0 {
				t.Errorf("fit is on %v, want zero", s.Want.Key)
			}
			if want := (hotkey.Combo{}); s.Want.Mods == want.Mods {
				t.Error("fit has no modifiers")
			}
			if got := ActionFit.String(); got != "fit" {
				t.Errorf("String() = %q", got)
			}
			return
		}
	}
	t.Fatal("fit is not claimed")
}

// TestEverythingClaimedSystemWideCanBeMoved.
//
// A key taken from the whole machine is a key taken from whatever the person
// was using, so whoever it inconveniences must be able to put it somewhere
// else. Quit is the deliberate exception: it is the way out of a desk that
// covers a display, and a person wearing glasses cannot see the menu bar.
func TestEverythingClaimedSystemWideCanBeMoved(t *testing.T) {
	byAction := map[Action]bool{}
	for _, s := range DefaultShortcuts() {
		byAction[s.Does] = true
	}
	named := map[Action]string{}
	for name, a := range configActions {
		named[a] = name
	}
	for a := range byAction {
		if a == ActionQuit {
			continue // the way out, deliberately fixed
		}
		if _, ok := named[a]; !ok {
			t.Errorf("%v is claimed system-wide but no settings file can move it", a)
		}
	}
	// And the names offered to a person mention the new ones.
	for _, want := range []string{"fit", "further", "closer", "apps"} {
		if !strings.Contains(actionNames(), want) {
			t.Errorf("%q is not offered: %s", want, actionNames())
		}
	}
}

// TestTheLayoutTheGlassesAskedForIsExpressible.
//
// The point of making these nameable: somebody can move the galleries off the
// arrows and put the distance there, with fit on Equal — without recompiling,
// and without this package deciding for them.
func TestTheLayoutTheGlassesAskedForIsExpressible(t *testing.T) {
	for _, name := range []string{"gallery-open", "apps-open", "further", "closer", "fit"} {
		if _, ok := actionByName(name); !ok {
			t.Errorf("%q cannot be bound from a settings file", name)
		}
	}
	// Spelled the way a settings file spells it, so the example in the README
	// is the thing that is tested.
	//
	// ⚠ "Equal", not "=". The separator between the parts of a combination is
	// "-", so Minus is written as a word, and Equal is written the same way for
	// the pair to read alike. Writing "=" is the obvious mistake, and the test
	// below pins that a person making it is TOLD rather than silently ignored.
	cfg := Config{Shortcuts: []ConfigShortcut{
		{Action: "further", Keys: "ctrl+alt+cmd+up"},
		{Action: "closer", Keys: "ctrl+alt+cmd+down"},
		{Action: "fit", Keys: "ctrl+alt+cmd+Equal"},
	}}
	if err := cfg.check(); err != nil {
		t.Fatalf("the layout was refused: %v", err)
	}
	got := cfg.ShortcutsOr(DefaultShortcuts())
	want := map[Action]hotkey.Key{
		ActionFurther: hotkey.KeyUpArrow,
		ActionCloser:  hotkey.KeyDownArrow,
		ActionFit:     hotkey.KeyEqual,
	}
	for _, s := range got {
		if k, ok := want[s.Does]; ok {
			if s.Want.Key != k {
				t.Errorf("%v is on %v, want %v", s.Does, s.Want.Key, k)
			}
			delete(want, s.Does)
		}
	}
	if len(want) != 0 {
		t.Errorf("%d of the rebindings did not take: %v", len(want), fmt.Sprint(want))
	}
}

// TestAKeyWrittenTheObviousWayIsRefusedWithTheNames.
//
// "=" is what a person reaches for, and it is not what this spells. Silently
// ignoring it would leave them pressing a key that does nothing and no way to
// find out why -- so it is refused, and the refusal lists the names.
func TestAKeyWrittenTheObviousWayIsRefusedWithTheNames(t *testing.T) {
	cfg := Config{Shortcuts: []ConfigShortcut{{Action: "fit", Keys: "ctrl+alt+cmd+="}}}
	err := cfg.check()
	if err == nil {
		t.Fatal(`"ctrl+alt+cmd+=" was accepted`)
	}
	if !strings.Contains(err.Error(), "Equal") {
		t.Errorf("the refusal does not offer the name to use: %v", err)
	}
}
