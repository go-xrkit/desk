// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
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
// Every window this opens and does not return is closed. An AXUIElement is a
// retained Core Foundation object, and leaking one per window per press of a
// key would be a leak nobody would find.
func (macBench) Windows(app string) ([]accessibility.Window, []string, error) {
	all, err := accessibility.AllWindows()
	if err != nil {
		return nil, nil, err
	}
	var ws []accessibility.Window
	var names []string
	for _, w := range all {
		if !matches(w.App(), app) {
			w.Close()
			continue
		}
		ws = append(ws, w)
		names = append(names, describeWindow(w))
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
