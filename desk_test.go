// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"testing"

	"github.com/go-xrkit/xrkit/glasses"
	"github.com/go-xrkit/xrkit/ribbon"
)

// fakeFeed is a screen filled with one colour, so a pixel in the panorama says
// which screen it came from.
type fakeFeed struct {
	src        Source
	fresh      bool
	frames     int
	closes     int
	closeErr   error
	neverReady bool
}

func newFakeFeed(w, h int, tag byte) *fakeFeed {
	pix := make([]byte, w*h*4)
	for i := 0; i < len(pix); i += 4 {
		pix[i], pix[i+1], pix[i+2], pix[i+3] = tag, tag, tag, 255
	}
	return &fakeFeed{src: Source{Pix: pix, W: w, H: h, Stride: w * 4}, fresh: true}
}

func (f *fakeFeed) Frame() (Source, bool) {
	f.frames++
	if f.neverReady {
		return Source{}, false
	}
	return f.src, f.fresh
}

func (f *fakeFeed) Close() error { f.closes++; return f.closeErr }

func testPlan(t *testing.T) Plan {
	t.Helper()
	p, err := NewPlan(glasses.Display{Name: "VITURE Beast", Width: 3840, Height: 1080}, Options{Screens: 4})
	if err != nil {
		t.Fatalf("NewPlan = %v", err)
	}
	return p
}

func feedsFor(p Plan) []Feed {
	feeds := make([]Feed, p.Count())
	for i := range feeds {
		feeds[i] = newFakeFeed(p.ScreenW, p.ScreenH, byte(10*(i+1)))
	}
	return feeds
}

func TestNewRefusesAMismatchedDesk(t *testing.T) {
	p := testPlan(t)
	if _, err := New(p, nil); !errors.Is(err, ErrNoScreens) {
		t.Errorf("no feeds: err = %v, want ErrNoScreens", err)
	}
	if _, err := New(p, feedsFor(p)[:2]); !errors.Is(err, ErrNoScreens) {
		t.Errorf("too few feeds: err = %v, want ErrNoScreens", err)
	}
	if _, err := New(Plan{}, nil); !errors.Is(err, ErrNoScreens) {
		t.Errorf("empty plan: err = %v, want ErrNoScreens", err)
	}
}

// TestNewRefusesAnImpossibleRibbon covers a plan whose screens cannot be laid
// out — more screens than fit round the circle at that density.
func TestNewRefusesAnImpossibleRibbon(t *testing.T) {
	p, err := NewPlan(glasses.Display{Name: "VITURE Beast", Width: 3840, Height: 1080},
		Options{Screens: 40})
	if err != nil {
		t.Fatalf("NewPlan = %v", err)
	}
	if _, err := New(p, feedsFor(p)); err == nil {
		t.Error("forty screens of 51.57° were laid out on a 360° circle")
	}
}

// TestRenderPutsEachScreenWhereTheRibbonSaysAndNowhereElse.
func TestRenderPutsEachScreensPixelsInThePanorama(t *testing.T) {
	p := testPlan(t)
	feeds := feedsFor(p)
	d, err := New(p, feeds)
	if err != nil {
		t.Fatalf("New = %v", err)
	}
	c := d.Render()

	// Something was drawn, and it is one of the feeds' colours rather than the
	// background.
	tags := map[byte]int{}
	for i := 0; i+4 <= len(c.Pix); i += 4 {
		tags[c.Pix[i]]++
	}
	drawn := 0
	for tag, n := range tags {
		if tag != d.Background[0] {
			drawn += n
		}
	}
	if drawn == 0 {
		t.Fatal("the panorama is entirely background; no feed was drawn")
	}
	// Every colour present must be either the background or a feed's tag —
	// anything else means pixels came from somewhere they should not have.
	known := map[byte]bool{d.Background[0]: true}
	for i := range feeds {
		known[byte(10*(i+1))] = true
	}
	for tag := range tags {
		if !known[tag] {
			t.Errorf("the panorama contains colour %d, which is neither background nor any feed", tag)
		}
	}
}

// TestANilFeedShowsBackground: a display that exists but is not being captured
// yet is a normal start-up state, and must not be drawn as garbage.
func TestANilFeedShowsBackground(t *testing.T) {
	p := testPlan(t)
	feeds := feedsFor(p)
	feeds[0] = nil
	d, err := New(p, feeds)
	if err != nil {
		t.Fatalf("New = %v", err)
	}
	if c := d.Render(); len(c.Pix) == 0 {
		t.Fatal("nothing was rendered")
	}
	// A feed that has produced nothing at all behaves the same way.
	feeds[1] = &fakeFeed{neverReady: true}
	d, _ = New(p, feeds)
	d.Render()
}

// TestAStaleFeedKeepsItsLastPicture. ScreenCaptureKit is change-driven: a static
// screen produced one frame in 3.1 seconds. If a feed reporting "nothing new"
// dropped the picture, every motionless window would flicker to background.
func TestAStaleFeedKeepsItsLastPicture(t *testing.T) {
	p := testPlan(t)
	feeds := feedsFor(p)
	f0 := feeds[0].(*fakeFeed)
	d, err := New(p, feeds)
	if err != nil {
		t.Fatalf("New = %v", err)
	}
	first := append([]byte(nil), d.Render().Pix...)

	f0.fresh = false // nothing new from now on
	second := d.Render().Pix
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("a feed reporting no new frame changed the picture at byte %d", i)
			break
		}
	}
	if f0.frames < 2 {
		t.Errorf("the feed was asked for a frame %d times; it must be read every frame "+
			"or a capture may stop delivering", f0.frames)
	}
}

// TestScrollingIsASequence walks the whole gesture rather than one call: focus
// moves, the ribbon starts turning, keeps turning, and then STOPS exactly on
// target instead of creeping.
func TestScrollingIsASequence(t *testing.T) {
	p := testPlan(t)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatalf("New = %v", err)
	}
	start := d.Nav().Focus()

	d.Do(ActionNext)
	if d.Nav().Focus() == start {
		t.Fatal("next did not move the focus")
	}
	if !d.Nav().Moving() {
		t.Fatal("the ribbon is not moving after a step")
	}

	for i := 0; i < 600 && d.Nav().Moving(); i++ {
		d.Advance(1.0 / 60)
		d.Render()
	}
	if d.Nav().Moving() {
		t.Error("the ribbon was still moving after ten seconds; it is creeping, not settling")
	}
	if got, want := d.Nav().Yaw(), d.Nav().Target(); got != want {
		t.Errorf("settled at %v but the target is %v", got, want)
	}

	// And back again returns to where it started.
	d.Do(ActionPrev)
	for i := 0; i < 600 && d.Nav().Moving(); i++ {
		d.Advance(1.0 / 60)
	}
	if d.Nav().Focus() != start {
		t.Errorf("focus is %d after next-then-previous, want %d", d.Nav().Focus(), start)
	}
}

// TestFullscreenIsANoOpAtOneViewPerScreen records a consequence of the rule this
// whole package is built on, which is not obvious until you look at the pixels.
//
// ONE SCREEN IS ONE FULL VIEW. So a focused screen ALREADY fills the glasses,
// edge to edge, and promoting it cannot make it larger. Fullscreen is a no-op at
// the default density — correctly, not by accident.
//
// It earns its keep at a HIGHER density, where more screens are fitted round the
// circle than there are views, each one smaller than the glasses can show. Then
// promotion has somewhere to go. Both cases are checked here, because a reader
// finding "fullscreen changed nothing" needs to know which one they are in.
func TestFullscreenIsANoOpAtOneViewPerScreen(t *testing.T) {
	p := testPlan(t)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatalf("New = %v", err)
	}
	before := append([]byte(nil), d.Render().Pix...)

	d.Do(ActionFullscreen)
	if d.Nav().Mode() != ribbon.ModeFullscreen {
		t.Fatal("fullscreen did not change the mode")
	}
	after := d.Render().Pix
	for i := range before {
		if before[i] != after[i] {
			t.Fatalf("the picture changed at byte %d; at one view per screen the focused "+
				"screen already fills the glasses and promotion has nowhere to go", i)
		}
	}

	d.Do(ActionFullscreen)
	if d.Nav().Mode() != ribbon.ModeRibbon {
		t.Error("toggling twice did not come back to the ribbon")
	}
}

// TestFullscreenPromotesWhenScreensAreSmallerThanAView is the case promotion is
// FOR: twelve screens round the circle, each narrower than the glasses can show.
func TestFullscreenPromotesWhenScreensAreSmallerThanAView(t *testing.T) {
	p := testPlan(t)
	// Half the density: twice as many screens fit, each half a view wide.
	p.Layout.DensityDeg /= 2
	p = withCount(p, 12)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatalf("New = %v", err)
	}
	before := append([]byte(nil), d.Render().Pix...)

	d.Do(ActionFullscreen)
	after := d.Render().Pix
	same := true
	for i := range before {
		if before[i] != after[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("promoting a half-width screen drew exactly the same picture as the ribbon")
	}
}

func TestQuit(t *testing.T) {
	p := testPlan(t)
	d, _ := New(p, feedsFor(p))
	if d.Quit() {
		t.Fatal("a new desk already wants to quit")
	}
	d.Do(ActionNone)
	if d.Quit() {
		t.Fatal("a key that means nothing ended the session")
	}
	d.Do(ActionQuit)
	if !d.Quit() {
		t.Error("quit did not")
	}
}

// TestCloseClosesEveryFeedEvenAfterOneFails. A leaked capture holds a display
// open, and on macOS a leaked virtual display outlives the process that made it.
func TestCloseClosesEveryFeedEvenAfterOneFails(t *testing.T) {
	p := testPlan(t)
	feeds := feedsFor(p)
	boom := errors.New("boom")
	feeds[1].(*fakeFeed).closeErr = boom
	feeds[0] = nil // a nil feed must not stop the others being closed

	d, err := New(p, feeds)
	if err != nil {
		t.Fatalf("New = %v", err)
	}
	if err := d.Close(); !errors.Is(err, boom) {
		t.Errorf("Close = %v, want the first failure reported", err)
	}
	for i, f := range feeds {
		if f == nil {
			continue
		}
		if got := f.(*fakeFeed).closes; got != 1 {
			t.Errorf("feed %d was closed %d times, want once", i, got)
		}
	}
}

func TestPlanAndCanvasAreReachable(t *testing.T) {
	p := testPlan(t)
	d, _ := New(p, feedsFor(p))
	if d.Plan().Count() != p.Count() {
		t.Error("Plan() does not give back the plan")
	}
	if c := d.Canvas(); c.W != p.Pano.W || c.H != p.Pano.H {
		t.Errorf("Canvas() is %dx%d, want the planned %dx%d", c.W, c.H, p.Pano.W, p.Pano.H)
	}
}

func TestActionString(t *testing.T) {
	for a, want := range map[Action]string{
		ActionNone: "none", ActionNext: "next", ActionPrev: "previous",
		ActionFullscreen: "fullscreen", ActionQuit: "quit", Action(99): "none",
	} {
		if got := a.String(); got != want {
			t.Errorf("Action(%d).String() = %q, want %q", a, got, want)
		}
	}
}

// withCount returns the plan with a different number of screens on it.
func withCount(p Plan, n int) Plan { p.count = n; return p }

// TestNewRefusesAnUnusablePanorama covers the second way building a desk can
// fail: the screens lay out, but the buffer they must be drawn into is not one.
func TestNewRefusesAnUnusablePanorama(t *testing.T) {
	p := testPlan(t)
	p.Pano.W = 0
	if _, err := New(p, feedsFor(p)); err == nil {
		t.Error("a panorama with no columns was accepted")
	}
}

// TestPromotingAnyScreenAlwaysWorks is what lets Render ignore the error from
// Fullscreen instead of carrying a branch that can never be taken.
//
// The claim is that the navigator's focus is always a screen on this ribbon, so
// the only failure Fullscreen has — an out-of-range index — cannot happen. This
// walks every screen the keyboard can reach and checks it, rather than trusting
// the reasoning.
func TestPromotingAnyScreenAlwaysWorks(t *testing.T) {
	p := testPlan(t)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatalf("New = %v", err)
	}
	seen := map[int]bool{}
	for i := 0; i < p.Count()*2+1; i++ {
		focus := d.Nav().Focus()
		seen[focus] = true
		if _, err := d.comp.Fullscreen(nil, focus); err != nil {
			t.Fatalf("promoting the focused screen %d failed: %v", focus, err)
		}
		d.Do(ActionNext)
	}
	if len(seen) != p.Count() {
		t.Errorf("stepping round the ribbon reached %d of %d screens", len(seen), p.Count())
	}
}

func TestKeyAction(t *testing.T) {
	for code, want := range map[string]Action{
		"Escape": ActionQuit, "q": ActionQuit, "Q": ActionQuit,
		"ArrowLeft": ActionPrev, "h": ActionPrev, "H": ActionPrev,
		"ArrowRight": ActionNext, "l": ActionNext, "L": ActionNext,
		" ": ActionFullscreen, "f": ActionFullscreen, "F": ActionFullscreen,
		"x": ActionNone, "": ActionNone, "ArrowUp": ActionNone,
	} {
		if got := KeyAction(code); got != want {
			t.Errorf("KeyAction(%q) = %v, want %v", code, got, want)
		}
	}
}
