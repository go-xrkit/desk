// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"testing"
	"time"
)

// desktop is a machine whose displays and pointer a test can arrange.
type desktop struct {
	rects   map[uint64]rect
	all     []uint64
	x, y    float64
	lost    bool // the window server will not say where the pointer is
	moved   []([2]float64)
	refuses error
}

func (dt *desktop) install(t *testing.T) {
	t.Helper()
	wasAt, wasRect, wasAll, wasMove := pointerAt, displayRect, allDisplays, movePointer
	pointerAt = func() (float64, float64, bool) {
		if dt.lost {
			return 0, 0, false
		}
		return dt.x, dt.y, true
	}
	displayRect = func(id uint64) (rect, bool) { r, ok := dt.rects[id]; return r, ok }
	allDisplays = func() []uint64 { return dt.all }
	movePointer = func(x, y float64) error {
		if dt.refuses != nil {
			return dt.refuses
		}
		dt.x, dt.y = x, y
		dt.moved = append(dt.moved, [2]float64{x, y})
		return nil
	}
	t.Cleanup(func() { pointerAt, displayRect, allDisplays, movePointer = wasAt, wasRect, wasAll, wasMove })
}

// twoScreens is a ribbon of two 2056x1156 screens side by side, and nothing
// else on the desktop.
func twoScreens() *desktop {
	return &desktop{
		rects: map[uint64]rect{
			10: {X: 0, Y: 0, W: 2056, H: 1156},
			11: {X: 2056, Y: 0, W: 2056, H: 1156},
		},
		all: []uint64{10, 11},
	}
}

var band = []uint64{10, 11}

// pushed holds the pointer where it is for as long as it takes, and reports
// what happened on the last look.
func pushed(t *testing.T, e *Edges, ids []uint64, for_ time.Duration) bool {
	t.Helper()
	start := time.Unix(0, 0)
	moved := false
	for at := time.Duration(0); at <= for_; at += 50 * time.Millisecond {
		got, err := e.Step(start.Add(at), ids)
		if err != nil {
			t.Fatalf("Step: %v", err)
		}
		moved = moved || got
	}
	return moved
}

func TestPushingLeftOfTheFirstScreenArrivesAtTheLast(t *testing.T) {
	dt := twoScreens()
	dt.install(t)
	dt.x, dt.y = 0, 600

	var e Edges
	if !pushed(t, &e, band, DefaultHold) {
		t.Fatal("a push held for the whole hold did not wrap")
	}
	if dt.x < 4000 {
		t.Errorf("pointer landed at x=%v, want the right-hand edge of the last screen", dt.x)
	}
	if dt.y != 600 {
		t.Errorf("pointer landed at y=%v, want the height it left at", dt.y)
	}
}

func TestPushingRightOfTheLastScreenArrivesAtTheFirst(t *testing.T) {
	dt := twoScreens()
	dt.install(t)
	dt.x, dt.y = 4111, 300

	var e Edges
	if !pushed(t, &e, band, DefaultHold) {
		t.Fatal("a push held for the whole hold did not wrap")
	}
	if dt.x > 100 {
		t.Errorf("pointer landed at x=%v, want the left-hand edge of the first screen", dt.x)
	}
}

func TestTouchingTheEdgeAndLettingGoDoesNotWrap(t *testing.T) {
	// The left-hand column of the first screen is where a close button lives.
	// Reaching it must not throw the pointer across the desk.
	dt := twoScreens()
	dt.install(t)
	dt.x, dt.y = 0, 600

	var e Edges
	if pushed(t, &e, band, DefaultHold-50*time.Millisecond) {
		t.Error("the pointer wrapped before the hold was up")
	}
	if len(dt.moved) != 0 {
		t.Errorf("the pointer was moved %d times, want none", len(dt.moved))
	}
}

func TestAPushThatChangesEndsStartsItsHoldAgain(t *testing.T) {
	dt := twoScreens()
	dt.install(t)
	start := time.Unix(0, 0)
	var e Edges

	dt.x, dt.y = 0, 600
	if _, err := e.Step(start, band); err != nil {
		t.Fatalf("Step: %v", err)
	}
	// Straight to the other end, later than the hold: it is a NEW push and
	// must not inherit the first one's clock.
	dt.x = 4111
	moved, err := e.Step(start.Add(DefaultHold+time.Second), band)
	if err != nil {
		t.Fatalf("Step: %v", err)
	}
	if moved {
		t.Error("a push at the other end wrapped on the first look")
	}
}

func TestLeavingTheEndForgetsThePush(t *testing.T) {
	dt := twoScreens()
	dt.install(t)
	start := time.Unix(0, 0)
	var e Edges

	dt.x, dt.y = 0, 600
	if _, err := e.Step(start, band); err != nil {
		t.Fatalf("Step: %v", err)
	}
	dt.x = 1000 // somewhere in the middle
	if moved, _ := e.Step(start.Add(10*time.Millisecond), band); moved {
		t.Error("the pointer wrapped from the middle of a screen")
	}
	dt.x = 0
	if moved, _ := e.Step(start.Add(DefaultHold+20*time.Millisecond), band); moved {
		t.Error("a push that had been let go still counted")
	}
}

func TestAnEdgeWithADisplayBeyondItIsNotAnEnd(t *testing.T) {
	// This Mac's own panel, immediately to the right of the screens this
	// program made. Wrapping there would take the real screen away.
	dt := twoScreens()
	dt.rects[1] = rect{X: 4112, Y: 0, W: 2056, H: 1329}
	dt.all = append(dt.all, 1)
	dt.install(t)
	dt.x, dt.y = 4111, 300

	var e Edges
	if pushed(t, &e, band, 2*DefaultHold) {
		t.Error("the pointer wrapped away from the way to this Mac's own screen")
	}
	// And the other end, where there IS nothing, still wraps.
	dt.x, dt.y = 0, 300
	if !pushed(t, &e, band, DefaultHold) {
		t.Error("the end that really is an end did not wrap")
	}
}

func TestAOneScreenRibbonWrapsAtBothOfItsEdges(t *testing.T) {
	dt := &desktop{rects: map[uint64]rect{10: {X: 0, Y: 0, W: 2056, H: 1156}}, all: []uint64{10}}
	dt.install(t)
	one := []uint64{10}

	dt.x, dt.y = 2055, 600
	var e Edges
	if !pushed(t, &e, one, DefaultHold) {
		t.Error("the right-hand edge of a one-screen ribbon did not wrap")
	}
	if dt.x > 100 {
		t.Errorf("pointer landed at x=%v, want the left-hand edge", dt.x)
	}
	dt.x, dt.y = 0, 600
	e = Edges{}
	if !pushed(t, &e, one, DefaultHold) {
		t.Error("the left-hand edge of a one-screen ribbon did not wrap")
	}
}

func TestTheHeightIsClampedIntoTheScreenItArrivesOn(t *testing.T) {
	dt := twoScreens()
	dt.rects[11] = rect{X: 2056, Y: 0, W: 2056, H: 400} // a shorter screen
	dt.install(t)
	dt.x, dt.y = 0, 1100 // below anything on the far screen

	var e Edges
	if !pushed(t, &e, band, DefaultHold) {
		t.Fatal("did not wrap")
	}
	if dt.y != 399 {
		t.Errorf("pointer landed at y=%v, want the last row of the screen it arrived on", dt.y)
	}
}

func TestAPushAboveOrBelowTheScreenIsNotAPush(t *testing.T) {
	dt := twoScreens()
	dt.install(t)
	dt.x, dt.y = 0, 2000 // level with no screen at all

	var e Edges
	if pushed(t, &e, band, 2*DefaultHold) {
		t.Error("the pointer wrapped from beside the ribbon rather than at its end")
	}
}

func TestNoScreensMeansNoEnds(t *testing.T) {
	dt := twoScreens()
	dt.install(t)
	dt.x, dt.y = 0, 600

	var e Edges
	if moved, err := e.Step(time.Unix(0, 0), nil); moved || err != nil {
		t.Errorf("Step with no screens = %v,%v, want false,nil", moved, err)
	}
	// Nor do screens nothing can measure.
	if moved, err := e.Step(time.Unix(0, 0), []uint64{999}); moved || err != nil {
		t.Errorf("Step with an unmeasurable screen = %v,%v, want false,nil", moved, err)
	}
}

func TestAPointerThatCannotBeFoundIsReported(t *testing.T) {
	dt := twoScreens()
	dt.lost = true
	dt.install(t)

	var e Edges
	moved, err := e.Step(time.Unix(0, 0), band)
	if moved || !errors.Is(err, ErrPointerLost) {
		t.Errorf("Step = %v,%v, want false,ErrPointerLost", moved, err)
	}
}

func TestAPointerThatWillNotMoveIsReported(t *testing.T) {
	dt := twoScreens()
	stuck := errors.New("the window server refused")
	dt.refuses = stuck
	dt.install(t)
	dt.x, dt.y = 0, 600

	var e Edges
	start := time.Unix(0, 0)
	if _, err := e.Step(start, band); err != nil {
		t.Fatalf("Step: %v", err)
	}
	if moved, err := e.Step(start.Add(DefaultHold), band); moved || !errors.Is(err, stuck) {
		t.Errorf("Step = %v,%v, want false and the refusal", moved, err)
	}
}

func TestHoldCanBeSetAndDefaults(t *testing.T) {
	if (&Edges{}).hold() != DefaultHold {
		t.Error("a zero Hold does not mean the default")
	}
	if got := (&Edges{Hold: time.Second}).hold(); got != time.Second {
		t.Errorf("hold = %v, want the second that was asked for", got)
	}
}

func TestThePlatformSeamsAnswerEverywhere(t *testing.T) {
	// The real ones, not the fakes: a display id no machine has, and a pointer
	// call that must not panic wherever the tests are run.
	if _, ok := platformDisplayRect(^uint64(0)); ok {
		t.Error("a display that does not exist reported a rectangle")
	}
	// Position and the display list are allowed to answer or not depending on
	// the machine; what is pinned is that they answer at all.
	_, _, _ = pointerAtReal()
	_ = platformDisplays()
}

// pointerAtReal names the platform seam without the fake installed by other
// tests in this file.
func pointerAtReal() (float64, float64, bool) { return platformPointerAt() }

func TestTheEndsAreGeometryAndNotTheOrderTheyWereListedIn(t *testing.T) {
	dt := twoScreens()
	dt.install(t)
	dt.x, dt.y = 0, 600

	// The band is given right-to-left. The end the pointer arrives at is the
	// one on the right of the DESKTOP, not the one first in the slice.
	var e Edges
	if !pushed(t, &e, []uint64{11, 10}, DefaultHold) {
		t.Fatal("did not wrap")
	}
	if dt.x < 4000 {
		t.Errorf("pointer landed at x=%v, want the rightmost screen", dt.x)
	}
}

func TestADisplayNothingCanMeasureDoesNotHoldTheEndOpen(t *testing.T) {
	dt := twoScreens()
	dt.all = append(dt.all, 404) // in the list, with no rectangle
	dt.install(t)
	dt.x, dt.y = 0, 600

	var e Edges
	if !pushed(t, &e, band, DefaultHold) {
		t.Error("a display that cannot be measured stopped the wrap")
	}
}

func TestTheHeightIsClampedUpwardsToo(t *testing.T) {
	dt := twoScreens()
	dt.rects[11] = rect{X: 2056, Y: 800, W: 2056, H: 400} // a screen hung low
	dt.install(t)
	dt.x, dt.y = 0, 100 // above anything on the far screen

	var e Edges
	if !pushed(t, &e, band, DefaultHold) {
		t.Fatal("did not wrap")
	}
	if dt.y != 800 {
		t.Errorf("pointer landed at y=%v, want the first row of the screen it arrived on", dt.y)
	}
}

func TestADisplayToTheLEFTOfTheFirstScreenKeepsThatEdgeToo(t *testing.T) {
	// The mirror image of the Mac's panel on the right: an arrangement where
	// the real screen sits to the left of the ones this program made.
	dt := twoScreens()
	dt.rects[1] = rect{X: -2056, Y: 0, W: 2056, H: 1329}
	dt.all = append(dt.all, 1)
	dt.install(t)
	dt.x, dt.y = 0, 600

	var e Edges
	if pushed(t, &e, band, 2*DefaultHold) {
		t.Error("the pointer wrapped away from the way to the display on the left")
	}
}
