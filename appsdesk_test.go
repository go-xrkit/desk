// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"testing"
)

// threeApps is what OnApps answers in these tests.
var threeApps = []App{
	{Name: "Code", Windows: 2},
	{Name: "Firefox", Windows: 1, On: []int{1}},
	{Name: "Mail", Windows: 1},
}

// appDesk is a four-screen desk wired to a list and a placement recorder.
func appDesk(t *testing.T, apps []App, listErr error) (*Desk, *[][]Placement) {
	t.Helper()
	d, err := New(testPlan(t), feedsFor(testPlan(t)))
	if err != nil {
		t.Fatalf("New = %v", err)
	}
	var placed [][]Placement
	d.OnApps = func() ([]App, error) { return apps, listErr }
	d.OnPlace = func(p []Placement) { placed = append(placed, p) }
	return d, &placed
}

func TestTheApplicationGalleryOpensOnWhatIsRunningNow(t *testing.T) {
	d, _ := appDesk(t, threeApps, nil)

	d.Do(ActionApps)
	if !d.inApps {
		t.Fatal("the gallery did not open")
	}
	if got := len(d.apps.apps); got != 3 {
		t.Errorf("the gallery holds %d applications, want 3", got)
	}
	// And it asks again every time, rather than keeping the first answer.
	d.Do(ActionApps) // leaves
	if d.inApps {
		t.Fatal("the same key did not leave the gallery")
	}
	d.OnApps = func() ([]App, error) { return threeApps[:1], nil }
	d.Do(ActionApps)
	if got := len(d.apps.apps); got != 1 {
		t.Errorf("the gallery holds %d applications after the list changed, want 1", got)
	}
}

// TestChoosingAnApplicationPutsItOnTheScreenInFront is the pairing the whole
// design turns on: no number typed, no second key — the screen the band is
// showing is the destination.
func TestChoosingAnApplicationPutsItOnTheScreenInFront(t *testing.T) {
	d, placed := appDesk(t, threeApps, nil)
	d.Do(ActionNext) // focus screen 2 (index 1)
	d.Do(ActionApps)
	d.Do(ActionNext) // second application: Firefox
	d.Do(ActionChoose)

	if len(*placed) != 1 || len((*placed)[0]) != 1 {
		t.Fatalf("placements = %v, want exactly one", *placed)
	}
	got := (*placed)[0][0]
	if got.App != "Firefox" {
		t.Errorf("placed %q, want Firefox", got.App)
	}
	if want := d.Nav().Focus() + 1; got.Pos != want {
		t.Errorf("placed on screen %d, want %d — the one in front", got.Pos, want)
	}
}

func TestChoosingWithNoApplicationsRefuses(t *testing.T) {
	d, placed := appDesk(t, nil, nil)
	d.Do(ActionApps)
	d.Do(ActionChoose)

	if len(*placed) != 0 {
		t.Errorf("placed %v with nothing to place", *placed)
	}
	if !errors.Is(d.Err(), ErrNoApps) {
		t.Errorf("Err() = %v, want ErrNoApps", d.Err())
	}
}

func TestSpreadFromTheBandAndFromTheGalleryAgree(t *testing.T) {
	// Four screens, three applications: three placements either way.
	for _, from := range []string{"band", "gallery"} {
		d, placed := appDesk(t, threeApps, nil)
		if from == "gallery" {
			d.Do(ActionApps)
		}
		d.Do(ActionSpread)

		if len(*placed) != 1 {
			t.Fatalf("from the %s: %d rounds of placement, want 1", from, len(*placed))
		}
		got := (*placed)[0]
		if len(got) != 3 {
			t.Fatalf("from the %s: %d placements, want 3", from, len(got))
		}
		for i, p := range got {
			if p.App != threeApps[i].Name || p.Pos != i+1 {
				t.Errorf("from the %s: placement %d is %+v", from, i, p)
			}
		}
	}
}

// TestSpreadStopsAtTheScreenCount: four screens, six applications.
func TestSpreadStopsAtTheScreenCount(t *testing.T) {
	many := []App{{Name: "A"}, {Name: "B"}, {Name: "C"}, {Name: "D"}, {Name: "E"}, {Name: "F"}}
	d, placed := appDesk(t, many, nil)
	d.Do(ActionSpread)

	got := (*placed)[0]
	if len(got) != d.Plan().Count() {
		t.Fatalf("%d placements for %d screens", len(got), d.Plan().Count())
	}
	for _, p := range got {
		if p.Pos > d.Plan().Count() {
			t.Errorf("placement %+v is past the last screen", p)
		}
	}
}

// TestAListThatCannotBeReadIsReportedAndOpensNothing: an empty gallery would
// read as "nothing is running", which is the wrong thing to tell somebody who
// has not granted Accessibility.
func TestAListThatCannotBeReadIsReportedAndOpensNothing(t *testing.T) {
	boom := errors.New("no Accessibility grant")
	d, placed := appDesk(t, nil, boom)

	d.Do(ActionApps)
	if d.inApps {
		t.Error("the gallery opened on a list that could not be read")
	}
	if !errors.Is(d.Err(), boom) {
		t.Errorf("Err() = %v, want the list's error", d.Err())
	}
	d.Do(ActionSpread)
	if len(*placed) != 0 {
		t.Errorf("spread %v from a list that could not be read", *placed)
	}
}

// TestTheApplicationGalleryKeepsTheSelectionOnTheSameApplication, not on the
// same index: a person's eye is on a name, and an application that quits shifts
// every index after it.
func TestTheApplicationGalleryKeepsTheSelectionOnTheSameApplication(t *testing.T) {
	d, _ := appDesk(t, threeApps, nil)
	d.Do(ActionApps)
	d.Do(ActionNext)
	d.Do(ActionNext) // Mail, index 2
	if app, _ := d.apps.app(); app.Name != "Mail" {
		t.Fatalf("selected %q, want Mail", app.Name)
	}

	// Code quits. Mail is now index 1.
	d.OnApps = func() ([]App, error) { return threeApps[1:], nil }
	d.Do(ActionApps) // leaves
	d.Do(ActionApps) // and comes back on a shorter list
	if app, _ := d.apps.app(); app.Name != "Mail" {
		t.Errorf("selected %q after the list shrank, want Mail", app.Name)
	}
}

// TestLeavingTheGalleryLeavesTheRibbonWhereItWas: the band keeps its focus
// underneath, which is what makes "the screen in front" mean anything.
func TestLeavingTheGalleryLeavesTheRibbonWhereItWas(t *testing.T) {
	d, _ := appDesk(t, threeApps, nil)
	d.Do(ActionNext)
	d.Do(ActionNext)
	was := d.Nav().Focus()

	d.Do(ActionApps)
	d.Do(ActionPrev) // moves the APPLICATION selection, not the band
	d.Do(ActionApps)

	if got := d.Nav().Focus(); got != was {
		t.Errorf("the ribbon is on %d, was on %d before the gallery", got, was)
	}
}

func TestTheApplicationGalleryAnswersLeaveTheGallery(t *testing.T) {
	d, _ := appDesk(t, threeApps, nil)
	d.Do(ActionApps)
	d.Do(ActionGalleryClose)
	if d.inApps {
		t.Error("leave-the-gallery left the application gallery up")
	}
}

func TestQuitAndSettingsWorkFromTheApplicationGallery(t *testing.T) {
	for _, c := range []struct {
		a        Action
		settings bool
	}{{ActionQuit, false}, {ActionSettings, true}} {
		d, _ := appDesk(t, threeApps, nil)
		d.Do(ActionApps)
		d.Do(c.a)
		if !d.Quit() {
			t.Errorf("%v from the gallery did not stop the desk", c.a)
		}
		if d.WantsSettings() != c.settings {
			t.Errorf("%v: WantsSettings = %v, want %v", c.a, d.WantsSettings(), c.settings)
		}
	}
}

// TestTheArrowsWalkTheApplicationGalleryThroughTheKeys, not only through the
// view: up and down have to reach the gallery from Do, which is where a key
// press actually arrives.
func TestTheArrowsWalkTheApplicationGalleryThroughTheKeys(t *testing.T) {
	eight := []App{
		{Name: "A"}, {Name: "B"}, {Name: "C"}, {Name: "D"},
		{Name: "E"}, {Name: "F"}, {Name: "G"}, {Name: "H"},
	}
	d, _ := appDesk(t, eight, nil)
	d.Do(ActionApps)
	// The grid needs a picture before it has columns to move by.
	d.Render()
	cols := d.apps.grid.Columns()
	if cols < 2 || len(eight) <= cols {
		t.Skipf("a %d-column grid of %d applications has no second row", cols, len(eight))
	}

	d.Do(ActionDown)
	if got := d.apps.selected(); got != cols {
		t.Errorf("down selected %d, want %d — one row", got, cols)
	}
	d.Do(ActionUp)
	if got := d.apps.selected(); got != 0 {
		t.Errorf("up selected %d, want back at 0", got)
	}
}

// TestChoosingPutsTheGalleryAway, like the screen gallery's choose does.
//
// Not tidiness: with it open the arrows move the SELECTION, so the band cannot
// be turned, so a second choose could only put another application on the same
// screen — which hides one of them.
func TestChoosingPutsTheGalleryAway(t *testing.T) {
	d, placed := appDesk(t, threeApps, nil)
	d.Do(ActionApps)
	d.Do(ActionChoose)

	if d.inApps {
		t.Error("the gallery is still up after choosing")
	}
	if len(*placed) != 1 {
		t.Fatalf("placements = %v, want one", *placed)
	}
	// And the band turns again, which is the point of closing.
	was := d.Nav().Focus()
	d.Do(ActionNext)
	if got := d.Nav().Focus(); got == was {
		t.Error("the band did not turn after the gallery closed")
	}
}

// TestChoosingNothingLeavesTheGalleryUp: there was nothing to place, so there
// is nothing to look at the result of.
func TestChoosingNothingLeavesTheGalleryUp(t *testing.T) {
	d, _ := appDesk(t, nil, nil)
	d.Do(ActionApps)
	d.Do(ActionChoose)
	if !d.inApps {
		t.Error("the gallery closed on a choose that placed nothing")
	}
}

// TestSpreadingFromTheGalleryPutsItAway too: what it did is on the band, and
// the list behind it now says where everything used to be.
func TestSpreadingFromTheGalleryPutsItAway(t *testing.T) {
	d, placed := appDesk(t, threeApps, nil)
	d.Do(ActionApps)
	d.Do(ActionSpread)

	if d.inApps {
		t.Error("the gallery is still up after spreading")
	}
	if len(*placed) != 1 || len((*placed)[0]) != 3 {
		t.Errorf("placements = %v, want one round of three", *placed)
	}
}

// TestInGalleryIsTrueForEitherGallery, because the bare keys are claimed for
// both and released when neither is up.
func TestInGalleryIsTrueForEitherGallery(t *testing.T) {
	d, _ := appDesk(t, threeApps, nil)
	if d.InGallery() {
		t.Error("a fresh desk says it is in a gallery")
	}
	d.Do(ActionGalleryOpen)
	if !d.InGallery() {
		t.Error("the screen gallery is open and InGallery says otherwise")
	}
	d.Do(ActionGalleryClose)
	d.Do(ActionApps)
	if !d.InGallery() {
		t.Error("the application gallery is open and InGallery says otherwise")
	}
	d.Do(ActionApps)
	if d.InGallery() {
		t.Error("both galleries are closed and InGallery says otherwise")
	}
}
