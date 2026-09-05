// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"context"
	"errors"

	"testing"
	"time"

	"github.com/go-macos/hotkey"
	"github.com/go-xrkit/xrkit/glasses"
)

// TestARestingWaitIgnoresTheDisplay is the whole point of Resting, and the one
// thing that could not be got right by accident.
//
// ⛔ THE GLASSES ARE STILL PLUGGED IN when somebody takes them off. An ordinary
// wait starts the moment a headset is in the list, so it would start again in
// the same second and there would be no way to stop short of quitting -- which
// is the thing this exists to avoid. So a resting wait must NOT return the
// display that is sitting right there.
func TestARestingWaitIgnoresTheDisplay(t *testing.T) {
	list, calls := lister([]glasses.Display{theGlasses})
	actions := make(chan Action, 1)
	go func() {
		time.Sleep(30 * time.Millisecond)
		actions <- ActionPause
	}()
	got, err := Await(context.Background(), AwaitOptions{
		Want: "VITURE", List: list, Actions: actions, Resting: true,
		Every: time.Millisecond,
	})
	if !errors.Is(err, ErrAwaitResume) {
		t.Fatalf("a resting wait ended with %v and the display %v", err, got)
	}
	if got != (glasses.Display{}) {
		t.Errorf("a resting wait handed back a display: %v", got)
	}
	// And it never even asked. Listing displays is what an ordinary wait does
	// once a second; resting waits for a person, so it has nothing to poll.
	if *calls != 0 {
		t.Errorf("a resting wait listed displays %d times", *calls)
	}
}

// TestARestingWaitAnswersTheMenuBar: every row that ends a wait still ends it,
// because while the glasses are down the menu is the ONLY control left alive --
// the shortcuts went back to the rest of the machine with the ribbon.
func TestARestingWaitAnswersTheMenuBar(t *testing.T) {
	for _, c := range []struct {
		a    Action
		want error
	}{
		{ActionPause, ErrAwaitResume},
		{ActionQuit, ErrAwaitQuit},
		{ActionSettings, ErrAwaitSettings},
	} {
		actions := make(chan Action, 2)
		// One that needs a ribbon first: there is none, so it must be
		// swallowed rather than mistaken for an answer.
		actions <- ActionNext
		actions <- c.a
		_, err := Await(context.Background(), AwaitOptions{
			List:    func() ([]glasses.Display, error) { return nil, nil },
			Actions: actions, Resting: true,
		})
		if !errors.Is(err, c.want) {
			t.Errorf("%v while resting gave %v, want %v", c.a, err, c.want)
		}
	}
}

// TestARestingWaitStopsWithTheContext: it waits for a person, and a process
// going down is not a person.
func TestARestingWaitStopsWithTheContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Await(ctx, AwaitOptions{
		List:    func() ([]glasses.Display, error) { return nil, nil },
		Resting: true,
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("a cancelled resting wait gave %v", err)
	}
}

// TestPuttingTheGlassesDownIsNotQuitting: ActionPause stops the ribbon like
// ActionQuit and ActionSettings do, and says which of the three it was.
//
// ⛔ THE CALLER CANNOT ACT ON Quit ALONE. All three bring the ribbon down, and
// only the second question says whether to open a window, wait for a person, or
// return -- so a pause that did not distinguish itself would be a quit.
func TestPuttingTheGlassesDownIsNotQuitting(t *testing.T) {
	p := testPlan(t)
	d, _ := New(p, feedsFor(p))
	d.Do(ActionPause)
	if !d.Quit() {
		t.Error("putting the glasses down left the ribbon up")
	}
	if !d.WantsPause() {
		t.Error("putting the glasses down did not say so")
	}
	if d.WantsSettings() {
		t.Error("putting the glasses down asked for the settings window")
	}

	// And the other way round, so the two are not one flag with two names.
	s, _ := New(p, feedsFor(p))
	s.Do(ActionSettings)
	if !s.WantsSettings() || s.WantsPause() {
		t.Errorf("the settings row reads as settings=%v paused=%v",
			s.WantsSettings(), s.WantsPause())
	}
	q, _ := New(p, feedsFor(p))
	q.Do(ActionQuit)
	if q.WantsPause() || q.WantsSettings() {
		t.Error("quitting reads as something other than quitting")
	}
}

// TestPuttingThemDownWorksFromEitherGalleryToo: the row is chosen from a menu
// bar, and a menu bar is reachable from wherever the person happens to be.
//
// ⛔ EACH VIEW HAS ITS OWN SWITCH. The desk answers actions in three places --
// the ribbon, the screen gallery, the applications -- and a row that works from
// one of them and silently does nothing from the other two is worse than no row
// at all: it is a control that works when you first try it and fails when you
// need it.
func TestPuttingThemDownWorksFromEitherGalleryToo(t *testing.T) {
	p := testPlan(t)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	d.Do(ActionGalleryOpen)
	d.Do(ActionPause)
	if !d.Quit() || !d.WantsPause() || d.WantsSettings() {
		t.Errorf("from the screen gallery: quit=%v paused=%v settings=%v",
			d.Quit(), d.WantsPause(), d.WantsSettings())
	}

	a, _ := appDesk(t, threeApps, nil)
	a.Do(ActionAppsOpen)
	if !a.inApps {
		t.Fatal("the applications did not open")
	}
	a.Do(ActionPause)
	if !a.Quit() || !a.WantsPause() || a.WantsSettings() {
		t.Errorf("from the applications: quit=%v paused=%v settings=%v",
			a.Quit(), a.WantsPause(), a.WantsSettings())
	}
}

// TestAKeyPicksTheGlassesBackUp: the resting wait holds one shortcut, and it
// is the one that ends the rest.
//
// ⛔ HALF A SWITCH IS THE DEFECT. Putting the glasses down releases every
// shortcut, which is what a person means by putting them down -- so without
// this the key that stopped the ribbon could never start it, and the menu bar
// would be the only way back.
func TestAKeyPicksTheGlassesBackUp(t *testing.T) {
	// The existing fake claim, and a press the moment it is asked for.
	claimedIt := make(chan struct{})
	withRegister(t, func(c hotkey.Combo, _ *hotkey.Options) (claimed, error) {
		f := &fakeClaim{got: c, want: c, ch: make(chan hotkey.Event, 1)}
		f.press()
		close(claimedIt)
		return f, nil
	})

	_, err := Await(context.Background(), AwaitOptions{
		List:    func() ([]glasses.Display, error) { return []glasses.Display{theGlasses}, nil },
		Resting: true,
		Resume:  []Shortcut{{hotkey.Combo{Key: hotkey.KeyISOSection}, ActionPause}},
	})
	if !errors.Is(err, ErrAwaitResume) {
		t.Fatalf("the key did not pick the glasses up: %v", err)
	}
	select {
	case <-claimedIt:
	default:
		t.Error("the resting wait never claimed the shortcut")
	}
}

// TestResumeOnlyTakesTheOneAndTakesItFromTheSameList.
//
// ⛔ TWO LISTS WOULD BE TWO ANSWERS TO ONE QUESTION. A settings file that moves
// this shortcut has to move both ends of it, so the resting claim reads the
// SAME list the session does rather than naming a combination of its own.
func TestResumeOnlyTakesTheOneAndTakesItFromTheSameList(t *testing.T) {
	moved := hotkey.Combo{Key: hotkey.KeyN7, Mods: hotkey.Command}
	got := ResumeOnly([]Shortcut{
		{hotkey.Combo{Key: hotkey.KeyS}, ActionSettings},
		{moved, ActionPause},
		{hotkey.Combo{Key: hotkey.KeyM}, ActionPoint},
	})
	if len(got) != 1 || got[0].Want != moved || got[0].Does != ActionPause {
		t.Errorf("the resting claim is %v, not the one the settings moved it to", got)
	}
	if got := ResumeOnly(nil); got != nil {
		t.Errorf("with nothing bound the resting claim is %v, want none", got)
	}
}
