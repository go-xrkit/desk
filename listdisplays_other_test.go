// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build !android

package desk

import (
	"testing"

	"github.com/go-widgets/window"
	"github.com/go-xrkit/xrkit/glasses"
)

// ⛔⛔ SCALE WAS DROPPED, AND IT KILLED THE WARNING THAT DEPENDS ON IT.
//
// cmd/xrdesk built glasses.Display field by field at two call sites and left
// Scale out, so it was always 0 — while glasses.ScalingAdvice opens with
// `if d.Scale <= 1 { return "" }`. The call `if advice :=
// glasses.ScalingAdvice(chosen); advice != ""` therefore could not fire on any
// platform, and the thing it was there to say is not cosmetic: a Beast was
// observed rendering 5120x1600 onto a 3840x1080 panel.
//
// This test is the reason the mapping is a function of its own: the field can
// only be pinned against displays that are not attached to the machine running
// the suite.
func TestAScaledModeStillReachesTheAdvice(t *testing.T) {
	t.Parallel()

	got := displaysFromScreens([]window.Screen{
		{Name: "Color LCD", Width: 1512, Height: 982, Scale: 2, Primary: true},
		{Name: "VITURE Beast", Width: 2560, Height: 800, Scale: 2},
	})
	want := []glasses.Display{
		{Name: "Color LCD", Width: 1512, Height: 982, Primary: true, Scale: 2},
		{Name: "VITURE Beast", Width: 2560, Height: 800, Scale: 2},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d displays, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("display %d = %+v, want %+v", i, got[i], want[i])
		}
	}

	// The assertion that actually matters: the advice now has something to say
	// about the glasses. With Scale dropped it said nothing at all.
	advice := glasses.ScalingAdvice(got[1])
	if advice == "" {
		t.Fatal("ScalingAdvice said nothing about a Beast in a 2x scaled mode — " +
			"which is exactly what dropping Scale produced")
	}

	// And it must stay quiet about an ordinary monitor in a scaled mode: that
	// is the user's business, and a warning that fires on everything is noise.
	if s := glasses.ScalingAdvice(got[0]); s != "" {
		t.Errorf("ScalingAdvice complained about a laptop panel: %q", s)
	}
}
