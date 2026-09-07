// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/go-widgets/toolkit"
)

// noticeSays reads the notice the way everything else does: UNDER THE DESK'S
// OWN LOCK.
//
// The handler that puts it up, the frame loop that ticks it and the list coming
// back all hold d.mu, so a test that peeked without it was the only thing
// racing -- and the race detector said so on the darwin lane while every local
// run passed.
func noticeSays(d *Desk) (text string, up bool, life int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.notice.toast.Text, d.notice.up(), d.notice.toast.Life().Get()
}

// deskAt is a small desk with its band at this distance, ready to be pressed at.
func deskAt(t *testing.T, distance float64) *Desk {
	t.Helper()
	p := testPlan(t).WithDistance(distance)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

// changedRows is which rows of the picture were painted on.
func changedRows(c *Canvas, before []byte) map[int]bool {
	rows := map[int]bool{}
	for i := 0; i+3 < len(c.Pix); i += 4 {
		if c.Pix[i] != before[i] || c.Pix[i+1] != before[i+1] || c.Pix[i+2] != before[i+2] {
			rows[(i/4)/c.W] = true
		}
	}
	return rows
}

// TestANoticeIsDrawnLowAndSmall.
//
// ⛔ In PIXELS and not in state: a notice nobody can see is a notice that did
// not happen, and the whole reason it exists is that a key which changes
// nothing visible reads as a key that did nothing.
//
// Low and small, both measured. Low because the badge and both galleries are in
// the middle and this must not cover what a person is reading; small because it
// is a sentence and the badge's size would make it a banner.
func TestANoticeIsDrawnLowAndSmall(t *testing.T) {
	c := NewCanvas(800, 600)
	before := append([]byte(nil), c.Pix...)

	n := newNotice(1, nil, nil)
	n.say("one screen, as large as these glasses show it")
	n.draw(c)

	rows := changedRows(c, before)
	if len(rows) == 0 {
		t.Fatal("the notice drew nothing")
	}
	lowest, highest := c.H, 0
	for y := range rows {
		if y < lowest {
			lowest = y
		}
		if y > highest {
			highest = y
		}
	}
	if lowest < c.H/2 {
		t.Errorf("the notice reaches row %d of %d: it is over the middle of the "+
			"view, where the badge and the galleries are", lowest, c.H)
	}
	if highest >= c.H {
		t.Errorf("the notice reaches row %d, past the bottom edge at %d", highest, c.H)
	}
	if h := highest - lowest; h > c.H/8 {
		t.Errorf("the notice is %d rows tall in a %d-row view: that is a banner", h, c.H)
	}
}

// TestAWaitStaysUntilItIsOver.
//
// ⛔ A placeholder with a life is worse than none: it vanishes while the work
// is still going and says the work failed. So a wait has NO life, and only
// clear takes it down.
func TestAWaitStaysUntilItIsOver(t *testing.T) {
	n := newNotice(0.1, nil, nil) // a very short life, if it had one
	n.waiting("looking for what is running...")
	for range 1000 {
		n.tick()
	}
	if !n.up() {
		t.Fatal("the placeholder expired while the work was still going")
	}
	n.clear()
	if n.up() {
		t.Error("clear did not take it down")
	}

	// A notice that DOES have a life goes away on its own, or it becomes part
	// of the picture.
	n.say("one screen, as large as these glasses show it")
	if !n.up() {
		t.Fatal("say put nothing up")
	}
	for range n.frames + 2 {
		n.tick()
	}
	if n.up() {
		t.Error("a notice with a life never went away")
	}
}

// TestFitSaysSoWhenThereIsNothingToDo.
//
// ⛔ THE REPORT, exactly: "le raccourci pour fit ne semble pas fonctionnel."
// It fired. The band starts at the near end, so on a session where nobody has
// pressed "further" there is nothing for it to change -- and a granted key that
// changes nothing is indistinguishable from one that was never granted.
func TestFitSaysSoWhenThereIsNothingToDo(t *testing.T) {
	d := deskAt(t, MinDistance)
	d.Badge(1, nil, nil)
	d.Do(ActionFit)
	text, up, _ := noticeSays(d)
	if !up {
		t.Fatal("fit at the near end said nothing at all")
	}
	if !strings.Contains(text, "already") {
		t.Errorf("fit said %q; it should say there was nothing to do", text)
	}

	// And from further out it says what it did, because a band that jumps is
	// worth a word too -- but a different one.
	d = deskAt(t, MaxDistance)
	d.Badge(1, nil, nil)
	d.Do(ActionFit)
	if text, _, _ := noticeSays(d); strings.Contains(text, "already") {
		t.Errorf("fit from %g said %q", MaxDistance, text)
	}
	if d.plan.Distance() != MinDistance {
		t.Errorf("the band is at %g, want %g", d.plan.Distance(), MinDistance)
	}
}

// TestTheAppListPutsAPlaceholderUpWhileItIsRead.
//
// ⛔ "on a l'impression que ca ne fonctionne pas le temps que ca arrive."
// Enumerating windows goes through the accessibility API and takes seconds on a
// busy machine; until it comes back the view is unchanged. So something is on
// the picture BEFORE the list is asked for, and it comes down when the list
// arrives -- not on a timer.
func TestTheAppListPutsAPlaceholderUpWhileItIsRead(t *testing.T) {
	d := deskAt(t, MinDistance)
	d.Badge(1, nil, nil)

	asked := make(chan struct{})
	release := make(chan struct{})
	d.OnApps = func() ([]App, error) {
		close(asked)
		<-release
		return []App{{Name: "Firefox"}}, nil
	}
	go d.Do(ActionAppsOpen)
	<-asked

	// While the list is being read.
	_, up, life := noticeSays(d)
	if !up {
		t.Fatal("nothing was on the picture while the list was being read")
	}
	if life != 0 {
		t.Error("the placeholder is counting down; it must wait for the list")
	}
	close(release)
	waitFor(t, func() bool { _, up, _ := noticeSays(d); return !up }, "the placeholder to come down")
}

// TestAListThatCannotBeReadReplacesThePlaceholder, rather than leaving it up
// for ever: a wait that ends in silence is the same picture as one still going.
func TestAListThatCannotBeReadReplacesThePlaceholder(t *testing.T) {
	d := deskAt(t, MinDistance)
	d.Badge(1, nil, nil)
	d.OnApps = func() ([]App, error) { return nil, errors.New("no accessibility grant") }
	d.Do(ActionAppsOpen)
	text, up, life := noticeSays(d)
	if !up {
		t.Fatal("a list that could not be read left nothing on the picture")
	}
	if !strings.Contains(text, "no accessibility grant") {
		t.Errorf("the notice says %q; it should say why", text)
	}
	if life == 0 {
		t.Error("the refusal has no life: it would stay on the picture for ever")
	}
}

// TestANilNoticeIsSilentAndNotACrash: a Desk that was never given a theme has
// none, and every one of these is reached from a global shortcut.
func TestANilNoticeIsSilentAndNotACrash(t *testing.T) {
	var none *notice
	none.say("something")
	none.waiting("something")
	none.clear()
	none.tick()
	none.draw(NewCanvas(10, 10))
	if none.up() {
		t.Error("a notice that does not exist is showing")
	}
}

// TestANoticeOnATinyPictureIsStillDrawn.
//
// The floors, both of them. A view small enough that a twentieth of it is
// nothing at all still gets type at scale one and an inset of one pixel: a
// notice that came out zero-high would be the silence it exists to prevent.
func TestANoticeOnATinyPictureIsStillDrawn(t *testing.T) {
	if got := noticeInk(10); got != 1 {
		t.Errorf("a 10-row view asked for scale %d", got)
	}
	if got := noticeInset(0); got != 1 {
		t.Errorf("a view of no height asked for an inset of %d", got)
	}
	c := NewCanvas(60, 20)
	before := append([]byte(nil), c.Pix...)
	n := newNotice(1, nil, nil)
	n.say("fit")
	n.draw(c)
	if len(changedRows(c, before)) == 0 {
		t.Error("nothing was drawn on a small view")
	}
}

// ⛔ A NOTICE IS WRITTEN DOWN AS WELL AS SHOWN, and the photograph is why. A
// notice lives about three seconds on a panel a hand's width from one eye, so a
// sentence carrying something needed LATER is a sentence somebody had one
// chance to read -- and only if they were wearing the glasses at that moment.
// "photograph saved: /Users/.../2026-09-06-170010.png" was said, was correct,
// was gone, and the next question asked was "where was the photo taken?".
func TestWhatANoticeSaysIsAlsoWrittenDown(t *testing.T) {
	var said []string
	n := newNotice(1, nil, func(f string, a ...any) { said = append(said, fmt.Sprintf(f, a...)) })

	n.say("photograph saved: /somewhere/2026-09-06-170010.png")
	n.waiting("taking a photograph...")
	n.clear()

	if len(said) != 2 {
		t.Fatalf("two sentences were said and %d were written: %q", len(said), said)
	}
	if !strings.Contains(said[0], "2026-09-06-170010.png") {
		t.Errorf("the path was not written down: %q", said[0])
	}
	if !strings.Contains(said[1], "taking a photograph") {
		t.Errorf("a wait was not written down: %q", said[1])
	}
	// clear says nothing: taking a notice down is not an event to report, and a
	// log full of blank lines is a log nobody reads.
}

// A notice with nowhere to write is a notice, not a crash: that is what every
// test here builds and what a caller that passes nil gets.
func TestANoticeWithNowhereToWriteStillShows(t *testing.T) {
	n := newNotice(1, nil, nil)
	n.say("something")
	if !n.up() {
		t.Error("a notice with no log did not go up")
	}
}

// TestALongNoticeIsWrappedToFitTheView.
//
// ⛔⛔ THE DEFECT THIS EXISTS FOR, AND IT HAD ALREADY SHIPPED. A Toast sizes
// itself to its widest line with no upper bound, and this one is docked to the
// bottom CENTRE -- so a sentence wider than the view is painted past both edges
// and the reader is left with the MIDDLE of it, which does not look truncated.
//
// Measured on a 1920-wide view at the size a notice draws at: "the camera was
// refused; it is turned on again in System Settings > Privacy & Security >
// Camera" came to 2373px, 1.2 times the width it had. Anybody whose camera was
// denied had been reading the middle of that sentence for weeks.
func TestALongNoticeIsWrappedToFitTheView(t *testing.T) {
	const long = "desk: the headset's camera was not found: \"VITURE Beast\" has " +
		"none. The camera hangs off the same USB hub as the glasses, so a headset " +
		"attached for its picture only -- over a display cable -- does not present " +
		"one. Name a camera with -photo-camera to override"
	c := NewCanvas(800, 600)

	// THE CONTROL FIRST. If this sentence already fits the view unwrapped, the
	// assertion below is vacuous and would pass with the cap removed.
	was := toolkit.CurrentFont()
	toolkit.SetFont(overlayFont(noticeInk(c.H)))
	wide := toolkit.TextWidth(long)
	toolkit.SetFont(was)
	if wide <= c.W {
		t.Fatalf("the sentence measures %dpx in a %dpx view: it already fits, so "+
			"this test is not measuring anything", wide, c.W)
	}

	n := newNotice(1, nil, nil)
	n.say(long)
	n.draw(c)

	r := n.toast.Bounds()
	if r.X < 0 || r.X+r.W > c.W {
		t.Errorf("the pill is %dpx wide at x=%d in a %dpx view: it hangs off the "+
			"edge, so both ends of the sentence are cut", r.W, r.X, c.W)
	}
	if cap := noticeMaxW(c.W); r.W > cap {
		t.Errorf("the pill is %dpx, over the %dpx it was capped at", r.W, cap)
	}
	// And it must have become several rows: a pill that fits by losing words
	// would pass the width check and still be wrong.
	if h := r.H; h <= toolkit.GlyphHeight() {
		t.Errorf("the pill is %dpx tall: a sentence that did not fit on one line "+
			"must have been broken across several", h)
	}
}

// TestNoticeMaxWOnAViewTooNarrowToTrim: a view too narrow to trim gets the view
// itself rather than a non-positive cap, which would leave the words no room.
func TestNoticeMaxWOnAViewTooNarrowToTrim(t *testing.T) {
	if got := noticeMaxW(0); got != 0 {
		t.Errorf("noticeMaxW(0) = %d, want 0", got)
	}
	if got, want := noticeMaxW(800), 800-2*40; got != want {
		t.Errorf("noticeMaxW(800) = %d, want %d (a twentieth off each side)", got, want)
	}
}
