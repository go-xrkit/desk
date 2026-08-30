// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"fmt"
	"sync"

	"github.com/go-widgets/toolkit"
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
	// ActionGalleryOpen opens the gallery and ActionGalleryClose leaves it,
	// each doing nothing when the gallery is already in that state.
	//
	// They exist alongside the toggle because a system-wide shortcut is pressed
	// blind: the viewer cannot see whether the gallery is open before deciding,
	// and one key that means "open" from outside and "close" from inside is one
	// key that does the wrong thing whenever they have lost track. Two keys
	// always mean what they say.
	ActionGalleryOpen
	ActionGalleryClose
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
	// ActionSettings asks for the settings window.
	//
	// It ENDS the ribbon, like ActionQuit, and says so through
	// [Desk.WantsSettings] -- because the two windows cannot be on screen at
	// once. A back-end holds one window: the ribbon covers a display entirely
	// and takes the keyboard, and a settings window opened behind it would be a
	// dialogue nobody can see waiting for an answer nobody can give.
	//
	// So the desk stops, the settings are changed, and the desk starts again on
	// them. Which is also the only order in which a changed setting can take
	// effect: the screen count, the headset and the shortcuts are all read on the
	// way in.
	ActionSettings
	// ActionCloser and ActionFurther move the band towards the viewer and away
	// from them, by [DistanceStep].
	//
	// Further away is what shows the screens EITHER SIDE of the one in front:
	// they do not move, they take up less of the view, which is what pulling a
	// monitor back does. Nearer stops at one screen filling the view exactly,
	// because closer than that means seeing part of a screen.
	ActionCloser
	ActionFurther
	// ActionFlatter and ActionRounder change the angle between one screen and
	// the next, by [SplayStep].
	//
	// Flatter ends at one plane -- every screen square on, which is the band this
	// package drew before there was an angle and still what somebody wanting a
	// single wide surface should get. Rounder turns each screen further towards
	// the viewer, the way the two beside the middle one on a desk of three
	// monitors are turned.
	ActionFlatter
	ActionRounder
	// ActionPoint asks for the mouse pointer to be brought to the screen in
	// front of the viewer.
	//
	// It is the answer to the thing that made the desk unusable: the picture
	// shows screens that applications are running on, and the pointer is
	// somewhere else on the desktop -- reachable only by dragging it blind across
	// displays whose contents are captures of somewhere else. One key ends that.
	//
	// Moving a pointer is the platform's business, not this package's, so it is
	// reported through OnPoint rather than acted on here -- the same seam as
	// ActionCycle and for the same reason.
	ActionPoint
	// ActionApps opens the gallery of running APPLICATIONS, or closes it.
	//
	// It is the other gallery. The screen gallery answers "which desktop am I
	// looking at"; this one answers "what is open, and where is it" — a grid of
	// the applications that have a window, each saying which screen it is on.
	//
	// From it, ActionChoose puts the highlighted application on the screen the
	// band is showing. That pairing is deliberate: a system-wide shortcut is
	// pressed blind, and "put this where I am looking" needs no second key and
	// no number typed at a picture the person may not be able to see.
	ActionApps
	// ActionSpread hands out one screen per application, in order, up to the
	// ribbon's screen count.
	//
	// One key, and a desk of six empty desktops is a desk of six applications.
	// Applications past the last screen are LEFT WHERE THEY ARE rather than
	// wrapped onto a screen that already has one: two windows on one screen
	// hides one of them, and a person who pressed one key cannot be expected to
	// guess which.
	ActionSpread
	// ActionRemove takes the selected screen off the band, in the gallery.
	//
	// The gallery is where a person looks at the desk they have, so it is where
	// they will want one fewer as well as one more -- and the alternative is a
	// settings file, which is not where anybody is when they run out of use for
	// a screen. It means nothing outside the gallery and nothing on the cell
	// that ADDS one.
	ActionRemove
	// ActionAppsOpen shows the application gallery, and does nothing when it is
	// already up.
	//
	// It exists beside the toggle for the same reason ActionGalleryOpen does: a
	// system-wide shortcut is pressed BLIND. One key that means "show me what is
	// running" from outside and "put it away" from inside does the wrong thing
	// every time the person has lost track of which they are in.
	ActionAppsOpen
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
	case ActionGalleryOpen:
		return "open the gallery"
	case ActionGalleryClose:
		return "leave the gallery"
	case ActionChoose:
		return "choose"
	case ActionUp:
		return "up"
	case ActionDown:
		return "down"
	case ActionQuit:
		return "quit"
	case ActionSettings:
		return "settings"
	case ActionCloser:
		return "closer"
	case ActionFurther:
		return "further"
	case ActionFlatter:
		return "flatter"
	case ActionRounder:
		return "rounder"
	case ActionPoint:
		return "bring the pointer here"
	case ActionApps:
		return "the applications"
	case ActionSpread:
		return "spread the applications"
	case ActionRemove:
		return "remove this screen"
	case ActionAppsOpen:
		return "show the applications"
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

	// apps is the other gallery — what is RUNNING rather than what is on the
	// band — and inApps whether it is up. It is not a ribbon mode: the band
	// keeps its focus underneath, because "put this application on the screen I
	// am looking at" needs that focus to still mean something.
	apps   *appsView
	inApps bool

	canvas  *Canvas
	feeds   []Feed
	sources []Source
	blits   []ribbon.Blit
	// slants is the frame when the screens are TURNED, and fan the chain that
	// produces them. A nil fan is the flat band: see [Plan.SplayDeg].
	slants []Slant
	fan    *Fan

	// mu serialises everything a viewer can change against everything the frame
	// loop reads. They are genuinely different goroutines — keys arrive on the
	// window back-end EVENT thread while the ribbon is advanced and drawn on a
	// TICKER — and neither ribbon.Nav nor these slices is safe on its own.
	mu sync.Mutex

	// Background is what the gaps between screens show.
	Background [4]byte

	// OnAdd, when set, is what the gallery's "add a screen" cell calls. It must
	// return a feed for the new screen — which on macOS means creating another
	// virtual display and capturing it.
	//
	// It is a callback and not something this package does, for the same reason
	// the feeds are handed in rather than opened here: making a display is the
	// platform's business and this file has no operating system in it.
	OnAdd func() (Feed, error)

	// OnCycle, when set, is called with the FOCUSED position when the viewer
	// asks for the next source there. It is called without the desk's lock held,
	// so a handler may call SetFeed — which is the whole point of it.
	OnCycle func(pos int)

	// OnPoint, when set, is called with the position whose screen the pointer
	// should be brought to. It is called without the desk's lock held.
	//
	// The desk knows which screen a person is looking at; only the application
	// knows what a screen IS to the window server. So this is the seam, like
	// OnCycle -- and it is the one that makes the desk usable rather than merely
	// visible: without it the pointer has to be dragged blind across displays
	// whose contents are captures of somewhere else.
	OnPoint func(pos int)

	// OnRemove, when set, is called after a screen has been taken off the band,
	// with the position it was at and the feed that was on it.
	//
	// The desk has already shrunk by then: what is left is the platform work --
	// closing the capture and giving the display back -- which this package does
	// not do, for the same reason it does not create one.
	OnRemove func(pos int, f Feed)

	// OnApps, when set, answers what is running whenever the application
	// gallery is opened. It is asked EVERY time rather than once, because the
	// list is what a person opened the gallery to see: an application that
	// quit two minutes ago must not still be offered a screen.
	//
	// A callback for the same reason as the others: enumerating windows talks to
	// the window server, and there is no operating system in this file.
	OnApps func() ([]App, error)

	// OnPlace, when set, is handed the placements the viewer asked for — one
	// from the application gallery, or a screen each from [ActionSpread].
	//
	// It is called without the desk's lock held, and it deliberately hands over
	// [Placement]s rather than doing anything: the live path is then the same
	// one the settings file uses at start-up, with the same menu-bar allowance
	// and the same reporting, instead of a second placement path that would
	// drift from it.
	OnPlace func(places []Placement)

	quit bool
	// settings is set with quit when the desk stopped to show the settings
	// window rather than to end the session. See [Desk.WantsSettings].
	settings bool
	// badge says which screen the viewer has arrived at, for a moment. Nil when
	// it has been turned off.
	badge *badge

	// marks number the gallery's cells and light the chosen one.
	marks *marks

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
	r, strip, grid, fan, err := build(plan)
	if err != nil {
		return nil, err
	}
	return &Desk{
		plan:       plan,
		nav:        ribbon.NewNav(r),
		strip:      strip,
		grid:       grid,
		canvas:     NewCanvas(plan.ScreenW, plan.ScreenH),
		feeds:      feeds,
		sources:    make([]Source, len(feeds)),
		blits:      make([]ribbon.Blit, 0, len(feeds)+2),
		slants:     make([]Slant, 0, 2*FanReach+1),
		fan:        fan,
		apps:       newAppsView(nil),
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

// WantsSettings reports whether the desk stopped in order to show the settings
// rather than to end the session.
//
// It is a separate question from [Desk.Quit] because both are true at once: the
// ribbon has to come down either way, and only the caller knows what to put in
// its place.
func (d *Desk) WantsSettings() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.settings
}

// InGallery reports whether a gallery covers the view -- either of them.
//
// It is what tells a caller when the BARE keys are worth claiming: while a
// gallery is up the person is looking at the desk rather than working in an
// application, and the arrows should mean what they look like they mean. See
// [GalleryShortcuts].
func (d *Desk) InGallery() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.inApps || d.nav.Mode() == ribbon.ModeGallery
}

// Do carries out an action.
func (d *Desk) Do(a Action) {
	var cycle, point func(int)
	var add func() (Feed, error)
	pos := -1
	drop := -1
	var place []Placement
	var list func() ([]App, error)
	d.mu.Lock()

	// The APPLICATION gallery comes first, and it is not a ribbon mode: the band
	// keeps its focus underneath, because "put this one on the screen I am
	// looking at" needs that focus to still mean something.
	if d.inApps {
		switch a {
		case ActionPrev:
			d.err = d.apps.move(ribbon.Left)
		case ActionNext:
			d.err = d.apps.move(ribbon.Right)
		case ActionUp:
			d.err = d.apps.move(ribbon.Up)
		case ActionDown:
			d.err = d.apps.move(ribbon.Down)
		case ActionChoose:
			// The screen the band is showing, which is why this gallery does not
			// take the ribbon out of its mode.
			if app, ok := d.apps.app(); ok {
				place = []Placement{{App: app.Name, Pos: d.nav.Focus() + 1}}
				// And the gallery goes away, exactly as the screen gallery's
				// choose does.
				//
				// Not tidiness: with it open the arrows move the SELECTION, so
				// the band cannot be turned, so the only thing a second choose
				// could do is put another application on the same screen — which
				// hides one of them. Closing is what lets the next one go
				// somewhere else, and it shows the person what just happened.
				d.inApps = false
			} else {
				d.err = ErrNoApps
			}
		case ActionSpread:
			place = Spread(d.apps.apps, d.plan.Count())
			// Away, for the same reason as choose: what it did is on the BAND,
			// and the list behind it now says where everything used to be.
			d.inApps = false
		case ActionApps, ActionGalleryClose:
			// Open, so both of these leave. GalleryClose too: a person who
			// pressed "leave the gallery" meant the picture in front of them,
			// whichever gallery it is.
			d.inApps = false
		case ActionAppsOpen:
			// Already showing them. Nothing, deliberately: this is the key that
			// always means the same thing however lost the person is.
		case ActionQuit:
			d.quit = true
		case ActionSettings:
			d.quit, d.settings = true, true
		}
		d.mu.Unlock()
		if place != nil && d.OnPlace != nil {
			d.OnPlace(place)
		}
		return
	}

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
			if d.grid.IsAdder(d.grid.Selected()) {
				add = d.OnAdd
				break
			}
			d.err = d.nav.Choose()
		case ActionRemove:
			// Not the adder: there is no screen behind it to take away.
			if !d.grid.IsAdder(d.grid.Selected()) {
				drop = d.grid.Selected()
			}
		case ActionGallery, ActionGalleryClose:
			// Already open, so both of these leave it.
			d.err = d.nav.ToggleGallery(d.grid)
		case ActionQuit:
			d.quit = true
		case ActionSettings:
			d.quit, d.settings = true, true
		case ActionPoint:
			// In the gallery the pointer goes to the SELECTED cell's screen, not
			// to the focused one: the gallery is where a person picks, and the
			// selection is what they are pointing at.
			if !d.grid.IsAdder(d.grid.Selected()) {
				point, pos = d.OnPoint, d.grid.Selected()
			}
		case ActionCloser, ActionFurther, ActionFlatter, ActionRounder:
			// The gallery is head-locked and shows every screen at a readable
			// size already, so there is no distance to change while it is open.
			// Left as nothing rather than made an error: a system-wide key is
			// pressed blind, and a refusal for pressing it in the wrong place is
			// a complaint about something the person could not have known.
		}
		d.mu.Unlock()
		if point != nil {
			point(pos)
		}
		if add != nil {
			// Outside the lock: making a display and capturing it takes long
			// enough that holding the desk shut for it would stall the frame
			// loop, and Grow takes the lock itself.
			d.grow(add)
		}
		if drop >= 0 {
			// Same seam, same reason: Shrink takes the lock, and giving a
			// display back talks to the window server.
			d.drop(drop)
		}
		return
	}

	switch a {
	case ActionNext:
		d.nav.Next()
	case ActionPrev:
		d.nav.Prev()
	case ActionGallery, ActionGalleryOpen:
		// Not open, so both of these open it. ActionGalleryClose falls through
		// to nothing, which is what leaving a gallery you are not in should do.
		d.err = d.nav.ToggleGallery(d.grid)
	case ActionApps, ActionAppsOpen:
		// Ask what is running, outside the lock, and only then put the gallery
		// up: a list read once at start-up would offer a screen to something
		// that quit an hour ago.
		list = d.OnApps
	case ActionSpread:
		// From the band, with whatever the last look at the gallery found. It
		// asks for a fresh list too — one key that spreads a stale list would
		// move the wrong windows.
		list = d.OnApps
	case ActionFullscreen:
		d.nav.ToggleFullscreen()
	case ActionCycle:
		// Answered after the lock is dropped: a handler that changes what this
		// position shows has to be able to take it.
		cycle, pos = d.OnCycle, d.nav.Focus()
	case ActionPoint:
		// Same seam, same reason: moving a pointer talks to the window server.
		point, pos = d.OnPoint, d.nav.Focus()
	case ActionQuit:
		d.quit = true
	case ActionSettings:
		d.quit, d.settings = true, true
	case ActionCloser:
		d.err = d.reshape(d.plan.WithDistance(d.plan.Distance() - DistanceStep))
	case ActionFurther:
		d.err = d.reshape(d.plan.WithDistance(d.plan.Distance() + DistanceStep))
	case ActionFlatter:
		d.err = d.reshape(d.plan.WithSplay(d.plan.SplayDeg() - SplayStep))
	case ActionRounder:
		d.err = d.reshape(d.plan.WithSplay(d.plan.SplayDeg() + SplayStep))
	}
	d.mu.Unlock()
	if cycle != nil {
		cycle(pos)
	}
	if point != nil {
		point(pos)
	}
	if list != nil {
		d.refresh(list, a)
	}
}

// refresh asks what is running and then either shows the gallery or spreads.
//
// Outside the lock, because enumerating windows talks to the window server and
// holding the desk shut for it would stall the frame loop — the same reason
// OnAdd is answered out here.
func (d *Desk) refresh(list func() ([]App, error), a Action) {
	apps, err := list()
	d.mu.Lock()
	if err != nil {
		// The Accessibility grant is the usual reason, and it is worth seeing:
		// a gallery that opened empty would look like "nothing is running".
		d.err = err
		d.mu.Unlock()
		return
	}
	d.apps.set(apps)
	var place []Placement
	switch a {
	case ActionApps, ActionAppsOpen:
		d.inApps = true
	case ActionSpread:
		place = Spread(apps, d.plan.Count())
	}
	d.mu.Unlock()
	if place != nil && d.OnPlace != nil {
		d.OnPlace(place)
	}
}

// Advance moves the ribbon towards where it is going, dt seconds later.
// Badge turns the arrival badge on for this many seconds, or off at zero. It
// must be set before the first Render.
func (d *Desk) Badge(seconds float64, theme *toolkit.Theme) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.badge = newBadge(seconds, theme)
	d.marks = newMarks(theme)
	d.apps = newAppsView(theme)
}

func (d *Desk) Advance(dt float64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.nav.Advance(dt)
	d.badge.tick()
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

	// A screen showing something that is not the shape of the glasses takes
	// that shape, rather than sitting in an empty band.
	d.fit()

	d.blits = d.blits[:0]
	inGallery := d.nav.Mode() == ribbon.ModeGallery
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
		// The band. Turned, when there is an angle to turn it by: the fan and the
		// strip draw the same screens in the same order, and at a splay of
		// nothing they draw the same pixels -- which is asserted, not assumed.
		//
		// The strip keeps the flat case because it is worth keeping: every panel
		// square on means every panel is a rectangle, and a rectangle is a run of
		// row copies instead of a pixel at a time.
		if d.fan != nil {
			// Where the band is comes from the STRIP, which takes it from the
			// ribbon's own arrangement. Two renderers with two ideas of where the
			// band is would be the mistake that put it half a screen out of step
			// with the navigator, made twice.
			d.slants = d.fan.Frame(d.slants[:0], d.nav.Focus(),
				d.strip.Toward(d.nav.Yaw(), d.nav.Focus()))
			d.canvas.ComposeSlants(d.slants, d.sources, d.Background)
			d.mark(inGallery)
			return d.canvas
		}
		d.blits = d.strip.Frame(d.blits, d.strip.Offset(d.nav.Yaw()))
	}

	d.canvas.Compose(d.blits, d.sources, d.Background)
	d.mark(inGallery)
	return d.canvas
}

// mark puts the numbers on the finished picture.
//
// After the screens, so it is ON the picture rather than under it -- and in one
// place, because there are two ways to compose a frame now and both of them end
// here.
//
// The two marks say different things. On the band the question is "which one am I
// looking at", and the answer goes up for a moment and leaves. In the gallery
// every screen is in front of the viewer at once, so the question is "which is
// which, and which would Enter take" -- and that has to be on the picture for as
// long as the gallery is.
func (d *Desk) mark(inGallery bool) {
	// The application gallery covers everything, including the screen gallery
	// underneath it: it is the picture the person is looking at, and two grids
	// at once would be two selections at once.
	if d.inApps {
		d.apps.draw(d.canvas)
		return
	}
	if inGallery {
		// Which cell is which, and which one Enter would take. Without this the
		// arrows move a selection that is nowhere on the picture.
		d.marks.draw(d.canvas, d.grid, d.grid.Selected())
		return
	}
	d.badge.show(d.nav.Focus(), d.plan.Count())
	d.badge.draw(d.canvas)
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
	// The band comes towards you and goes away. Both spellings of each, because a
	// keyboard has two plus keys and a person reaches for whichever is nearer.
	case "-", "_":
		return ActionFurther
	case "+", "=":
		return ActionCloser
	// The angle between the screens, on the brackets next to them: a keyboard has
	// no key that means "more turned", and these are where a person expects a
	// pair of opposites they were not told about.
	case "[", "{":
		return ActionFlatter
	case "]", "}":
		return ActionRounder
	// The pointer, on the key next to the one that means "here" in every editor.
	case "m", "M":
		return ActionPoint
	// The applications, on their own initial, and the spread on the key next to
	// it that no other action wanted.
	case "a", "A":
		return ActionApps
	case "x", "X":
		return ActionSpread
	// Taking a screen away, on the two keys every list on every system deletes
	// with. Both, because a Mac keyboard's big one reports as Backspace and the
	// full-size one has Delete beside it.
	case "Backspace", "Delete":
		return ActionRemove
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

// Click chooses a screen from the gallery, at a point in the picture's own
// coordinates. It reports whether the click landed on a screen.
//
// One click goes there, rather than one to highlight and another to confirm.
// The gallery exists to be left: somebody who has found the screen they want
// has already decided, and asking them to say so twice is asking them to aim
// twice at a tile in a headset.
//
// On the band it does nothing. A click on a captured desktop is not this
// application's to interpret — the desktop under the cursor will have its own
// idea of what was clicked.
func (d *Desk) Click(x, y int) bool {
	var add func() (Feed, error)
	d.mu.Lock()
	if d.nav.Mode() != ribbon.ModeGallery {
		d.mu.Unlock()
		return false
	}
	i, ok := d.grid.At(x, y)
	if !ok {
		d.mu.Unlock()
		return false
	}
	// At only ever returns a cell the grid has, so Select cannot refuse it and
	// there is no branch here to leave untested.
	_ = d.grid.Select(i)
	if d.grid.IsAdder(i) {
		add = d.OnAdd
		d.mu.Unlock()
		if add == nil {
			return false
		}
		d.grow(add)
		d.mu.Lock()
		defer d.mu.Unlock()
		return d.err == nil
	}
	d.err = d.nav.Choose()
	defer d.mu.Unlock()
	return d.err == nil
}

// Grow puts another screen on the band, at the end, and returns its position.
//
// Everything that was sized for the old count is rebuilt: the placement, the
// band, the gallery. Rebuilt rather than grown, because the layout is DERIVED
// from the count — the pitch between screens is a share of the whole turn — so
// there is no version of this that adjusts one number and leaves the rest
// standing.
//
// The viewer keeps the screen they were facing. Adding a seventh screen must
// not move the band, or a person who added one to put something on it would
// find themselves somewhere else.
func (d *Desk) Grow(f Feed) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Through the same swap a change of distance makes: everything the band, the
	// gallery and the navigator are made of comes from the plan, so one more
	// screen and a different distance are the same operation on a different
	// plan. reshape touches nothing until all three exist, which is what leaves
	// a refusal with the desk exactly as it was rather than half grown.
	plan := d.plan.WithScreens(d.plan.Count() + 1)
	if err := d.reshape(plan); err != nil {
		return 0, err
	}
	d.feeds = append(d.feeds, f)
	d.sources = append(d.sources, Source{})
	d.blits = make([]ribbon.Blit, 0, plan.Count()+2)
	return plan.Count() - 1, nil
}

// largeEnoughToArrive is a step long enough that the navigator finishes any
// turn in one go. Adding a screen should not start the band gliding.
const largeEnoughToArrive = 1e6

// grow asks the caller for a screen and puts it on the band.
//
// Whatever goes wrong is kept where a viewer can be shown it, rather than
// logged: they pressed a key expecting a screen, and nothing appearing with no
// explanation is the worst of the outcomes.
func (d *Desk) grow(add func() (Feed, error)) {
	f, err := add()
	if err != nil {
		d.mu.Lock()
		d.err = fmt.Errorf("desk: cannot add a screen: %w", err)
		d.mu.Unlock()
		return
	}
	if _, err := d.Grow(f); err != nil {
		if f != nil {
			// The feed was opened for a screen that does not exist now.
			_ = f.Close()
		}
		d.mu.Lock()
		d.err = err
		d.mu.Unlock()
	}
}

// build lays a plan out: the placement, the flat band, and the gallery.
//
// One function, used by New and by Grow, because a desk that has grown must be
// built exactly as one that started that size. Two copies of this would be two
// chances for the band and the gallery to disagree about how many screens there
// are, which is the kind of difference that shows up as a blank tile.
func build(plan Plan) (*ribbon.Ribbon, *Strip, *Grid, *Fan, error) {
	r, err := ribbon.Place(plan.Screens(), plan.Layout)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("desk: placing %d screens: %w", plan.Count(), err)
	}
	placed := make([]ribbon.Placed, r.Len())
	for i := range placed {
		placed[i] = r.At(i)
	}
	// The band, flat, at whatever distance the plan is seen from.
	//
	// Distance is the pixel SCALE of the band and nothing else: the whole band
	// divided by it, so every screen and every gap shrinks together and the ring
	// stays the same ring. At 1 a screen is exactly the view, which is where this
	// started; at 2 it is half of it and its two neighbours are in shot.
	band := int(float64(plan.Count()*(plan.ScreenW+DefaultGapPx)) / plan.Distance())
	strip, err := NewStrip(placed, band,
		plan.ScreenW, plan.ScreenH, plan.ScreenW, plan.ScreenH)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("desk: laying out the band: %w", err)
	}
	// And a screen that is not the shape of the glasses reads its own pixels
	// through its own width. The ARC it takes already came from that shape,
	// through Plan.Screens above.
	widths := make([]int, plan.Count())
	for i := range widths {
		widths[i] = plan.ScreenWidth(i)
	}
	strip.SetSourceWidths(widths)
	// Every screen at once, folded into the same view. It is head-locked, so
	// unlike the band it has no offset to follow and nothing to rebuild.
	grid, err := NewGrid(plan.Count(), plan.ScreenW, plan.ScreenH,
		plan.ScreenW, plan.ScreenH, DefaultGapPx)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("desk: folding the gallery: %w", err)
	}
	grid.SetSourceWidths(widths)
	// The fan, when there is an angle to turn the screens by. A nil fan is the
	// flat band and the strip draws it: every panel square on is every panel a
	// rectangle, which is a run of row copies rather than a pixel at a time.
	//
	// The error is dropped because there is none to have. NewFan refuses three
	// things -- no screens, no size, and a splay of nothing -- and all three are
	// already impossible here: the first two would have stopped NewStrip above,
	// and the third is the branch this is inside.
	// TestAValidPlanAlwaysBuildsItsFan pins that down rather than leaving a
	// branch that can never be taken and therefore never be tested.
	var fan *Fan
	if plan.SplayDeg() > 0 {
		fan, _ = NewFan(plan)
		// And the turned panels read their own widths too. Without this a panel
		// gathers columns past the end of a narrower source, which is not a
		// stretched picture but a PANIC.
		fan.SetSourceWidths(widths)
	}
	return r, strip, grid, fan, nil
}

// reshape swaps in a plan of the same screens seen differently, keeping the
// position the viewer is on.
//
// It is [Desk.Grow] without the growing: the band, the gallery and the
// navigator are all derived from the plan, so a plan that differs only in its
// pixel scale still means rebuilding the three of them. Nothing is touched until
// all three exist, so a refusal leaves the desk exactly as it was rather than
// half moved -- the same order Grow keeps, and for the same reason.
//
// The caller holds the lock.
func (d *Desk) reshape(plan Plan) error {
	if plan.Distance() == d.plan.Distance() && plan.Count() == d.plan.Count() &&
		plan.SplayDeg() == d.plan.SplayDeg() && plan.sameShapes(d.plan) {
		// Already there: at either end of the range every further press means
		// this, and rebuilding the band to put it back exactly as it was would
		// make the desk twitch for nothing.
		return nil
	}
	was := d.nav.Focus()
	r, strip, grid, fan, err := build(plan)
	if err != nil {
		return err
	}
	d.plan = plan
	d.strip, d.grid, d.fan = strip, grid, fan
	d.nav = ribbon.NewNav(r)
	// CLAMPED, not just attempted. A band that lost a screen can leave the focus
	// past the end, and GoTo refuses one it has no screen for -- leaving the new
	// navigator where it starts, which is screen ONE. So a person who deletes
	// the screen they were looking at was thrown back to the beginning of the
	// band. The nearest remaining screen is what they meant.
	if was >= plan.Count() {
		was = plan.Count() - 1
	}
	_ = d.nav.GoTo(was)
	d.nav.Advance(largeEnoughToArrive)
	return nil
}

// Distance is how far the band is from the viewer. See [Plan.Distance].
func (d *Desk) Distance() float64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.plan.Distance()
}

// Shrink takes screen pos off the band and returns the feed that was on it.
//
// The feed is RETURNED rather than closed, exactly as [Desk.SetFeed] does: what
// a caller wants to do with a capture it is no longer showing is the caller's
// business, and a method that closed it would make "put it away" and "throw it
// away" the same gesture.
//
// It refuses to take the last screen. A desk of nothing is not a desk, and
// everything here — the ribbon, the gallery, the navigator — is built from a
// plan of at least one; the person who wants none wants to quit.
func (d *Desk) Shrink(pos int) (Feed, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if pos < 0 || pos >= len(d.feeds) {
		return nil, fmt.Errorf("%w: screen %d of %d", ErrPosition, pos, len(d.feeds))
	}
	if len(d.feeds) == 1 {
		return nil, fmt.Errorf("%w: the last screen cannot be taken away", ErrScreens)
	}

	// Through the same swap as Grow and a change of distance: one fewer screen
	// is a different plan, and reshape touches nothing until everything it needs
	// exists — so a refusal leaves the desk exactly as it was.
	plan := d.plan.WithScreens(d.plan.Count() - 1)
	// reshape cannot fail for a SMALLER plan. Everything it builds -- the
	// ribbon, the strip, the gallery, the fan -- refuses only a count of
	// nothing, and the last screen has already been refused above; a gallery
	// that folded n cells folds n-1. TestShrinkingAnyDeskAlwaysWorks pins that
	// down, rather than leaving a branch that can never be taken and therefore
	// never be tested -- the same way Fullscreen is handled.
	_ = d.reshape(plan)
	gone := d.feeds[pos]
	// Copied into fresh slices rather than shifted in place. append(s[:i],
	// s[i+1:]...) rewrites the UNDERLYING ARRAY, which is the one the caller
	// handed to New: a removal here would silently reorder their slice, and a
	// test comparing what came back with what they passed in caught exactly
	// that. One small allocation, on an action a person takes by hand.
	feeds := make([]Feed, 0, len(d.feeds)-1)
	feeds = append(append(feeds, d.feeds[:pos]...), d.feeds[pos+1:]...)
	sources := make([]Source, 0, len(d.sources)-1)
	sources = append(append(sources, d.sources[:pos]...), d.sources[pos+1:]...)
	d.feeds, d.sources = feeds, sources
	d.blits = make([]ribbon.Blit, 0, plan.Count()+2)
	return gone, nil
}

// drop takes a screen off the band and hands what was on it to the caller.
//
// Whatever goes wrong is kept where a viewer can be shown it rather than
// logged: they pressed a key expecting a screen to go, and one that stays with
// no explanation is the worst of the outcomes.
func (d *Desk) drop(pos int) {
	f, err := d.Shrink(pos)
	if err != nil {
		d.mu.Lock()
		d.err = err
		d.mu.Unlock()
		return
	}
	d.mu.Lock()
	on := d.OnRemove
	d.mu.Unlock()
	if on != nil {
		on(pos, f)
	}
}

// Look turns the band to a screen without a key being pressed.
//
// It is what lets the ribbon FOLLOW something — the pointer, today. A position
// it has no screen for is refused; the position it is already on is nothing,
// deliberately, because "follow" is asked several times a second and a band that
// re-aimed at the screen it is already on would never settle.
//
// It is not a shortcut for [Desk.Do]: an action is what a person asked for, and
// this is the desk keeping up with them.
func (d *Desk) Look(pos int) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if pos < 0 || pos >= d.plan.Count() {
		return fmt.Errorf("%w: screen %d of %d", ErrPosition, pos, d.plan.Count())
	}
	if pos == d.nav.Focus() {
		return nil
	}
	// Not while a gallery is up: the person is choosing there, and a band
	// turning underneath a grid they are reading is a picture that moves for
	// no reason they can see.
	if d.inApps || d.nav.Mode() == ribbon.ModeGallery {
		return nil
	}
	return d.nav.GoTo(pos)
}

// Focus is the ribbon position in front of the viewer.
//
// It is what a caller needs to put something THERE -- the screen a wandering
// pointer went to, most often -- without having to track every action that
// could have turned the band since it last looked.
func (d *Desk) Focus() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.nav.Focus()
}

// fit gives a screen whose pixels are not the shape of the band a shape of its
// own, and rebuilds the band when one changes.
//
// It is driven by the PIXELS rather than by whoever opened the capture, because
// what a position shows changes while the desk runs -- a mirror of this Mac's
// own panel arrives on one key -- and the shape arrives with the first frame,
// not with the decision.
//
// Called with the lock held, from Render.
func (d *Desk) fit() {
	next, changed := d.plan, false
	for i, s := range d.sources {
		// Only a source that is the band's height: the capture is asked for
		// that height, so anything else is a frame that has not settled and not
		// a shape somebody chose.
		if s.W <= 0 || s.H != d.plan.ScreenH || s.W == d.plan.ScreenWidth(i) {
			continue
		}
		next, changed = next.WithScreenWidth(i, s.W), true
	}
	if !changed || next.sameShapes(d.plan) {
		return
	}
	// Reported rather than dropped, and it really can happen: a wide enough
	// screen stops the band fitting in 360 degrees, which is a property of the
	// shape AND of how many screens there are, so no fixed limit on one screen
	// can rule it out. TestAShapeTheBandCannotHoldIsRefused measures it. The
	// band is left exactly as it was: reshape builds before it assigns.
	if err := d.reshape(next); err != nil {
		d.err = err
	}
}
