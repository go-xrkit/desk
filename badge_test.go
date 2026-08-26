// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"testing"
)

// TestTheBadgeSaysWhichScreenAndThenGoesAway.
//
// The band moving is silent otherwise: six identical desktops slide past and
// nothing says which one is now in front.
func TestTheBadgeSaysWhichScreenAndThenGoesAway(t *testing.T) {
	b := newBadge(1.5, nil)
	if b == nil {
		t.Fatal("a badge of a second and a half was refused")
	}
	if b.up() {
		t.Error("the badge is up before anything has moved")
	}
	b.show(2, 6)
	if !b.up() {
		t.Fatal("arriving at a screen said nothing")
	}
	// The number and nothing else: it is drawn large enough that "screen 3 of 6"
	// would be a banner across the whole view, and a person turning the band
	// already knows what they are counting. Screens are still counted from 1.
	if got, want := b.toast.Text, "3"; got != want {
		t.Errorf("the badge says %q, want %q", got, want)
	}

	// It goes away on its own, and at about the time it was asked to.
	frames := 0
	for b.up() && frames < 10000 {
		b.tick()
		frames++
	}
	if b.up() {
		t.Fatal("the badge never went away")
	}
	if want := badgeFrames(1.5); frames != want {
		t.Errorf("the badge lasted %d frames, want %d", frames, want)
	}
}

// TestStandingStillDoesNotKeepTheBadgeUp.
//
// Render calls show on every frame. Re-arming each time would leave the number
// on the picture for as long as the band is at rest, which is the opposite of
// the point.
func TestStandingStillDoesNotKeepTheBadgeUp(t *testing.T) {
	b := newBadge(0.5, nil)
	b.show(0, 6)
	for i := 0; i < badgeFrames(0.5); i++ {
		b.show(0, 6) // the same screen, every frame
		b.tick()
	}
	if b.up() {
		t.Error("the badge is still up after its time, because standing still re-armed it")
	}
	// And moving puts it back.
	b.show(1, 6)
	if !b.up() {
		t.Error("moving to another screen said nothing")
	}
}

// TestABadgeTurnedOffDrawsNothingAndTicksNothing. A nil badge has to be safe
// everywhere, because that is what "off" is.
func TestABadgeTurnedOffDrawsNothingAndTicksNothing(t *testing.T) {
	for name, seconds := range map[string]float64{
		"zero":       0,
		"negative":   -1,
		"a very odd": -0.0001,
	} {
		b := newBadge(seconds, nil)
		if b != nil {
			t.Errorf("%s seconds gave a badge", name)
			continue
		}
		// Every method on the nil badge, because Render and Advance call them
		// without asking.
		b.show(1, 6)
		b.tick()
		if b.up() {
			t.Errorf("%s: a badge that is off says it is up", name)
		}
		b.draw(NewCanvas(64, 64))
	}
	if got := badgeFrames(0); got != 0 {
		t.Errorf("badgeFrames(0) = %d", got)
	}
	if got := BadgeDuration(0); got != 0 {
		t.Errorf("BadgeDuration(0) = %v", got)
	}
}

// TestTheBadgeIsDrawnOnThePicture, and in the MIDDLE of it.
//
// It was small and at the bottom, and it was asked for as a large ephemeral
// overlay of the screen you are on — which is right: this is not a label to be
// read, it is an answer to be caught out of the corner of an eye while the band
// is still moving.
func TestTheBadgeIsDrawnOnThePicture(t *testing.T) {
	c := NewCanvas(400, 300)
	before := append([]byte(nil), c.Pix...)

	b := newBadge(1, nil)
	b.show(0, 6)
	b.draw(c)

	changedRows := map[int]bool{}
	for i := 0; i+3 < len(c.Pix); i += 4 {
		if c.Pix[i] != before[i] || c.Pix[i+1] != before[i+1] || c.Pix[i+2] != before[i+2] {
			changedRows[(i/4)/c.W] = true
		}
	}
	if len(changedRows) == 0 {
		t.Fatal("the badge drew nothing")
	}
	lowest, highest := c.H, 0
	for y := range changedRows {
		if y < lowest {
			lowest = y
		}
		if y > highest {
			highest = y
		}
	}
	// Centred, and big: a number that covers a good part of the view is the
	// point of it, and one hugging an edge would be the thing it replaced.
	mid := (lowest + highest) / 2
	if off := mid - c.H/2; off > c.H/10 || off < -c.H/10 {
		t.Errorf("the badge is centred on row %d of %d", mid, c.H)
	}
	if highest >= c.H || lowest < 0 {
		t.Errorf("the badge reaches rows %d..%d, outside the picture", lowest, highest)
	}
	if highest-lowest < c.H/20 {
		t.Errorf("the badge is only %d rows tall in a picture %d tall",
			highest-lowest, c.H)
	}

	// A canvas with no pixels is not a crash.
	b.draw(&Canvas{})
	b.draw(nil)
}

// TestTheDeskShowsTheBadgeWhenTheBandArrives, through the public surface: the
// badge is set once and then Advance and Render do the rest, which is all the
// loop does.
func TestTheDeskShowsTheBadgeWhenTheBandArrives(t *testing.T) {
	p := testPlan(t)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	d.Badge(0.3, nil)

	// The first frame arrives somewhere, so it says so.
	d.Render()
	if !d.badge.up() {
		t.Fatal("the first frame said nothing about which screen it is")
	}

	// It goes away while nothing moves.
	for i := 0; i < badgeFrames(0.3); i++ {
		d.Advance(FrameInterval.Seconds())
		d.Render()
	}
	if d.badge.up() {
		t.Error("the badge is still up after its time")
	}

	// Turning the band brings it back.
	d.Do(ActionNext)
	d.Advance(10) // arrive
	d.Render()
	if !d.badge.up() {
		t.Error("turning the band said nothing")
	}

	// And in the gallery it says nothing: every screen is in front of the
	// viewer at once, so which one is focused is not a question being asked.
	for i := 0; i < badgeFrames(0.3); i++ {
		d.Advance(FrameInterval.Seconds())
		d.Render()
	}
	d.Do(ActionGalleryOpen)
	d.Do(ActionNext)
	d.Render()
	if d.badge.up() {
		t.Error("the gallery showed an arrival badge")
	}

	// Setting it to zero turns it off through the same door.
	d.Badge(0, nil)
	d.Do(ActionGalleryClose)
	d.Do(ActionNext)
	d.Advance(10)
	d.Render()
	if d.badge != nil {
		t.Error("Badge(0) left a badge")
	}
}
