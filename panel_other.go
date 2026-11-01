// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build !android

package desk

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-widgets/toolkit"
	"github.com/go-widgets/window"
)

// windowPanel shows the ribbon in a full-screen window on the chosen display.
type windowPanel struct {
	win     window.Backend
	repaint func()
}

// openPanel opens a full-screen window on the display the run chose.
func openPanel(_ context.Context, opt RunOptions, title string,
	logf func(string, ...any), damage *damagedNames) (panel, error) {

	// Ask the window system for the target display NOW, rather than trusting
	// the one the caller looked up earlier.
	//
	// Creating virtual displays REARRANGES the desktop: on the first live run a
	// VITURE Beast sat at -1920,0 before three screens were made and at
	// -3840,0 afterwards, so the frame captured beforehand matched nothing and
	// the window refused to open with "the requested screen is no longer
	// attached". The name survives that; a rectangle does not.
	screen, err := currentScreen(opt.Screen.Name)
	if err != nil {
		// The same distinction the watch makes: a name damaged in memory still
		// names a display that is right there, and refusing to start on it
		// would be refusing over a bug in a string.
		var damaged errDamagedName
		if !errors.As(err, &damaged) {
			return nil, err
		}
		if s := damage.report(time.Now(), damaged.want, damaged.got); s != "" {
			logf("%s", s)
		}
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
		// A picture, not something to work in: the keyboard and the pointer
		// belong to the applications the desk is showing.
		Passive: !opt.Interactive,
		Theme:   toolkit.DefaultDark(),
	})
	if err != nil {
		return nil, fmt.Errorf("desk: cannot open a window on %s: %w", opt.Screen, err)
	}
	p := &windowPanel{win: win}
	// A window that can be repainted from any goroutine is what lets the loop
	// live outside the event loop. Without it the picture would only move when
	// the viewer did something, which for a desk full of other people's windows
	// is exactly backwards.
	if r, ok := win.(window.Repainter); ok {
		p.repaint = r.Repaint
	} else {
		logf("this back-end cannot be repainted from another goroutine; " +
			"the picture will only move on input")
	}
	return p, nil
}

func (p *windowPanel) Size() (int, int) { return p.win.Size() }

func (p *windowPanel) Show(frame func() ([]byte, int, int), in func(toolkit.Event)) error {
	surface := toolkit.NewSurface(frame)
	surface.OnInput = in
	return p.win.Run(surface)
}

func (p *windowPanel) Repaint() {
	if p.repaint != nil {
		p.repaint()
	}
}

func (p *windowPanel) Close() error { return p.win.Close() }

// displayStillThere reports whether the run's display is still attached.
func displayStillThere(_ context.Context, name string) error {
	_, err := currentScreen(name)
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
	all := ss.All()
	plain := make([]string, len(all))
	for i, s := range all {
		plain[i] = s.Name
	}
	// Which screen, and what an absence means -- an empty list among them. See
	// displaylist.go, which is where that decision is made and tested.
	i, err := lookUpScreen(name, plain)
	if i < 0 {
		return nil, err
	}
	return &all[i], err
}
