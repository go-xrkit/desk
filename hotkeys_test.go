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

	const mods = hotkey.Control | hotkey.Option | hotkey.Command
	for combo, want := range map[hotkey.Combo]Action{
		{Key: hotkey.KeyLeftArrow, Mods: mods}:  ActionPrev,
		{Key: hotkey.KeyRightArrow, Mods: mods}: ActionNext,
		{Key: hotkey.KeyF3, Mods: mods}:         ActionGalleryOpen,
		{Key: hotkey.KeyF4, Mods: mods}:         ActionAppsOpen,
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
		"previous: ⌃⌥⌘←\n",
		"next: ⌃⌥⇧⌘→ (asked for ⌃⌥⌘→, it was taken)",
		"no global shortcut for ⌃⌥⌘F3 (open the gallery): every candidate is spoken for",
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
	// However many there are: a count typed here becomes wrong the day a
	// shortcut is added, which is what happened when the gallery gained a key
	// to enter by and a key to leave by.
	if n, want := strings.Count(h.Describe(), "no global shortcut"), len(DefaultShortcuts()); n != want {
		t.Errorf("Describe() reports %d unclaimed shortcuts, want all %d", n, want)
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

// TestThereIsAlwaysAWayOut.
//
// The desk takes a whole display and puts a picture over it. The pointer that
// wanders onto that display is invisible -- the picture is a capture of somewhere
// else, so it does not show where the mouse is -- and the window does not hold
// the keyboard, on purpose, because the applications on the screens need it.
//
// Somebody in that position has nothing to click and nothing to press. It was
// measured on a pair of glasses, and the way out was UNPLUGGING THEM. So quitting
// is system-wide, and this is the test that says it must stay so.
func TestThereIsAlwaysAWayOut(t *testing.T) {
	want := map[Action]bool{ActionQuit: false, ActionSettings: false}
	keys := map[string]string{}
	for _, s := range DefaultShortcuts() {
		if _, ok := want[s.Does]; ok {
			want[s.Does] = true
		}
		// And no two of them are the same combination, which would leave one of
		// the pair unreachable however carefully it was chosen.
		k := s.Want.String()
		if was, seen := keys[k]; seen {
			t.Errorf("%s is claimed by both %s and %s", k, was, s.Does)
		}
		keys[k] = s.Does.String()
	}
	for a, found := range want {
		if !found {
			t.Errorf("no system-wide shortcut asks for %v, so a person whose "+
				"pointer is lost on the glasses cannot reach it", a)
		}
	}
}

// TestEveryShortcutIsAnActionTheDeskAnswers: a combination claimed for an action
// nothing acts on takes a key from the whole machine and gives nothing back.
func TestEveryShortcutIsAnActionTheDeskAnswers(t *testing.T) {
	for _, s := range DefaultShortcuts() {
		if s.Does == ActionNone || s.Does.String() == "none" {
			t.Errorf("%s is claimed for %v, which the desk does not answer",
				s.Want.String(), s.Does)
		}
	}
}

// TestTheWindowKeysAndTheGlobalOnesAgree.
//
// Every action the desk offers on a key in its own window is offered on a
// system-wide combination too, and the two use the SAME key where a key exists.
//
// It is not tidiness. The window deliberately does not hold the keyboard -- the
// applications on the screens need it -- so a shortcut that only works in the
// window works nowhere a person actually is. And a person who learns [ and ] in
// one place and finds them somewhere else has learnt nothing.
//
// The exceptions are named rather than skipped: the gallery's own arrows, and
// the actions that only mean something inside it.
func TestTheWindowKeysAndTheGlobalOnesAgree(t *testing.T) {
	// Which key, if any, the window offers for each action.
	window := map[Action]string{}
	for _, code := range []string{
		"Escape", "q", "ArrowLeft", "h", "ArrowRight", "l", " ", "f", "Tab", "c",
		"g", "Enter", "Return", "ArrowUp", "k", "ArrowDown", "j",
		"-", "_", "+", "=", "[", "{", "]", "}", "m", "M",
	} {
		if a := KeyAction(code); a != ActionNone {
			if _, seen := window[a]; !seen {
				window[a] = code
			}
		}
	}

	global := map[Action]hotkey.Combo{}
	for _, s := range DefaultShortcuts() {
		global[s.Does] = s.Want
	}

	// The ones the band needs while another application has the keyboard.
	// ActionGalleryClose is deliberately NOT here. Leaving is only ever wanted
	// while a gallery is up, and while one is up the desk holds the BARE
	// Escape -- see GalleryShortcuts -- so it is reachable system-wide without
	// spending a modifier combination on it. ⌃⌥⌘Space toggles from outside for
	// anyone who wants one.
	for _, a := range []Action{
		ActionPrev, ActionNext, ActionGalleryOpen, ActionAppsOpen,
		ActionChoose, ActionCloser, ActionFurther, ActionFlatter, ActionRounder,
		ActionPoint, ActionQuit,
	} {
		if _, ok := global[a]; !ok {
			t.Errorf("%v has no system-wide combination, so it cannot be used "+
				"while the applications have the keyboard", a)
		}
	}

	// And where both exist, the KEY is the same one -- with one named exception.
	//
	// The gallery TOGGLE is "g" in the window and Space system-wide, and that is
	// deliberate: Space is what a person reaches for blind, it was the first
	// choice before the Finder was found to own the plain combination, and the
	// toggle is not how the gallery is meant to be used from outside anyway --
	// open and leave have their own two keys, precisely because a system-wide
	// shortcut is pressed without seeing which state the gallery is in.
	exempt := map[Action]bool{ActionGallery: true}
	for a, code := range window {
		combo, ok := global[a]
		if !ok || exempt[a] {
			continue
		}
		if len([]rune(code)) != 1 {
			continue // a named key: Escape, Tab, the arrows
		}
		if got := combo.Key.Name(); !strings.EqualFold(got, code) &&
			!strings.EqualFold(got, keyWord(code)) {
			t.Errorf("%v is %q in the window and %q system-wide", a, code, got)
		}
	}
}

// keyWord is the spelled name of a one-character key, for the two the hotkey
// package spells rather than prints.
func keyWord(code string) string {
	switch code {
	case "-", "_":
		return "Minus"
	case "+", "=":
		return "Equal"
	case "[", "{":
		return "LeftBracket"
	case "]", "}":
		return "RightBracket"
	}
	return code
}

// TestTheGallerysKeysAreBareAndClaimNoNeighbour.
//
// Bare, because a person looking at a grid should not hold three modifiers to
// walk it. And with NO fallback ladder: a bare arrow that cannot be claimed
// must stay unclaimed rather than becoming ⇧←, which is a selection in every
// text field on the machine.
func TestTheGallerysKeysAreBareAndClaimNoNeighbour(t *testing.T) {
	want := map[hotkey.Key]Action{
		hotkey.KeyLeftArrow:  ActionPrev,
		hotkey.KeyRightArrow: ActionNext,
		hotkey.KeyUpArrow:    ActionUp,
		hotkey.KeyDownArrow:  ActionDown,
		hotkey.KeyReturn:     ActionChoose,
		hotkey.KeyEscape:     ActionGalleryClose,
		hotkey.KeyDelete:     ActionRemove,
	}
	got := GalleryShortcuts()
	if len(got) != len(want) {
		t.Fatalf("%d gallery keys, want %d", len(got), len(want))
	}
	for _, s := range got {
		if s.Want.Mods != 0 {
			t.Errorf("%v carries a modifier; the whole point is that it does not", s.Want)
		}
		if w, ok := want[s.Want.Key]; !ok || w != s.Does {
			t.Errorf("%v does %v, want %v", s.Want, s.Does, w)
		}
	}

	// And the claim asks for a bare key on purpose, with no ladder.
	claims := map[hotkey.Combo]*fakeClaim{}
	withRegister(t, func(want hotkey.Combo, opts *hotkey.Options) (claimed, error) {
		if opts == nil || !opts.BareKey {
			t.Errorf("%v was claimed without asking for a bare key", want)
		}
		if opts != nil && len(opts.Ladder) != 0 {
			t.Errorf("%v was claimed with a ladder of %d: a neighbour is not acceptable here",
				want, len(opts.Ladder))
		}
		c := &fakeClaim{got: want, want: want, ch: make(chan hotkey.Event)}
		claims[want] = c
		return c, nil
	})
	h := ClaimGallery()
	defer h.Close()
	if len(claims) != len(want) {
		t.Errorf("claimed %d keys, want %d", len(claims), len(want))
	}
}

// TestNoCombinationIsClaimedTwice, in either set.
//
// Asked from the glasses, and worth an answer that cannot rot: "tu sembles
// utiliser ctrl+option+command+flèche basse pour deux usages ?" It was one use,
// but the question is the right one — a table edited by hand over fourteen
// entries is exactly where two actions quietly end up on one key, and the
// second one would simply never fire.
//
// The two sets are checked separately because they are claimed at different
// times: the bare keys only while a gallery is up. Within each, a combination
// belongs to one action.
func TestNoCombinationIsClaimedTwice(t *testing.T) {
	for _, set := range []struct {
		name string
		of   []Shortcut
	}{
		{"the desk's own", DefaultShortcuts()},
		{"the gallery's bare keys", GalleryShortcuts()},
	} {
		seen := map[hotkey.Combo]Action{}
		for _, s := range set.of {
			if was, dup := seen[s.Want]; dup {
				t.Errorf("%s: %v is claimed for both %v and %v; the second would "+
					"never fire", set.name, s.Want, was, s.Does)
				continue
			}
			seen[s.Want] = s.Does
		}
	}

	// And no action has two combinations in the SAME set, which would be two
	// keys to teach for one thing.
	does := map[Action]hotkey.Combo{}
	for _, s := range GalleryShortcuts() {
		if was, dup := does[s.Does]; dup {
			t.Errorf("%v is on both %v and %v while a gallery is up", s.Does, was, s.Want)
		}
		does[s.Does] = s.Want
	}
}

// TestGrantedIsWhatWasClaimedAndNotWhatWasAsked.
//
// ⛔ The whole reason this exists. The ladder substitutes when a combination is
// taken, and a menu that printed the WANTED one would send a person to press a
// key that does nothing -- which is the one case they cannot work out for
// themselves.
func TestGrantedIsWhatWasClaimedAndNotWhatWasAsked(t *testing.T) {
	const mods = hotkey.Control | hotkey.Option | hotkey.Command
	asked := hotkey.Combo{Key: hotkey.KeyEqual, Mods: mods}
	got := hotkey.Combo{Key: hotkey.KeyN0, Mods: mods | hotkey.Shift}
	withRegister(t, func(want hotkey.Combo, _ *hotkey.Options) (claimed, error) {
		if want.Key == hotkey.KeyM {
			return nil, errors.New("taken")
		}
		return &fakeClaim{got: got, want: want, ch: make(chan hotkey.Event)}, nil
	})

	h := ClaimGlobal([]Shortcut{
		{asked, ActionFit},
		{hotkey.Combo{Key: hotkey.KeyM, Mods: hotkey.Command}, ActionPoint},
	}, nil)
	defer h.Close()

	keys := h.Granted()
	if keys[ActionFit] != got {
		t.Errorf("fit was granted %v, want the combination it actually got, %v",
			keys[ActionFit], got)
	}
	if _, ok := keys[ActionPoint]; ok {
		t.Errorf("an action nothing was granted for is in the map: %q", keys[ActionPoint])
	}

	// And the menu form, not the written one: "⌃⌥⌘Equal" is a key somebody
	// looks for and does not find.
	withRegister(t, func(want hotkey.Combo, _ *hotkey.Options) (claimed, error) {
		return &fakeClaim{got: want, want: want, ch: make(chan hotkey.Event)}, nil
	})
	h2 := ClaimGlobal([]Shortcut{{asked, ActionFit}}, nil)
	defer h2.Close()
	if k := h2.Granted()[ActionFit]; k != asked {
		t.Errorf("fit was granted %v, want the combination asked for, %v", k, asked)
	}

	// Nothing claimed at all is no map rather than an empty one, so a caller
	// can tell "not asked yet" from "asked and refused".
	var none *Hotkeys
	if none.Granted() != nil {
		t.Error("a set that claimed nothing gave a map back")
	}
}

// TestAskedForIsWhatTheMenuSaysBeforeThereIsASession.
//
// ⛔ THE ITEM OUTLIVES EVERY SESSION AND THE CLAIMS DO NOT. The menu bar item
// is built once for the whole process; the shortcuts are claimed only while a
// session runs. So waiting for a headset, sitting in the settings window, or
// with the glasses put down, the menu had NO key equivalents -- and a row with
// none draws exactly like a row whose action was never granted. Measured: "the
// menu draws 0 key equivalents" before this, 7 after.
func TestAskedForIsWhatTheMenuSaysBeforeThereIsASession(t *testing.T) {
	settings := hotkey.Combo{Key: hotkey.KeyS, Mods: hotkey.Command}
	first := hotkey.Combo{Key: hotkey.KeyM, Mods: hotkey.Command}
	second := hotkey.Combo{Key: hotkey.KeyN, Mods: hotkey.Command}

	got := AskedFor([]Shortcut{
		{settings, ActionSettings},
		{first, ActionPoint},
		// ⛔ THE FIRST WINS, like a claim: a list naming one action twice
		// claims the first and substitutes or refuses the second, so a menu
		// built from the second would name a key that does something else.
		{second, ActionPoint},
	})
	if len(got) != 2 {
		t.Fatalf("AskedFor gave %d entries, want 2", len(got))
	}
	if got[ActionSettings] != settings {
		t.Errorf("the settings row names %v", got[ActionSettings])
	}
	if got[ActionPoint] != first {
		t.Errorf("the pointer row names %v, want the first of the two", got[ActionPoint])
	}
	if got := AskedFor(nil); len(got) != 0 {
		t.Errorf("an empty list gave %v", got)
	}
}
