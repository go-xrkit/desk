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

// DefaultShortcuts are the three the ribbon needs.
//
// Option+Command with the arrows and the space bar, chosen because they are
// what a person reaches for and because the arrows already mean "along the
// band" inside the application. ⌥⌘Space is the Finder's search window on a
// stock macOS, so the gallery always falls back — see [DefaultLadder], and see
// [Hotkeys.Describe] for what was actually granted.
func DefaultShortcuts() []Shortcut {
	const mods = hotkey.Option | hotkey.Command
	return []Shortcut{
		{hotkey.Combo{Key: hotkey.KeyLeftArrow, Mods: mods}, ActionPrev},
		{hotkey.Combo{Key: hotkey.KeyRightArrow, Mods: mods}, ActionNext},
		{hotkey.Combo{Key: hotkey.KeySpace, Mods: mods}, ActionGallery},
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
func (h *Hotkeys) Describe() string {
	var b strings.Builder
	for i, k := range h.held {
		fmt.Fprintf(&b, "%s: %s", h.does[i], k.Combo())
		if k.Substituted() {
			fmt.Fprintf(&b, " (asked for %s, it was taken)", k.Wanted())
		}
		b.WriteByte('\n')
	}
	for _, err := range h.unmet {
		fmt.Fprintf(&b, "no global shortcut for %v\n", err)
	}
	return b.String()
}
