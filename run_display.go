// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-macos/hotkey"
	"github.com/go-widgets/toolkit"
	"github.com/go-widgets/window"
	"github.com/go-xrkit/xrkit/glasses"
)

// RunOptions are the choices a caller makes about the session itself, as
// opposed to the arrangement, which is the Plan's business.
type RunOptions struct {
	// Title names the window. Nobody sees it in the glasses; a window manager
	// and an accessibility tree do.
	Title string
	// Screen is the display to take over.
	Screen glasses.Display
	// For stops the session after this long. Zero runs until the viewer quits,
	// which is what a person wants and what a test does not.
	For time.Duration
	// Logf receives progress. A nil Logf says nothing.
	Logf func(string, ...any)

	// Shortcuts are the system-wide combinations to claim. Nil asks for
	// [DefaultShortcuts].
	Shortcuts []Shortcut
	// Hotkeys is how to claim them — chiefly the fallback ladder. Nil asks for
	// [DefaultLadder].
	Hotkeys *hotkey.Options

	// Actions are actions from somewhere other than the keyboard: a menu-bar
	// item, a script, a remote. They are treated exactly like a global shortcut
	// -- the same actions, the same loop -- because the difference between
	// pressing a key and choosing a menu row is the caller's business and not
	// this loop's.
	//
	// Nil is no such source, which is the default.
	Actions <-chan Action

	// Badge is how long the screen's number stays up after the band moves, in
	// seconds. Zero turns it off.
	Badge float64

	// Windowed leaves the glasses display's own menu bar and Dock on top of the
	// picture instead of covering them. See [Config.Immersive].
	Windowed bool

	// Interactive lets the desk's own window take the keyboard and the mouse.
	//
	// It does NOT by default, and that is the whole design rather than a
	// precaution. The desk is a picture of screens that applications are running
	// on: the keyboard has to reach THOSE applications, and a window that takes it
	// is a window that stops the person using their own desk. The pointer is
	// worse -- it wanders onto the display the desk owns, where the picture is a
	// capture of somewhere else and so does not show where the mouse is. Measured:
	// the way out was unplugging the glasses.
	//
	// So the desk is driven from outside, which is what the system-wide shortcuts
	// and the menu-bar item are for, and the applications keep the keyboard and
	// the pointer they always had.
	//
	// Interactive is for a session on a desktop -- with -windowed, to try the
	// thing out -- where clicking the picture is the only way in.
	Interactive bool

	// NoGlobal leaves the system-wide shortcuts unclaimed.
	//
	// Claiming them takes them away from everything else on the machine for as
	// long as the session lasts, which is what makes them useful and what makes
	// them worth being able to refuse.
	NoGlobal bool

	// Screens are the desk's displays in ribbon order, so the band can follow
	// the pointer onto one of them. Nil turns that off: without the ids there is
	// no way to tell one of the desk's screens from the machine's own.
	//
	// It is the arrangement at START-UP. See [RunOptions.Showing] for what a
	// position is showing NOW, which is not the same thing the moment somebody
	// mirrors this Mac's own panel onto the ribbon.
	Screens []uint64

	// Showing answers, per ribbon position, the display it is showing right now,
	// or 0 for a position showing nothing.
	//
	// Without it the band follows the pointer onto the displays this program
	// MADE and nothing else -- so a position mirroring this Mac's own screen is
	// a position the band will not follow the pointer onto, and moving the mouse
	// there loses it in exactly the way following was written to prevent. The
	// list changes while it runs, so it is asked for rather than handed over.
	//
	// Nil falls back to [RunOptions.Screens].
	Showing func() []uint64

	// Snapshot, when set, is handed the first frame actually drawn — the picture
	// the glasses were shown. It is written by the caller, so this package never
	// decides where a capture of somebody's screens lands.
	Snapshot func(pix []byte, w, h int)
}

// FrameInterval is how often the ribbon is advanced and redrawn.
//
// It is not tied to the display's refresh rate. The captures are change-driven
// and mostly still, the warp costs 2.8 ms and the composite under one, so
// drawing more often would spend the budget re-deriving a picture nobody
// changed.
const FrameInterval = 16 * time.Millisecond

// Run shows the desk until the viewer quits.
//
// Everything here is portable. The window, the toolkit and the warp all run on
// macOS, Linux, Windows and in a browser, so this loop is the same everywhere —
// only the screens and their pixels arrive by different roads.
func Run(ctx context.Context, plan Plan, d *Desk, opt RunOptions) error {
	logf := opt.Logf
	if logf == nil {
		logf = func(string, ...any) {}
	}
	title := opt.Title
	if title == "" {
		title = "xrdesk"
	}

	// Ask the window system for the target display NOW, rather than trusting the
	// one the caller looked up earlier.
	//
	// Creating virtual displays REARRANGES the desktop: on the first live run a
	// VITURE Beast sat at -1920,0 before three screens were made and at -3840,0
	// afterwards, so the frame captured beforehand matched nothing and the window
	// refused to open with "the requested screen is no longer attached". The name
	// survives that; a rectangle does not.
	screen, err := currentScreen(opt.Screen.Name)
	if err != nil {
		return err
	}
	win, err := window.Open(window.Config{
		Title: title,
		// A self-rendering root composes its own pixels at whatever size it is
		// handed, which is exactly what NativeScale is for: no logical-point
		// scaling between us and the panel.
		RenderScale: window.NativeScale,
		Screen:      screen,
		Fullscreen:  true,
		// Above the menu bar and the Dock rather than under them. A full-screen
		// window covers the DESKTOP and nothing more; on a display that carries
		// the furniture it appears on top of the picture, which on glasses
		// showing a captured desktop reads as two menu bars.
		Immersive: !opt.Windowed,
		// A picture, not something to work in: the keyboard and the pointer belong
		// to the applications the desk is showing.
		Passive: !opt.Interactive,
		Theme:   toolkit.DefaultDark(),
	})
	if err != nil {
		return fmt.Errorf("desk: cannot open a window on %s: %w", opt.Screen, err)
	}
	defer win.Close()

	fbW, fbH := win.Size()
	logf("framebuffer %dx%d", fbW, fbH)
	if opt.Interactive {
		logf("this window takes the keyboard and the mouse, so the applications on " +
			"the screens do not")
	} else {
		logf("the keyboard and the pointer belong to the applications on the " +
			"screens; this window is a picture, driven by the shortcuts above")
	}
	v, err := newView(plan, fbW, fbH)
	if err != nil {
		return err
	}
	logf("the panorama reaches %.1f%% of the view", 100*v.Coverage)
	v.Snapshot = opt.Snapshot
	d.Badge(opt.Badge, toolkit.DefaultDark())
	if opt.Badge > 0 {
		logf("the screen's number shows for %v when the band moves", BadgeDuration(opt.Badge))
	}

	// sync claims or releases the gallery's bare keys, and is filled in below —
	// declared here because the window's own key handler calls it too.
	sync := func() {}

	// THE BAND FOLLOWS THE POINTER.
	//
	// This is what makes a desk of captured screens usable rather than
	// something to look at. The band shows ONE screen; the pointer lives on the
	// desktop, which is several. Move the mouse off the right-hand edge of the
	// screen in front of you and on a real desk you have arrived at the next
	// monitor -- here you had simply lost it somewhere you could not see.
	//
	// Nothing is warped and nothing is synthesised: the pointer stays exactly
	// where the person put it, and the picture catches up. Asked once a frame,
	// which is two CoreGraphics calls and no allocation.
	// What each position is showing, now: a mirrored panel is a screen of the
	// band like any other, and the pointer is just as lost on it.
	showing := opt.Showing
	if showing == nil {
		showing = func() []uint64 { return opt.Screens }
	}
	var followed = -1
	follow := func() {
		pos, ok := PositionOf(showing())
		if !ok || pos == followed {
			return
		}
		followed = pos
		if err := d.Look(pos); err != nil {
			logf("following the pointer: %v", err)
			return
		}
		logf("the pointer is on screen %d", pos+1)
	}

	// AND IT COMES BACK AT THE OTHER END.
	//
	// The band is a circle and the desktop is a line. Pushed to the left of the
	// first screen the pointer is against a wall: the band cannot follow it
	// anywhere, and the last screen is all the way back across every screen in
	// between. Edges closes the circle -- only where the desktop really ends,
	// so the edge that leads to this Mac's own panel still leads there.
	//
	// Before follow, so that the band catches up with the arrival in the same
	// frame rather than the next one.
	// The desk's own screen, by id: the glasses sit beside the ribbon, and a
	// pointer that walks onto them is on a picture of this program.
	owned, _ := OwnDisplay(opt.Screen.Name)
	edges := &Edges{Own: owned}
	wrap := func(now time.Time) {
		moved, err := edges.Step(now, showing())
		if err != nil {
			// Once a frame; only worth a line when it says something new.
			if !errors.Is(err, ErrPointerLost) {
				logf("wrapping the pointer: %v", err)
			}
			return
		}
		if moved {
			logf("the pointer came back at the other end of the band")
		}
	}

	// AND IT GOES AND GETS THE SCREEN THE POINTER WENT TO.
	//
	// Following ends where the ribbon does. A pointer that leaves every screen
	// a position is showing -- onto this Mac's own panel, most often, which
	// sits right beside the ribbon -- is invisible to somebody wearing the
	// glasses. They have not lost the mouse exactly: they have lost the screen
	// it is on. So the desk fetches that screen onto the position in front of
	// them, which is also the answer to "I never saw my Mac's screen in here".
	// Read once: the seam is set before Run and the frame loop is another
	// goroutine.
	lost, onLost := &Lost{}, d.OnLost
	fetch := func(now time.Time) {
		if onLost == nil {
			return
		}
		id, ok := underPointer()
		if want := lost.Step(now, uint64(id), ok, showing(), owned); want != 0 {
			logf("the pointer is on display %d, which no screen is showing", want)
			onLost(want, d.Focus())
		}
	}
	surface := toolkit.NewSurface(v.frame)
	surface.OnInput = func(ev toolkit.Event) {
		switch ev.Kind {
		case toolkit.EventKeyDown:
			if a := KeyAction(ev.Code); a != ActionNone {
				act(d, logf, "window", a)
				sync()
			}
		case toolkit.EventClick:
			// Through the same two tables the picture was drawn with, so a
			// click cannot land somewhere the pixel under it did not come
			// from.
			if cx, cy, ok := v.canvasAt(ev.X, ev.Y); ok {
				d.Click(cx, cy)
			}
		}
	}

	// A window that can be repainted from any goroutine is what lets the loop
	// live outside the event loop. Without it the picture would only move when
	// the viewer did something, which for a desk full of other people's windows
	// is exactly backwards.
	repaint := func() {}
	if r, ok := win.(window.Repainter); ok {
		repaint = r.Repaint
	} else {
		logf("this back-end cannot be repainted from another goroutine; " +
			"the picture will only move on input")
	}

	// The global shortcuts. They work while another application has the
	// keyboard, which is the difference between a desk you use and a desk you
	// have to click on first.
	var global <-chan Action
	if opt.Shortcuts == nil {
		opt.Shortcuts = DefaultShortcuts()
	}
	if !opt.NoGlobal {
		hk := ClaimGlobal(opt.Shortcuts, opt.Hotkeys)
		defer hk.Close()
		global = hk.C()
		for _, line := range strings.Split(strings.TrimRight(hk.Describe(), "\n"), "\n") {
			if line != "" {
				logf("%s", line)
			}
		}
	}

	// And the BARE keys, claimed only while a gallery covers the view.
	//
	// A person looking at a grid should not have to hold three modifiers to walk
	// it. Claiming a bare arrow system-wide is a serious thing to do to a
	// machine, so it is done when a gallery opens and undone when it closes —
	// nobody is typing into anything while a gallery is in front of their eyes.
	var bare *Hotkeys
	bareC := make(chan Action)
	if !opt.NoGlobal {
		sync = func() {
			switch {
			case d.InGallery() && bare == nil:
				bare = ClaimGallery()
				go func(h *Hotkeys) {
					for a := range h.C() {
						bareC <- a
					}
				}(bare)
				// SAY WHAT WAS ACTUALLY CLAIMED.
				//
				// This line used to announce the bare keys whether or not the
				// machine had granted a single one -- and on a session where
				// every one of them was refused, it said they were the
				// gallery's and then nothing moved. Fifty presses arrived one
				// evening and none the next, with the same sentence printed
				// both times. A claim is not a grant.
				for _, line := range strings.Split(strings.TrimRight(bare.Describe(), "\n"), "\n") {
					if line != "" {
						logf("  in the gallery: %s", line)
					}
				}
			case !d.InGallery() && bare != nil:
				bare.Close()
				bare = nil
				logf("the arrows are the machine's again")
			}
		}
		defer func() {
			if bare != nil {
				bare.Close()
			}
		}()
	}

	// A PULSE, so a picture that has stopped moving says so.
	//
	// "l'app a bugué" and a log full of shortcuts that arrived is not enough to
	// tell a frozen picture from a key nobody pressed. Frames drawn are the one
	// number that separates them, and a desk drawing sixty a second says it in a
	// line every five.
	frames, beatAt := 0, time.Now()
	// shapes is the width each screen had at the last beat, so a screen that
	// takes the shape of what it shows is reported once rather than every frame.
	var shapes []int
	beat := func(now time.Time) {
		frames++
		since := now.Sub(beatAt)
		if since >= 5*time.Second {
			logf("%d frames in %v", frames, since.Round(time.Millisecond))
			frames, beatAt = 0, now
			// AND THE SCREEN IT IS ON IS STILL THERE.
			//
			// Unplug the glasses and this program is drawing for nobody, on a
			// window the window server has moved somewhere else -- while it
			// holds this Mac's backlight off and six displays that do not
			// exist. "j'ai debranche les lunettes car j'avais perdu l'acces et
			// il n'y avait pas l'icon dans la tray pour que je coupe
			// l'application": the icon was there, on a panel that had been
			// turned off. So the desk stops itself, and stopping is what puts
			// everything back.
			// And a screen that has taken the shape of what it shows says so.
			// Without this the only evidence that it worked is a picture
			// somebody has to be wearing the glasses to see.
			for i := 0; i < d.Plan().Count(); i++ {
				w := d.Plan().ScreenWidth(i)
				if i < len(shapes) && shapes[i] == w {
					continue
				}
				for len(shapes) <= i {
					shapes = append(shapes, 0)
				}
				if shapes[i] != 0 {
					logf("screen %d is now %dx%d, the shape of what it shows",
						i+1, w, d.Plan().ScreenH)
				}
				shapes[i] = w
			}
			if _, err := currentScreen(opt.Screen.Name); err != nil {
				logf("%v -- stopping, and putting back everything this changed", err)
				d.Do(ActionQuit)
			}
		}
	}

	stop := make(chan struct{})
	done := make(chan struct{})
	// crashed is what the loop panicked with, if it did. A panic in here is a
	// panic in another goroutine: left alone it kills the process without
	// running the deferred calls that put this Mac's backlight back and take the
	// virtual displays away. See Safely.
	var crashed error
	go func() {
		defer close(done)
		crashed = Safely(logf, func() {
			t := time.NewTicker(FrameInterval)
			defer t.Stop()
			var deadline <-chan time.Time
			if opt.For > 0 {
				timer := time.NewTimer(opt.For)
				defer timer.Stop()
				deadline = timer.C
			}
			last := time.Now()
			for {
				select {
				case <-stop:
					return
				case <-ctx.Done():
					d.Do(ActionQuit)
				case <-deadline:
					d.Do(ActionQuit)
				case a := <-global:
					act(d, logf, "shortcut", a)
					sync()
				case a := <-bareC:
					act(d, logf, "gallery key", a)
					sync()
				case a := <-opt.Actions:
					act(d, logf, "menu bar", a)
					sync()
				case now := <-t.C:
					dt := now.Sub(last).Seconds()
					last = now
					wrap(now)
					follow()
					fetch(now)
					d.Advance(dt)
					v.draw(d.Render())
					repaint()
					beat(now)
				}
				if d.Quit() {
					_ = win.Close()
					return
				}
			}
		})
		// However it ended, the window has to stop or the run never returns
		// and nothing it changed goes back.
		_ = win.Close()
	}()

	err = win.Run(surface)
	close(stop)
	<-done
	if crashed != nil {
		return crashed
	}
	return err
}

// currentScreen finds a display by name, right now.
//
// Matching by NAME rather than by the rectangle a caller saw earlier is what
// survives the desktop being rearranged underneath — which creating a virtual
// display does, every time.
//
// An earlier version of this waited until two readings of the arrangement came
// back identical, on the theory that the desktop was still moving. It settled
// INSTANTLY on a false answer, every time, and that is worth recording: on macOS
// `+[NSScreen screens]` was a cache that a process with no running NSApplication
// never saw refreshed, so every reading agreed with every other because they all
// came from the same stale snapshot. A heuristic that waits for agreement cannot
// tell a settled arrangement from a frozen one. go-widgets/window v0.51.0 reads
// the geometry from the window server instead, so one reading is now the truth.
func currentScreen(name string) (*window.Screen, error) {
	ss, err := window.Screens()
	if err != nil {
		return nil, fmt.Errorf("desk: cannot list displays: %w", err)
	}
	for i := range ss {
		if ss[i].Name == name {
			return &ss[i], nil
		}
	}
	names := make([]string, len(ss))
	for i, s := range ss {
		names[i] = strconv.Quote(s.Name)
	}
	return nil, fmt.Errorf("desk: %q is not attached any more; there is %s",
		name, strings.Join(names, ", "))
}

// act carries out an action and SAYS SO.
//
// A desk driven entirely by system-wide shortcuts is a program a person cannot
// see working: press a key, and either something happens on a pair of glasses or
// nothing does, with no way to tell "the key never arrived" from "the key
// arrived and the thing it asked for failed". Both were reported on the same
// evening, and neither could be told from the other without this.
//
// Refusals are printed too. Do records them in Err rather than returning them —
// the gallery has directions it has no cell for — and a refusal nobody can see
// is a key that silently does nothing.
func act(d *Desk, logf func(string, ...any), from string, a Action) {
	d.Do(a)
	if err := d.Err(); err != nil {
		logf("%s: %v — %v", from, a, err)
		return
	}
	logf("%s: %v", from, a)
}
