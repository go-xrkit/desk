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
	// DisplayH is the height in pixels of the display the window will appear
	// on, so its type can be magnified to the same SIZE TO LOOK AT there. Zero
	// leaves the scale alone.
	//
	// A 5x7 bitmap glyph on a 2160-row panel is three millimetres tall, which
	// is how this window came to be reported as too small on an 8K screen.
	DisplayH int

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

	// Magnify the whole window for the display it is about to appear on, and put
	// the scale back afterwards: it is global toolkit state, and the ribbon that
	// starts next draws its own badges through the same knob.
	was := toolkit.MetricScale()
	defer toolkit.SetMetricScale(was)
	if h := opt.DisplayH; h > 0 {
		toolkit.SetMetricScale(SettingsScale(h))
		logf("scaling the settings window by %.2f for a %d-row display",
			toolkit.MetricScale(), h)
	}

	// The system face, before anything is laid out.
	//
	// The built-in font is a 5x7 bitmap: magnified for an 8K panel it is legible
	// and it is blocky, which is what "not legible enough" was about. A real
	// face at a real size is the answer, and the platform already has one.
	//
	// It has to be installed BEFORE the height is computed, because the font is
	// what decides how tall a label row is: window.SystemFontTTF is
	// package-level for exactly that, so no window need exist yet. And a host
	// that chooses a font chooses its size too, so the size is computed here
	// instead of being left to the metric scale.
	if ttf, err := window.SystemFontTTF(); err != nil {
		logf("no system font to use, keeping the built-in one: %v", err)
	} else if f, err := toolkit.NewTrueTypeFont(ttf, toolkit.Scaled(SettingsFontPx)); err != nil {
		logf("the system font could not be read, keeping the built-in one: %v", err)
	} else {
		wasFont := toolkit.CurrentFont()
		toolkit.SetFont(f)
		defer toolkit.SetFont(wasFont)
		logf("the settings window is in the system face at %d pixels",
			toolkit.Scaled(SettingsFontPx))
	}

	// LOGICAL points, which is the unit Config asks for: the back-end multiplies
	// by the render scale itself. settingsHeight counts device pixels, because
	// that is what the widget tree is laid out in, so it is divided back.
	deep := settingsHeight(cfg, attached)
	win, err := window.Open(window.Config{
		Title:  "xrdesk settings",
		Width:  SettingsWidth,
		Height: int(float64(deep) / toolkit.MetricScale()),
		Theme:  toolkit.DefaultDark(),
	})
	if err != nil {
		return fmt.Errorf("desk: cannot open the settings window: %w", err)
	}
	defer win.Close()
	logf("the settings window wants %dx%d device pixels", toolkit.Scaled(SettingsWidth), deep)

	root, apply := settingsRoot(&cfg, attached, logf, func() { _ = win.Close() })
	_ = apply
	return win.Run(root)
}
