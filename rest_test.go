// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"context"
	"errors"
	"testing"
	"time"

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
