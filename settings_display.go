// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"fmt"

	"github.com/go-widgets/toolkit"
	"github.com/go-widgets/window"
	"github.com/go-xrkit/xrkit/glasses"
)

// SettingsOptions are the choices a caller makes about the settings window.
type SettingsOptions struct {
	// Attached are the headsets to choose between. Nil asks the bus.
	Attached []glasses.USB
	// Logf receives progress. A nil Logf says nothing.
	Logf func(string, ...any)
}

// screenCounts are the sizes of desk the window offers.
//
// Three, six and nine first, because those are the ones that fold into a
// gallery three columns wide with nothing ragged — the shape a person keeps a
// map of.
var screenCounts = []int{3, 6, 9, 1, 2, 4, 12}

// RunSettings shows the settings window and returns when it is closed.
//
// It is a window of widgets and not a drawn picture: every control here is a
// toolkit widget, so it is navigable by keyboard, readable by an accessibility
// tree, and themed like everything else on the machine.
func RunSettings(opt SettingsOptions) error {
	logf := opt.Logf
	if logf == nil {
		logf = func(string, ...any) {}
	}
	cfg, err := LoadConfig()
	if err != nil {
		// Settings that will not load are exactly when a person needs this
		// window, so it opens on the defaults rather than refusing.
		logf("%v", err)
		logf("opening on the defaults; saving will replace that file")
		cfg = Config{}
	}
	attached := opt.Attached
	if attached == nil {
		attached = Peripherals()
	}

	win, err := window.Open(window.Config{
		Title:  "xrdesk settings",
		Width:  520,
		Height: 400,
		Theme:  toolkit.DefaultDark(),
	})
	if err != nil {
		return fmt.Errorf("desk: cannot open the settings window: %w", err)
	}
	defer win.Close()

	root, apply := settingsRoot(&cfg, attached, logf, func() { _ = win.Close() })
	_ = apply
	return win.Run(root)
}
