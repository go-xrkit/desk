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

// standCamera replaces the seam that would otherwise open a real camera.
func standCamera(t *testing.T, src HeadSource, err error) *int {
	t.Helper()
	closes := 0
	was := openCameraHead
	openCameraHead = func(string) (HeadSource, func() error, error) {
		if err != nil {
			return nil, nil, err
		}
		return src, func() error { closes++; return nil }, nil
	}
	t.Cleanup(func() { openCameraHead = was })
	return &closes
}

// TestTheMenuRowOpensTheCameraAndPutsItAway.
//
// ⭐ THE CAMERA IS KEPT ONCE OPENED, AND CLOSED WHEN THE FEATURE GOES OFF. One
// reopened per toggle would relearn the room each time, and the light would
// flicker with the menu.
func TestTheMenuRowOpensTheCameraAndPutsItAway(t *testing.T) {
	p := stereoPlan(t)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	h := &fakeHead{ok: true}
	closes := standCamera(t, h, nil)

	d.toggleFollowHead()
	if !d.FollowingHead() {
		t.Fatal("the row did not switch it on")
	}
	if *closes != 0 {
		t.Errorf("the camera was closed %d times while still in use", *closes)
	}
	d.toggleFollowHead()
	if d.FollowingHead() {
		t.Error("the row did not switch it off")
	}
	if *closes != 1 {
		t.Errorf("the camera was closed %d times, want once", *closes)
	}
}

// TestARowThatCannotOpenACameraSaysWhy.
//
// ⛔⛔ THE COMMONEST FAILURE IN THE ROOM, and the one a silent menu row would
// hide: a headset attached for its picture only, over a display cable, presents
// no camera at all. Switching on must not report success with nothing behind it.
func TestARowThatCannotOpenACameraSaysWhy(t *testing.T) {
	p := stereoPlan(t)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	standCamera(t, nil, ErrNoRoomCamera)
	d.toggleFollowHead()
	if d.FollowingHead() {
		t.Error("head tracking switched on with no camera behind it")
	}
}

// TestOpeningTwiceKeepsTheFirstCamera: openHead is called on every switch-on,
// and a second open would leak the first and relight the room for nothing.
func TestOpeningTwiceKeepsTheFirstCamera(t *testing.T) {
	p := stereoPlan(t)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	opens := 0
	was := openCameraHead
	openCameraHead = func(string) (HeadSource, func() error, error) {
		opens++
		return &fakeHead{ok: true}, func() error { return nil }, nil
	}
	t.Cleanup(func() { openCameraHead = was })

	if err := d.openHead(); err != nil {
		t.Fatalf("openHead: %v", err)
	}
	if err := d.openHead(); err != nil {
		t.Fatalf("openHead twice: %v", err)
	}
	if opens != 1 {
		t.Errorf("the camera was opened %d times, want once", opens)
	}
}

// TestClosingWithNothingOpenIsHarmless, because the row can be switched off by
// a desk that never managed to switch it on.
func TestClosingWithNothingOpenIsHarmless(t *testing.T) {
	p := stereoPlan(t)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	d.closeHead() // must not panic on the nil closer
}

// TestTheActionReachesTheCamera: the menu row goes through Do like every other,
// and the handler runs OUTSIDE the lock because opening a camera is slow enough
// to stall the frame loop.
func TestTheActionReachesTheCamera(t *testing.T) {
	p := stereoPlan(t)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	standCamera(t, &fakeHead{ok: true}, nil)
	d.Do(ActionFollowHead)
	if !d.FollowingHead() {
		t.Error("ActionFollowHead did not reach the camera")
	}
	d.Do(ActionFollowHead)
	if d.FollowingHead() {
		t.Error("a second ActionFollowHead did not switch it off")
	}
}

// TestTheActionHasAName, because a row with no name is a row nobody can be told
// about when it fails.
func TestTheActionHasAName(t *testing.T) {
	if got := ActionFollowHead.String(); got == "" || got == "ActionFollowHead" {
		t.Errorf("ActionFollowHead is called %q", got)
	}
}

// TestTheKeyboardStillWorksWhileFollowingTheHead.
//
// ⛔⛔ IT DID NOT, AND THE FAILURE WAS THE WORST SHAPE ONE CAN HAVE. followHead
// wrote the yaw every frame, which makes yaw and target equal, so Advance had
// nothing left to ease and a keyboard turn never moved the picture. Measured:
// the focus went from 0 to 1 while the view stayed exactly where it was --
// AN ACTIVE SCREEN NOBODY IS LOOKING AT, with new windows opening on it.
//
// It was found by checking a remark rather than agreeing with it: "we still have
// the keyboard" was an assumption, and it was false.
func TestTheKeyboardStillWorksWhileFollowingTheHead(t *testing.T) {
	p := stereoPlan(t)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	h := &fakeHead{ok: true}
	d.SetHeadSource(h)
	d.FollowHead(true)
	d.Advance(0.02)

	was := d.Nav().Focus()
	d.Nav().Next()
	target := d.Nav().Target()
	for range 60 {
		d.Advance(0.02)
	}
	if d.Nav().Focus() == was {
		t.Fatalf("the focus never left screen %d", was)
	}
	if got := d.Nav().Yaw(); math.Abs(got-target) > 1e-9 {
		t.Errorf("the focus moved to %d but the view is at %v, not the %v it "+
			"was sent to: that is an active screen nobody is looking at",
			d.Nav().Focus(), got, target)
	}
}

// TestTheHeadTakesOverAgainWhereTheKeyboardLeftOff.
//
// ⭐ THE ORIGIN MOVES WITH THE TURN. Without that, the first frame after a
// keyboard move would apply the old offset and drag the view straight back to
// where it came from -- a key that appears to work and then undoes itself.
func TestTheHeadTakesOverAgainWhereTheKeyboardLeftOff(t *testing.T) {
	p := stereoPlan(t)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	h := &fakeHead{ok: true}
	d.SetHeadSource(h)
	d.FollowHead(true)
	h.yaw = 0.3 // the head is not at its origin when the key is pressed
	d.Advance(0.02)

	d.Nav().Next()
	for range 60 {
		d.Advance(0.02)
	}
	landed := d.Nav().Yaw()
	if h.recenters < 2 {
		t.Errorf("the origin was retaken %d time(s): once for switching on, once "+
			"for the turn landing", h.recenters)
	}

	// The head now moves a little from its new origin.
	h.yaw = 0.1
	d.Advance(0.02)
	if got := d.Nav().Yaw(); math.Abs(got-(landed+0.1)) > 1e-9 {
		t.Errorf("the head moved 0.1 from %v and the view went to %v", landed, got)
	}
}

// TestSwitchingItOffGivesTheKeyboardBackExACTLY.
//
// ⭐ THE COMPARISON IS AGAINST A DESK THAT HAS NEVER HEARD OF A HEAD, not
// against an idea of how the keyboard used to behave. Advance now calls
// followHead on every frame of every desk, so "off" has to mean the same
// numbers as "absent" -- and the only way to know that is to run both.
func TestSwitchingItOffGivesTheKeyboardBackExACTLY(t *testing.T) {
	turn := func(d *Desk) (int, float64) {
		for range 3 {
			d.Nav().Next()
			for range 40 {
				d.Advance(0.02)
			}
		}
		d.Nav().Prev()
		for range 40 {
			d.Advance(0.02)
		}
		return d.Nav().Focus(), d.Nav().Yaw()
	}

	p := stereoPlan(t)
	plain, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	wantFocus, wantYaw := turn(plain)

	// The same desk, with a head source that was switched on and then off.
	used, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	h := &fakeHead{ok: true, yaw: 0.9}
	used.SetHeadSource(h)
	used.FollowHead(true)
	used.Advance(0.02)
	used.FollowHead(false)
	used.Nav().SetYaw(plainStart(t, p))

	gotFocus, gotYaw := turn(used)
	if gotFocus != wantFocus {
		t.Errorf("focus %d with head tracking switched off, %d on a desk without it",
			gotFocus, wantFocus)
	}
	if math.Abs(gotYaw-wantYaw) > 1e-9 {
		t.Errorf("yaw %v with head tracking switched off, %v on a desk without it",
			gotYaw, wantYaw)
	}
}

// plainStart is where a fresh desk's yaw sits, so the two can be compared from
// the same place.
func plainStart(t *testing.T, p Plan) float64 {
	t.Helper()
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	return d.Nav().Yaw()
}
