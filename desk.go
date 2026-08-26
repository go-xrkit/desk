// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"fmt"
	"sync"

	"github.com/go-xrkit/xrkit/ribbon"
)

// ErrNoScreens means a desk was asked for with nothing to show.
var ErrNoScreens = errors.New("desk: no screens")

// Feed is one screen's pixels, arriving over time.
//
// It is an interface rather than a concrete capture handle because what is
// behind it differs completely per platform — ScreenCaptureKit, X11 shared
// memory, a Windows duplication texture — while what this package needs of it is
// the same everywhere: the most recent pixels, and whether they are new.
//
// Frame must NOT allocate and must NOT copy: it hands back a borrowed view of
// whatever the capture already has. The whole frame budget is 16.6 ms and a
// capture that copies a 4K screen spends a fifth of it doing nothing useful.
type Feed interface {
	// Frame returns the latest pixels, and whether they are newer than the last
	// call. A feed with nothing yet returns a zero Source and false.
	Frame() (Source, bool)
	// Close releases the capture. It must be safe to call twice.
	Close() error
}

// Action is something the viewer asked for.
type Action int

// The actions a desk understands.
const (
	// ActionNone is a key that means nothing here.
	ActionNone Action = iota
	// ActionNext and ActionPrev turn the ribbon by one screen, the short way
	// round.
	ActionNext
	ActionPrev
	// ActionFullscreen promotes the focused screen to fill the view, or puts it
	// back.
	ActionFullscreen
	// ActionGallery opens the gallery — every screen at once, as a grid — or
	// closes it again leaving the ribbon exactly as it was.
	ActionGallery
	// ActionChoose takes the selected screen and returns to the ribbon focused
	// on it. It means nothing outside the gallery.
	ActionChoose
	// ActionUp and ActionDown move the selection in the gallery. On the ribbon
	// they mean nothing: a band has no rows.
	ActionUp
	ActionDown
	// ActionCycle asks for the next source on the focused screen. What that
	// means is the application's business — this package knows the ribbon, not
	// what a platform has to offer — so it is reported through OnCycle rather
	// than acted on here.
	ActionCycle
	// ActionQuit ends the session.
	ActionQuit
)

// String renders an action for a log.
func (a Action) String() string {
	switch a {
	case ActionNext:
		return "next"
	case ActionPrev:
		return "previous"
	case ActionFullscreen:
		return "fullscreen"
	case ActionCycle:
		return "cycle"
	case ActionGallery:
		return "gallery"
	case ActionChoose:
		return "choose"
	case ActionUp:
		return "up"
	case ActionDown:
		return "down"
	case ActionQuit:
		return "quit"
	default:
		return "none"
	}
}

// Desk is the running arrangement: a plan, the screens on their ribbon, the
// feeds filling them, and the panorama they are drawn into.
//
// Everything here is portable. It knows nothing about how a screen was created
// or how its pixels arrive — only that a [Feed] will hand them over. That is
// what lets the same logic run over ScreenCaptureKit, X11 and Windows, and what
// lets it be tested against feeds that are not screens at all.
type Desk struct {
	plan Plan
	nav  *ribbon.Nav

	// strip is the band, flat, and grid is every screen at once. Both are in
	// VIEW coordinates: the canvas is the picture rather than a panorama it is
	// then a projection of.
	strip *Strip
	grid  *Grid

	canvas  *Canvas
	feeds   []Feed
	sources []Source
	blits   []ribbon.Blit

	// mu serialises everything a viewer can change against everything the frame
	// loop reads. They are genuinely different goroutines — keys arrive on the
	// window back-end EVENT thread while the ribbon is advanced and drawn on a
	// TICKER — and neither ribbon.Nav nor these slices is safe on its own.
	mu sync.Mutex

	// Background is what the gaps between screens show.
	Background [4]byte

	// OnCycle, when set, is called with the FOCUSED position when the viewer
	// asks for the next source there. It is called without the desk's lock held,
	// so a handler may call SetFeed — which is the whole point of it.
	OnCycle func(pos int)

	quit bool
	// err is the last thing a viewer's action returned. The gallery can refuse
	// — a direction it has no cell for, choosing outside it — and a refusal that
	// nobody can see is a key that silently does nothing.
	err error
}

// DefaultBackground is a near-black that is not black, so a gap between screens
// reads as part of the arrangement rather than as a dead panel.
var DefaultBackground = [4]byte{12, 14, 18, 255}

// New builds a desk from a plan and one feed per screen.
//
// The feeds are taken in the plan's own order, so feeds[i] fills the screen the
// plan calls screen-(i+1). A nil feed is allowed and simply shows background:
// a display that exists but is not being captured yet is a normal state during
// start-up, not an error.
func New(plan Plan, feeds []Feed) (*Desk, error) {
	if plan.Count() <= 0 {
		return nil, ErrNoScreens
	}
	if len(feeds) != plan.Count() {
		return nil, fmt.Errorf("%w: %d feeds for %d screens",
			ErrNoScreens, len(feeds), plan.Count())
	}
	r, err := ribbon.Place(plan.Screens(), plan.Layout)
	if err != nil {
		return nil, fmt.Errorf("desk: placing screens: %w", err)
	}
	// The band, flat. One screen is one full view, so the view is a screen and
	// a screen at rest fills it exactly.
	placed := make([]ribbon.Placed, r.Len())
	for i := range placed {
		placed[i] = r.At(i)
	}
	strip, err := NewStrip(placed, rad(plan.HFOVDeg),
		plan.ScreenW, plan.ScreenH, plan.ScreenW, plan.ScreenH)
	if err != nil {
		return nil, fmt.Errorf("desk: laying out the band: %w", err)
	}
	// Every screen at once, folded into the same view. It is head-locked, so
	// unlike the band it has no offset to follow and nothing to rebuild.
	grid, err := NewGrid(plan.Count(), plan.ScreenW, plan.ScreenH,
		plan.ScreenW, plan.ScreenH, DefaultGapPx)
	if err != nil {
		return nil, fmt.Errorf("desk: folding the gallery: %w", err)
	}
	return &Desk{
		plan:       plan,
		nav:        ribbon.NewNav(r),
		strip:      strip,
		grid:       grid,
		canvas:     NewCanvas(ribbon.Pano{W: plan.ScreenW, H: plan.ScreenH}),
		feeds:      feeds,
		sources:    make([]Source, len(feeds)),
		blits:      make([]ribbon.Blit, 0, len(feeds)+2),
		Background: DefaultBackground,
	}, nil
}

// Plan returns what this desk was built from.
func (d *Desk) Plan() Plan { return d.plan }

// Nav exposes the navigator, for a caller that wants to ask where the ribbon is.
func (d *Desk) Nav() *ribbon.Nav { return d.nav }

// Canvas is the panorama, for the warp to read.
func (d *Desk) Canvas() *Canvas { return d.canvas }

// Err reports what the last action returned, or nil. The gallery can refuse —
// a direction it has no cell for — and a refusal nobody can see is a key that
// silently does nothing.
func (d *Desk) Err() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.err
}

// Quit reports whether the viewer has asked to stop.
func (d *Desk) Quit() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.quit
}

// Do carries out an action.
func (d *Desk) Do(a Action) {
	var cycle func(int)
	pos := -1
	d.mu.Lock()
	// In the gallery the arrows move a selection in a grid; on the ribbon they
	// turn the band. Same keys, and they must not mean both at once.
	if d.nav.Mode() == ribbon.ModeGallery {
		switch a {
		case ActionPrev:
			d.err = d.grid.Move(ribbon.Left)
		case ActionNext:
			d.err = d.grid.Move(ribbon.Right)
		case ActionUp:
			d.err = d.grid.Move(ribbon.Up)
		case ActionDown:
			d.err = d.grid.Move(ribbon.Down)
		case ActionChoose:
			d.err = d.nav.Choose()
		case ActionGallery:
			d.err = d.nav.ToggleGallery(d.grid)
		case ActionQuit:
			d.quit = true
		}
		d.mu.Unlock()
		return
	}

	switch a {
	case ActionNext:
		d.nav.Next()
	case ActionPrev:
		d.nav.Prev()
	case ActionGallery:
		d.err = d.nav.ToggleGallery(d.grid)
	case ActionFullscreen:
		d.nav.ToggleFullscreen()
	case ActionCycle:
		// Answered after the lock is dropped: a handler that changes what this
		// position shows has to be able to take it.
		cycle, pos = d.OnCycle, d.nav.Focus()
	case ActionQuit:
		d.quit = true
	}
	d.mu.Unlock()
	if cycle != nil {
		cycle(pos)
	}
}

// Advance moves the ribbon towards where it is going, dt seconds later.
func (d *Desk) Advance(dt float64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.nav.Advance(dt)
}

// Render draws the current frame into the panorama and returns it.
//
// Every feed is asked for its latest pixels whether or not it is visible: a feed
// that is never read may stop delivering, and the cost is a pointer.
func (d *Desk) Render() *Canvas {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i, f := range d.feeds {
		if f == nil {
			continue
		}
		if s, fresh := f.Frame(); fresh || d.sources[i].Pix == nil {
			d.sources[i] = s
		}
	}

	d.blits = d.blits[:0]
	switch d.nav.Mode() {
	case ribbon.ModeGallery:
		// Head-locked: no offset, because the grid is in front of the viewer
		// rather than out on the band.
		d.blits = d.grid.Frame(d.blits)
	case ribbon.ModeFullscreen:
		// The only error Fullscreen can return is an out-of-range screen, and
		// the navigator's focus is always a screen on this very band — so it
		// cannot happen here. TestPromotingAnyScreenAlwaysWorks pins that
		// assumption down, rather than leaving a branch that can never be taken
		// and therefore never be tested.
		d.blits, _ = d.strip.Fullscreen(d.blits, d.nav.Focus())
	default:
		d.blits = d.strip.Frame(d.blits, d.strip.Offset(d.nav.Yaw()))
	}

	d.canvas.Compose(d.blits, d.sources, d.Background)
	return d.canvas
}

// Close shuts every feed down, returning the first error but closing them all.
func (d *Desk) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	var first error
	for _, f := range d.feeds {
		if f == nil {
			continue
		}
		if err := f.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// KeyAction maps a key to what it does.
//
// These are the keys that work while the desk itself has focus. The shortcuts
// the viewer actually reaches for — turning the ribbon while working INSIDE one
// of the screens — cannot be these, because the focus is then in somebody else's
// application. Those are system-wide shortcuts and they are registered
// separately; this table is what remains useful when the desk is in front.
func KeyAction(code string) Action {
	switch code {
	case "Escape", "q", "Q":
		return ActionQuit
	case "ArrowLeft", "h", "H":
		return ActionPrev
	case "ArrowRight", "l", "L":
		return ActionNext
	case " ", "f", "F":
		return ActionFullscreen
	case "Tab", "c", "C":
		return ActionCycle
	case "g", "G":
		return ActionGallery
	case "Enter", "Return":
		return ActionChoose
	case "ArrowUp", "k", "K":
		return ActionUp
	case "ArrowDown", "j", "J":
		return ActionDown
	}
	return ActionNone
}

// SetFeed replaces what one ribbon position shows, while the desk is running.
//
// It returns the feed that was there, and does NOT close it. That is deliberate:
// a viewer who parks a screen to look at something else usually wants it back,
// and a method that closed it would make "swap" and "discard" the same gesture.
// A caller that really is finished with the old feed closes it itself.
//
// The last picture from the old feed is dropped with it, so a position whose new
// feed has not produced anything yet shows background rather than the previous
// screen's contents — which would be a lie about what is on that panel.
func (d *Desk) SetFeed(i int, f Feed) (Feed, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if i < 0 || i >= len(d.feeds) {
		return nil, fmt.Errorf("%w: position %d of %d", ErrNoScreens, i, len(d.feeds))
	}
	old := d.feeds[i]
	d.feeds[i] = f
	d.sources[i] = Source{}
	return old, nil
}

// FeedAt reports what is at a ribbon position, or nil for an empty one.
func (d *Desk) FeedAt(i int) Feed {
	d.mu.Lock()
	defer d.mu.Unlock()
	if i < 0 || i >= len(d.feeds) {
		return nil
	}
	return d.feeds[i]
}
