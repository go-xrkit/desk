// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"testing"
	"time"

	"github.com/go-macos/multitouch"
)

// TestWhichWayTheRibbonTurns pins the convention rather than an opinion.
//
// Fingers moving LEFT bring the next screen in from the right, which is what
// the same gesture does to spaces on this platform. Backwards is not a bug a
// person reports; it is one they blame themselves for.
func TestWhichWayTheRibbonTurns(t *testing.T) {
	for _, c := range []struct {
		name string
		sw   multitouch.Swipe
		want Action
		ok   bool
	}{
		{"fingers left", multitouch.Swipe{Fingers: 3, Dx: -0.3}, ActionNext, true},
		{"fingers right", multitouch.Swipe{Fingers: 3, Dx: 0.3}, ActionPrev, true},
		{"up", multitouch.Swipe{Fingers: 3, Dy: 0.3}, ActionNone, false},
		{"down", multitouch.Swipe{Fingers: 3, Dy: -0.3}, ActionNone, false},
	} {
		got, ok := actionForSwipe(c.sw)
		if got != c.want || ok != c.ok {
			t.Errorf("%s: actionForSwipe = %v,%v want %v,%v", c.name, got, ok, c.want, c.ok)
		}
	}
}

// feeder stands in for a trackpad: it hands the watcher's callback whatever
// frames a test wants, from a test, on any machine.
func feeder(t *testing.T) chan<- multitouch.Frame {
	t.Helper()
	frames := make(chan multitouch.Frame, 64)
	was := watchTouches
	t.Cleanup(func() { watchTouches = was })
	watchTouches = func(fn func(multitouch.Frame)) (*multitouch.Watcher, error) {
		go func() {
			for f := range frames {
				fn(f)
			}
		}()
		return &multitouch.Watcher{}, nil
	}
	return frames
}

// hand is three fingers whose centre is at x.
func hand(at time.Duration, n int, x float32) multitouch.Frame {
	f := multitouch.Frame{At: at}
	for i := range n {
		f.Contacts = append(f.Contacts, multitouch.Contact{ID: i, X: x + float32(i)*0.02, Y: 0.5})
	}
	return f
}

func TestASwipeReachesTheRibbon(t *testing.T) {
	frames := feeder(t)
	g := ClaimGestures()
	defer g.Close()
	if g.Why() != nil {
		t.Fatalf("Why = %v, want nothing wrong", g.Why())
	}

	for i := 0; i <= 10; i++ {
		frames <- hand(time.Duration(i)*20*time.Millisecond, 3, 0.7-float32(i)*0.03)
	}
	select {
	case a := <-g.C():
		if a != ActionNext {
			t.Errorf("fingers left gave %v, want the next screen", a)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the swipe never reached the ribbon")
	}
}

// TestNoTrackpadIsNotAFailure: the desk is driven by the keyboard, and the
// gesture is an addition. A machine without one must not lose the desk.
func TestNoTrackpadIsNotAFailure(t *testing.T) {
	was := watchTouches
	t.Cleanup(func() { watchTouches = was })
	want := errors.New("no trackpad on this machine")
	watchTouches = func(func(multitouch.Frame)) (*multitouch.Watcher, error) { return nil, want }

	g := ClaimGestures()
	defer g.Close()
	if !errors.Is(g.Why(), want) {
		t.Errorf("Why = %v, want the reason", g.Why())
	}
	select {
	case a, open := <-g.C():
		t.Errorf("a channel that should stay empty gave %v (open=%v)", a, open)
	case <-time.After(100 * time.Millisecond):
	}
}

// TestTurnsAreDroppedRatherThanQueued, for the same reason a held-down key is:
// a queue of turns keeps turning the ribbon long after the hand has stopped.
func TestTurnsAreDroppedRatherThanQueued(t *testing.T) {
	frames := feeder(t)
	g := ClaimGestures()
	defer g.Close()

	// Twenty separate swipes with nobody reading. The channel holds four.
	at := time.Duration(0)
	for s := 0; s < 20; s++ {
		for i := 0; i <= 10; i++ {
			frames <- hand(at, 3, 0.7-float32(i)*0.03)
			at += 20 * time.Millisecond
		}
		frames <- multitouch.Frame{At: at} // fingers lift, so the next one counts
		at += 20 * time.Millisecond
	}
	time.Sleep(300 * time.Millisecond)
	if n := len(g.C()); n > 4 {
		t.Errorf("%d turns are queued; the buffer is 4 and the rest must be dropped", n)
	}
}

func TestCloseTwice(t *testing.T) {
	feeder(t)
	g := ClaimGestures()
	if err := g.Close(); err != nil {
		t.Errorf("first Close: %v", err)
	}
	if err := g.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}

// TestClosingWhileTheHandIsMoving is a race regression, and the race was worse
// than a race: Close closed the channel while a callback could still be sending
// on it, which is a panic rather than a wrong answer. Stopping the device does
// not promise that no callback is already in flight.
func TestClosingWhileTheHandIsMoving(t *testing.T) {
	frames := feeder(t)
	g := ClaimGestures()

	done := make(chan struct{})
	go func() {
		defer close(done)
		at := time.Duration(0)
		for s := 0; s < 200; s++ {
			for i := 0; i <= 10; i++ {
				select {
				case frames <- hand(at, 3, 0.7-float32(i)*0.03):
				default:
				}
				at += time.Millisecond
			}
		}
	}()
	time.Sleep(20 * time.Millisecond)
	if err := g.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	<-done
	// And a send after the close is dropped rather than fatal.
	g.send(ActionNext)
}
