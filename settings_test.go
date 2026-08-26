// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-widgets/toolkit"
	"github.com/go-xrkit/xrkit/glasses"
)

// The window the settings open at.
var settingsW = SettingsWidth

func settingsH(cfg Config, attached []glasses.USB) int { return settingsHeight(cfg, attached) }

// arranged builds the settings tree, gives it a window's worth of space, and
// returns every leaf's rectangle with a name for it.
//
// This is how a layout is checked without a person looking at one. A widget's
// bounds are the layout's whole output, and they are computed with no display,
// no window server and no pixels — so the thing that decides whether a window
// reads as a form or as a row of slivers is a pure function this can call.
func arranged(t *testing.T, cfg *Config, attached []glasses.USB) ([]toolkit.Rect, int) {
	t.Helper()
	root, _ := settingsRoot(cfg, attached, nil, func() {})
	root.SetBounds(toolkit.Rect{X: 0, Y: 0, W: settingsW, H: settingsH(*cfg, attached)})
	// Past the margin: the root is the padding, and the rows are in the column
	// it holds. Reading the padding's own children would report four empty
	// spacers and one very large box, which is true and says nothing.
	c, ok := root.(*toolkit.Container)
	if !ok {
		t.Fatalf("the root is a %T, not a container", root)
	}
	// Down to the column of fields: the root is the margin, which holds the
	// frame, which holds a scroll view and the buttons.
	var column *toolkit.Container
	var walk func(toolkit.Widget)
	walk = func(w toolkit.Widget) {
		if inner, ok := w.(*toolkit.Container); ok {
			for _, kid := range inner.Children() {
				if _, isField := kid.(*toolkit.FormField); isField {
					column = inner
					return
				}
			}
		}
		if p, ok := w.(interface{ Children() []toolkit.Widget }); ok {
			for _, kid := range p.Children() {
				if column == nil {
					walk(kid)
				}
			}
		}
	}
	walk(c)
	if column == nil {
		t.Fatal("the window holds no column of fields")
	}
	// The spacer that keeps the buttons at the bottom is not a control and has
	// nothing to read; it holds whatever height is left over, which is exactly
	// what the assertions below say a ROW must not do.
	var out []toolkit.Rect
	for _, w := range column.Children() {
		if c, ok := w.(*toolkit.Container); ok && len(c.Children()) == 0 {
			continue
		}
		out = append(out, w.Bounds())
	}
	return out, settingsH(*cfg, attached)
}

// TestTheSettingsAreAColumnAndNotARow.
//
// A BoxLayout is HORIZONTAL by default and gives an item with no config an
// EQUAL FLEX SHARE. The first version of this window used one of those with
// AddWidget for everything, so eleven controls were laid out side by side, each
// a tenth of the window wide. It was reported — accurately — as looking like
// nothing, by a person who had to open it to find out.
//
// So: the rows go DOWN the window, they do not overlap, none is a sliver, and
// every one of them is inside the window.
func TestTheSettingsAreAColumnAndNotARow(t *testing.T) {
	cfg := &Config{}
	rects, height := arranged(t, cfg, []glasses.USB{oneS, luma})
	if len(rects) < 4 {
		t.Fatalf("the column has %d rows; it should have the glasses, the screen "+
			"count, the menu bar and the shortcuts", len(rects))
	}
	for i, r := range rects {
		if r.W <= 0 || r.H <= 0 {
			t.Errorf("row %d has no size: %+v", i, r)
			continue
		}
		if r.X < 0 || r.Y < 0 || r.X+r.W > settingsW || r.Y+r.H > height {
			t.Errorf("row %d is outside the window: %+v", i, r)
		}
		// And inside the margin, not flush against the frame: text starting at a
		// window's own edge reads as spilled rather than laid out.
		if r.X < 8 {
			t.Errorf("row %d starts at x=%d, against the window frame", i, r.X)
		}
		// A row spans the window. Anything much narrower is the horizontal
		// layout coming back.
		if r.W < settingsW/2 {
			t.Errorf("row %d is %d wide in a %d window; the rows are side by side "+
				"again", i, r.W, settingsW)
		}
		if r.H < 20 {
			t.Errorf("row %d is %d tall, which is not enough for a control", i, r.H)
		}
		if i > 0 {
			prev := rects[i-1]
			if r.Y < prev.Y+prev.H {
				t.Errorf("row %d starts at y=%d, inside row %d which ends at y=%d",
					i, r.Y, i-1, prev.Y+prev.H)
			}
		}
	}
}

// TestTheGlassesRowIsSizedToItsHeadsets: it is a list of what is attached, and
// a list of two stretched over half the window is a large empty box with its
// label stranded at the top. Everything else here is a control of a known
// height.
func TestTheGlassesRowIsSizedToItsHeadsets(t *testing.T) {
	two, _ := arranged(t, &Config{}, []glasses.USB{oneS, luma})
	none, _ := arranged(t, &Config{}, nil)
	if len(two) != len(none) {
		t.Fatalf("%d rows with glasses, %d without", len(two), len(none))
	}
	if two[0].H <= none[0].H {
		t.Errorf("the glasses row is %d tall with two headsets and %d with none",
			two[0].H, none[0].H)
	}
	if got, want := two[0].H, fieldH(2); got != want {
		t.Errorf("two headsets give a row %d tall, want %d", got, want)
	}
	for i := 1; i < len(two); i++ {
		if two[i].H != none[i].H {
			t.Errorf("row %d changed height (%d then %d) because the list did",
				i, two[i].H, none[i].H)
		}
	}
	// And a field is never shorter than one row, whatever it is asked for.
	if fieldH(0) != fieldH(1) || fieldH(-3) != fieldH(1) {
		t.Error("a field with nothing in it is not one row tall")
	}
}

// TestTheSettingsWindowReadsItsControlsBack is the other half: what the person
// chose has to arrive in the file.
func TestTheSettingsWindowReadsItsControlsBack(t *testing.T) {
	cfg := &Config{}
	root, read := settingsRoot(cfg, []glasses.USB{oneS, luma}, nil, func() {})
	root.SetBounds(toolkit.Rect{X: 0, Y: 0, W: settingsW, H: settingsH(*cfg, nil)})

	// Pick the second headset, the third screen count, and turn the menu bar
	// covering off — through the widgets, the way a person would.
	c := root.(*toolkit.Container)
	var list *toolkit.ListBox
	var drop *toolkit.DropDown
	var check *toolkit.CheckButton
	// Down the tree, because the controls are inside form fields: the window a
	// person sees is what is at the leaves, not what the root holds.
	var walk func(toolkit.Widget)
	walk = func(w toolkit.Widget) {
		switch v := w.(type) {
		case *toolkit.ListBox:
			list = v
		case *toolkit.DropDown:
			drop = v
		case *toolkit.CheckButton:
			check = v
		}
		if p, ok := w.(interface{ Children() []toolkit.Widget }); ok {
			for _, kid := range p.Children() {
				walk(kid)
			}
		}
	}
	walk(root)
	_ = c
	if list == nil || drop == nil || check == nil {
		t.Fatalf("the window is missing a control: list=%v drop=%v check=%v",
			list != nil, drop != nil, check != nil)
	}
	list.Selected().Set(1)
	drop.Selected().Set(2)
	check.Checked().Set(false)

	read()
	if got := cfg.Model(); got != "VITURE Luma Ultra" {
		t.Errorf("model is %q, want the one that was selected", got)
	}
	if got, want := cfg.Screens(), screenCounts[2]; got != want {
		t.Errorf("screens is %d, want %d", got, want)
	}
	if cfg.Immersive() {
		t.Error("the menu bar covering was left on")
	}
	// And what it produced must be settings the application will load.
	if err := cfg.check(); err != nil {
		t.Errorf("the window produced settings that will not load: %v", err)
	}
}

// buttonsOf finds the window's buttons by label.
func buttonsOf(root toolkit.Widget) map[string]*toolkit.Button {
	out := map[string]*toolkit.Button{}
	var walk func(toolkit.Widget)
	walk = func(w toolkit.Widget) {
		if b, ok := w.(*toolkit.Button); ok {
			out[b.Label] = b
		}
		if p, ok := w.(interface{ Children() []toolkit.Widget }); ok {
			for _, kid := range p.Children() {
				walk(kid)
			}
		}
	}
	walk(root)
	return out
}

// TestTheSettingsWindowOpensOnWhatIsAlreadyChosen: a person opening it to
// change one thing must not have to re-pick the others.
func TestTheSettingsWindowOpensOnWhatIsAlreadyChosen(t *testing.T) {
	cfg := &Config{
		Ribbon:  &ConfigRibbon{Screens: ptr(9)},
		Glasses: &ConfigGlasses{Model: ptr("VITURE Luma Ultra")},
	}
	root, read := settingsRoot(cfg, []glasses.USB{oneS, luma}, nil, func() {})
	root.SetBounds(toolkit.Rect{X: 0, Y: 0, W: settingsW, H: settingsH(*cfg, nil)})
	// Read straight back without touching anything: what was there is what
	// comes out.
	read()
	if got := cfg.Model(); got != "VITURE Luma Ultra" {
		t.Errorf("the glasses row opened on %q", got)
	}
	if got := cfg.Screens(); got != 9 {
		t.Errorf("the screen count opened on %d, want 9", got)
	}
}

// TestSaveAndCloseAreWiredUp, without a window server: the buttons are widgets
// and their handlers are functions, so pressing one is calling it.
func TestSaveAndCloseAreWiredUp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "desk.hcl")
	t.Setenv(EnvConfig, path)

	closed := 0
	cfg := &Config{}
	root, _ := settingsRoot(cfg, []glasses.USB{luma}, nil, func() { closed++ })
	root.SetBounds(toolkit.Rect{X: 0, Y: 0, W: settingsW, H: settingsH(*cfg, nil)})
	b := buttonsOf(root)
	if b["Save"] == nil || b["Close"] == nil {
		t.Fatalf("the window has buttons %v", b)
	}

	b["Save"].OnClick()
	if closed != 1 {
		t.Errorf("Save left the window open")
	}
	got, err := LoadConfigFile(path)
	if err != nil {
		t.Fatalf("what Save wrote does not load: %v", err)
	}
	if got.Model() != "VITURE Luma Ultra" || got.Screens() != screenCounts[0] {
		t.Errorf("Save wrote %q, %d screens", got.Model(), got.Screens())
	}

	b["Close"].OnClick()
	if closed != 2 {
		t.Error("Close did not close the window")
	}
}

// TestSaveSaysSoWhenItCannot rather than closing on a file that was not
// written: the one thing worse than losing a setting is being told it was kept.
func TestSaveSaysSoWhenItCannot(t *testing.T) {
	t.Setenv(EnvConfig, "")
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("AppData", "")

	closed := 0
	var said []string
	cfg := &Config{}
	root, _ := settingsRoot(cfg, nil, func(f string, a ...any) {
		said = append(said, fmt.Sprintf(f, a...))
	}, func() { closed++ })
	root.SetBounds(toolkit.Rect{X: 0, Y: 0, W: settingsW, H: settingsH(*cfg, nil)})

	buttonsOf(root)["Save"].OnClick()
	if closed != 0 {
		t.Error("the window closed on a save that did not happen")
	}
	if len(said) == 0 || !strings.Contains(strings.Join(said, "\n"), "settings") {
		t.Errorf("nothing was said about the failure: %v", said)
	}
}

// TestOnlyModelsAreOfferedAsGlasses: an entry no product id names is a BRAND,
// and a brand is not a headset to choose.
func TestOnlyModelsAreOfferedAsGlasses(t *testing.T) {
	brandOnly := glasses.USB{Vendor: 0x3318, Product: 0xffff, Name: "XREAL Something"}
	got := headsetNames([]glasses.USB{brandOnly, luma})
	if len(got) != 1 || got[0] != "VITURE Luma Ultra" {
		t.Errorf("headsetNames = %v, want only the one a product id names", got)
	}
}

// TestTheWindowIsTallEnoughForWhateverItHasToSay.
//
// How much this window has to say depends on the machine. One that grants no
// global shortcut at all reports three long refusals instead of three short
// grants, and a window sized for the short version puts its Save button below
// the frame — which is what CI found on Linux, where nothing can be claimed.
func TestTheWindowIsTallEnoughForWhateverItHasToSay(t *testing.T) {
	for name, attached := range map[string][]glasses.USB{
		"two headsets": {oneS, luma},
		"none":         nil,
	} {
		cfg := &Config{}
		h := settingsH(*cfg, attached)
		root, _ := settingsRoot(cfg, attached, nil, func() {})
		root.SetBounds(toolkit.Rect{X: 0, Y: 0, W: settingsW, H: h})

		// Every leaf inside the frame, buttons included.
		var deepest int
		var walk func(toolkit.Widget)
		walk = func(w toolkit.Widget) {
			if b := w.Bounds(); b.Y+b.H > deepest {
				deepest = b.Y + b.H
			}
			if p, ok := w.(interface{ Children() []toolkit.Widget }); ok {
				for _, kid := range p.Children() {
					walk(kid)
				}
			}
		}
		walk(root)
		if deepest > h {
			t.Errorf("%s: the window is %d tall and something reaches %d",
				name, h, deepest)
		}
		// And the buttons really are in it.
		b := buttonsOf(root)
		for _, label := range []string{"Save", "Close"} {
			r := b[label].Bounds()
			if r.H <= 0 || r.Y+r.H > h {
				t.Errorf("%s: %s is at %+v in a window %d tall", name, label, r, h)
			}
		}
	}
}
