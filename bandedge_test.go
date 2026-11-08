// Copyright (c) the go-xrkit/desk authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"fmt"
	"math"
	"testing"
)

// TestARefusalIsTheBandsEdgeAndNotAGap is the measurement that lets the fold
// protocol exempt a run ending at the frame's own edge.
//
// [Fan.Frame] walks the chain outward from the focus and STOPS at the first
// panel it cannot draw, on the premise that "going outward a panel only gets
// more oblique and smaller, so a refusal is the edge of what can be drawn and
// not a gap to step over". That premise was reasoned about on a flat band and
// then inherited by the curved one, where a screen is a row of facets that turn
// away one by one. If it is false anywhere, Frame steps over drawable panels
// and leaves holes -- and the fold protocol's exemption would be hiding them.
//
// ⛔⛔ SO IT IS ASKED, NOT ASSUMED, AND IT IS ASKED BEYOND THE STOP. The weak
// version of this test -- "is the panel the walk stopped before drawable?" --
// answers no by definition and proves nothing. This one keeps looking: after
// the stop it scans every remaining panel inside the same 180° guard and
// reports a GAP if any of them could have been drawn.
//
// Measured 2026-11-08 at v0.4.1, both directions, all seven shapes, seven
// splays, five distances: 5740 stops, 5740 edges, 0 gaps. Earlier runs also
// found 0 stops caused by the reach running out and 0 by the guard firing while
// a drawable panel waited.
//
// ⚠ AND IT CARRIES ITS OWN POSITIVE CONTROL. `edges` must be non-zero: if no
// walk ever stopped on a refusal, "0 gaps" would be the answer to a question
// nobody asked, and the exemption in fold_test.go would rest on nothing.
func TestARefusalIsTheBandsEdgeAndNotAGap(t *testing.T) {
	t.Parallel()
	var edges, gaps int
	var where []string

	for _, anchor := range []Anchoring{AnchorOnGaze, AnchorFixed} {
		for _, shape := range theDesksToSweep {
			base, widths := shape.plan(t)
			base = base.WithAnchoring(anchor)
			for _, splay := range []float64{SplayStep, 10, 20, 30, 40, 51.6, MaxSplayDeg} {
				for _, dist := range []float64{1, 1.5, 2, 3, MaxDistance} {
					plan := base.WithSplay(splay).WithDistance(dist).WithBend(DefaultBend)
					f, err := NewFan(plan)
					if err != nil {
						t.Fatalf("%s at %g°, %gx: NewFan = %v", shape.name, splay, dist, err)
					}
					f.SetSourceWidths(widths)
					f.SetBends(plan.Bends())
					f.SetAnchoring(anchor)
					for focus := range plan.Count() {
						// ⛔ BOTH DIRECTIONS. The protocol's complaints split
						// evenly between a run's first column and its last, so
						// a verdict from the rightward walk alone would be a
						// verdict about half the subject.
						for _, dir := range []int{-1, +1} {
							gap, stop, again := scanPastTheStop(f, focus, dir)
							switch {
							case gap:
								gaps++
								if len(where) < 8 {
									where = append(where, fmt.Sprintf(
										"%s, %s, %g°, %gx, screen %d, %s: stopped at k=%d, "+
											"drawable again at k=%d",
										anchor, shape.name, splay, dist, focus+1,
										sideName(dir), stop, again))
								}
							case stop > 0:
								edges++
							}
						}
					}
				}
			}
		}
	}

	// The positive control, first: a question with no subject has no answer.
	if edges == 0 {
		t.Fatal("no walk in the whole sweep stopped on a refused panel, so \"0 gaps\" " +
			"says nothing and the fold protocol's frame-edge exemption rests on nothing")
	}
	if gaps > 0 {
		for _, w := range where {
			t.Errorf("GAP: %s", w)
		}
		t.Fatalf("%d of %d stops step over a drawable panel: Fan.Frame's `break` on a "+
			"refusal is wrong, and fold_test.go's frame-edge exemption is hiding the "+
			"holes it leaves", gaps, edges+gaps)
	}
	t.Logf("%d walks stop on a refusal, every one of them at the band's edge", edges)
}

// scanPastTheStop walks the chain in one direction the way [Fan.Frame] does and
// then keeps going. It reports whether a drawable panel lies beyond the stop,
// where the walk stopped, and where the drawable one was.
func scanPastTheStop(f *Fan, focus, dir int) (gap bool, stop, again int) {
	angleAt, gapAt := f.chainOf(focus)
	hwOf := f.hwOf(focus)
	reach := f.reachIn()
	scratch := make([]SlantCol, 0, f.viewW)

	// ⚠ THE VERDICT IS slantOf's, not this test's. Reimplementing the decision
	// here would measure the copy: a fake built from a misreading can only
	// confirm it.
	drawable := func(k int) bool {
		j := dir * k
		lx, lz, rx, rz := slantChain(j, angleAt, hwOf, gapAt, f.distance, 0)
		at, x0, x1 := f.faceAt(focus, j)
		_, ok := slantOf(scratch, at, lx, lz, rx, rz,
			f.panelH, f.f, f.viewW, f.viewH, x0, x1, f.srcH)
		return ok
	}
	// The same guard Frame uses, so this walk stops where that one does.
	past := func(k int) bool {
		return math.Abs(angleAt(float64(dir*k))-angleAt(0)) > 180
	}

	for k := 1; k <= reach; k++ {
		if past(k) {
			return false, 0, 0
		}
		if !drawable(k) {
			stop = k
			break
		}
	}
	if stop == 0 {
		return false, 0, 0
	}
	for k := stop + 1; k <= reach && !past(k); k++ {
		if drawable(k) {
			return true, stop, k
		}
	}
	return false, stop, 0
}

func sideName(dir int) string {
	if dir < 0 {
		return "leftward"
	}
	return "rightward"
}
