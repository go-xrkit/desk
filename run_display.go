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
		Theme:       toolkit.DefaultDark(),
	})
	if err != nil {
		return fmt.Errorf("desk: cannot open a window on %s: %w", opt.Screen, err)
	}
	defer win.Close()

	fbW, fbH := win.Size()
	logf("framebuffer %dx%d", fbW, fbH)
	v, err := newView(plan, fbW, fbH)
	if err != nil {
		return err
	}
	logf("the panorama reaches %.1f%% of the view", 100*v.Coverage)
	v.Snapshot = opt.Snapshot

	surface := toolkit.NewSurface(v.frame)
	surface.OnInput = func(ev toolkit.Event) {
		if ev.Kind != toolkit.EventKeyDown {
			return
		}
		if a := KeyAction(ev.Code); a != ActionNone {
			d.Do(a)
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
