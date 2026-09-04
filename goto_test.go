// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/go-macos/hotkey"
	"github.com/go-xrkit/xrkit/ribbon"
)

// TestThereIsADigitForEveryScreenAndNoneSpare.
//
// MaxScreens is nine and there are nine digit actions. If the ceiling ever
// moves, this fails — which is the point: a tenth screen with no key is a
// screen only the gallery can reach, and that should be a decision rather than
// an oversight.
func TestThereIsADigitForEveryScreenAndNoneSpare(t *testing.T) {
	if got := int(ActionScreen9-ActionScreen1) + 1; got != MaxScreens {
		t.Errorf("%d screen actions for %d screens", got, MaxScreens)
	}
	for i := 0; i < MaxScreens; i++ {
		a := ActionScreen1 + Action(i)
		n, ok := screenOf(a)
		if !ok || n != i {
			t.Errorf("screenOf(%v) = %d,%v want %d,true", a, n, ok, i)
		}
		if want := fmt.Sprintf("screen %d", i+1); a.String() != want {
			t.Errorf("String() = %q, want %q", a.String(), want)
		}
	}
	for _, a := range []Action{ActionNone, ActionNext, ActionChoose, ActionScreen1 - 1, ActionScreen9 + 1} {
		if _, ok := screenOf(a); ok {
			t.Errorf("%v was taken for a screen action", a)
		}
	}
}

// TestGoingStraightToAScreenFromTheBand.
func TestGoingStraightToAScreenFromTheBand(t *testing.T) {
	p := testPlan(t)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	for _, want := range []int{3, 0, p.Count() - 1, 1} {
		d.Do(ActionScreen1 + Action(want))
		d.Advance(10) // let the turn finish
		if got := d.Nav().Focus(); got != want {
			t.Errorf("after screen %d, the focus is %d", want+1, got)
		}
		if err := d.Err(); err != nil {
			t.Errorf("screen %d: %v", want+1, err)
		}
	}
}

// TestAScreenThatIsNotThereSaysHowManyThereAre.
//
// The nine combinations are registered whatever the desk carries today, so
// pressing one for a screen that does not exist is an ordinary thing to do.
// What it must not do is fail silently or blame the person without telling
// them the number.
func TestAScreenThatIsNotThereSaysHowManyThereAre(t *testing.T) {
	p := testPlan(t)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	was := d.Nav().Focus()

	d.Do(ActionScreen9)
	err = d.Err()
	if err == nil {
		t.Fatal("a screen that is not there was accepted")
	}
	if !errors.Is(err, ErrPosition) {
		t.Errorf("error = %v, want it to read as a position problem", err)
	}
	if !strings.Contains(err.Error(), fmt.Sprintf("this desk has %d", p.Count())) {
		t.Errorf("error = %q, which does not say how many there are", err)
	}
	if got := d.Nav().Focus(); got != was {
		t.Errorf("the band moved to %d anyway", got)
	}
}

// TestOneKeyMeansOneThingWhereverItIsPressed.
//
// A system-wide shortcut is pressed blind. ⌃⌥⌘3 with the gallery open has to
// take the viewer to screen three, not mean something else because of where
// they had lost track of being.
func TestOneKeyMeansOneThingWhereverItIsPressed(t *testing.T) {
	p := testPlan(t)

	t.Run("from the screen gallery", func(t *testing.T) {
		d, err := New(p, feedsFor(p))
		if err != nil {
			t.Fatal(err)
		}
		defer d.Close()

		d.Do(ActionGalleryOpen)
		if d.Nav().Mode() != ribbon.ModeGallery {
			t.Fatal("the gallery did not open")
		}
		d.Do(ActionScreen3)
		d.Advance(10)
		if err := d.Err(); err != nil {
			t.Fatal(err)
		}
		if d.Nav().Mode() != ribbon.ModeRibbon {
			t.Error("the gallery is still up")
		}
		if got := d.Nav().Focus(); got != 2 {
			t.Errorf("the focus is %d, want screen 3", got+1)
		}
	})

	t.Run("from the application list", func(t *testing.T) {
		d, err := New(p, feedsFor(p))
		if err != nil {
			t.Fatal(err)
		}
		defer d.Close()
		d.OnApps = func() ([]App, error) { return []App{{Name: "Something"}}, nil }

		d.Do(ActionAppsOpen)
		if !d.inApps {
			t.Fatal("the application list did not open")
		}
		d.Do(ActionScreen2)
		d.Advance(10)
		if err := d.Err(); err != nil {
			t.Fatal(err)
		}
		if d.inApps {
			t.Error("the application list is still up")
		}
		if got := d.Nav().Focus(); got != 1 {
			t.Errorf("the focus is %d, want screen 2", got+1)
		}
	})
}

// TestTheDigitsAreClaimedSystemWide, on the modifier set the gallery's own
// keys already use.
func TestTheDigitsAreClaimedSystemWide(t *testing.T) {
	byAction := map[Action]hotkey.Combo{}
	for _, s := range DefaultShortcuts() {
		byAction[s.Does] = s.Want
	}
	keys := []hotkey.Key{
		hotkey.KeyN1, hotkey.KeyN2, hotkey.KeyN3, hotkey.KeyN4, hotkey.KeyN5,
		hotkey.KeyN6, hotkey.KeyN7, hotkey.KeyN8, hotkey.KeyN9,
	}
	want := byAction[ActionChoose].Mods // the gallery's own modifier set
	for i, k := range keys {
		a := ActionScreen1 + Action(i)
		c, ok := byAction[a]
		if !ok {
			t.Errorf("%v is not claimed", a)
			continue
		}
		if c.Key != k {
			t.Errorf("%v is on key %v, want %v", a, c.Key, k)
		}
		if c.Mods != want {
			t.Errorf("%v uses %v, want the same modifiers as choose (%v)", a, c.Mods, want)
		}
	}
}

// TestASettingsFileCanMoveThem.
func TestASettingsFileCanMoveThem(t *testing.T) {
	for i := 1; i <= MaxScreens; i++ {
		name := fmt.Sprintf("screen-%d", i)
		a, ok := actionByName(name)
		if !ok {
			t.Errorf("%q is not a name a settings file may use", name)
			continue
		}
		if want := ActionScreen1 + Action(i-1); a != want {
			t.Errorf("%q is %v, want %v", name, a, want)
		}
	}
	if !strings.Contains(actionNames(), "screen-1") {
		t.Errorf("the list of names offered to a person does not mention them: %s", actionNames())
	}
}
