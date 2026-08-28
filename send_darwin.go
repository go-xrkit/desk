// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"fmt"

	"github.com/go-macos/accessibility"
)

// macBench is the real machine.
type macBench struct{}

// TheBench is the platform, for [Send].
//
// macOS is the only one wired up. Moving another application's window is the
// Accessibility API's job there and needs its grant; Linux has the same idea in
// each window manager's own protocol and Windows in SetWindowPos, and until
// those are written those platforms answer that they cannot.
func TheBench() Bench { return macBench{} }

func (macBench) Trusted() bool { return accessibility.Trusted() }

func (macBench) Displays() ([]accessibility.Display, error) {
	return accessibility.Displays()
}

// Windows lists the windows of every application whose name matches, and their
// human-readable names alongside — "Safari — go-xrkit/desk" rather than a
// pointer — so that what was moved can be reported to the person who asked.
//
// It asks the MATCHING APPLICATIONS, not the machine. accessibility.AllWindows
// sweeps every application and skips the ones AX declines to describe, which is
// right for "what is there" and wrong here: a person who named an application in
// their settings is owed the reason it could not be placed. Measured — Activity
// Monitor answers AXError -25204 to kAXWindows on this machine, and sweeping
// turned that into "no such application, or it has no window open", which is
// false twice over. So a refusal from an application whose name matched is
// returned as an error; only a name nothing answers to is an absence.
//
// Every window this opens and does not return is closed. An AXUIElement is a
// retained Core Foundation object, and leaking one per window per press of a
// key would be a leak nobody would find.
func (macBench) Windows(app string) ([]accessibility.Window, []string, error) {
	apps, err := accessibility.Applications()
	if err != nil {
		return nil, nil, err
	}
	var ws []accessibility.Window
	var names []string
	var refused []error
	matched := 0
	for _, a := range apps {
		if !matches(a.Name, app) {
			continue
		}
		matched++
		found, err := accessibility.WindowsOf(a.PID, a.Name)
		if err != nil {
			if errors.Is(err, accessibility.ErrNoWindow) {
				// Running with no window open. Not a refusal.
				continue
			}
			refused = append(refused, fmt.Errorf("%s (pid %d): %w", a.Name, a.PID, err))
			continue
		}
		for _, w := range found {
			ws = append(ws, w)
			names = append(names, describeWindow(w))
		}
	}
	// A refusal only matters when nothing came back: an application with one
	// window readable and one not should still have the readable one placed.
	if len(ws) == 0 && len(refused) > 0 {
		return nil, nil, errors.Join(refused...)
	}
	return ws, names, nil
}

// describeWindow names a window the way a person would.
func describeWindow(w *accessibility.AXWindow) string {
	if t := w.Title(); t != "" {
		return w.App() + " — " + t
	}
	return w.App()
}
