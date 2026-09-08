// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"math"
	"testing"

	"github.com/go-widgets/toolkit"
)

// fakeHead stands in for a camera, because a test cannot arrange a room.
type fakeHead struct {
	yaw       float64
	ok        bool
	blind     int
	recenters int
}

func (f *fakeHead) Yaw() (float64, bool) { return f.yaw, f.ok }
func (f *fakeHead) Blind() int           { return f.blind }
func (f *fakeHead) Recenter()            { f.recenters++; f.yaw = 0 }

// deskForHead builds the smallest desk these tests need.
func deskForHead(t *testing.T) (*Desk, *fakeHead) {
	t.Helper()
	p := stereoPlan(t)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	h := &fakeHead{ok: true}
	d.SetHeadSource(h)
	return d, h
}

// TestSwitchingOnDoesNotMoveTheDesk.
//
// ⭐ THE ONE BEHAVIOUR THAT WOULD STARTLE SOMEBODY WEARING IT. The source counts
// from its own origin, so without taking the current yaw as the offset, enabling
// the feature would fling the view to wherever the source happened to be.
func TestSwitchingOnDoesNotMoveTheDesk(t *testing.T) {
	d, h := deskForHead(t)
	d.Nav().SetYaw(1.2)
	h.yaw = 0.7 // the source is not at zero when this is switched on
	d.FollowHead(true)
	if h.recenters != 1 {
		t.Errorf("switching on recentred the source %d times, want once", h.recenters)
	}
	d.Advance(0.02)
	if got := d.Nav().Yaw(); math.Abs(got-1.2) > 1e-9 {
		t.Errorf("switching on moved the view from 1.2 to %v", got)
	}
}

// TestTheHeadMovesTheView, which is the whole feature.
func TestTheHeadMovesTheView(t *testing.T) {
	d, h := deskForHead(t)
	d.Nav().SetYaw(0.5)
	d.FollowHead(true)
	h.yaw = 0.25
	d.Advance(0.02)
	if got := d.Nav().Yaw(); math.Abs(got-0.75) > 1e-9 {
		t.Errorf("the view is at %v, want 0.5 + 0.25", got)
	}
	h.yaw = -0.1
	d.Advance(0.02)
	if got := d.Nav().Yaw(); math.Abs(got-0.4) > 1e-9 {
		t.Errorf("the view is at %v, want 0.5 - 0.1", got)
	}
}

// TestNothingHappensUntilItIsAskedFor: a desk with a source attached but the
// feature off must not move on its own.
func TestNothingHappensUntilItIsAskedFor(t *testing.T) {
	d, h := deskForHead(t)
	d.Nav().SetYaw(0.3)
	h.yaw = 2.0
	d.Advance(0.02)
	if got := d.Nav().Yaw(); math.Abs(got-0.3) > 1e-9 {
		t.Errorf("the view moved to %v with head tracking off", got)
	}
	if d.FollowingHead() {
		t.Error("FollowingHead is true having never been switched on")
	}
}

// TestABlindMomentLeavesTheViewAlone.
//
// ⛔ "NO INFORMATION" AND "NO MOTION" ARE THE SAME NUMBER AND OPPOSITE FACTS. A
// desk that took a blind frame for stillness would sit still while somebody
// turned their head, which reads as broken; one that snapped home would be worse.
func TestABlindMomentLeavesTheViewAlone(t *testing.T) {
	d, h := deskForHead(t)
	d.FollowHead(true)
	h.yaw = 0.4
	d.Advance(0.02)
	at := d.Nav().Yaw()
	h.ok, h.blind = false, 1
	h.yaw = 99 // whatever it holds must not be believed
	d.Advance(0.02)
	if got := d.Nav().Yaw(); got != at {
		t.Errorf("a blind frame moved the view from %v to %v", at, got)
	}
}

// TestItSaysWhenItCannotSee, once, after a run rather than a blink.
func TestItSaysWhenItCannotSee(t *testing.T) {
	d, h := deskForHead(t)
	d.FollowHead(true)
	d.Advance(0.02)
	h.ok = false
	for h.blind = 1; h.blind < blindEnoughToSaySo; h.blind++ {
		d.Advance(0.02)
		if d.head.lost {
			t.Fatalf("it gave up after %d blind frame(s); a blink past a blank "+
				"wall is not a dark room", h.blind)
		}
	}
	d.Advance(0.02)
	if !d.head.lost {
		t.Fatalf("still not lost after %d blind frames", h.blind)
	}
}

// TestComingBackRetakesTheOrigin.
//
// ⛔⛔ CARRYING ON FROM THE OLD OFFSET WOULD APPLY AN UNKNOWN ERROR AS MOTION.
// Nobody knows how far the head turned while this could not see it, so the only
// honest thing is to call wherever it is now the new zero.
func TestComingBackRetakesTheOrigin(t *testing.T) {
	d, h := deskForHead(t)
	d.FollowHead(true)
	h.yaw = 0.2
	d.Advance(0.02)
	away := d.Nav().Yaw()

	h.ok, h.blind = false, blindEnoughToSaySo
	d.Advance(0.02)
	if !d.head.lost {
		t.Fatal("it did not notice it had gone blind")
	}
	recentresBefore := h.recenters

	// Sight returns, and the source has wandered while nobody could check it.
	h.ok, h.blind, h.yaw = true, 0, 1.9
	d.Advance(0.02)
	if h.recenters != recentresBefore+1 {
		t.Error("coming back did not retake the origin")
	}
	if got := d.Nav().Yaw(); got != away {
		t.Errorf("the view jumped to %v on the first frame back, from %v: that "+
			"is an unknown error being applied as motion", got, away)
	}
}

// TestTheGalleryIsNotSteeredByTheHead: it is not the ribbon, and turning it
// behind the gallery is motion nobody can see that snaps into view on cancel.
func TestTheGalleryIsNotSteeredByTheHead(t *testing.T) {
	d, h := deskForHead(t)
	d.FollowHead(true)
	d.Nav().SetYaw(0.6)
	if err := d.Nav().ToggleGallery(d.grid); err != nil {
		t.Fatalf("ToggleGallery: %v", err)
	}
	h.yaw = 1.5
	d.Advance(0.02)
	if got := d.Nav().Yaw(); math.Abs(got-0.6) > 1e-9 {
		t.Errorf("the head moved the view to %v while the gallery was open", got)
	}
}

// TestANilSourceTurnsTheFeatureOff rather than leaving it on with nothing
// behind it, which would be a switch that reports it is doing something.
func TestANilSourceTurnsTheFeatureOff(t *testing.T) {
	d, _ := deskForHead(t)
	d.FollowHead(true)
	if !d.FollowingHead() {
		t.Fatal("it did not switch on")
	}
	d.SetHeadSource(nil)
	if d.FollowingHead() {
		t.Error("head tracking stayed on with no source behind it")
	}
	d.Advance(0.02) // must not panic on the nil source
}

// TestEveryThingHeadTrackingSaysCanBeDrawn.
//
// ⛔⛔ THE BUILT-IN FONT HAS NO SEMICOLON, AND THIS CAUGHT ONE. "following your
// head; the camera light is on" was written, compiled, passed every other test,
// and would have appeared on somebody's glasses with a hole in the middle of it.
// A rune with no glyph still ADVANCES -- the columns stay aligned and the letter
// is simply absent -- so nothing about the layout gives it away either.
//
// The existing legibility test walks the settings window. These strings go to
// the toast instead, which is drawn with the same font and was not covered.
func TestEveryThingHeadTrackingSaysCanBeDrawn(t *testing.T) {
	for _, s := range []string{
		"Too dark to follow your head",
		"no longer following your head",
		"following your head, with the camera light on while it does",
	} {
		if bad := toolkit.BitmapMissing(s); len(bad) > 0 {
			t.Errorf("%q cannot be drawn: the font has no %q", s, string(bad))
		}
	}
}
