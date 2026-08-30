// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"testing"
)

// desktop is a machine whose displays and pointer a test can arrange.
type desktop struct {
	rects   map[uint64]rect
	all     []uint64
	x, y    float64
	lost    bool // the window server will not say where the pointer is
	moved   [][2]float64
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

// asMeasured is this machine with the glasses on and six screens up, as the
// window server reported it: the glasses at x=-13440, the ribbon running
// -11520..0, and the Mac's own panel at 0..2056.
func asMeasured() *desktop {
	dt := &desktop{rects: map[uint64]rect{
		1: {X: 0, Y: 0, W: 2056, H: 1329},
		3: {X: -13440, Y: 0, W: 1920, H: 1080},
	}}
	for i, x := 0, -11520.0; i < 6; i, x = i+1, x+1920 {
		dt.rects[uint64(59+i)] = rect{X: x, Y: 0, W: 1920, H: 1080}
	}
	for id := range dt.rects {
		dt.all = append(dt.all, id)
	}
	return dt
}

var band = []uint64{59, 60, 61, 62, 63, 64}

func TestTheFenceLeavesAPointerOnItsOwnScreenAlone(t *testing.T) {
	dt := asMeasured()
	dt.install(t)
	dt.x, dt.y = -10000, 600 // on screen 1, which is 59: -11520..-9600

	var f Fence
	if moved, err := f.Step(band, 0); moved || err != nil {
		t.Errorf("Step = %v,%v, want the pointer left where it is", moved, err)
	}
	if len(dt.moved) != 0 {
		t.Errorf("the pointer was moved %d times, want none", len(dt.moved))
	}
}

func TestTheFencePutsAWanderingPointerBackOnTheEdgeItLeftBy(t *testing.T) {
	dt := asMeasured()
	dt.install(t)
	dt.x, dt.y = -10000, 600

	var f Fence
	f.Step(band, 0) // it is holding screen 1 now

	// Off the right-hand edge of screen 1, onto screen 2 -- which is a screen
	// of the band, and still not the one being looked at.
	dt.x = -9000
	moved, err := f.Step(band, 0)
	if !moved || err != nil {
		t.Fatalf("Step = %v,%v, want it brought back", moved, err)
	}
	if dt.x != -9601 || dt.y != 600 {
		t.Errorf("pointer landed at %v,%v, want the last column of screen 1 at the same height", dt.x, dt.y)
	}
}

func TestTurningTheBandTakesThePointerToTheMiddleOfTheNewScreen(t *testing.T) {
	dt := asMeasured()
	dt.install(t)
	dt.x, dt.y = -10000, 600

	var f Fence
	f.Step(band, 0)

	// The keyboard turned the band to screen 3, which is display 61 at
	// -7680..-5760. The pointer is nowhere near it.
	moved, err := f.Step(band, 2)
	if !moved || err != nil {
		t.Fatalf("Step = %v,%v, want the pointer brought to the new screen", moved, err)
	}
	if dt.x != -6720 || dt.y != 540 {
		t.Errorf("pointer landed at %v,%v, want the middle of screen 3", dt.x, dt.y)
	}
	// And a band turned to a screen the pointer is ALREADY on leaves it where
	// it is: arriving is not a reason to move a pointer that has arrived.
	before := len(dt.moved)
	if moved, _ := f.Step(band, 2); moved {
		t.Error("the pointer was moved again on the screen it was already on")
	}
	if len(dt.moved) != before {
		t.Error("the pointer was moved on a screen it was already on")
	}
}

func TestTheFenceHoldsThePointerToAMirroredMacScreenToo(t *testing.T) {
	// The whole point of holding it to what a position SHOWS rather than to
	// what this program made: a position mirroring the Mac's own panel is how
	// somebody reaches their real desktop, and the pointer has to be able to
	// live there.
	dt := asMeasured()
	dt.install(t)
	mirrored := []uint64{1, 60, 61, 62, 63, 64} // screen 1 shows display 1
	dt.x, dt.y = -10000, 600

	var f Fence
	if moved, err := f.Step(mirrored, 0); !moved || err != nil {
		t.Fatalf("Step = %v,%v, want the pointer taken to the Mac's panel", moved, err)
	}
	if dt.x != 1028 || dt.y != 664.5 {
		t.Errorf("pointer landed at %v,%v, want the middle of the Mac's panel", dt.x, dt.y)
	}
	// And once there it is left alone anywhere on that panel.
	dt.x, dt.y = 2000, 1300
	if moved, _ := f.Step(mirrored, 0); moved {
		t.Error("a pointer inside the mirrored panel was moved")
	}
}

func TestAPositionShowingNothingIsNotAFence(t *testing.T) {
	dt := asMeasured()
	dt.install(t)
	dt.x, dt.y = 1000, 600

	var f Fence
	for _, c := range []struct {
		name    string
		showing []uint64
		focus   int
	}{
		{"a position showing nothing", []uint64{0, 60}, 0},
		{"a display this machine will not measure", []uint64{404, 60}, 0},
		{"a focus off the end of the band", band, 99},
		{"a focus below the beginning", band, -1},
		{"no positions at all", nil, 0},
	} {
		f = Fence{}
		moved, err := f.Step(c.showing, c.focus)
		if moved || err != nil {
			t.Errorf("%s: Step = %v,%v, want the pointer left alone", c.name, moved, err)
		}
	}
	if len(dt.moved) != 0 {
		t.Errorf("the pointer was moved %d times, want none", len(dt.moved))
	}
}

func TestAPointerThatCannotBeFoundIsReported(t *testing.T) {
	dt := asMeasured()
	dt.lost = true
	dt.install(t)

	var f Fence
	if moved, err := f.Step(band, 0); moved || !errors.Is(err, ErrPointerLost) {
		t.Errorf("Step = %v,%v, want false,ErrPointerLost", moved, err)
	}
}

func TestAPointerThatWillNotMoveIsReported(t *testing.T) {
	dt := asMeasured()
	stuck := errors.New("the window server refused")
	dt.refuses = stuck
	dt.install(t)
	dt.x, dt.y = 5000, 600 // nowhere near the band

	var f Fence
	if moved, err := f.Step(band, 0); moved || !errors.Is(err, stuck) {
		t.Errorf("Step = %v,%v, want false and the refusal", moved, err)
	}
}

func TestThePlatformSeamsAnswerEverywhere(t *testing.T) {
	// The real ones, not the fakes: a display id no machine has, and calls that
	// must not panic wherever the tests are run.
	if _, ok := platformDisplayRect(^uint64(0)); ok {
		t.Error("a display that does not exist reported a rectangle")
	}
	_, _, _ = platformPointerAt()
	_ = platformDisplays()
}

func TestThePointerIsBroughtBackOnBothEdgesAndBothWays(t *testing.T) {
	dt := asMeasured()
	dt.install(t)
	// Screen 1 is display 59 at -11520..-9600, 0..1080.
	for _, c := range []struct {
		name         string
		x, y         float64
		wantX, wantY float64
	}{
		{"off the left edge", -12000, 600, -11520, 600},
		{"off the right edge", -9000, 600, -9601, 600},
		{"above the top", -10000, -50, -10000, 0},
		{"below the bottom", -10000, 2000, -10000, 1079},
	} {
		var f Fence
		dt.x, dt.y = -10000, 500
		f.Step(band, 0) // arrive, so the next step is a clamp and not a jump
		dt.x, dt.y = c.x, c.y
		if moved, err := f.Step(band, 0); !moved || err != nil {
			t.Fatalf("%s: Step = %v,%v", c.name, moved, err)
		}
		if dt.x != c.wantX || dt.y != c.wantY {
			t.Errorf("%s: landed at %v,%v, want %v,%v", c.name, dt.x, dt.y, c.wantX, c.wantY)
		}
	}
}
