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
var screenCounts = []int{3, 6, 9, 1, 2, 4}

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

	// The display the window will appear on, and the room it has there.
	//
	// Not the tallest attached: the tallest is where the type wants to be big,
	// and the two are only the same display by luck. A window magnified for a
	// 2160-row panel and placed by the platform on a 1200-row one has its Save
	// button below the frame, so the settings cannot be saved and nothing says
	// why. So the display is CHOSEN, passed to the back-end, and the
	// magnification is decided against that display's usable area.
	target := settingsScreen(logf)
	maxW, maxH := 0, 0
	want := 1.0
	if target != nil {
		maxW = int(float64(target.VisibleWidth) * SettingsRoom)
		maxH = int(float64(target.VisibleHeight) * SettingsRoom)
		want = SettingsScale(target.Height)
	}

	// The system face, before anything is measured.
	//
	// The built-in font is a 5x7 bitmap: magnified for an 8K panel it is legible
	// and it is blocky, which is what "not legible enough" was about. A real
	// face at a real size is the answer, and the platform already has one.
	//
	// It has to be installed BEFORE the size is computed, because the font is
	// what decides how tall a label row is: window.SystemFontTTF is
	// package-level for exactly that, so no window need exist yet. And a host
	// that chooses a font chooses its size too, so the size is computed here
	// instead of being left to the metric scale.
	ttf, ferr := window.SystemFontTTF()
	if ferr != nil {
		logf("no system font to use, keeping the built-in one: %v", ferr)
	}
	was := toolkit.MetricScale()
	defer toolkit.SetMetricScale(was)
	wasFont := toolkit.CurrentFont()
	defer toolkit.SetFont(wasFont)

	// size installs a candidate magnification with its matching face and reports
	// what the window would then be, in the pixels the tree is laid out in. It
	// is what FitScale searches over, and it leaves the winning scale installed.
	size := func(scale float64) (int, int) {
		toolkit.SetMetricScale(scale)
		toolkit.SetFont(wasFont)
		if ferr == nil {
			if f, err := toolkit.NewTrueTypeFont(ttf, toolkit.Scaled(SettingsFontPx)); err != nil {
				logf("the system face would not parse, keeping the built-in one: %v", err)
				ferr = err
			} else {
				toolkit.SetFont(f)
			}
		}
		return settingsSize(cfg, attached)
	}
	scale := FitScale(want, maxW, maxH, size)
	w, h := size(scale)
	logf("the settings window is %dx%d pixels at scale %.2f, in %s",
		w, h, scale, faceName(ferr))

	// Width and Height are LOGICAL points and the back-end allocates one
	// framebuffer pixel per point unless RenderScale says otherwise -- measured,
	// not assumed: a window asked for 560x570 comes back reporting exactly
	// 560x570. So the tree's own pixel size is what goes in. Dividing by the
	// metric scale, as this did once, asks for a window three times too small
	// for the widgets it is about to be given, and every control spills out of
	// the frame.
	win, err := window.Open(window.Config{
		Title:  "xrdesk settings",
		Width:  w,
		Height: h,
		Screen: target,
		Theme:  toolkit.DefaultDark(),
	})
	if err != nil {
		return fmt.Errorf("desk: cannot open the settings window: %w", err)
	}
	defer win.Close()

	root, apply := settingsRoot(&cfg, attached, logf, func() { _ = win.Close() })
	_ = apply
	// Wrapped in a popover host, which is what makes the drop-down work: a
	// DropDown draws its closed face and hands the option list to whoever owns
	// the surface. Without this the list opens onto nothing.
	return win.Run(settingsSurface(root))
}

// faceName says which font the window ended up in, for the log line.
func faceName(err error) string {
	if err != nil {
		return "the built-in bitmap face"
	}
	return "the system face"
}

// settingsScreen picks the display to put the settings window on: the largest
// attached, by area.
//
// Largest rather than primary. This window is shown before the glasses are
// chosen, so it appears on the desktop, and the desktop's own display may be a
// laptop panel with a 7680-pixel monitor beside it. The big screen is where
// there is room for a magnified dialogue and where a person reading it is
// looking.
//
// A platform that cannot enumerate displays gets a nil, which Config.Screen
// documents as "let the platform choose" -- the behaviour before any of this.
func settingsScreen(logf func(string, ...any)) *window.Screen {
	ss, err := window.Screens()
	if err != nil {
		logf("cannot enumerate the displays, letting the platform place the window: %v", err)
		return nil
	}
	var best *window.Screen
	for i := range ss {
		if best == nil || ss[i].Width*ss[i].Height > best.Width*best.Height {
			best = &ss[i]
		}
	}
	if best != nil {
		logf("the settings window goes on %q, %dx%d with %dx%d usable",
			best.Name, best.Width, best.Height, best.VisibleWidth, best.VisibleHeight)
	}
	return best
}
