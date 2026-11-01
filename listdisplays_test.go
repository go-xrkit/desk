// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"testing"

	"github.com/go-xrkit/android"
	"github.com/go-xrkit/xrkit/glasses"
)

// ⛔ THE HOST'S ANSWER IS THE ONE THAT MATTERS ON A PHONE, and on 2026-11-01
// xrdesk asked X11 instead: "neither WAYLAND_DISPLAY nor DISPLAY is set", with
// the glasses on the USB-C port the whole time.
//
// The numbers below are the platform's own, from the display probe on a Pixel
// 11 Pro Fold with a VITURE Beast attached:
//
//	APPEARED id 12 "VITURE Beast" 1920x1080 @110dpi 60.0Hz flags 0x8088 presentation true
func TestTheHostsDisplaysReachTheChooser(t *testing.T) {
	t.Parallel()

	got := displaysFromHost([]android.Display{
		{ID: 0, Name: "Built-in Screen", Width: 2076, Height: 2152, DensityDPI: 420},
		{ID: 12, Name: "VITURE Beast", Width: 1920, Height: 1080, DensityDPI: 110, RefreshRate: 60},
	})
	want := []glasses.Display{
		{Name: "Built-in Screen", Width: 2076, Height: 2152, Primary: true, Scale: 1},
		{Name: "VITURE Beast", Width: 1920, Height: 1080, Primary: false, Scale: 1},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d displays, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("display %d = %+v, want %+v", i, got[i], want[i])
		}
	}

	// And the chooser must then find the glasses in it, because mapping the
	// list correctly and still not choosing from it would be the same silence
	// in a different place.
	chosen, err := glasses.ChooseDisplay(got, "")
	if err != nil {
		t.Fatalf("the chooser refused a list carrying a Beast: %v", err)
	}
	if chosen.Name != "VITURE Beast" {
		t.Errorf("chose %q, want the glasses", chosen.Name)
	}
}

// ⛔ AN EMPTY LIST MUST STAY EMPTY, not become one zero-valued display.
// `make([]T, len(in))` is right and `append` to a sized slice would not be;
// a phantom display with no name is exactly what displaylist.go's
// errDisplaysUnreadable exists to tell apart from a real one.
func TestNoDisplaysIsNoDisplays(t *testing.T) {
	t.Parallel()

	if got := displaysFromHost(nil); len(got) != 0 {
		t.Errorf("an empty host answer produced %d display(s): %+v", len(got), got)
	}
}
