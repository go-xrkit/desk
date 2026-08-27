// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-macos/hotkey"
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
// TestAsManyScreensAsAPersonWants.
//
// A curved band had to fit in 360 degrees, and at one screen per view that was
// seven of them and no more. Flat, the circle is a fiction -- the yaw says how
// far along the band the viewer is -- so nine screens lay out as readily as
// three, and every one of them is still exactly one view wide.
//
// The ceiling is therefore not the geometry's. See TestTheCeilingOnScreens.
func TestAsManyScreensAsAPersonWants(t *testing.T) {
	for _, n := range []int{1, 2, 3, 4, 6, 9} {
		p, err := NewPlan(glasses.Display{Name: "VITURE Beast", Width: 3840, Height: 1080},
			Options{Screens: n})
		if err != nil {
			t.Errorf("%d screens: NewPlan = %v", n, err)
			continue
		}
		d, err := New(p, feedsFor(p))
		if err != nil {
			t.Errorf("%d screens: New = %v", n, err)
			continue
		}
		for i := 0; i < n; i++ {
			if got := d.strip.width[i]; got != p.ScreenW {
				t.Errorf("%d screens: screen %d is %d wide on the band, want the view's %d",
					n, i, got, p.ScreenW)
			}
		}
		d.Close()
	}
}

// TestRenderPutsEachScreensPixelsInThePicture: where the band says, and
// nowhere else.
func TestRenderPutsEachScreensPixelsInThePicture(t *testing.T) {
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
	// Half the arc per screen, with the pitch left alone: twelve screens round
	// the band, each half a view wide, with the other half of every pitch left
	// as space. This is the shape promotion is FOR.
	p = withCount(p, 12)
	p, err := NewPlan(glasses.Display{Name: "VITURE Beast", Width: 3840, Height: 1080},
		Options{Screens: 12})
	if err != nil {
		t.Fatalf("NewPlan = %v", err)
	}
	p.Layout.DensityDeg /= 2
	p.Layout.FullWidthDeg /= 2
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
	// The canvas is the PICTURE now, not a panorama the picture is a projection
	// of: one view is one screen.
	if c := d.Canvas(); c.W != p.ScreenW || c.H != p.ScreenH {
		t.Errorf("Canvas() is %dx%d, want the view's %dx%d", c.W, c.H, p.ScreenW, p.ScreenH)
	}
}

func TestActionString(t *testing.T) {
	for a, want := range map[Action]string{
		ActionNone: "none", ActionNext: "next", ActionPrev: "previous",
		ActionFullscreen: "fullscreen", ActionCycle: "cycle", ActionQuit: "quit",
		ActionGallery: "gallery", ActionChoose: "choose",
		ActionGalleryOpen: "open the gallery", ActionGalleryClose: "leave the gallery",
		ActionUp: "up", ActionDown: "down", Action(99): "none",
	} {
		if got := a.String(); got != want {
			t.Errorf("Action(%d).String() = %q, want %q", a, got, want)
		}
	}
}

// withCount returns the plan with a different number of screens on it.
func withCount(p Plan, n int) Plan { p.count = n; return p }

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
		if _, err := d.strip.Fullscreen(nil, focus); err != nil {
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
		"Tab": ActionCycle, "c": ActionCycle, "C": ActionCycle,
		"g": ActionGallery, "G": ActionGallery,
		"Enter": ActionChoose, "Return": ActionChoose,
		"ArrowUp": ActionUp, "k": ActionUp, "K": ActionUp,
		"ArrowDown": ActionDown, "j": ActionDown, "J": ActionDown,
		"x": ActionNone, "": ActionNone, "PageUp": ActionNone,
	} {
		if got := KeyAction(code); got != want {
			t.Errorf("KeyAction(%q) = %v, want %v", code, got, want)
		}
	}
}

// TestTheViewerAndTheFrameLoopAreDifferentGoroutines is the test that should
// have existed the moment Run was written.
//
// Keys arrive on the window back-end's EVENT thread; the ribbon is advanced and
// drawn on a TICKER. Neither ribbon.Nav nor the desk's own slices are safe on
// their own, and every test until now was single-goroutine, so nothing could
// see it. Under -race this fails without the lock and passes with it.
func TestTheViewerAndTheFrameLoopAreDifferentGoroutines(t *testing.T) {
	p := testPlan(t)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatalf("New = %v", err)
	}
	defer d.Close()

	var wg sync.WaitGroup

	// The frame loop. It runs a fixed number of turns rather than until a stop
	// signal: a goroutine waited on by the same WaitGroup that would close the
	// signal never gets one, which is how the first version of this test hung.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			d.Advance(1.0 / 60)
			d.Render()
			_ = d.Quit()
		}
	}()

	// The viewer, on another goroutine, doing everything a viewer can do.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			d.Do(ActionNext)
			d.Do(ActionFullscreen)
			d.Do(ActionPrev)
			_ = d.FeedAt(i % p.Count())
		}
	}()

	// And a third changing what the screens show underneath both.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			old, err := d.SetFeed(i%p.Count(), newFakeFeed(p.ScreenW, p.ScreenH, byte(i)))
			if err != nil {
				t.Errorf("SetFeed = %v", err)
				return
			}
			if old != nil {
				_ = old.Close()
			}
		}
	}()

	wg.Wait()
}

// TestSetFeedHandsBackTheOldOneWithoutClosingIt: a viewer parking a screen to
// look at something else usually wants it back, so swapping and discarding must
// not be the same gesture.
func TestSetFeedHandsBackTheOldOneWithoutClosingIt(t *testing.T) {
	p := testPlan(t)
	feeds := feedsFor(p)
	first := feeds[1].(*fakeFeed)
	d, err := New(p, feeds)
	if err != nil {
		t.Fatalf("New = %v", err)
	}
	d.Render() // give position 1 a picture to forget

	replacement := newFakeFeed(p.ScreenW, p.ScreenH, 200)
	old, err := d.SetFeed(1, replacement)
	if err != nil {
		t.Fatalf("SetFeed = %v", err)
	}
	if old != Feed(first) {
		t.Error("SetFeed did not hand back the feed that was there")
	}
	if first.closes != 0 {
		t.Errorf("SetFeed closed the old feed %d times; swapping is not discarding", first.closes)
	}
	if got := d.FeedAt(1); got != Feed(replacement) {
		t.Error("FeedAt does not report the new feed")
	}

	// The old picture must be dropped with the old feed: showing the previous
	// screen's contents on a panel that now holds something else is a lie about
	// what is on it.
	replacement.neverReady = true
	c := d.Render()
	blits := d.strip.Frame(nil, d.strip.Offset(d.nav.Yaw()))
	for _, b := range blits {
		if b.Screen != 1 {
			continue
		}
		o := ((b.Dst.Y+b.Dst.H/2)*c.W + b.Dst.X + b.Dst.W/2) * 4
		if c.Pix[o] == 20 { // the tag the old feed painted
			t.Error("the old feed's picture survived the swap")
		}
	}
}

func TestSetFeedRefusesAPositionThatIsNotThere(t *testing.T) {
	p := testPlan(t)
	d, _ := New(p, feedsFor(p))
	for _, i := range []int{-1, p.Count(), 999} {
		if _, err := d.SetFeed(i, nil); !errors.Is(err, ErrNoScreens) {
			t.Errorf("SetFeed(%d) = %v, want ErrNoScreens", i, err)
		}
		if d.FeedAt(i) != nil {
			t.Errorf("FeedAt(%d) returned something", i)
		}
	}
}

// TestCycleIsAnsweredWithoutTheLock is what lets a handler change the very
// thing the action is about.
//
// The obvious implementation calls OnCycle while holding the desk's lock, and
// then SetFeed — the only useful thing a handler can do — deadlocks. It would
// deadlock in the application and in no test, because no test called SetFeed
// from a handler until this one.
func TestCycleIsAnsweredWithoutTheLock(t *testing.T) {
	p := testPlan(t)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatalf("New = %v", err)
	}
	defer d.Close()

	var got int
	done := make(chan struct{})
	d.OnCycle = func(pos int) {
		got = pos
		// The whole point: a handler must be able to take the lock.
		if _, err := d.SetFeed(pos, newFakeFeed(p.ScreenW, p.ScreenH, 77)); err != nil {
			t.Errorf("SetFeed from the handler = %v", err)
		}
		_ = d.FeedAt(pos)
		close(done)
	}

	d.Do(ActionNext) // move the focus somewhere that is not the default
	want := d.Nav().Focus()

	// Do runs on its OWN goroutine here, because a deadlock inside it would
	// otherwise take the test goroutine with it and this would HANG rather than
	// fail. A test that hangs in CI is worse than one that fails: it says
	// nothing, slowly.
	go d.Do(ActionCycle)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("OnCycle deadlocked: it is being called with the desk's lock held")
	}
	if got != want {
		t.Errorf("OnCycle was told position %d, want the focused %d", got, want)
	}
}

// TestCycleWithNoHandlerDoesNothing: a key nobody wired must not be an error.
func TestCycleWithNoHandlerDoesNothing(t *testing.T) {
	p := testPlan(t)
	d, _ := New(p, feedsFor(p))
	before := d.Nav().Focus()
	d.Do(ActionCycle)
	if d.Nav().Focus() != before {
		t.Error("cycling with no handler moved the focus")
	}
	if d.Quit() {
		t.Error("cycling with no handler ended the session")
	}
}

// TestTheArrowsMeanTwoThings is the whole of wiring a gallery into a ribbon:
// the same keys turn the band in one mode and move a selection in a grid in the
// other, and they must not mean both at once.
func TestTheArrowsMeanTwoThings(t *testing.T) {
	p := testPlan(t)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatalf("New = %v", err)
	}
	defer d.Close()

	yawBefore := d.Nav().Yaw()
	focusBefore := d.Nav().Focus()

	d.Do(ActionGallery)
	if d.Nav().Mode() != ribbon.ModeGallery {
		t.Fatalf("the gallery did not open: mode is %v, err %v", d.Nav().Mode(), d.Err())
	}
	// In the gallery, Next must move the SELECTION and leave the ribbon alone.
	for _, a := range []Action{ActionNext, ActionDown, ActionPrev, ActionUp} {
		d.Do(a)
		if err := d.Err(); err != nil {
			t.Fatalf("%v in the gallery: %v", a, err)
		}
	}
	d.Do(ActionNext)
	if d.Nav().Yaw() != yawBefore {
		t.Error("moving in the gallery turned the ribbon")
	}

	// And the picture must be the grid, not the band.
	gallery := append([]byte(nil), d.Render().Pix...)

	// Closing restores the ribbon exactly — not nearly.
	d.Do(ActionGallery)
	if d.Nav().Mode() != ribbon.ModeRibbon {
		t.Fatalf("the gallery did not close: mode is %v", d.Nav().Mode())
	}
	if d.Nav().Yaw() != yawBefore || d.Nav().Focus() != focusBefore {
		t.Errorf("closing left yaw %v focus %d, want %v and %d",
			d.Nav().Yaw(), d.Nav().Focus(), yawBefore, focusBefore)
	}
	ribbonPix := d.Render().Pix
	same := true
	for i := range gallery {
		if gallery[i] != ribbonPix[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("the gallery drew exactly the same picture as the ribbon")
	}

	// Out of the gallery, Up and Down mean nothing: a band has no rows.
	d.Do(ActionUp)
	if d.Nav().Yaw() != yawBefore {
		t.Error("Up turned the ribbon; a band has no rows")
	}
}

// TestChoosingFromTheGalleryLandsOnTheHighlightedScreen.
func TestChoosingFromTheGalleryLandsOnTheHighlightedScreen(t *testing.T) {
	p := testPlan(t)
	d, _ := New(p, feedsFor(p))
	defer d.Close()

	d.Do(ActionGallery)
	d.Do(ActionNext)
	d.Do(ActionNext)
	want := d.grid.Selected()

	d.Do(ActionChoose)
	if err := d.Err(); err != nil {
		t.Fatalf("choosing: %v", err)
	}
	if d.Nav().Mode() != ribbon.ModeRibbon {
		t.Error("choosing did not return to the ribbon")
	}
	if got := d.Nav().Focus(); got != want {
		t.Errorf("focus is %d, want the highlighted %d", got, want)
	}
}

// TestQuittingFromTheGallery: Escape must stop the application from inside the
// gallery too. A mode that swallows quit is a mode you cannot leave.
func TestQuittingFromTheGallery(t *testing.T) {
	p := testPlan(t)
	d, _ := New(p, feedsFor(p))
	defer d.Close()

	d.Do(ActionGallery)
	d.Do(ActionCycle) // means nothing here, and must not crash or quit
	if d.Quit() {
		t.Fatal("Cycle quit the application")
	}
	d.Do(ActionQuit)
	if !d.Quit() {
		t.Error("Escape did not stop the application from inside the gallery")
	}
}

// TestNewRefusesAPlanItCannotDraw.
//
// Plan is a plain struct with exported fields, so a caller can hand New one
// whose screens have no pixels. Both the band and the gallery are laid out
// from those numbers, and neither can be laid out from nothing.
func TestNewRefusesAPlanItCannotDraw(t *testing.T) {
	for name, spoil := range map[string]func(*Plan){
		"screens with no columns": func(p *Plan) { p.ScreenW = 0 },
		"screens with no rows":    func(p *Plan) { p.ScreenH = 0 },
		// The screens are placed, and each takes so little of the band that it
		// rounds to no pixels at all.
		"screens too narrow to draw": func(p *Plan) {
			p.Layout.DensityDeg = 1e-12
			p.Layout.FullWidthDeg = 1e-12
		},
		// The band lays out — every screen is a hundred and forty pixels wide —
		// and no fold of twenty of them leaves a cell in a hundred-pixel view.
		"more screens than any gallery holds": func(p *Plan) {
			p.ScreenW, p.ScreenH = 100, 100
			p.count = 20
			p.Layout.DensityDeg = 17
			p.Layout.FullWidthDeg = 17
			p.Layout.GapDeg = 0.5
		},
	} {
		p := testPlan(t)
		spoil(&p)
		if _, err := New(p, feedsFor(p)); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

// TestOpenAndLeaveTheGalleryAreSeparateKeys.
//
// A system-wide shortcut is pressed BLIND: the viewer cannot see whether the
// gallery is open before deciding, so one key meaning "open" from outside and
// "close" from inside does the wrong thing every time they have lost track.
func TestOpenAndLeaveTheGalleryAreSeparateKeys(t *testing.T) {
	p := testPlan(t)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	// Leaving a gallery you are not in does nothing at all.
	d.Do(ActionGalleryClose)
	if d.Nav().Mode() != ribbon.ModeRibbon {
		t.Fatalf("leaving from the band put the mode at %v", d.Nav().Mode())
	}
	if err := d.Err(); err != nil {
		t.Errorf("leaving from the band complained: %v", err)
	}

	// Up goes in, and pressing it again keeps it in rather than toggling out.
	d.Do(ActionGalleryOpen)
	if d.Nav().Mode() != ribbon.ModeGallery {
		t.Fatalf("opening did not open: %v, %v", d.Nav().Mode(), d.Err())
	}
	d.Do(ActionGalleryOpen)
	if d.Nav().Mode() != ribbon.ModeGallery {
		t.Error("pressing open twice left the gallery")
	}

	// Down comes out, and again does nothing.
	d.Do(ActionGalleryClose)
	if d.Nav().Mode() != ribbon.ModeRibbon {
		t.Fatalf("leaving did not leave: %v", d.Nav().Mode())
	}
	d.Do(ActionGalleryClose)
	if d.Nav().Mode() != ribbon.ModeRibbon {
		t.Error("pressing leave twice went back in")
	}
}

// TestTheGalleryKeysAreTheOnesThatWereAskedFor: control+option+command with the
// up and down arrows, claimed system-wide.
func TestTheGalleryKeysAreTheOnesThatWereAskedFor(t *testing.T) {
	want := map[Action]hotkey.Combo{
		ActionGalleryOpen: {Key: hotkey.KeyUpArrow,
			Mods: hotkey.Control | hotkey.Option | hotkey.Command},
		ActionGalleryClose: {Key: hotkey.KeyDownArrow,
			Mods: hotkey.Control | hotkey.Option | hotkey.Command},
	}
	got := map[Action]hotkey.Combo{}
	for _, s := range DefaultShortcuts() {
		got[s.Does] = s.Want
	}
	for a, c := range want {
		if got[a] != c {
			t.Errorf("%v is %v, want %v", a, got[a], c)
		}
	}
}

// TestTheCeilingOnScreens: nine, clamped where a program asks and refused where
// a person does.
//
// The distinction is the point. A plan composed in code takes the nearest legal
// answer, because a caller that asked for twelve wants a desk, not an error. A
// number in somebody's settings file is a sentence they wrote: running six
// because they asked for sixty, silently, is worse than saying what the ceiling
// is -- and since the ceiling is arbitrary, it has to be stated wherever it
// bites.
func TestTheCeilingOnScreens(t *testing.T) {
	if MaxScreens != DefaultColumns*DefaultColumns {
		t.Errorf("MaxScreens is %d and the gallery is %d columns: nine was chosen "+
			"because it fills three rows of three exactly",
			MaxScreens, DefaultColumns)
	}

	d := glasses.Display{Name: "VITURE Beast", Width: 3840, Height: 1080}
	for _, n := range []int{MaxScreens + 1, 12, 40, 1000} {
		p, err := NewPlan(d, Options{Screens: n})
		if err != nil {
			t.Fatalf("%d screens: NewPlan = %v", n, err)
		}
		if got := p.Count(); got != MaxScreens {
			t.Errorf("%d screens came back as %d, want the ceiling %d", n, got, MaxScreens)
		}
	}
	// A negative count is a caller's mistake rather than a big desk, and has
	// always been refused.
	if _, err := NewPlan(d, Options{Screens: -3}); !errors.Is(err, ErrScreens) {
		t.Errorf("a negative count gave %v, want ErrScreens", err)
	}
	// The geometry itself clamps at both ends, which is what Grow leans on.
	base, err := NewPlan(d, Options{Screens: 3})
	if err != nil {
		t.Fatalf("NewPlan = %v", err)
	}
	if got := base.WithScreens(1000).Count(); got != MaxScreens {
		t.Errorf("WithScreens(1000) gave %d, want the ceiling %d", got, MaxScreens)
	}
	if got := base.WithScreens(0).Count(); got != 1 {
		t.Errorf("WithScreens(0) gave %d, want the floor 1", got)
	}

	// A settings file naming more than the ceiling is refused, with the number
	// and the ceiling both in the message.
	over := MaxScreens + 1
	cfg := Config{Ribbon: &ConfigRibbon{Screens: &over}}
	err = cfg.check()
	if err == nil {
		t.Fatalf("a settings file asking for %d screens was accepted", over)
	}
	for _, want := range []string{"10", "9"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not say %q: %v", want, err)
		}
	}
	// The ceiling itself is fine.
	ok := MaxScreens
	if err := (Config{Ribbon: &ConfigRibbon{Screens: &ok}}).check(); err != nil {
		t.Errorf("%d screens, the ceiling itself, was refused: %v", ok, err)
	}

	// Nothing the settings window offers is above the ceiling, so a person
	// cannot pick something the file would then refuse.
	for _, n := range screenCounts {
		if n < 1 || n > MaxScreens {
			t.Errorf("the window offers %d screens, outside 1..%d", n, MaxScreens)
		}
	}
}
