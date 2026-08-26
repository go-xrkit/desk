// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-macos/accessibility"
)

// fakeWindow is a window on a bench with no window server behind it. It
// records where it was put, and it really moves — Frame reads back what
// SetPosition wrote, which is what the library's own read-back check needs to
// see to call a move done.
type fakeWindow struct {
	frame  accessibility.Rect
	raised int
	refuse bool
	// wander, when set, is where this window really goes whatever it is told.
	// Real applications do this: one with a minimum size, or a document window
	// an application insists on keeping beside its own inspector.
	wander *accessibility.Point
}

func (w *fakeWindow) Frame() (accessibility.Rect, error) { return w.frame, nil }

func (w *fakeWindow) SetPosition(p accessibility.Point) error {
	if w.refuse {
		return nil // the window server's own silence: success, and nothing moved
	}
	if w.wander != nil {
		p = *w.wander
	}
	w.frame.X, w.frame.Y = p.X, p.Y
	return nil
}

func (w *fakeWindow) SetSize(s accessibility.Size) error {
	if w.refuse {
		return nil
	}
	w.frame.W, w.frame.H = s.W, s.H
	return nil
}

func (w *fakeWindow) Raise() error { w.raised++; return nil }

// fakeBench is a desk with two screens and whatever windows a test gives it.
type fakeBench struct {
	trusted     bool
	displays    []accessibility.Display
	windows     map[string][]*fakeWindow
	displaysErr error
	windowsErr  error
}

func (b *fakeBench) Trusted() bool { return b.trusted }

func (b *fakeBench) Displays() ([]accessibility.Display, error) {
	return b.displays, b.displaysErr
}

func (b *fakeBench) Windows(app string) ([]accessibility.Window, []string, error) {
	if b.windowsErr != nil {
		return nil, nil, b.windowsErr
	}
	var ws []accessibility.Window
	var names []string
	for name, list := range b.windows {
		if !matches(name, app) {
			continue
		}
		for _, w := range list {
			ws = append(ws, w)
			names = append(names, name)
		}
	}
	return ws, names, nil
}

// bench is two 1920x1200 screens side by side, ids 101 and 102, with Safari on
// a big monitor away to the left of both.
func bench(t *testing.T) (*fakeBench, []uint64, *fakeWindow) {
	t.Helper()
	safari := &fakeWindow{frame: accessibility.Rect{X: -7680, Y: 0, W: 900, H: 700}}
	return &fakeBench{
		trusted: true,
		displays: []accessibility.Display{
			{ID: 101, Bounds: accessibility.Rect{X: 0, Y: 0, W: 1920, H: 1200}},
			{ID: 102, Bounds: accessibility.Rect{X: 1920, Y: 0, W: 1920, H: 1200}},
			{ID: 4, Bounds: accessibility.Rect{X: -7680, Y: 0, W: 7680, H: 2160}, Main: true},
		},
		windows: map[string][]*fakeWindow{"Safari": {safari}},
	}, []uint64{101, 102}, safari
}

// TestAnApplicationGoesToTheScreenItWasGiven, and fills it.
//
// Fill rather than the library's default: a ribbon screen is a whole desktop
// and an application sent to one is meant to have it. Scaling by the ratio of
// the two displays would put a window a quarter the size on a screen that is
// entirely free.
func TestAnApplicationGoesToTheScreenItWasGiven(t *testing.T) {
	b, ids, safari := bench(t)
	done, err := Send(b, ids, []Placement{{App: "safari", Pos: 2}})
	if err != nil {
		t.Fatalf("Send = %v", err)
	}
	if len(done) != 1 || !strings.Contains(done[0], "screen 2") {
		t.Errorf("Send reported %v", done)
	}
	if got, want := safari.frame, (accessibility.Rect{X: 1920, Y: 0, W: 1920, H: 1200}); got != want {
		t.Errorf("Safari is at %v, want the whole of screen 2 at %v", got, want)
	}
	if safari.raised == 0 {
		t.Error("the window was moved and not raised; \"send it there\" means \"and let me see it\"")
	}
}

// TestOneApplicationMissingDoesNotCostTheOthers.
//
// A desk of six applications where one is not running should place the other
// five and say which one it did not — not stop at the first.
func TestOneApplicationMissingDoesNotCostTheOthers(t *testing.T) {
	b, ids, safari := bench(t)
	done, err := Send(b, ids, []Placement{
		{App: "nothing here", Pos: 1},
		{App: "safari", Pos: 1},
	})
	if err == nil {
		t.Error("an application that is not running was not reported")
	} else if !errors.Is(err, ErrNoSuchApp) {
		t.Errorf("err = %v, want an ErrNoSuchApp", err)
	}
	if len(done) != 1 {
		t.Fatalf("Send placed %v; the one that IS running should still have moved", done)
	}
	if safari.frame.X != 0 {
		t.Errorf("Safari is at x=%g, want screen 1 at x=0", safari.frame.X)
	}
}

// TestAScreenThatIsNotThere covers both ways a position can be wrong: outside
// the desk, and inside it but naming a display the window server has dropped —
// which is what a headset being unplugged mid-session looks like.
func TestAScreenThatIsNotThere(t *testing.T) {
	b, ids, _ := bench(t)
	for name, p := range map[string]Placement{
		"past the end":   {App: "safari", Pos: 3},
		"before the one": {App: "safari", Pos: 0},
	} {
		done, err := Send(b, ids, []Placement{p})
		if err == nil || len(done) != 0 {
			t.Errorf("%s: Send = %v, %v", name, done, err)
		}
		if err != nil && !strings.Contains(err.Error(), "screen") {
			t.Errorf("%s: the error does not say which screen: %v", name, err)
		}
	}
	// A position inside the desk whose display has gone away.
	b.displays = b.displays[1:]
	if _, err := Send(b, ids, []Placement{{App: "safari", Pos: 1}}); err == nil {
		t.Error("a screen the window server is not driving was accepted")
	}
}

// TestWithoutTheGrantNothingIsAttempted, and the refusal says what to do.
func TestWithoutTheGrantNothingIsAttempted(t *testing.T) {
	b, ids, safari := bench(t)
	b.trusted = false
	before := safari.frame
	done, err := Send(b, ids, []Placement{{App: "safari", Pos: 1}})
	if err == nil {
		t.Fatal("an untrusted process was allowed to try")
	}
	if !errors.Is(err, accessibility.ErrNotTrusted) {
		t.Errorf("err = %v, want an ErrNotTrusted", err)
	}
	if !strings.Contains(err.Error(), "System Settings") {
		t.Errorf("the refusal does not say where to grant it: %v", err)
	}
	if len(done) != 0 || safari.frame != before {
		t.Error("something was moved anyway")
	}
}

// TestNothingToPlaceIsNotAnError: a settings file with no `place` block is the
// normal case, and it must not ask the machine for anything — including the
// Accessibility grant.
func TestNothingToPlaceIsNotAnError(t *testing.T) {
	b, ids, _ := bench(t)
	b.trusted = false
	b.displaysErr = errors.New("this should never be called")
	if done, err := Send(b, ids, nil); err != nil || done != nil {
		t.Errorf("Send with nothing to place = %v, %v", done, err)
	}
}

// TestTheMachineRefusing covers the two ways the platform itself can say no.
func TestTheMachineRefusing(t *testing.T) {
	b, ids, _ := bench(t)
	b.displaysErr = errors.New("no window server")
	if _, err := Send(b, ids, []Placement{{App: "safari", Pos: 1}}); err == nil {
		t.Error("a machine that cannot list displays was not reported")
	}

	b, ids, _ = bench(t)
	b.windowsErr = errors.New("the application will not say")
	if _, err := Send(b, ids, []Placement{{App: "safari", Pos: 1}}); err == nil {
		t.Error("a machine that cannot list windows was not reported")
	}
}

// TestAWindowThatDoesNotMoveIsReported.
//
// A write to kAXPositionAttribute returns success whether or not the
// application honours it. The library reads back rather than trusting the
// status, and this is the desk seeing that refusal.
func TestAWindowThatDoesNotMoveIsReported(t *testing.T) {
	b, ids, safari := bench(t)
	safari.refuse = true
	done, err := Send(b, ids, []Placement{{App: "safari", Pos: 1}})
	if err == nil {
		t.Error("a window that ignored the move was reported as placed")
	}
	if len(done) != 0 {
		t.Errorf("Send reported %v as done", done)
	}
}

// TestMatchesIsWhatAPersonWouldType.
func TestMatchesIsWhatAPersonWouldType(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		is         bool
	}{
		{"Safari", "safari", true},
		{"Safari", "  SAFARI ", true},
		{"Visual Studio Code", "code", true},
		{"Safari", "firefox", false},
		{"Safari", "", true}, // an empty pattern is in every name; the caller refuses it
	} {
		if got := matches(tc.name, tc.want); got != tc.is {
			t.Errorf("matches(%q, %q) = %v", tc.name, tc.want, got)
		}
	}
}

// TestTheRealBenchIsWhatShips: no test may leave a fake in place of it.
func TestTheRealBenchIsWhatShips(t *testing.T) {
	if TheBench() == nil {
		t.Fatal("TheBench() is nil")
	}
	// It answers on every platform, whether or not it can do anything.
	_ = TheBench().Trusted()
}

// TestTheFurnitureAllowanceCannotForgiveTheWrongScreen.
//
// macOS reserves the top of a display for the menu bar and clamps a window out
// of it, so a window told to fill a panel lands thirty-one points down. That
// has to count as placed. What makes the allowance safe rather than lax is
// that it is smaller than half a screen: a window within it is centred on the
// screen it was sent to and CANNOT be centred on a neighbour.
func TestTheFurnitureAllowanceCannotForgiveTheWrongScreen(t *testing.T) {
	b, ids, safari := bench(t)
	// Thirty-one points down: the menu bar. Placed.
	safari.wander = &accessibility.Point{X: 0, Y: 31}
	if _, err := Send(b, ids, []Placement{{App: "safari", Pos: 1}}); err != nil {
		t.Errorf("a window clamped out of the menu bar was reported as refused: %v", err)
	}

	// And the geometry that makes that safe, stated rather than assumed: the
	// allowance is smaller than half the narrowest screen on the band, so no
	// rectangle within it has its centre on another one.
	for _, d := range b.displays[:2] {
		if furniture >= d.Bounds.W/2 || furniture >= d.Bounds.H/2 {
			t.Errorf("the allowance of %d points is not smaller than half of %s; "+
				"a window within it could be centred on a neighbouring screen",
				furniture, d.Bounds)
		}
	}

	// A window that goes somewhere else entirely is refused, by the read-back.
	b, ids, safari = bench(t)
	safari.wander = &accessibility.Point{X: -7680, Y: 0}
	done, err := Send(b, ids, []Placement{{App: "safari", Pos: 1}})
	if err == nil {
		t.Error("a window that landed on the main monitor was reported as placed")
	}
	if len(done) != 0 {
		t.Errorf("Send reported %v as done", done)
	}
}
