// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build darwin

package desk_test

import (
	"context"
	"os"
	"testing"

	"github.com/go-xrkit/desk"
	"github.com/go-xrkit/xrkit/glasses"
)

// TestSourcesOfferTheDesksScreensInTheBandsOrder makes real virtual displays,
// so it runs only when XRDESK_LIVE is set. It needs no glasses.
//
// THE DEFECT IT PINS. The band draws position i from Screens.IDs[i]. What
// position i is SHOWING comes from the inventory, which walks Sources. Those
// two orders were different, because ScreenCaptureKit does not list displays in
// the order they were created:
//
//	position 1 draws display 107, and the inventory said 108
//	position 2 draws display 108, and the inventory said 107
//
// Everything that has to agree with the picture goes through the inventory --
// which screen the pointer is held to, and where the pointer is drawn -- so the
// pointer was held to a screen the band was not showing. The report was "la
// souris ne correspond pas a celle du systeme" and "je ne peux plus interragir
// avec les app".
func TestSourcesOfferTheDesksScreensInTheBandsOrder(t *testing.T) {
	if os.Getenv("XRDESK_LIVE") == "" {
		t.Skip("set XRDESK_LIVE=1 to run this against real virtual displays")
	}
	ctx := context.Background()
	plan, err := desk.NewPlan(glasses.Display{Name: "test", Width: 1920, Height: 1080},
		desk.Options{Screens: 4, SplayDeg: -1})
	if err != nil {
		t.Fatalf("NewPlan = %v", err)
	}
	s, err := desk.Provide(ctx, plan, nil)
	if err != nil {
		t.Fatalf("Provide = %v", err)
	}
	defer s.Close()
	if !s.Virtual {
		t.Skipf("no virtual displays on this machine: %s", s.Why)
	}

	offers, err := desk.Sources(ctx, s)
	if err != nil {
		t.Fatalf("Sources = %v", err)
	}
	inv, err := desk.NewInventory(plan.Count(), offers)
	if err != nil {
		t.Fatalf("NewInventory = %v", err)
	}
	// Filled the way a session fills it: one Cycle per position, in order.
	for i := 0; i < plan.Count(); i++ {
		o, ok := inv.Cycle(i)
		if !ok {
			t.Fatalf("position %d was offered nothing", i+1)
		}
		id, ok := desk.DisplayOf(o)
		if !ok {
			t.Fatalf("position %d was offered %s, which names no display", i+1, o)
		}
		if id != s.IDs[i] {
			t.Errorf("position %d is showing display %d and drawing display %d",
				i+1, id, s.IDs[i])
		}
	}
}
