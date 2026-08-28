// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build darwin && integration

// This file creates REAL virtual displays on the machine running it, so it is
// behind both a build tag and an environment variable: a CI runner has no window
// server, and a display left behind would appear on somebody's desktop.
//
//	XRDESK_INTEGRATION=1 go test -tags integration -v -run Integration ./...
//
// It exists because of a question no unit test can answer: "why do I not see the
// virtual screens in the macOS display list?". The answer is that they live
// exactly as long as the process holds them — which is a claim about the
// SYSTEM's list, not about ours, so the system's own list is what is read here.

package desk

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/go-macos/virtualdisplay"
	"github.com/go-xrkit/xrkit/glasses"
)

func requireIntegration(t *testing.T) {
	t.Helper()
	if os.Getenv("XRDESK_INTEGRATION") == "" {
		t.Skip("set XRDESK_INTEGRATION=1 to run tests that create real displays")
	}
	if err := virtualdisplay.Available(); err != nil {
		t.Skipf("virtual displays are not available here: %v", err)
	}
}

// listed returns the IDs macOS currently reports as active.
func listed(t *testing.T) map[uint32]virtualdisplay.DisplayInfo {
	t.Helper()
	ds, err := virtualdisplay.ActiveDisplays()
	if err != nil {
		t.Fatalf("ActiveDisplays: %v", err)
	}
	byID := map[uint32]virtualdisplay.DisplayInfo{}
	for _, d := range ds {
		byID[d.ID] = d
	}
	return byID
}

func TestIntegrationTheScreensAppearInTheSystemDisplayListAndThenGo(t *testing.T) {
	requireIntegration(t)

	plan, err := NewPlan(glasses.Display{Name: "integration", Width: 3840, Height: 1080}, Options{Screens: 6})
	if err != nil {
		t.Fatalf("NewPlan: %v", err)
	}
	before := listed(t)

	s, err := Provide(context.Background(), plan, t.Logf)
	if err != nil {
		t.Fatalf("Provide: %v", err)
	}
	defer s.Close()
	if !s.Virtual {
		t.Skipf("fell back to real displays (%s); there is nothing to look for", s.Why)
	}
	if len(s.IDs) != plan.Count() {
		t.Fatalf("got %d screens, want %d", len(s.IDs), plan.Count())
	}

	// macOS must list every one of them, at the size the plan asked for. This is
	// the same list System Settings > Displays draws.
	now := listed(t)
	for i, id := range s.IDs {
		d, ok := now[uint32(id)]
		if !ok {
			t.Fatalf("screen %d (id %d) is not in the system display list", i+1, id)
		}
		if d.Mode.PixelsWide != plan.ScreenW || d.Mode.PixelsHigh != plan.ScreenH {
			t.Errorf("screen %d is %s, want %dx%d pixels", i+1, d.Mode, plan.ScreenW, plan.ScreenH)
		}
		if _, existed := before[uint32(id)]; existed {
			t.Errorf("screen %d (id %d) was already there before Provide", i+1, id)
		}
	}

	// And they must be NAMED, because a name is what a person recognises in that
	// window. system_profiler reads the same source the pane does.
	out, err := exec.Command("system_profiler", "SPDisplaysDataType").Output()
	if err != nil {
		t.Fatalf("system_profiler: %v", err)
	}
	for i := 1; i <= plan.Count(); i++ {
		want := "XR desk " + string(rune('0'+i)) + ":"
		if !strings.Contains(string(out), want) {
			t.Errorf("macOS does not name a display %q; it lists:\n%s", strings.TrimSuffix(want, ":"), displayNames(out))
		}
	}

	// Closing must be OBSERVABLE by the time it returns — no sleep here on
	// purpose. virtualdisplay.Close alone returns in microseconds and macOS goes
	// on listing the displays for up to a couple of seconds, which reads exactly
	// like a leak; Screens.Close waits for that.
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	gone := listed(t)
	for i, id := range s.IDs {
		if _, ok := gone[uint32(id)]; ok {
			t.Errorf("screen %d (id %d) is still in the system display list after Close returned", i+1, id)
		}
	}
	if len(gone) != len(before) {
		t.Errorf("the machine has %d displays, had %d: something was left behind", len(gone), len(before))
	}
}

// displayNames pulls the display names out of system_profiler's output, for a
// failure message that says what macOS DOES list.
func displayNames(out []byte) string {
	var names []string
	for _, line := range strings.Split(string(out), "\n") {
		t := strings.TrimSpace(line)
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if indent >= 8 && strings.HasSuffix(t, ":") && !strings.Contains(t, "Displays") {
			names = append(names, "  "+strings.TrimSuffix(t, ":"))
		}
	}
	return strings.Join(names, "\n")
}
