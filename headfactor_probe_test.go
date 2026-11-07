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
	}{
		{"six screens all alike", 0, false},
		{"six screens, the first a mirrored ultrawide", 3840, false},
		// ⛔⛔ AND THE SAME TWO AS THE APPLICATION BUILDS THEM. cmd/xrdesk ends
		// its plan with `plan.WithBend(bendR).WithReach(*reach)`, and -reach
		// defaults to 0, documented as "0 = the setting, or
		// ComfortableYawDeg". Every case above skips that line, which is why
		// they measure 2.051 for a desk the application runs at tens.
		{"all alike, built as the application builds it", 0, true},
		{"the mirrored ultrawide, built as the application builds it", 3840, true},
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
				p = p.WithBend(20).WithReach(0)
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
			// ⛔⛔ AND IT PASSES TODAY ON A FIXTURE THAT CANNOT FAIL IT, which
			// is said here so that a green run is not mistaken for a desk
			// without the defect. The application reported head→band 46.679
			// with 57 facets; the best this fixture reaches is 8 facets and
			// 3.008, because what cuts the facets is the curve and WithBend(20)
			// here produces nothing like the runtime ring. The bound is
			// therefore a TRIPWIRE for the day the fixture reaches the real
			// facet count, not evidence that the factor is sound.
			//
			// ⭐ What the fixture CAN show is the relation, and it is monotone
			// and measured: 6 facets → 1.238, 7 → 2.051, 8 → 3.008, and the
			// device → 46.679. That is what localised the defect — the
			// numerator is a SCREEN step and centre(focus, 1) is one FACET.
			if factor > 10*gain {
				t.Errorf("head→band is %.3f with a gain of %.3f: a head turn moves the "+
					"band %.1f times what the gain asks for, which is a facet step "+
					"standing in for a screen step", factor, gain, factor/gain)
			}
		})
	}
}
