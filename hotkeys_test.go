// Copyright (c) the go-xrkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause licence that can be
// found in the LICENSE file.

package desk

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-macos/hotkey"
)

// fakeClaim is a shortcut the test presses itself.
type fakeClaim struct {
	got, want hotkey.Combo
	ch        chan hotkey.Event
	closeErr  error
	closes    int
}

func (f *fakeClaim) Combo() hotkey.Combo    { return f.got }
func (f *fakeClaim) Wanted() hotkey.Combo   { return f.want }
func (f *fakeClaim) Substituted() bool      { return f.got != f.want }
func (f *fakeClaim) C() <-chan hotkey.Event { return f.ch }
func (f *fakeClaim) Close() error           { f.closes++; close(f.ch); return f.closeErr }
func (f *fakeClaim) press()                 { f.ch <- hotkey.Event{Combo: f.got} }

// withRegister installs a fake platform for the duration of one test.
func withRegister(t *testing.T, fn func(hotkey.Combo, *hotkey.Options) (claimed, error)) {
	t.Helper()
	old := register
	register = fn
	t.Cleanup(func() { register = old })
}

// newFake claims everything asked for, unchanged.
func newFake(t *testing.T) map[hotkey.Combo]*fakeClaim {
	claims := map[hotkey.Combo]*fakeClaim{}
	withRegister(t, func(want hotkey.Combo, _ *hotkey.Options) (claimed, error) {
		f := &fakeClaim{got: want, want: want, ch: make(chan hotkey.Event)}
		claims[want] = f
		return f, nil
	})
	return claims
}

func TestAGlobalPressTurnsTheRibbon(t *testing.T) {
	claims := newFake(t)
	h := ClaimGlobal(DefaultShortcuts(), nil)
	defer h.Close()

	const mods = hotkey.Option | hotkey.Command
	for combo, want := range map[hotkey.Combo]Action{
		{Key: hotkey.KeyLeftArrow, Mods: mods}:  ActionPrev,
		{Key: hotkey.KeyRightArrow, Mods: mods}: ActionNext,
		{Key: hotkey.KeySpace, Mods: mods}:      ActionGallery,
	} {
		f, ok := claims[combo]
		if !ok {
			t.Fatalf("%s was never claimed", combo)
		}
		f.press()
		select {
		case got := <-h.C():
			if got != want {
				t.Errorf("%s produced %v, want %v", combo, got, want)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("%s produced nothing", combo)
		}
	}
}

// TestAHeldKeyDoesNotQueue: presses arrive faster than a ribbon turns, and a
// queue of them would keep turning it after the finger came off.
func TestAHeldKeyDoesNotQueue(t *testing.T) {
	claims := newFake(t)
	h := ClaimGlobal(DefaultShortcuts()[:1], nil)
	defer h.Close()

	f := claims[DefaultShortcuts()[0].Want]
	for i := 0; i < 200; i++ {
		f.press()
	}
	// Drain what is there; the buffer is one per shortcut, so a held key must
	// not have left two hundred behind.
	n := 0
	for {
		select {
		case <-h.C():
			n++
			continue
		case <-time.After(200 * time.Millisecond):
		}
		break
	}
	if n > 3 {
		t.Errorf("200 presses left %d actions queued", n)
	}
	if n == 0 {
		t.Error("200 presses left nothing at all")
	}
}

func TestDescribeNamesWhatWasActuallyClaimed(t *testing.T) {
	// The middle one is taken, so the ladder substitutes; the last cannot be
	// claimed at all.
	taken := DefaultShortcuts()[1].Want
	refused := DefaultShortcuts()[2].Want
	withRegister(t, func(want hotkey.Combo, _ *hotkey.Options) (claimed, error) {
		switch want {
		case refused:
			return nil, errors.New("every candidate is spoken for")
		case taken:
			got := want
			got.Mods |= hotkey.Shift
			return &fakeClaim{got: got, want: want, ch: make(chan hotkey.Event)}, nil
		}
		return &fakeClaim{got: want, want: want, ch: make(chan hotkey.Event)}, nil
	})
	h := ClaimGlobal(DefaultShortcuts(), nil)
	defer h.Close()

	d := h.Describe()
	for _, want := range []string{
		"previous: ⌥⌘←\n",
		"next: ⌥⇧⌘→ (asked for ⌥⌘→, it was taken)",
		"no global shortcut for ⌥⌘Space (gallery): every candidate is spoken for",
	} {
		if !strings.Contains(d, want) {
			t.Errorf("Describe() is missing %q; it says:\n%s", want, d)
		}
	}
}

// TestNoGlobalShortcutsIsNotAReasonNotToRun is the whole point of the type
// returning no error: Linux and Windows have no Carbon, and the ribbon still
// works from its own window.
func TestNoGlobalShortcutsIsNotAReasonNotToRun(t *testing.T) {
	withRegister(t, func(hotkey.Combo, *hotkey.Options) (claimed, error) {
		return nil, hotkey.ErrUnsupported
	})
	h := ClaimGlobal(DefaultShortcuts(), nil)
	if h == nil {
		t.Fatal("ClaimGlobal returned nothing on a platform with no hot keys")
	}
	if err := h.Close(); err != nil {
		t.Errorf("Close = %v", err)
	}
	if _, open := <-h.C(); open {
		t.Error("C is not closed after Close")
	}
	if n := strings.Count(h.Describe(), "no global shortcut"); n != 3 {
		t.Errorf("Describe() reports %d unclaimed shortcuts, want 3", n)
	}
}

func TestCloseReportsAFailedReleaseAndOnlyRunsOnce(t *testing.T) {
	claims := newFake(t)
	boom := errors.New("the window server said no")
	h := ClaimGlobal(DefaultShortcuts()[:2], nil)
	first := claims[DefaultShortcuts()[0].Want]
	first.closeErr = boom

	if err := h.Close(); !errors.Is(err, boom) {
		t.Errorf("Close = %v, want it to report %v", err, boom)
	}
	// A second Close must not close an already-closed channel, and must not
	// release a claim twice.
	if err := h.Close(); err != nil {
		t.Errorf("the second Close = %v, want nil", err)
	}
	if first.closes != 1 {
		t.Errorf("the claim was released %d times", first.closes)
	}
}

// TestTheRealPlatformIsWhatShipsChecks that the package variable really is the
// platform call, so no test ever leaves a fake in place of it.
func TestTheRealPlatformIsWhatShips(t *testing.T) {
	_, err := register(hotkey.Combo{}, nil)
	if err == nil {
		t.Fatal("registering an empty combination succeeded")
	}
}
