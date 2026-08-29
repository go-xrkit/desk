// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"context"
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
				logf("the arrows, Enter and Escape are the gallery's for as long as it is up")
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

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
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
				d.Advance(dt)
				v.draw(d.Render())
				repaint()
			}
			if d.Quit() {
				_ = win.Close()
				return
			}
		}
	}()

	err = win.Run(surface)
	close(stop)
	<-done
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
