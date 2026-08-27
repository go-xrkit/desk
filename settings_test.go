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
	// One list -- the glasses -- and one drop-down for the screen count.
	var lists []*toolkit.ListBox
	var count *toolkit.DropDown
	var check *toolkit.CheckButton
	// Down the tree, because the controls are inside form fields: the window a
	// person sees is what is at the leaves, not what the root holds.
	var walk func(toolkit.Widget)
	walk = func(w toolkit.Widget) {
		switch v := w.(type) {
		case *toolkit.ListBox:
			lists = append(lists, v)
		case *toolkit.DropDown:
			count = v
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
	if len(lists) != 1 || count == nil || check == nil {
		t.Fatalf("the window has %d lists, count=%v, check=%v; want the glasses, "+
			"the screen count and the menu bar", len(lists), count != nil, check != nil)
	}
	lists[0].Selected().Set(1) // the second headset
	count.Select(2)            // the third screen count, as a click would
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
			// The label is an Observable now: the toolkit made every widget's
			// reactive state one, which is the rule this fleet asked for.
			out[b.Label().Get()] = b
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

// TestWhenToAskWhichGlasses.
//
// Asking is for the case where nobody has decided and the machine cannot.
// Asking when the answer is already written down would be a window in the way;
// NOT asking when two headsets are attached would be a guess.
func TestWhenToAskWhichGlasses(t *testing.T) {
	chosen := Config{Glasses: &ConfigGlasses{Model: ptr("VITURE Luma Ultra")}}
	brandOnly := glasses.USB{Vendor: 0x3318, Product: 0xffff, Name: "XREAL Something"}
	for name, tc := range map[string]struct {
		cfg      Config
		screen   string
		attached []glasses.USB
		want     bool
	}{
		"two attached and nobody has said":  {Config{}, "", []glasses.USB{oneS, luma}, true},
		"a display named on the line":       {Config{}, "VITURE", []glasses.USB{oneS, luma}, false},
		"a display named with spaces":       {Config{}, "  VITURE ", []glasses.USB{oneS, luma}, false},
		"already written in the settings":   {chosen, "", []glasses.USB{oneS, luma}, false},
		"only one attached":                 {Config{}, "", []glasses.USB{luma}, false},
		"none attached":                     {Config{}, "", nil, false},
		"two on the bus but one is a brand": {Config{}, "", []glasses.USB{brandOnly, luma}, false},
	} {
		if got := ShouldChoose(tc.cfg, tc.screen, tc.attached); got != tc.want {
			t.Errorf("%s: ShouldChoose = %v, want %v", name, got, tc.want)
		}
	}
}

// TestSettingsScale covers every answer the magnification gives, including the
// two it refuses to go past.
//
// The interesting part is not the arithmetic, it is the two clamps. Below a
// 720-row display the window is left alone: a 1366x768 laptop is what the
// window was drawn for, and a 600-row panel is one where making the type BIGGER
// is how the buttons fall off the bottom. Above four the window stops being a
// window: 4x on this Mac's 2160-row panel is already a dialogue 2240 pixels
// tall, and a person on a taller one would rather have a small dialogue than
// one they have to scroll.
func TestSettingsScale(t *testing.T) {
	for _, c := range []struct {
		what string
		h    int
		want float64
	}{
		{"nothing known about the display", 0, 1},
		{"a small panel is left alone", 600, 1},
		{"the size it was drawn for", 720, 1},
		{"a 1440p display", 1440, 2},
		{"the 8K panel this was reported on", 2160, 3},
		{"exactly at the clamp", 2880, 4},
		{"past the clamp", 4320, 4},
	} {
		if got := SettingsScale(c.h); got != c.want {
			t.Errorf("%s (%d rows): scale %.2f, want %.2f", c.what, c.h, got, c.want)
		}
	}

	// The scale is monotonic: a taller display never gets SMALLER type. Nothing
	// in the two clamps may invert the order between them.
	last := 0.0
	for h := 0; h <= 6000; h += 37 {
		if s := SettingsScale(h); s < last {
			t.Fatalf("%d rows scaled %.2f after %.2f: taller and smaller", h, s, last)
		} else {
			last = s
		}
	}
}

// TestTheScreenCountCanBeChosenWithTheMouse is the defect a person reported as
// "la combobox n'est pas fonctionelle": the control opened and nothing could be
// selected.
//
// It goes through settingsSurface, which is what the window is given, and drives
// the widgets the way a mouse does -- open the list, click the third row -- so
// it fails if the wrapper is ever dropped again. The negative control clicks the
// identical point on the bare form, where nothing happens.
func TestTheScreenCountCanBeChosenWithTheMouse(t *testing.T) {
	pick := func(surface func(toolkit.Widget) toolkit.Widget) int {
		cfg := &Config{}
		root, read := settingsRoot(cfg, []glasses.USB{oneS, luma}, nil, func() {})
		top := surface(root)
		top.SetBounds(toolkit.Rect{X: 0, Y: 0, W: settingsW, H: settingsH(*cfg, nil)})

		var count *toolkit.DropDown
		var walk func(toolkit.Widget)
		walk = func(w toolkit.Widget) {
			if d, ok := w.(*toolkit.DropDown); ok {
				count = d
			}
			if p, ok := w.(interface{ Children() []toolkit.Widget }); ok {
				for _, kid := range p.Children() {
					walk(kid)
				}
			}
		}
		walk(top)
		if count == nil {
			t.Fatal("the window has no drop-down for the screen count")
		}

		// Click the control to open it, then the third row of the list it puts up.
		b := count.Bounds()
		top.OnEvent(toolkit.Event{Kind: toolkit.EventClick, X: b.X + 4, Y: b.Y + b.H/2})
		if !count.Open().Get() {
			t.Fatal("clicking the control did not open the list")
		}
		pb := count.PopoverBounds()
		row := pb.H / len(count.Options)
		top.OnEvent(toolkit.Event{
			Kind: toolkit.EventClick,
			X:    pb.X + 4,
			Y:    pb.Y + 2*row + row/2,
		})
		read()
		return cfg.Screens()
	}

	if got, want := pick(settingsSurface), screenCounts[2]; got != want {
		t.Errorf("clicking the third option chose %d screens, want %d", got, want)
	}
	bare := func(w toolkit.Widget) toolkit.Widget { return w }
	if got := pick(bare); got == screenCounts[2] {
		t.Error("negative control: the bare form routed the click on its own, so " +
			"this test no longer proves the popover host is needed")
	}
}

// TestFitScale: the magnification has to give way to the display.
//
// Legibility asks for one number and the screen allows another. What made this
// worth a function of its own is that getting it wrong is silent: a window
// magnified past its display keeps its Save button below the frame, so the
// settings cannot be saved and nothing on screen says why.
func TestFitScale(t *testing.T) {
	// A window whose size is exactly proportional to the scale.
	linear := func(w, h int) func(float64) (int, int) {
		return func(s float64) (int, int) {
			return int(float64(w) * s), int(float64(h) * s)
		}
	}

	// It fits at the scale asked for: nothing to do.
	if got := FitScale(3, 2000, 2000, linear(560, 570)); got != 3 {
		t.Errorf("a window that fits was still shrunk to %.2f", got)
	}
	// Too tall: comes back at or under the room available.
	got := FitScale(3, 4000, 1000, linear(560, 570))
	if got >= 3 {
		t.Errorf("a window three times too tall kept its scale %.2f", got)
	}
	if _, h := linear(560, 570)(got); h > 1000 {
		t.Errorf("at scale %.2f the window is %d tall, past the 1000 available", got, h)
	}
	// Too wide, with all the height in the world.
	got = FitScale(4, 800, 0, linear(560, 570))
	if w, _ := linear(560, 570)(got); w > 800 {
		t.Errorf("at scale %.2f the window is %d wide, past the 800 available", got, w)
	}
	// A display too small for even the unmagnified window: it stops at 1 rather
	// than shrinking the type into illegibility.
	if got := FitScale(3, 10, 10, linear(560, 570)); got != 1 {
		t.Errorf("an impossible display gave scale %.2f, want 1", got)
	}
	// No display known: no constraint, so legibility wins outright.
	if got := FitScale(3, 0, 0, linear(560, 570)); got != 3 {
		t.Errorf("with no room given the scale became %.2f", got)
	}
	// Below 1 is not a magnification anybody asked for.
	if got := FitScale(0.25, 0, 0, linear(560, 570)); got != 1 {
		t.Errorf("a scale under 1 was honoured: %.2f", got)
	}
	// A size that does not shrink with the scale still terminates, on the
	// fallback decrement, instead of looping.
	stuck := func(float64) (int, int) { return 5000, 5000 }
	if got := FitScale(3, 100, 100, stuck); got < 1 || got > 3 {
		t.Errorf("a window that ignores the scale gave %.2f", got)
	}

	// And a size that will not fit within the rounds allowed comes back with
	// the best candidate reached rather than looping forever: the search is
	// bounded because it runs while a person waits for a window.
	constant := func(float64) (int, int) { return 0, 400 }
	if got := FitScale(3, 0, 300, constant); got < 1 || got >= 3 {
		t.Errorf("the bounded search gave %.2f", got)
	}
}
