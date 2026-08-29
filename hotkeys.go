// Copyright (c) the go-xrkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause licence that can be
// found in the LICENSE file.

package desk

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/go-macos/hotkey"
)

// Shortcut is one system-wide combination and what pressing it does.
//
// System-wide is the whole point. A virtual desktop in glasses is used while
// something else has the keyboard: the viewer is reading in one screen and
// wants the next one, without first clicking on a window they cannot see. A
// shortcut that only works when xrdesk is frontmost would be a shortcut for
// nothing.
type Shortcut struct {
	Want hotkey.Combo
	Does Action
}

// DefaultShortcuts are what the ribbon needs from the whole machine.
//
// ONE prefix for all of them: Control+Option+Command. It was not so at first —
// the band was on ⌥⌘← and ⌥⌘→ while everything added later took the third
// modifier — and a person who had learnt the desk pressed ⌃⌥⌘← for the band and
// got nothing at all. A set of shortcuts with two prefixes is a set nobody can
// remember, and the one that gives way is the one with fewer keys on it.
//
// ⌥⌘← and ⌥⌘→ were also Safari's tab navigation, which the desk was quietly
// taking for the length of a session.
//
// ⌥⌘Space is the Finder's search window on a stock macOS, so the gallery always
// falls back — see [DefaultLadder], and [Hotkeys.Describe] for what was actually
// granted.
func DefaultShortcuts() []Shortcut {
	const mods = hotkey.Option | hotkey.Command
	return []Shortcut{
		{hotkey.Combo{Key: hotkey.KeyLeftArrow, Mods: mods | hotkey.Control}, ActionPrev},
		{hotkey.Combo{Key: hotkey.KeyRightArrow, Mods: mods | hotkey.Control}, ActionNext},
		{hotkey.Combo{Key: hotkey.KeySpace, Mods: mods}, ActionGallery},
		// Open and leave, each on its own key.
		//
		// A system-wide shortcut is pressed BLIND: the viewer cannot see whether
		// the gallery is open before deciding, so one key meaning "open" from
		// outside and "close" from inside does the wrong thing every time they
		// have lost track. Up goes in and down comes out, which is also the
		// direction the grid is in relative to the band.
		{hotkey.Combo{Key: hotkey.KeyUpArrow, Mods: mods | hotkey.Control}, ActionGalleryOpen},
		{hotkey.Combo{Key: hotkey.KeyDownArrow, Mods: mods | hotkey.Control}, ActionGalleryClose},
		// And choosing, system-wide with the rest.
		//
		// Enter alone is a key in the window, which is no use to somebody who
		// opened the gallery from another application: they could move the
		// selection with the global arrows and then have nothing to confirm it
		// with, because Enter would go wherever the keyboard was pointing.
		{hotkey.Combo{Key: hotkey.KeyReturn, Mods: mods | hotkey.Control}, ActionChoose},
		// Nearer and further, system-wide with the rest.
		//
		// On the two keys a keyboard puts smaller and larger on, which is the one
		// pair a person does not have to be told. Registered by CODE, so they are
		// the same two positions on a French keyboard whatever is printed there.
		{hotkey.Combo{Key: hotkey.KeyMinus, Mods: mods | hotkey.Control}, ActionFurther},
		{hotkey.Combo{Key: hotkey.KeyEqual, Mods: mods | hotkey.Control}, ActionCloser},
		// And a way OUT, system-wide.
		//
		// This one is not a convenience. The desk takes a whole display and puts a
		// picture over it, and the pointer that wanders onto that display is
		// invisible -- the picture is a capture of somewhere else, so it does not
		// show where the mouse is. Somebody in that position, with the desk's own
		// window not holding the keyboard, has nothing to click and nothing to
		// press: it was measured, and the way out was UNPLUGGING THE GLASSES.
		//
		// Escape rather than a letter because it is the key everybody already
		// tries, and with three modifiers it is not one anybody hits by accident.
		{hotkey.Combo{Key: hotkey.KeyEscape, Mods: mods | hotkey.Control}, ActionQuit},
		// And the pointer, which has to be system-wide or it is nothing: the whole
		// point of it is to be pressed while another application has the keyboard.
		{hotkey.Combo{Key: hotkey.KeyM, Mods: mods | hotkey.Control}, ActionPoint},
		// The angle between the screens, on the same two keys the window uses.
		//
		// System-wide because a window-only shortcut is no use to a surface that
		// does not hold the keyboard, which this one deliberately does not -- and
		// on the SAME keys, because a person who learns [ and ] in one place and
		// finds them somewhere else has learnt nothing. It needed KeyLeftBracket
		// upstream (go-macos/hotkey v0.5.0), which is why the angle was the one
		// setting that could only be given on the command line for an evening.
		{hotkey.Combo{Key: hotkey.KeyLeftBracket, Mods: mods | hotkey.Control}, ActionFlatter},
		{hotkey.Combo{Key: hotkey.KeyRightBracket, Mods: mods | hotkey.Control}, ActionRounder},
		// The settings, for the same reason: the tray menu is the other way, and it
		// needs a pointer.
		{hotkey.Combo{Key: hotkey.KeyS, Mods: mods | hotkey.Control}, ActionSettings},
		// The applications, on their own initial, and the spread beside it.
		//
		// These two matter more system-wide than most: choosing which application
		// goes on which screen is the one thing a person does while looking AT
		// the screens, so the keys have to work while the keyboard belongs to
		// whatever is running on them.
		{hotkey.Combo{Key: hotkey.KeyA, Mods: mods | hotkey.Control}, ActionApps},
		{hotkey.Combo{Key: hotkey.KeyX, Mods: mods | hotkey.Control}, ActionSpread},
		// And taking one away, on the key that deletes.
		{hotkey.Combo{Key: hotkey.KeyDelete, Mods: mods | hotkey.Control}, ActionRemove},
	}
}

// claimed is the part of *hotkey.Hotkey this package uses.
//
// It exists so that the policy above — what is claimed, what is reported, what
// happens when the platform has no global shortcuts at all — is tested on every
// operating system rather than only on the one that can register them.
type claimed interface {
	Combo() hotkey.Combo
	Wanted() hotkey.Combo
	Substituted() bool
	C() <-chan hotkey.Event
	Close() error
}

// register is the platform call, replaced in tests.
var register = func(want hotkey.Combo, opts *hotkey.Options) (claimed, error) {
	return hotkey.Register(want, opts)
}

// Hotkeys is a set of claimed global shortcuts, feeding one channel of actions.
type Hotkeys struct {
	held  []claimed
	does  []Action
	ch    chan Action
	wg    sync.WaitGroup
	once  sync.Once
	unmet []error
}

// ClaimGlobal claims each shortcut, substituting when it has to.
//
// It returns a usable set even when nothing could be claimed: a platform with
// no global shortcuts, or a desktop where every candidate is spoken for, is a
// reason to run without them, not a reason not to run. What went unclaimed is
// in [Hotkeys.Describe], for the application to show.
func ClaimGlobal(shortcuts []Shortcut, opts *hotkey.Options) *Hotkeys {
	if opts == nil {
		// The ladder matters more than any other default here: without one, the
		// gallery key is simply refused on every stock macOS.
		opts = &hotkey.Options{Ladder: DefaultLadder}
	}
	h := &Hotkeys{ch: make(chan Action, len(shortcuts))}
	for _, s := range shortcuts {
		k, err := register(s.Want, opts)
		if err != nil {
			h.unmet = append(h.unmet, fmt.Errorf("%s (%s): %w", s.Want, s.Does, err))
			continue
		}
		h.held = append(h.held, k)
		h.does = append(h.does, s.Does)
		h.wg.Add(1)
		go h.pump(k, s.Does)
	}
	return h
}

// pump forwards one claim's presses as actions.
//
// A press is DROPPED rather than queued when the reader is behind. Holding a
// key down produces presses faster than a ribbon can turn, and a queue of them
// would keep turning it long after the finger came off.
func (h *Hotkeys) pump(k claimed, a Action) {
	defer h.wg.Done()
	for range k.C() {
		select {
		case h.ch <- a:
		default:
		}
	}
}

// C is the actions the claimed shortcuts produce. It closes when every claim
// has been released.
func (h *Hotkeys) C() <-chan Action { return h.ch }

// Close releases every claim and closes C.
func (h *Hotkeys) Close() error {
	var errs []error
	h.once.Do(func() {
		for _, k := range h.held {
			if err := k.Close(); err != nil {
				errs = append(errs, err)
			}
		}
		h.wg.Wait()
		close(h.ch)
	})
	return errors.Join(errs...)
}

// Describe says what was claimed and what was not, in the form a person needs
// to see rather than the form a log wants.
//
// The claimed combination is SHOWN, not merely recorded. One of the three kinds
// of conflict — an application's own menu key — cannot be detected by anything,
// so a shortcut that registers without complaint may still be one another
// application was quietly using. Naming what was taken is what lets the viewer
// notice.
func (h *Hotkeys) Describe() string { return h.describe(hotkey.Combo.String) }

// DescribeNames is the same, spelled out: "Option-Command-Left" rather than
// "⌥⌘←".
//
// The glyphs are what macOS prints on a menu and what a terminal shows
// perfectly. They are NOT in every font — rendered in a window with the
// toolkit's own, ⌥ ⌘ ⇧ ⌃ ← → all came out as nothing at all, so a line meant
// to say which combination was granted said "previous:" and stopped. Anywhere
// the font is not known, this is the one to use.
func (h *Hotkeys) DescribeNames() string { return h.describe(hotkey.Combo.Names) }

func (h *Hotkeys) describe(render func(hotkey.Combo) string) string {
	var b strings.Builder
	for i, k := range h.held {
		fmt.Fprintf(&b, "%s: %s", h.does[i], render(k.Combo()))
		if k.Substituted() {
			fmt.Fprintf(&b, " (asked for %s, it was taken)", render(k.Wanted()))
		}
		b.WriteByte('\n')
	}
	for _, err := range h.unmet {
		fmt.Fprintf(&b, "no global shortcut for %v\n", err)
	}
	return b.String()
}

// GalleryShortcuts are the BARE keys a gallery claims for as long as it is up.
//
// A person looking at a grid of screens or of applications should not have to
// hold three modifiers to walk it — "quand on est dans la galerie se déplacer
// avec juste les flèches devrait suffir", which is exactly right. So while a
// gallery covers the view, the arrows, Return and Escape mean what they look
// like they mean.
//
// They are claimed system-wide, because the desk's window deliberately does not
// take the keyboard. That is a serious thing to do to a machine — a bare arrow
// claimed for ever would break typing everywhere — so it is done only while a
// gallery is up and undone the moment it closes. go-macos/hotkey v0.6.0 asks a
// caller to say so with Options.BareKey rather than allowing it by accident.
//
// Nobody is typing into anything while a gallery covers their view.
func GalleryShortcuts() []Shortcut {
	return []Shortcut{
		{hotkey.Combo{Key: hotkey.KeyLeftArrow}, ActionPrev},
		{hotkey.Combo{Key: hotkey.KeyRightArrow}, ActionNext},
		{hotkey.Combo{Key: hotkey.KeyUpArrow}, ActionUp},
		{hotkey.Combo{Key: hotkey.KeyDownArrow}, ActionDown},
		{hotkey.Combo{Key: hotkey.KeyReturn}, ActionChoose},
		{hotkey.Combo{Key: hotkey.KeyEscape}, ActionGalleryClose},
		{hotkey.Combo{Key: hotkey.KeyDelete}, ActionRemove},
	}
}

// ClaimGallery claims [GalleryShortcuts] with no fallback ladder.
//
// No ladder on purpose: a bare arrow that could not be claimed must stay
// unclaimed rather than becoming ⇧← , which is a selection in every text field
// on the machine.
func ClaimGallery() *Hotkeys {
	return ClaimGlobal(GalleryShortcuts(), &hotkey.Options{
		BareKey: true,
		Ladder:  []hotkey.Modifier{}, // deliberately empty: no neighbour is acceptable
	})
}
