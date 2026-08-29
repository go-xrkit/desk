// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk_test

import (
	"os"
	"testing"

	"github.com/go-widgets/window"
	"github.com/go-xrkit/desk"
)

// TestOwnDisplayFindsEveryAttachedScreen needs a real window server and real
// displays, so it runs only when XRDESK_LIVE is set.
//
// It is the one piece of the darkening and the pointer wrap that cannot be
// unit-tested: turning the screen the desk was GIVEN, which it knows by name,
// into the display id everything else speaks in. Both rules that protect a
// person -- never darken the screen you are looking through, and bring back a
// pointer that walks onto it -- are switched off silently if this returns
// nothing, so it is worth being able to run it on a machine that has screens.
func TestOwnDisplayFindsEveryAttachedScreen(t *testing.T) {
	if os.Getenv("XRDESK_LIVE") == "" {
		t.Skip("set XRDESK_LIVE=1 to run this against the displays this machine has")
	}
	ss, err := window.Screens()
	if err != nil {
		t.Fatalf("listing screens: %v", err)
	}
	if len(ss) == 0 {
		t.Skip("no screens attached")
	}
	seen := map[uint64]string{}
	for _, s := range ss {
		id, ok := desk.OwnDisplay(s.Name)
		if !ok {
			t.Errorf("OwnDisplay(%q) found nothing; it is at %d,%d %dx%d",
				s.Name, s.X, s.Y, s.Width, s.Height)
			continue
		}
		if other, dup := seen[id]; dup {
			t.Errorf("OwnDisplay(%q) and OwnDisplay(%q) are both display %d",
				s.Name, other, id)
		}
		seen[id] = s.Name
		t.Logf("%q at %d,%d %dx%d is display %d", s.Name, s.X, s.Y, s.Width, s.Height, id)
	}
	if _, ok := desk.OwnDisplay("no screen is called this"); ok {
		t.Error("OwnDisplay found a display for a name no screen has")
	}
}
