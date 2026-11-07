// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"testing"

	"github.com/go-xrkit/xrkit/glasses"
)

// TestWhatTheHeadFactorActuallyIsOnARealBand measures Desk.headToBand on a desk
// built the way the application builds one, rather than on a Fan literal.
//
// ⛔⛔ EVERY EXISTING TEST OF THIS TAKES ANOTHER PATH. The gain table uses
// Screens: 1, where headToBand returns early; the Fan tests build literals whose
// ring is nil, which hwOf and chainOf answer with a UNIFORM chain, one panel per
// screen. NewFan builds a ring, so every desk the application makes walks
// FACETS -- and headToBand divides a whole SCREEN step by what centre(focus, 1)
// returns, which on a faceted chain is the next FACET.
//
// Measured on a VITURE Beast on 2026-11-03, head-following on, the band drawn
// "from 57 facet(s) for 6 screen(s)": every frame moved the band, by up to
// 2160° -- six full turns in sixteen milliseconds.
func TestWhatTheHeadFactorActuallyIsOnARealBand(t *testing.T) {
	beast := glasses.Display{Name: "VITURE Beast", Width: 1920, Height: 1080}

	for _, c := range []struct {
		name  string
		wides int  // the width of screen 0; 0 leaves every screen alike
		asApp bool // build the plan the way cmd/xrdesk does
		last  bool // look at the LAST screen, whose next is the first
	}{
		{"six screens all alike", 0, false, false},
		{"six screens, the first a mirrored ultrawide", 3840, false, false},
		// ⛔⛔ AND THE SAME TWO AS THE APPLICATION BUILDS THEM. cmd/xrdesk ends
		// its plan with `plan.WithBend(bendR).WithReach(*reach)`, and -reach
		// defaults to 0, documented as "0 = the setting, or
		// ComfortableYawDeg". Every case above skips that line, which is why
		// they measure 2.051 for a desk the application runs at tens.
		{"all alike, built as the application builds it", 0, true, false},
		{"the mirrored ultrawide, built as the application builds it", 3840, true, false},
		// ⛔ ROUND THE RING. The last screen's next is the FIRST, whose middle
		// facet has a lower index, so the facet offset comes out negative and
		// has to be carried round by the total. Without this case the carry is
		// a branch nothing executes — and the band is a circle precisely so
		// that looking at the last screen and turning right is an ordinary
		// thing to do, not an edge.
		{"looking at the last screen, where the next one wraps", 3840, true, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			p, err := NewPlan(beast, Options{Screens: 6})
			if err != nil {
				t.Fatal(err)
			}
			if c.asApp {
				// ⛔⛔ THE BEND IS WHAT CUTS THE FACETS, and leaving it out is
				// why this probe cleared the facet hypothesis by mistake.
				// Without a curve a screen is ONE facet, so centre(focus, 1)
				// is the next SCREEN and headToBand's division is sound. The
				// application runs "curved 20.0°", which cuts six screens into
				// 57 facets — and then centre(focus, 1) is the next FACET
				// while the numerator is still a whole screen.
				p = p.WithBend(DefaultBend).WithReach(0)
			}
			feeds := make([]Feed, p.Count())
			for i := range feeds {
				w := p.ScreenW
				if i == 0 && c.wides > 0 {
					w = c.wides
				}
				feeds[i] = &shapedFeed{w: w, h: p.ScreenH}
			}
			d, err := New(p, feeds)
			if err != nil {
				t.Fatal(err)
			}
			// A plan does not know a screen's shape until a source has arrived.
			d.Render()

			if c.last {
				if err := d.Look(p.Count() - 1); err != nil {
					t.Fatalf("Look at the last screen: %v", err)
				}
			}

			d.mu.Lock()
			gain := d.headGain()
			factor := d.headToBand()
			faceted := d.fan != nil && d.fan.ring != nil
			d.mu.Unlock()

			// ⛔ THE FACET COUNT BESIDE IT. The application reported "the band
			// is drawn from 57 facet(s) for 6 screen(s)" and a head→band of
			// 46.679; this probe measures 2.051 for what looks like the same
			// desk. A number that differs by 22x between a test and the thing
			// it tests is a test of something else, and the facet count is the
			// first place to look: headToBand divides a SCREEN step by what
			// centre(focus, 1) returns, which on a faceted chain is a FACET.
			t.Logf("reach %.2f°: gain %.3f, head→band %.3f, faceted=%v, %d facet(s)",
				d.plan.ReachDeg(), gain, factor, faceted, d.Facets())

			// ⚠ THE BOUND IS THE DOCTRINE'S OWN. headToBand exists to make the
			// picture hold still: one degree of head should move the band by
			// about one degree, times whatever amplification a screen wider
			// than the view demands. Anything past ten is not a correction.
			//
			// ⭐⭐ AND IT REPRODUCES THE DEVICE, which is what makes it a guard
			// rather than a tripwire. The application reported head→band
			// 46.679 with 57 facets; this fixture reaches 46.679 with 57
			// facets, to three decimals, with no hardware.
			//
			// ⛔ IT TOOK THE RIGHT UNIT TO GET THERE. bendFacets computes
			// `arc = views * fovDeg / bend`, so bend is a RADIUS in multiples
			// of the viewing distance and LARGER IS FLATTER. An earlier version
			// of this test passed WithBend(20) -- twenty times too flat -- and
			// reached 8 facets and 3.008, which cleared the facet hypothesis by
			// mistake for an hour. cmd/xrdesk passes desk.DefaultBend, which is
			// 1.0, and that is what this must pass too.
			//
			// Before the fix / after, on the same fixture:
			//
			//	6 x 1920 flat          6 facets   1.238 / 1.238   (untouched)
			//	3840 + 5 x 1920 flat   7 facets   2.051 / 2.051   (untouched)
			//	3840 + 5 x 1920 curved 57 facets 46.679 / 1.222
			if factor > 10*gain {
				t.Errorf("head→band is %.3f with a gain of %.3f: a head turn moves the "+
					"band %.1f times what the gain asks for, which is a facet step "+
					"standing in for a screen step", factor, gain, factor/gain)
			}
		})
	}
}
