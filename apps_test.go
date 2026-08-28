// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-macos/accessibility"
)

// aDesk is six screens' worth of display ids, in ribbon order.
var aDesk = []uint64{101, 102, 103, 104, 105, 106}

// win is one line of a listing.
func win(app string, display uint32) accessibility.WindowInfo {
	return accessibility.WindowInfo{App: app, Display: display}
}

func TestAppsFromGroupsWindowsAndFindsTheirScreens(t *testing.T) {
	got := AppsFrom([]accessibility.WindowInfo{
		win("Firefox", 101),
		win("Firefox", 101),
		win("Thunderbird", 103),
		win("Finder", 7), // a real display, not one of ours
	}, aDesk)

	if len(got) != 3 {
		t.Fatalf("got %d applications, want 3: %v", len(got), got)
	}
	// Sorted by name, so a gallery does not reshuffle between two looks.
	if got[0].Name != "Finder" || got[1].Name != "Firefox" || got[2].Name != "Thunderbird" {
		t.Errorf("order is %v, want Finder, Firefox, Thunderbird", names(got))
	}
	ff := got[1]
	if ff.Windows != 2 {
		t.Errorf("Firefox has %d windows, want 2", ff.Windows)
	}
	// Two windows on ONE screen is one position, not two.
	if len(ff.On) != 1 || ff.On[0] != 0 {
		t.Errorf("Firefox is on %v, want [0]", ff.On)
	}
	if !ff.Here() {
		t.Error("Firefox is on the desk and Here() says otherwise")
	}
	if got[0].Here() {
		t.Error("Finder is on a display that is not the desk's, and Here() says it is here")
	}
}

func TestAppsFromCountsWhatIsMinimized(t *testing.T) {
	down := win("Mail", 102)
	down.Minimized = true
	got := AppsFrom([]accessibility.WindowInfo{down, win("Mail", 102)}, aDesk)

	if len(got) != 1 || got[0].Windows != 2 || got[0].Minimized != 1 {
		t.Fatalf("got %v, want one application with 2 windows and 1 minimized", got)
	}
}

func TestAppsFromIgnoresAWindowWithNoApplication(t *testing.T) {
	got := AppsFrom([]accessibility.WindowInfo{win("", 101), win("   ", 101), win("Firefox", 101)}, aDesk)
	if len(got) != 1 || got[0].Name != "Firefox" {
		t.Errorf("got %v, want Firefox alone", names(got))
	}
}

func TestAppsFromWithNoDeskPutsNobodyAnywhere(t *testing.T) {
	got := AppsFrom([]accessibility.WindowInfo{win("Firefox", 101)}, nil)
	if len(got) != 1 || got[0].Here() {
		t.Errorf("got %v, want Firefox on no screen at all", got)
	}
}

func TestAppSaysWhereItIs(t *testing.T) {
	for _, c := range []struct {
		app  App
		want string
	}{
		{App{Name: "Firefox"}, "Firefox"},
		{App{Name: "Firefox", Windows: 1, On: []int{0}}, "Firefox on screen 1"},
		{App{Name: "Code", Windows: 3, On: []int{1, 4}}, "Code (3 windows) on screens 2, 5"},
	} {
		if got := c.app.String(); got != c.want {
			t.Errorf("String() = %q, want %q", got, c.want)
		}
	}
}

func TestSpreadGivesOneScreenEachInOrder(t *testing.T) {
	apps := []App{{Name: "A"}, {Name: "B"}, {Name: "C"}}
	got := Spread(apps, 6)

	if len(got) != 3 {
		t.Fatalf("got %d placements, want 3: %v", len(got), got)
	}
	for i, p := range got {
		if p.App != apps[i].Name || p.Pos != i+1 {
			t.Errorf("placement %d is %+v, want %s on %d", i, p, apps[i].Name, i+1)
		}
	}
}

// TestSpreadNeverWrapsOntoAnAssignedScreen is the rule that makes one key
// predictable: an application past the last screen is LEFT WHERE IT IS. Two
// applications on one screen hides one of them.
func TestSpreadNeverWrapsOntoAnAssignedScreen(t *testing.T) {
	apps := []App{{Name: "A"}, {Name: "B"}, {Name: "C"}, {Name: "D"}, {Name: "E"}}
	got := Spread(apps, 2)

	if len(got) != 2 {
		t.Fatalf("got %d placements for 2 screens, want 2: %v", len(got), got)
	}
	seen := map[int]bool{}
	for _, p := range got {
		if seen[p.Pos] {
			t.Errorf("screen %d was handed out twice", p.Pos)
		}
		seen[p.Pos] = true
		if p.Pos > 2 {
			t.Errorf("placement %+v is past the last screen", p)
		}
	}
	if strings.Contains(fmt.Sprint(got), `"C"`) {
		t.Errorf("C was placed: %v", got)
	}
}

func TestSpreadWithNoScreensPlacesNothing(t *testing.T) {
	for _, n := range []int{0, -1} {
		if got := Spread([]App{{Name: "A"}}, n); got != nil {
			t.Errorf("Spread(_, %d) = %v, want nothing", n, got)
		}
	}
}

func TestSpreadWithNoApplicationsPlacesNothing(t *testing.T) {
	if got := Spread(nil, 6); got != nil {
		t.Errorf("Spread(nil, 6) = %v, want nothing", got)
	}
}

// names is for messages.
func names(apps []App) []string {
	out := make([]string, len(apps))
	for i, a := range apps {
		out[i] = a.Name
	}
	return out
}
