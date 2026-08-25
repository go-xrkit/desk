// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"fmt"

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
	comp *ribbon.Compositor

	canvas  *Canvas
	feeds   []Feed
	sources []Source
	blits   []ribbon.Blit

	// Background is what the gaps between screens show.
	Background [4]byte

	quit bool
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
	comp, err := ribbon.NewCompositor(r, plan.Pano)
	if err != nil {
		return nil, fmt.Errorf("desk: preparing the panorama: %w", err)
	}
	return &Desk{
		plan:       plan,
		nav:        ribbon.NewNav(r),
		comp:       comp,
		canvas:     NewCanvas(plan.Pano),
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
func (d *Desk) Quit() bool      { return d.quit }

// Do carries out an action.
func (d *Desk) Do(a Action) {
	switch a {
	case ActionNext:
		d.nav.Next()
	case ActionPrev:
		d.nav.Prev()
	case ActionFullscreen:
		d.nav.ToggleFullscreen()
	case ActionQuit:
		d.quit = true
	}
}

// Advance moves the ribbon towards where it is going, dt seconds later.
func (d *Desk) Advance(dt float64) { d.nav.Advance(dt) }

// Render draws the current frame into the panorama and returns it.
//
// Every feed is asked for its latest pixels whether or not it is visible: a feed
// that is never read may stop delivering, and the cost is a pointer.
func (d *Desk) Render() *Canvas {
	for i, f := range d.feeds {
		if f == nil {
			continue
		}
		if s, fresh := f.Frame(); fresh || d.sources[i].Pix == nil {
			d.sources[i] = s
		}
	}

	d.blits = d.blits[:0]
	if d.nav.Mode() == ribbon.ModeFullscreen {
		// The only error Fullscreen can return is an out-of-range screen, and
		// the navigator's focus is always a screen on this very ribbon — so it
		// cannot happen here. TestPromotingAnyScreenAlwaysWorks pins that
		// assumption down, rather than leaving a branch that can never be taken
		// and therefore never be tested.
		d.blits, _ = d.comp.Fullscreen(d.blits, d.nav.Focus())
	} else {
		d.blits = d.comp.Frame(d.blits, d.nav.Yaw())
	}

	d.canvas.Compose(d.blits, d.sources, d.Background)
	return d.canvas
}

// Close shuts every feed down, returning the first error but closing them all.
func (d *Desk) Close() error {
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
