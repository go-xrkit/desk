// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"sync"

	"github.com/go-xrkit/xrkit/glasses"
)

// ErrNoDimming is what a platform with no backlight control answers.
var ErrNoDimming = errors.New("desk: darkening a display is not available on this platform")

// dimDisplay is the seam: the platform turns a panel off and hands back the way
// home. Tests replace it, and so does every platform that cannot do it.
var dimDisplay = platformDim

// Dimmer keeps the machine's own panels dark while a copy of them is on the
// ribbon.
//
// It exists because of what mirroring MEANS. A person wearing display glasses
// that show their Mac's screen is looking at a copy; the panel itself is then a
// second, brighter copy of private work at reading distance, facing whoever
// walks past, lit at full power for nobody.
//
// Turning the backlight off is not the same as covering the screen with a black
// window. The framebuffer is untouched, so the picture ON THE RIBBON does not
// change — there is no window for the capture to exclude, no stream to rebuild,
// and nothing another program can raise itself above.
//
// The way home is read before anything is changed and kept here, so [Restore]
// can always be deferred. A program that darkens somebody's screen and then
// dies leaves them with a black panel and no idea why.
type Dimmer struct {
	mu sync.Mutex
	on map[uint64]func() error
}

// Showing tells the dimmer which of the machine's own displays are on the
// ribbon now. Panels that have just arrived are darkened; panels that have left
// are put back.
//
// It is the whole API on purpose: the caller says what the world looks like and
// the dimmer works out the difference, rather than the caller having to
// remember what it darkened and pair every change with its undo.
func (m *Dimmer) Showing(ids []uint64) error {
	want := make(map[uint64]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	var errs []error
	for id, restore := range m.on {
		if want[id] {
			continue
		}
		delete(m.on, id)
		if err := restore(); err != nil {
			errs = append(errs, err)
		}
	}
	for id := range want {
		if m.on[id] != nil {
			continue
		}
		restore, err := dimDisplay(id)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if m.on == nil {
			m.on = make(map[uint64]func() error, len(want))
		}
		m.on[id] = restore
	}
	return errors.Join(errs...)
}

// Restore puts every panel back the way it was found. It is safe to call twice,
// which is what lets it be deferred beside a Showing that runs many times.
func (m *Dimmer) Restore() error { return m.Showing(nil) }

// Dark returns how many panels this dimmer is currently holding dark.
func (m *Dimmer) Dark() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.on)
}

// displaySize is the seam for a display's size, in the same units the chosen
// screen is described in. Tests replace it; a platform that cannot answer says
// so and nothing is darkened, which is the safe direction.
var displaySize = platformDisplaySize

// Mirrors picks, out of what the ribbon is showing, the machine's own panels
// that may be darkened.
//
// Two things are left out, for two different reasons.
//
// The displays this program MADE are not the machine's: there is no panel
// behind them and nothing to darken.
//
// And any display the size of the one the desk itself is on is left alone.
// That is deliberately more than it needs to be: a ribbon position CAN be
// pointed at the glasses' own display — the sources list offers it like any
// other — and darkening that one would black out the very thing the viewer is
// looking at, with the key that undoes it now invisible. There is no reliable
// way here to turn the chosen screen's NAME into a display id, so the size is
// used, and the failure it can produce is the harmless one: a second panel of
// exactly the same size stays lit.
func Mirrors(on []Offer, ours []uint64, mine glasses.Display) []uint64 {
	virtual := make(map[uint64]bool, len(ours))
	for _, id := range ours {
		virtual[id] = true
	}
	var out []uint64
	seen := make(map[uint64]bool, len(on))
	for _, o := range on {
		id, ok := DisplayOf(o)
		if !ok || virtual[id] || seen[id] {
			continue
		}
		w, h, ok := displaySize(id)
		if !ok || looksLike(w, h, mine) {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// looksLike says whether a display of this size could be the one the desk is
// on. Both the declared size and the size in points are accepted, because a
// screen described in framebuffer pixels and a rectangle measured in points are
// the same screen at a scale factor apart.
func looksLike(w, h int, mine glasses.Display) bool {
	if w == mine.Width && h == mine.Height {
		return true
	}
	if s := mine.Scale; s > 1 {
		return w == int(float64(mine.Width)/s) && h == int(float64(mine.Height)/s)
	}
	return false
}
