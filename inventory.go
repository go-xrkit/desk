// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Errors an inventory can refuse with.
var (
	// ErrNoSuchOffer means nothing in the inventory has that identifier.
	ErrNoSuchOffer = errors.New("desk: no such source")
	// ErrPosition means the ribbon has no such position.
	ErrPosition = errors.New("desk: no such ribbon position")
	// ErrAlreadyShown means the source is already on another position.
	ErrAlreadyShown = errors.New("desk: that source is already on the ribbon")
	// ErrNoApps means no application has a window to show.
	ErrNoApps = errors.New("desk: no application with a window")
)

// Kind is what sort of thing a source is.
type Kind int

// The kinds of source a ribbon position can hold.
const (
	// KindDisplay is a display whose pixels are captured — a real monitor, or
	// one this program created.
	KindDisplay Kind = iota
	// KindPanel is a surface this program renders onto and reads back. On
	// Android that is a virtual display carrying a WebView or a decoder: things
	// the platform can draw and a CGO-free Go process cannot.
	KindPanel
)

// String names a kind for a person.
func (k Kind) String() string {
	switch k {
	case KindDisplay:
		return "display"
	case KindPanel:
		return "panel"
	default:
		return "unknown"
	}
}

// Offer is something that can be put on a ribbon position.
//
// It is a description, not a handle: an inventory can be shown to a person, and
// a list of open capture streams cannot. Turning one into pixels is the
// platform's job, and happens only when a position actually takes it.
type Offer struct {
	// ID identifies this source within an inventory, and is stable enough to
	// survive being written down and used later.
	ID string
	// Name is what a person should be shown.
	Name string
	// Kind is what sort of thing it is.
	Kind Kind
	// W and H are its pixel size, or zero when it has none yet — a panel's size
	// is decided when it is opened.
	W, H int
}

// String renders an offer the way a person would identify it.
func (o Offer) String() string {
	if o.W > 0 && o.H > 0 {
		return fmt.Sprintf("%s %q %dx%d", o.Kind, o.Name, o.W, o.H)
	}
	return fmt.Sprintf("%s %q", o.Kind, o.Name)
}

// Inventory is what CAN be shown, and what currently is.
//
// It holds no pixels and opens nothing. It exists so that choosing what a screen
// shows is a decision a person makes over a list they can read, separately from
// the machinery that then makes it appear — and so that the choosing can be
// tested without a display anywhere in sight.
//
// A source may be on at most ONE position at a time. Two positions showing the
// same display would each cost a capture of the same pixels, and a viewer
// turning the ribbon would pass the same screen twice and wonder which is real.
type Inventory struct {
	mu     sync.Mutex
	offers []Offer
	byID   map[string]Offer
	at     []string // position -> offer ID, "" for an empty position
}

// NewInventory describes a ribbon of the given length and what may go on it.
//
// Offers with a duplicate or empty ID are refused rather than silently
// collapsed: an identifier that names two things cannot be used to choose
// between them.
func NewInventory(positions int, offers []Offer) (*Inventory, error) {
	if positions <= 0 {
		return nil, fmt.Errorf("%w: %d", ErrPosition, positions)
	}
	inv := &Inventory{
		byID: make(map[string]Offer, len(offers)),
		at:   make([]string, positions),
	}
	for _, o := range offers {
		if strings.TrimSpace(o.ID) == "" {
			return nil, fmt.Errorf("%w: an offer with no identifier (%s)", ErrNoSuchOffer, o.Name)
		}
		if _, dup := inv.byID[o.ID]; dup {
			return nil, fmt.Errorf("%w: %q names two sources", ErrNoSuchOffer, o.ID)
		}
		inv.byID[o.ID] = o
		inv.offers = append(inv.offers, o)
	}
	return inv, nil
}

// Positions is how many places the ribbon has.
func (inv *Inventory) Positions() int {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	return len(inv.at)
}

// Offers is everything that could be shown, in the order it was given.
func (inv *Inventory) Offers() []Offer {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	return append([]Offer(nil), inv.offers...)
}

// At reports what is on a position, and whether anything is.
func (inv *Inventory) At(pos int) (Offer, bool) {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	return inv.atLocked(pos)
}

func (inv *Inventory) atLocked(pos int) (Offer, bool) {
	if pos < 0 || pos >= len(inv.at) || inv.at[pos] == "" {
		return Offer{}, false
	}
	o, ok := inv.byID[inv.at[pos]]
	return o, ok
}

// Where reports which position is showing a source, or -1.
func (inv *Inventory) Where(id string) int {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	return inv.whereLocked(id)
}

func (inv *Inventory) whereLocked(id string) int {
	for i, at := range inv.at {
		if at == id && id != "" {
			return i
		}
	}
	return -1
}

// Assign puts a source on a position, replacing whatever was there.
//
// It refuses a source that is already elsewhere on the ribbon rather than moving
// it, because a viewer who meant to move it and a viewer who forgot where it was
// make the same gesture, and only one of them wants the other position emptied.
// Clear the other position first to say which was meant.
func (inv *Inventory) Assign(pos int, id string) error {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	if pos < 0 || pos >= len(inv.at) {
		return fmt.Errorf("%w: %d of %d", ErrPosition, pos, len(inv.at))
	}
	if _, ok := inv.byID[id]; !ok {
		return fmt.Errorf("%w: %q", ErrNoSuchOffer, id)
	}
	if other := inv.whereLocked(id); other >= 0 && other != pos {
		return fmt.Errorf("%w: %q is on position %d", ErrAlreadyShown, id, other)
	}
	inv.at[pos] = id
	return nil
}

// Clear empties a position. Clearing an empty one is not an error: a viewer
// pressing the same key twice meant it both times.
func (inv *Inventory) Clear(pos int) error {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	if pos < 0 || pos >= len(inv.at) {
		return fmt.Errorf("%w: %d of %d", ErrPosition, pos, len(inv.at))
	}
	inv.at[pos] = ""
	return nil
}

// Unused is everything not currently on the ribbon, in the order offered.
func (inv *Inventory) Unused() []Offer {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	return inv.unusedLocked()
}

// Cycle moves a position to the next source that is not already shown, and to
// empty after the last one.
//
// It is the whole of "choose what this screen shows" reduced to one key. A
// picker that lists everything is better once there is somewhere to draw it;
// until then this makes the choice reachable with the keyboard alone, and it
// stays useful afterwards for the viewer who just wants the next thing.
//
// The second return is false when the position ended up empty.
func (inv *Inventory) Cycle(pos int) (Offer, bool) {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	if pos < 0 || pos >= len(inv.at) {
		return Offer{}, false
	}

	// Start just after whatever is here, so cycling walks the offers in their
	// own order rather than jumping back to the first free one every time.
	start := 0
	if cur := inv.at[pos]; cur != "" {
		for i, o := range inv.offers {
			if o.ID == cur {
				start = i + 1
				break
			}
		}
	}
	for i := start; i < len(inv.offers); i++ {
		o := inv.offers[i]
		if other := inv.whereLocked(o.ID); other >= 0 && other != pos {
			continue
		}
		inv.at[pos] = o.ID
		return o, true
	}
	inv.at[pos] = ""
	return Offer{}, false
}

// Describe renders the whole arrangement for a log or a person.
func (inv *Inventory) Describe() string {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	var b strings.Builder
	for i := range inv.at {
		o, ok := inv.atLocked(i)
		if ok {
			fmt.Fprintf(&b, "%d: %s\n", i+1, o)
		} else {
			fmt.Fprintf(&b, "%d: empty\n", i+1)
		}
	}
	if unused := inv.unusedLocked(); len(unused) > 0 {
		names := make([]string, len(unused))
		for i, o := range unused {
			names[i] = o.Name
		}
		sort.Strings(names)
		fmt.Fprintf(&b, "not shown: %s\n", strings.Join(names, ", "))
	}
	return b.String()
}

func (inv *Inventory) unusedLocked() []Offer {
	shown := make(map[string]bool, len(inv.at))
	for _, id := range inv.at {
		if id != "" {
			shown[id] = true
		}
	}
	var out []Offer
	for _, o := range inv.offers {
		if !shown[o.ID] {
			out = append(out, o)
		}
	}
	return out
}

// DisplayOf returns the display an offer captures.
//
// The identifier's shape — "display-N" — is the convention EVERY platform's
// Sources builds with, and it is derived from the display rather than handed
// out in order so that the same screen keeps the same identity across a
// restart. Reading it back is what lets a caller say "this position is showing
// one of the machine's own panels, and it is that one".
//
// It answers false for anything else, a window among other things: there is no
// display behind a window's picture to speak of.
func DisplayOf(o Offer) (uint64, bool) {
	if o.Kind != KindDisplay {
		return 0, false
	}
	rest, ok := strings.CutPrefix(o.ID, "display-")
	if !ok {
		return 0, false
	}
	id, err := strconv.ParseUint(rest, 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}
