// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"math"
	"testing"
)

// ⛔⛔ THE CIRCULAR RULE, AND IT IS THE ONE THE MEASURE ITSELF GOT WRONG FIRST.
// The band is a circle: 359° and 1° are two degrees apart, not 358. A measure
// that answered 358 would report a WRAP as a lurch — and that is exactly what
// the first version of the head beat did, by subtracting Nav.Yaw's wrapped
// angle from an unwrapped want. It produced 720.00°, 1080.00°, 2160.00° and
// 7560.00°: every one an exact multiple of 360, every one an artefact, and
// three hypotheses chased on the strength of them.
func TestTheBandIsACircleAndTheMeasureKnowsIt(t *testing.T) {
	t.Parallel()

	const turn = 2 * math.Pi

	t.Run("wrapTo2Pi folds into one turn, negatives included", func(t *testing.T) {
		t.Parallel()
		for _, c := range []struct{ in, want float64 }{
			{0, 0},
			{math.Pi, math.Pi},
			// ⛔ THE NEGATIVE BRANCH. Go's Mod keeps the sign of its first
			// argument, so -0.1 rad comes back as -0.1 and not as 2π-0.1 —
			// which would put the band a whole turn away from where it is.
			{-0.1, turn - 0.1},
			{-math.Pi, math.Pi},
			{turn + 0.5, 0.5},
			{3*turn + 0.25, 0.25},
			{-3*turn - 0.25, turn - 0.25},
		} {
			if got := wrapTo2Pi(c.in); math.Abs(got-c.want) > 1e-9 {
				t.Errorf("wrapTo2Pi(%g) = %g, want %g", c.in, got, c.want)
			}
			// And the answer is always inside one turn, which is the whole
			// contract: Nav.Yaw answers in that space and nothing may be
			// compared against it from outside.
			if got := wrapTo2Pi(c.in); got < 0 || got >= turn {
				t.Errorf("wrapTo2Pi(%g) = %g, outside [0, 2π)", c.in, got)
			}
		}
	})

	t.Run("shortestArc takes the short way round", func(t *testing.T) {
		t.Parallel()
		d2r := func(deg float64) float64 { return deg * math.Pi / 180 }
		for _, c := range []struct {
			name string
			a, b float64
			want float64 // degrees
		}{
			{"nothing", d2r(10), d2r(10), 0},
			{"a short step forward", d2r(12), d2r(10), 2},
			{"a short step back", d2r(10), d2r(12), -2},
			// ⛔⛔ THE TWO CASES THE FIRST MEASURE FAILED. Across the seam the
			// short way is two degrees, not 358, and its SIGN says which way.
			{"across the seam forward", d2r(1), d2r(359), 2},
			{"across the seam back", d2r(359), d2r(1), -2},
			// Exactly half a turn is the boundary: either sign is the same
			// distance, and what must not happen is an answer past 180.
			{"half a turn", d2r(180), 0, 180},
		} {
			got := shortestArc(c.a, c.b) * 180 / math.Pi
			if math.Abs(got-c.want) > 1e-9 && math.Abs(math.Abs(got)-math.Abs(c.want)) > 1e-9 {
				t.Errorf("%s: shortestArc = %.3f°, want %.3f°", c.name, got, c.want)
			}
			if math.Abs(got) > 180+1e-9 {
				t.Errorf("%s: shortestArc = %.3f°, which is the long way round",
					c.name, got)
			}
		}
	})
}

// ⛔ THE BEAT RESETS, AND A READER WHO DOES NOT KNOW THAT WOULD PRINT A RUNNING
// TOTAL. One line every five seconds is the point: a total that never cleared
// would hide the moment somebody started moving under the weight of the minutes
// they were still.
//
// ⚠ AND IT SAYS WHETHER THERE WAS ANYTHING TO COUNT. A desk nobody is following
// must report on=false so the caller stays silent rather than printing a row of
// zeroes every five seconds for the whole of an ordinary session.
func TestTheHeadBeatCountsAndThenForgets(t *testing.T) {
	t.Parallel()

	d, h := deskForHead(t)

	// Nothing asked for: nothing to say, whatever the source would answer.
	if _, _, _, _, _, _, _, _, on := d.HeadBeat(); on {
		t.Error("HeadBeat says it is following a head nobody asked it to follow")
	}

	d.FollowHead(true)
	h.yaw = 0.2
	d.Advance(0.016)
	d.Advance(0.016)

	judged, moved, biggest, travel, lo, hi, factor, _, on := d.HeadBeat()
	if !on {
		t.Fatal("HeadBeat says it is not following a head it was told to follow")
	}
	if judged != 2 {
		t.Errorf("judged %d frame(s), want the 2 that were advanced", judged)
	}
	if moved == 0 {
		t.Error("no frame moved the band, with a head that turned 0.2 rad")
	}
	if biggest <= 0 || travel <= 0 {
		t.Errorf("biggest %.3f° and travel %.3f°, want both positive", biggest, travel)
	}
	// The source's own number, in degrees, as the journal prints it.
	if want := 0.2 * 180 / math.Pi; math.Abs(lo-want) > 1e-6 || math.Abs(hi-want) > 1e-6 {
		t.Errorf("the source read %.3f°…%.3f°, want %.3f° both ways", lo, hi, want)
	}
	if factor <= 0 {
		t.Errorf("factor %.3f, want the conversion it actually used", factor)
	}

	// ⛔ AND THE SECOND CALL IS EMPTY, which is what makes the line a RATE.
	judged, moved, biggest, travel, _, _, _, _, on = d.HeadBeat()
	if !on {
		t.Error("following stopped between two reads")
	}
	if judged != 0 || moved != 0 || biggest != 0 || travel != 0 {
		t.Errorf("the second read returned %d/%d, %.3f°, %.3f° — the beat did not forget",
			moved, judged, biggest, travel)
	}
}
