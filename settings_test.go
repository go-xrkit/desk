// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit"
	"github.com/go-xrkit/xrkit/glasses"
)

// The window the settings open at.
var settingsW = SettingsWidth

func settingsH(cfg Config, attached []glasses.USB) int {
	_, h := settingsSize(cfg, attached)
	return h
}

// found collects every widget of one kind in the tree, in visual order.
func found[T toolkit.Widget](root toolkit.Widget) []T {
	var out []T
	var walk func(toolkit.Widget)
	walk = func(w toolkit.Widget) {
		if v, ok := w.(T); ok {
			out = append(out, v)
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

// arranged builds the settings tree, gives it a window's worth of space, and
// returns every settings row's rectangle.
//
// This is how a layout is checked without a person looking at one. A widget's
// bounds are the layout's whole output, and they are computed with no display,
// no window server and no pixels — so the thing that decides whether a window
// reads as a form or as a row of slivers is a pure function this can call.
func arranged(t *testing.T, cfg *Config, attached []glasses.USB) ([]toolkit.Rect, int) {
	t.Helper()
	root, _ := settingsRoot(cfg, attached, nil, func() {})
	h := settingsH(*cfg, attached)
	root.SetBounds(toolkit.Rect{X: 0, Y: 0, W: settingsW, H: h})

	rows := found[*toolkit.SettingRow](root)
	if len(rows) == 0 {
		t.Fatal("the window holds no settings rows")
	}
	out := make([]toolkit.Rect, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Bounds())
	}
	return out, h
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
		t.Fatalf("the window has %d rows; it should have the glasses, the screen "+
			"count, the menu bar and a row per shortcut", len(rects))
	}
	for i, r := range rects {
		if r.W <= 0 || r.H <= 0 {
			t.Errorf("row %d has no size: %+v", i, r)
			continue
		}
		if r.X < 0 || r.Y < 0 || r.X+r.W > settingsW || r.Y+r.H > height {
			t.Errorf("row %d is outside the %dx%d window: %+v",
				i, settingsW, height, r)
		}
		// And inside the margin, not flush against the frame: text starting at a
		// window's own edge reads as spilled rather than laid out.
		if r.X < toolkit.Scaled(SettingsPadX) {
			t.Errorf("row %d starts at x=%d, inside the %d margin",
				i, r.X, toolkit.Scaled(SettingsPadX))
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

// TestTheButtonsCannotBeDrawnOverTheContent.
//
// This is the defect a person found by dragging the window smaller: the buttons
// came down over the shortcut text. The cause was a stack of fixed-height fields
// -- right at one size and wrong at every other.
//
// Two things answer it now, and this holds both. The buttons own a BorderLayout
// band that the content cannot enter, whatever size the frame is given; and the
// window is not resizable, so the only size that matters is the one it opens at,
// where the page has been measured to fit. The smaller sizes below are therefore
// not a promise about resizing -- they are there because a band that only holds
// at one size is not a band.
func TestTheButtonsCannotBeDrawnOverTheContent(t *testing.T) {
	cfg := &Config{}
	attached := []glasses.USB{oneS, luma}
	root, _ := settingsRoot(cfg, attached, nil, func() {})
	w, h := settingsSize(*cfg, attached)

	for _, size := range []toolkit.Rect{
		{W: w, H: h},
		{W: w, H: h / 2},
		{W: w / 2, H: h / 3},
		{W: toolkit.Scaled(200), H: toolkit.Scaled(120)},
	} {
		root.SetBounds(size)

		buttons := found[*toolkit.Button](root)
		if len(buttons) != 2 {
			t.Fatalf("%+v: the window has %d buttons", size, len(buttons))
		}
		cards := found[*toolkit.SettingsGroup](root)
		if len(cards) != 2 {
			t.Fatalf("%+v: the window has %d cards", size, len(cards))
		}

		// The band is the bottom of the frame, and every button is in it.
		band := size.H - toolkit.Scaled(SettingsPadY) - toolkit.Scaled(ButtonBarH)
		for _, b := range buttons {
			bb := b.Bounds()
			if bb.Y < band {
				t.Errorf("at %+v the %q button starts at y=%d, above its band at %d",
					size, b.Label().Get(), bb.Y, band)
			}
			if bb.Y+bb.H > size.H {
				t.Errorf("at %+v the %q button ends at y=%d, past the window",
					size, b.Label().Get(), bb.Y+bb.H)
			}
		}
		// And at the size the window OPENS at, no card reaches into the band.
		//
		// Only at that size: the page is measured to fit it, and the window
		// cannot be made smaller. A frame deliberately forced below its content
		// has nowhere to put the difference -- there is no scroll view any more,
		// on purpose -- so asserting it here would be asserting something the
		// design does not claim.
		if size.W != w || size.H != h {
			continue
		}
		for i, c := range cards {
			cb := c.Bounds()
			if cb.H > 0 && cb.Y+cb.H > band {
				t.Errorf("at the window's own size %+v, card %d ends at y=%d, "+
					"inside the button band at %d", size, i, cb.Y+cb.H, band)
			}
		}
	}
}

// TestTheWindowHasNothingToScroll: it opens at the size its content measures and
// cannot be resized, so a scroll view would be a gutter taking a strip of width
// from every row to hold a scrollbar that can never appear.
func TestTheWindowHasNothingToScroll(t *testing.T) {
	cfg := &Config{}
	attached := []glasses.USB{oneS, luma}
	root, _ := settingsRoot(cfg, attached, nil, func() {})
	if views := found[*toolkit.ScrollView](root); len(views) != 0 {
		t.Errorf("the window holds %d scroll views", len(views))
	}
}

// TestTheWindowIsSizedToWhatItsPageMeasures, not to a table of constants.
//
// The height used to be a sum of field heights the caller worked out. Asking the
// page means the number cannot drift from the tree: add a row, or change the
// font, and the window follows.
func TestTheWindowIsSizedToWhatItsPageMeasures(t *testing.T) {
	cfg := &Config{}
	attached := []glasses.USB{oneS, luma}
	page, _ := settingsPage(cfg, attached)
	w, h := settingsSize(*cfg, attached)

	_, want := page.Measure(w-2*toolkit.Scaled(SettingsPadX), 0)
	chrome := toolkit.Scaled(ButtonBarH) + 2*toolkit.Scaled(SettingsPadY) +
		toolkit.Scaled(PageSpacing)
	if h != want+chrome {
		t.Errorf("the window is %d tall; the page measures %d and the chrome is %d",
			h, want, chrome)
	}
	if want <= 0 {
		t.Error("the page measures nothing at all")
	}
	// A machine with more to say gets a taller window: the shortcut card has a
	// row per shortcut, so this is not a constant in disguise.
	if h <= chrome {
		t.Errorf("the window is %d tall and its chrome alone is %d", h, chrome)
	}
}

// TestTheSettingsWindowReadsItsControlsBack is the other half: what the person
// chose has to arrive in the file.
func TestTheSettingsWindowReadsItsControlsBack(t *testing.T) {
	cfg := &Config{}
	root, read := settingsRoot(cfg, []glasses.USB{oneS, luma}, nil, func() {})
	root.SetBounds(toolkit.Rect{X: 0, Y: 0, W: settingsW, H: settingsH(*cfg, nil)})

	// Pick the second headset, the third screen count, and turn the menu bar
	// covering off, through the widgets, the way a person would.
	//
	// Two drop-downs in visual order (the headset, then the count) and one
	// switch: a settings row puts its control in the trailing slot, so what a
	// person sees is at the leaves and not at the root.
	tiles := found[*toolkit.IconGrid](root)
	switches := found[*toolkit.Switch](root)
	if len(tiles) != 1 || len(switches) != 1 {
		t.Fatalf("the window has %d grids and %d switches; want the glasses tiles "+
			"and the menu bar", len(tiles), len(switches))
	}
	tiles[0].SetSelected(1) // the second headset
	switches[0].On().Set(false)

	read()
	if got := cfg.Model(); got != "VITURE Luma Ultra" {
		t.Errorf("model is %q, want the one that was selected", got)
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

	// Turn the menu-bar covering off first, so what Save writes can be told
	// apart from the defaults it started with.
	sw := found[*toolkit.Switch](root)
	if len(sw) != 1 {
		t.Fatalf("the window has %d switches", len(sw))
	}
	sw[0].On().Set(false)

	b["Save"].OnClick()
	if closed != 1 {
		t.Errorf("Save left the window open")
	}
	got, err := LoadConfigFile(path)
	if err != nil {
		t.Fatalf("what Save wrote does not load: %v", err)
	}
	if got.Model() != "VITURE Luma Ultra" {
		t.Errorf("Save wrote %q", got.Model())
	}
	// The menu-bar setting made the round trip too, so Save writes the whole
	// form and not one field of it.
	if got.Immersive() {
		t.Error("the menu-bar switch was turned off and came back on")
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
		{"a laptop panel", 900, 1},
		{"a 1200-row display", 1200, 1.2},
		{"a 1440p display", 1440, 1.35},
		{"the 8K panel this was reported on", 2160, 1.35},
		{"a very tall panel is not magnified further", 4320, 1.35},
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

// TestTheHeadsetCanBeChosenWithTheMouse: a click on the second tile chooses the
// second headset, and it arrives in the settings.
//
// A control nobody can operate is the defect this window has had twice -- a
// drop-down whose popover no host drew, and then a click that never left a
// scroll view. Both were invisible to every test that only built the tree, so
// this one drives the widgets the way a mouse does, through the same surface the
// window is given.
func TestTheHeadsetCanBeChosenWithTheMouse(t *testing.T) {
	cfg := &Config{}
	root, read := settingsRoot(cfg, []glasses.USB{oneS, luma}, nil, func() {})
	top := settingsSurface(root)
	w, h := settingsSize(*cfg, []glasses.USB{oneS, luma})
	top.SetBounds(toolkit.Rect{X: 0, Y: 0, W: w, H: h})

	grids := found[*toolkit.IconGrid](top)
	if len(grids) != 1 {
		t.Fatalf("the window has %d grids, want the glasses tiles", len(grids))
	}
	tiles := grids[0]
	if got := tiles.Selected().Get(); got != 0 {
		t.Fatalf("the window opened with tile %d selected, want the first", got)
	}

	// A point in the second tile, asked of the grid rather than guessed at.
	//
	// IndexAt takes WIDGET-LOCAL coordinates, like OnEvent -- so the search runs
	// in local space and the click is sent at the grid's origin plus that. Asking
	// it in surface coordinates finds a point that means a different cell, which
	// is a mistake worth leaving written down.
	b := tiles.Bounds()
	lx, ly := -1, -1
	for y := 0; y < b.H && ly < 0; y++ {
		for x := 0; x < b.W; x++ {
			if tiles.IndexAt(x, y) == 1 {
				lx, ly = x, y
				break
			}
		}
	}
	if ly < 0 {
		t.Fatalf("the grid %+v has no second cell to click", b)
	}
	top.OnEvent(toolkit.Event{Kind: toolkit.EventClick, X: b.X + lx, Y: b.Y + ly})

	if got := tiles.Selected().Get(); got != 1 {
		t.Fatalf("clicking the second tile selected %d", got)
	}
	read()
	if got, want := cfg.Model(), "VITURE Luma Ultra"; got != want {
		t.Errorf("the settings say %q, want %q", got, want)
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

// TestShortcutRows: one row per shortcut, the combination in the trailing slot,
// and a refusal wrapped across the two text lines a row has.
//
// The two shapes matter because they are what the machine actually says. A
// GRANT is "gallery: Control-Option-Command-Space" -- a name and a combination,
// which is a row with a value. A REFUSAL is a sentence three times as long with
// no combination in it at all, and a row draws its title on one line, so it has
// to be broken up or it runs off the card.
func TestShortcutRows(t *testing.T) {
	rows := shortcutRowsFrom("previous: Option-Command-Left\n" +
		"gallery: Control-Option-Command-Space\n")
	if len(rows) != 2 {
		t.Fatalf("two grants gave %d rows", len(rows))
	}
	if rows[0].Title != "previous" {
		t.Errorf("the first row is titled %q", rows[0].Title)
	}
	if rows[0].Control == nil {
		t.Fatal("a granted shortcut has no combination in its trailing slot")
	}
	// The value is sized to its own text: a row right-aligns a control at the
	// size the control carries, so a value with no size would be given the
	// default 48-pixel slot and be cut off.
	if b := rows[0].Control.Bounds(); b.W <= 0 || b.H <= 0 {
		t.Errorf("the combination is %+v, so the row cannot place it", b)
	}

	// A refusal: no colon, so no value, and the sentence is wrapped over the
	// row's two lines rather than run off the end.
	long := "no global shortcut for previous next or the gallery, " +
		strings.Repeat("every combination was refused ", 3)
	rows = shortcutRowsFrom(long)
	if len(rows) != 1 {
		t.Fatalf("one refusal gave %d rows", len(rows))
	}

	// A grant whose combination is still too wide for half a card goes under the
	// name, where there is a whole line for it, rather than over the name.
	wide := shortcutRowsFrom("gallery: " + strings.Repeat("Control-Option-Command-", 6))
	if len(wide) != 1 {
		t.Fatalf("one long grant gave %d rows", len(wide))
	}
	if wide[0].Control != nil {
		t.Error("a combination too wide for the slot was put in it anyway")
	}
	if !strings.Contains(wide[0].Subtitle, "Control-Option-Command-") {
		t.Errorf("the combination was dropped rather than moved: %q", wide[0].Subtitle)
	}

	// A refusal that has nothing to wrap is a single line, not two.
	if got := shortcutRowsFrom("nothing was granted"); len(got) != 1 || got[0].Subtitle != "" {
		t.Errorf("a short refusal gave %d rows with subtitle %q",
			len(got), got[0].Subtitle)
	}

	// A blank line contributes nothing rather than an empty row.
	if got := shortcutRowsFrom("a: b\n\n\nc: d\n"); len(got) != 2 {
		t.Errorf("blank lines gave %d rows", len(got))
	}
	if got := shortcutRowsFrom("   \n"); len(got) != 0 {
		t.Errorf("nothing but whitespace gave %d rows", len(got))
	}
}

// TestSplitToFit: a sentence too long for one line of a settings row is broken
// across the two the row has, at the word boundary nearest the MIDDLE.
//
// Nearest the middle rather than the last boundary that fits, because two lines
// of similar length read as a sentence that was laid out, and a long line with
// one word under it reads as a mistake. This matters for exactly one kind of
// text: a machine that grants no shortcut at all reports a refusal three times
// the length of a grant.
func TestSplitToFit(t *testing.T) {
	// Room enough: returned whole, with nothing on the second line.
	if head, tail := splitToFit("previous", 10_000); head != "previous" || tail != "" {
		t.Errorf("a short sentence was split into %q and %q", head, tail)
	}
	// No room known: the same, rather than a sentence cut to nothing.
	if head, tail := splitToFit("previous", 0); head != "previous" || tail != "" {
		t.Errorf("with no room given the sentence became %q and %q", head, tail)
	}

	// Broken near the middle, both halves non-empty, and nothing lost.
	s := "no global shortcut for the gallery, every combination was already taken"
	head, tail := splitToFit(s, toolkit.TextWidth(s)/2)
	if head == "" || tail == "" {
		t.Fatalf("the sentence was not split: %q / %q", head, tail)
	}
	if head+" "+tail != s {
		t.Errorf("the split lost or changed text: %q + %q", head, tail)
	}
	// Near the middle: neither half is more than twice the other.
	if len(head) > 2*len(tail) || len(tail) > 2*len(head) {
		t.Errorf("the split is lopsided: %d and %d characters", len(head), len(tail))
	}

	// One word with no boundary in it is left whole: cutting a combination in
	// half is worse than letting it overhang, and a row is not the place to
	// hyphenate.
	long := strings.Repeat("Control-Option-Command-", 4)
	if head, tail := splitToFit(long, 10); head != long || tail != "" {
		t.Errorf("an unbreakable word was cut: %q / %q", head, tail)
	}
}

// TestAShortcutThatHadToBeSubstitutedSaysSoUnderItsName.
//
// The report for a substituted combination is "the granted one (asked for the
// other, it was taken)" — three times the width of the trailing slot. It used
// to be put there whole, and the row drew it straight over its own title so
// neither could be read. The bracket goes under the name, where there is a
// line for it.
func TestAShortcutThatHadToBeSubstitutedSaysSoUnderItsName(t *testing.T) {
	rows := shortcutRowsFrom("next: Control-Option-Shift-Command-Right " +
		"(asked for Control-Option-Command-Right, it was taken)")
	if len(rows) != 1 {
		t.Fatalf("one line gave %d rows", len(rows))
	}
	r := rows[0]
	if r.Title != "next" {
		t.Errorf("the row is titled %q, want next", r.Title)
	}
	if r.Subtitle == "" {
		t.Error("nothing under the name says which combination was asked for")
	}
	if strings.Contains(r.Subtitle, "(") && !strings.Contains(r.Subtitle, "asked for") {
		t.Errorf("the aside is %q, which does not say what was asked for", r.Subtitle)
	}
	if r.Control == nil {
		t.Error("the granted combination is not in the trailing slot")
	}
}

// glassesTilePixels draws the headset tiles and hands back the pixels.
//
// PIXELS, because the claim being tested is about what a person can SEE: an
// outline that inks a fourteenth of its box and a filled symbol that inks most
// of it are the same widget with the same label, and only what lands on screen
// separates them. Counting ink alone would not have done it -- the chosen
// tile is filled behind its icon, so those pixels count as ink whatever the
// icon is, and the outline and the symbol came out to the same number.
func glassesTilePixels(t *testing.T) []byte {
	t.Helper()
	cfg := &Config{}
	root, _ := settingsRoot(cfg, []glasses.USB{oneS}, nil, func() {})
	root.SetBounds(toolkit.Rect{X: 0, Y: 0, W: settingsW, H: settingsH(*cfg, nil)})
	grids := found[*toolkit.IconGrid](root)
	if len(grids) != 1 {
		t.Fatalf("the window has %d icon grids, want the headset tiles", len(grids))
	}
	b := grids[0].Bounds()
	if b.W <= 0 || b.H <= 0 {
		t.Fatalf("the tiles measure %dx%d", b.W, b.H)
	}
	buf := make([]byte, b.W*b.H*4)
	p := painter.NewPixelPainterBGRA(buf, b.W, b.H)
	grids[0].SetBounds(toolkit.Rect{X: 0, Y: 0, W: b.W, H: b.H})
	theme := toolkit.DefaultLight()
	p.FillRect(toolkit.Rect{W: b.W, H: b.H}, theme.Surface)
	grids[0].Draw(p, theme)
	return buf
}

// TestTheSettingsTilesShowTheSystemSymbolWhenThereIsOne pins the wiring, not
// the platform: the seam is fed a stencil on every machine, so a runner with no
// system symbols tests the same thing a Mac does.
func TestTheSettingsTilesShowTheSystemSymbolWhenThereIsOne(t *testing.T) {
	was := glassesIcon
	t.Cleanup(func() { glassesIcon = was })

	// A solid square of alpha: the densest thing a symbol could be, which is
	// the point -- a system glyph inks most of its box where the drawn outline
	// inks a fourteenth of it.
	box := 0
	glassesIcon = func(px int) toolkit.IconFunc {
		if px <= 0 {
			t.Errorf("the tiles asked for an icon of %d pixels", px)
			px = 1
		}
		pix := make([]byte, px*px*4)
		for i := range pix {
			pix[i] = 0xFF
		}
		stencil := toolkit.StencilIcon(pix, px, px)
		return func(p painter.Painter, r toolkit.Rect, ink toolkit.RGBA) {
			box = r.W * r.H
			stencil(p, r, ink)
		}
	}
	symbol := glassesTilePixels(t)

	glassesIcon = func(int) toolkit.IconFunc { return nil }
	drawn := glassesTilePixels(t)

	if box == 0 {
		t.Fatal("the tiles never asked for an icon")
	}
	changed := 0
	for i := 0; i+3 < len(symbol) && i+3 < len(drawn); i += 4 {
		if symbol[i] != drawn[i] || symbol[i+1] != drawn[i+1] || symbol[i+2] != drawn[i+2] {
			changed++
		}
	}
	// Half the icon box at least: the stencil is solid, the toolkit's glasses
	// are an outline, and swapping one for the other has to repaint most of
	// the square. A window still drawing the outline would change nothing.
	if changed < box/2 {
		t.Errorf("swapping the outline for the system symbol changed %d pixels of "+
			"a %d-pixel icon box; the window is not showing the symbol", changed, box)
	}
}
