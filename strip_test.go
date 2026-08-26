// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"math"
	"testing"

	"github.com/go-xrkit/xrkit/ribbon"
)

// testSpan is the arc one screen takes when n of them fill the turn, less the
// share of it the gap between two screens wants. It is the same arithmetic the
// plan does, written out, so a screen comes to exactly one view wide.
func testSpan(n int) float64 {
	pitch := 2 * math.Pi / float64(n)
	return pitch * 1920 / float64(1920+DefaultGapPx)
}

// evenScreens places n screens of hfov radians each, evenly, the way a ribbon
// of equal screens does.
func evenScreens(n int, span float64) []ribbon.Placed {
	out := make([]ribbon.Placed, n)
	for i := range out {
		out[i] = ribbon.Placed{
			Screen:   ribbon.Screen{ID: "s", W: 1920, H: 1200},
			Centre:   float64(i) * 2 * math.Pi / float64(n),
			HalfSpan: span / 2,
		}
	}
	return out
}

func testStrip(t *testing.T, n int) *Strip {
	t.Helper()
	// Evenly spread, each screen exactly one view wide, which is what a plan of
	// equal screens gives.
	s, err := NewStrip(evenScreens(n, testSpan(n)), n*(1920+DefaultGapPx), 1920, 1200, 1920, 1200)
	if err != nil {
		t.Fatalf("NewStrip = %v", err)
	}
	return s
}

// TestAScreenAtRestFillsTheViewExactly is the rule the whole plan is built on:
// one screen is one full view. At rest it must cover every column and every
// row, at one source pixel per output pixel — no resampling, and no seam of
// background down either edge.
func TestAScreenAtRestFillsTheViewExactly(t *testing.T) {
	s := testStrip(t, 4)
	for i := 0; i < s.Screens(); i++ {
		blits := s.Frame(nil, s.centre[i])
		var covered int
		for _, b := range blits {
			if b.Screen != i {
				continue
			}
			covered += b.Dst.W
			if b.Dst.H != 1200 {
				t.Errorf("screen %d covers %d rows, want 1200", i, b.Dst.H)
			}
			if b.SrcXStep != one {
				t.Errorf("screen %d resamples at %v source columns per column, want 1",
					i, float64(b.SrcXStep)/float64(one))
			}
		}
		if covered != 1920 {
			t.Errorf("screen %d covers %d of 1920 columns at rest", i, covered)
		}
	}
}

// TestTurningTheBandNeverShowsAHole: at every offset all the way round, every
// column of the view is either a screen or the gap between two — never a column
// nothing claimed because of an off-by-one at a seam.
func TestTurningTheBandNeverShowsAHole(t *testing.T) {
	s := testStrip(t, 4)
	var dst []ribbon.Blit
	for offset := -s.Width(); offset <= 2*s.Width(); offset += 7 {
		dst = s.Frame(dst[:0], offset)
		claimed := make([]int, 1920)
		for _, b := range dst {
			for x := b.Dst.X; x < b.Dst.X+b.Dst.W; x++ {
				if x < 0 || x >= 1920 {
					t.Fatalf("offset %d: a blit reaches column %d, outside the view", offset, x)
				}
				claimed[x]++
			}
		}
		for x, n := range claimed {
			if n > 1 {
				t.Fatalf("offset %d: column %d is claimed by %d screens at once", offset, x, n)
			}
		}
	}
}

// TestTheBandCloseOnItself: turning right past the last screen arrives at the
// first, and the screen straddling the join is drawn from both sides.
func TestTheBandClosesOnItself(t *testing.T) {
	s := testStrip(t, 4)
	// Half a screen before screen 0, which puts screen 3's right half on the
	// left of the view and screen 0's left half on the right.
	blits := s.Frame(nil, -960)
	seen := map[int]int{}
	for _, b := range blits {
		seen[b.Screen] += b.Dst.W
	}
	if seen[3] == 0 {
		t.Errorf("turning back past the first screen did not reach the last: %v", seen)
	}
	if seen[0] == 0 {
		t.Errorf("the first screen is not there: %v", seen)
	}

	// And a whole turn is the identity.
	a := s.Frame(nil, 137)
	b := s.Frame(nil, 137+s.Width())
	if len(a) != len(b) {
		t.Fatalf("a whole turn gave %d blits, then %d", len(a), len(b))
	}
	for i := range a {
		if a[i].Screen != b[i].Screen || a[i].Dst != b[i].Dst {
			t.Errorf("blit %d differs a whole turn later: %+v vs %+v", i, a[i], b[i])
		}
	}
}

// TestOffsetFollowsTheNavigatorsYaw: the navigator thinks in angles because a
// ribbon is a circle, and it keeps working without knowing the picture is flat.
func TestOffsetFollowsTheNavigatorsYaw(t *testing.T) {
	s := testStrip(t, 4)
	for _, tc := range []struct {
		yaw  float64
		want int
	}{
		{0, 0},
		{math.Pi / 2, s.Width() / 4},
		{math.Pi, s.Width() / 2},
		{2 * math.Pi, 0},                  // a whole turn is where it started
		{-math.Pi / 2, 3 * s.Width() / 4}, // and turning back wraps
	} {
		if got := s.Offset(tc.yaw); got != tc.want {
			t.Errorf("Offset(%.4f) = %d, want %d", tc.yaw, got, tc.want)
		}
	}
}

// TestFullscreenIsOneScreenAndNothingElse.
func TestFullscreenIsOneScreenAndNothingElse(t *testing.T) {
	s := testStrip(t, 4)
	for i := 0; i < s.Screens(); i++ {
		blits, err := s.Fullscreen(nil, i)
		if err != nil {
			t.Fatalf("Fullscreen(%d) = %v", i, err)
		}
		if len(blits) != 1 {
			t.Fatalf("Fullscreen(%d) gave %d blits, want the one", i, len(blits))
		}
		if b := blits[0]; b.Screen != i || b.Dst.X != 0 || b.Dst.W != 1920 || b.Dst.H != 1200 {
			t.Errorf("Fullscreen(%d) = %+v", i, b)
		}
	}
	if _, err := s.Fullscreen(nil, 4); !errors.Is(err, ErrScreens) {
		t.Errorf("Fullscreen off the end = %v, want an ErrScreens", err)
	}
	if _, err := s.Fullscreen(nil, -1); !errors.Is(err, ErrScreens) {
		t.Errorf("Fullscreen(-1) = %v, want an ErrScreens", err)
	}
}

// TestAScreenThatIsNotTheViewsShapeIsScaled: the rule is one screen one view,
// but nothing here should break if a plan ever hands over something else.
func TestAScreenThatIsNotTheViewsShapeIsScaled(t *testing.T) {
	s, err := NewStrip(evenScreens(2, testSpan(2)), 2*(1920+DefaultGapPx), 3840, 2400, 1920, 1200)
	if err != nil {
		t.Fatalf("NewStrip = %v", err)
	}
	blits := s.Frame(nil, 0)
	if len(blits) == 0 {
		t.Fatal("nothing was drawn")
	}
	if got, want := blits[0].SrcXStep, 2*one; got != want {
		t.Errorf("SrcXStep = %v, want %v source columns per column", got, want)
	}
	if got, want := s.srcY[600], int32(1200); got != want {
		t.Errorf("row 600 reads source row %d, want %d", got, want)
	}
}

func TestNewStripRefusesWhatItCannotLayOut(t *testing.T) {
	for name, tc := range map[string]struct {
		n, srcW, srcH, viewW, viewH, total int
		is                                 error
	}{
		"no screens":          {0, 1920, 1200, 1920, 1200, 4000, ErrNoScreens},
		"no source":           {2, 0, 1200, 1920, 1200, 4000, ErrScreens},
		"no source rows":      {2, 1920, 0, 1920, 1200, 4000, ErrScreens},
		"no view":             {2, 1920, 1200, 0, 1200, 4000, ErrScreens},
		"no view rows":        {2, 1920, 1200, 1920, 0, 4000, ErrScreens},
		"a band of no pixels": {2, 1920, 1200, 1920, 1200, 0, ErrScreens},
	} {
		_, err := NewStrip(evenScreens(tc.n, 1), tc.total,
			tc.srcW, tc.srcH, tc.viewW, tc.viewH)
		if !errors.Is(err, tc.is) {
			t.Errorf("%s: NewStrip = %v, want a %v", name, err, tc.is)
		}
	}
}

// TestAScreenTooNarrowToDrawIsRefused.
//
// A screen whose arc rounds to no pixels at all cannot be laid on the band, and
// silently dropping it would leave the viewer looking for a screen the desk
// says it has.
func TestAScreenTooNarrowToDrawIsRefused(t *testing.T) {
	placed := evenScreens(2, testSpan(2))
	placed[1].HalfSpan = 1e-12
	if _, err := NewStrip(placed, 2*(1920+DefaultGapPx), 1920, 1200, 1920, 1200); !errors.Is(err, ErrScreens) {
		t.Errorf("NewStrip = %v, want an ErrScreens", err)
	}
}
