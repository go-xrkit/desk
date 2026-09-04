// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-widgets/toolkit"
	"github.com/go-xrkit/xrkit/glasses"
)

// fitPanels are displays this window actually opens on, smallest first.
//
// The small ones are the point. The window was sized against the biggest screen
// attached and then reported as unusable, so a panel that is merely typical --
// a laptop lid, a pair of glasses at 1080p -- is the case that has to hold.
var fitPanels = []struct {
	name string
	w, h int
}{
	{"a 13-inch laptop lid", 1280, 800},
	{"a MacBook Air", 1512, 982},
	{"the glasses at 1080p", 1920, 1080},
	{"the DELL U3417W", 3440, 1440},
	{"an 8K panel", 7680, 4320},
}

// TestTheWindowNeverOutgrowsItsScreen.
//
// ⛔ The invariant, in the shape it was asked for: the window is not bigger
// than the screen it opens on, and its buttons are reachable there. Both halves
// matter -- clamping the height alone would keep Save under the bottom edge and
// merely stop admitting it.
//
// This walks the same road RunSettings does, minus the window: magnify for the
// panel, measure the page, clamp, then build the tree with the room that leaves
// and lay it out at the size the window would be.
func TestTheWindowNeverOutgrowsItsScreen(t *testing.T) {
	was := toolkit.MetricScale()
	t.Cleanup(func() { toolkit.SetMetricScale(was) })

	for _, p := range fitPanels {
		t.Run(p.name, func(t *testing.T) {
			// The usable area, as the platform reports it: a menu bar and a Dock
			// come off the top and bottom before anything is placed.
			visW, visH := p.w, p.h-96
			maxW := int(float64(visW) * SettingsRoom)
			maxH := int(float64(visH) * SettingsRoom)

			cfg := &Config{}
			attached := []glasses.USB{oneS, luma}
			size := func(scale float64) (int, int) {
				toolkit.SetMetricScale(scale)
				return settingsSize(*cfg, attached)
			}
			scale := FitScale(SettingsScale(p.h), maxW, maxH, size)
			w, h := size(scale)
			w, h, roomH := fitWindow(w, h, maxW, maxH, func(string, ...any) {})

			if w > maxW || h > maxH {
				t.Fatalf("the window is %dx%d on a screen with %dx%d to give",
					w, h, maxW, maxH)
			}

			root, _ := settingsRoot(cfg, attached, roomH, nil, func() {})
			surface := settingsSurface(root)
			surface.SetBounds(toolkit.Rect{X: 0, Y: 0, W: w, H: h})

			// Save is the one that has to be pressable: settings that cannot be
			// saved are the failure this is about.
			save := findButton(surface, "Save")
			if save == nil {
				t.Fatal("no Save button in the window")
			}
			r := save.Bounds()
			if r.Y+r.H > h || r.X+r.W > w || r.H <= 0 {
				t.Errorf("Save is at %d,%d %dx%d in a %dx%d window: it cannot be clicked",
					r.X, r.Y, r.W, r.H, w, h)
			}

			// And a page too tall for the frame is the case that must scroll,
			// rather than simply be cut off at the clamp.
			if roomH > 0 && !hasScrollView(surface) {
				t.Errorf("the page did not fit in %d pixels and there is no scroll view", roomH)
			}
			t.Logf("%dx%d at scale %.2f, room %d, Save at y=%d", w, h, scale, roomH, r.Y)
		})
	}
}

// findButton returns the first button with this label, anywhere in the tree.
func findButton(w toolkit.Widget, label string) *toolkit.Button {
	if b, ok := w.(*toolkit.Button); ok && b.Label().Get() == label {
		return b
	}
	if p, ok := w.(interface{ Children() []toolkit.Widget }); ok {
		for _, kid := range p.Children() {
			if b := findButton(kid, label); b != nil {
				return b
			}
		}
	}
	return nil
}

// hasScrollView says whether anything in the tree can scroll.
func hasScrollView(w toolkit.Widget) bool {
	if _, ok := w.(*toolkit.ScrollView); ok {
		return true
	}
	if p, ok := w.(interface{ Children() []toolkit.Widget }); ok {
		for _, kid := range p.Children() {
			if hasScrollView(kid) {
				return true
			}
		}
	}
	return false
}

// TestAPageTallerThanTheRoomScrolls.
//
// ⛔ The other half of the invariant, and the half a window is needed for: the
// window is clamped to the screen, so the PAGE is what gives. A page measured
// taller than the room left above the button bar is wrapped in a scroll view --
// and one that fits is not, because a permanent gutter takes a strip of width
// from every row to hold a scrollbar that can never appear, which is what got
// the scroll view removed the first time.
func TestAPageTallerThanTheRoomScrolls(t *testing.T) {
	cfg := &Config{}
	attached := []glasses.USB{oneS, luma}

	// Room enough for a couple of rows and no more.
	said := ""
	root, _ := settingsRoot(cfg, attached, 40, func(f string, a ...any) {
		said = fmt.Sprintf(f, a...)
	}, func() {})
	if !hasScrollView(root) {
		t.Error("a page with 40 pixels of room did not scroll")
	}
	if !strings.Contains(said, "it scrolls") {
		t.Errorf("nothing was logged about scrolling: %q", said)
	}

	// Room enough for anything: no gutter.
	root, _ = settingsRoot(cfg, attached, 100_000, nil, func() {})
	if hasScrollView(root) {
		t.Error("a page with room to spare was given a scrollbar gutter")
	}

	// And no room GIVEN at all -- every caller who is not opening a window --
	// is "no limit" rather than "no room".
	root, _ = settingsRoot(cfg, attached, 0, nil, func() {})
	if hasScrollView(root) {
		t.Error("a caller that named no screen was given a scrollbar gutter")
	}
}
