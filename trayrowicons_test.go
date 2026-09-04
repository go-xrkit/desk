// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"bytes"
	"errors"
	"image/png"
	"runtime"
	"strings"
	"testing"
)

// TestEveryMenuRowCarriesASymbol.
//
// ⛔ rowIcon answers nil for a symbol the system does not have, on purpose: a
// misnamed glyph must cost a row its picture and not its menu. That silence is
// exactly what would swallow a typo, so this is the thing that makes it loud --
// every row a person can choose names a symbol, and no two rows name the same
// one, because two rows drawn alike are two rows that have to be read.
func TestEveryMenuRowCarriesASymbol(t *testing.T) {
	seen := map[string]string{}
	for _, r := range TrayRows() {
		if r.Action == ActionNone {
			if r.Symbol != "" {
				t.Errorf("a separator carries the symbol %q", r.Symbol)
			}
			continue
		}
		if r.Symbol == "" {
			t.Errorf("%q has no symbol", r.Title)
			continue
		}
		if other, ok := seen[r.Symbol]; ok {
			t.Errorf("%q and %q are both drawn as %q", other, r.Title, r.Symbol)
		}
		seen[r.Symbol] = r.Title
	}
}

// TestEverySymbolTheMenuNamesExists.
//
// The names are strings handed to the window server, so a typo is not a
// compile error and not a run-time one either -- it is a row that quietly has
// no picture. Only a Mac can answer, so only a Mac asks.
func TestEverySymbolTheMenuNamesExists(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("system symbols are macOS's")
	}
	for _, r := range TrayRows() {
		if r.Symbol == "" {
			continue
		}
		b := rowIcon(r.Symbol)
		if len(b) == 0 {
			t.Errorf("%q asks for %q and this system has no such symbol",
				r.Title, r.Symbol)
			continue
		}
		cfg, err := png.DecodeConfig(bytes.NewReader(b))
		if err != nil {
			t.Errorf("%q is not a PNG: %v", r.Symbol, err)
			continue
		}
		// ⛔ THE SAME BOX, EVERY ROW. tray normalises a row icon's height and
		// lets its width follow, so glyphs of different proportions are drawn at
		// different sizes unless they arrive in one square.
		if cfg.Width != TrayRowPx || cfg.Height != TrayRowPx {
			t.Errorf("%q is %dx%d, not the %d square every row is drawn in",
				r.Symbol, cfg.Width, cfg.Height, TrayRowPx)
		}
	}
}

// TestARowWhoseSymbolIsMissingKeepsItsMenu.
//
// The degradation itself, on both platforms, through the seam: a system that
// answers nothing leaves a row with a label and an action and no picture.
func TestARowWhoseSymbolIsMissingKeepsItsMenu(t *testing.T) {
	was := systemSymbol
	t.Cleanup(func() { systemSymbol = was; rowIcons.Clear() })
	rowIcons.Clear()
	systemSymbol = func(string, int) ([]byte, error) {
		return nil, errors.New("no such symbol")
	}

	h := headless(t)
	actions := make(chan Action, TrayQueue)
	item, err := OpenTray(nil, actions)
	if err != nil {
		t.Fatalf("OpenTray = %v", err)
	}
	defer func() { _ = item.Close() }()
	go func() { _ = item.Hold() }()
	waitFor(t, func() bool {
		_, _, menu := h.Snapshot()
		return menu != nil && len(menu.Items) > 0
	}, "the menu to arrive")

	_, _, menu := h.Snapshot()
	rows := TrayRows()
	if len(menu.Items) != len(rows) {
		t.Fatalf("the menu has %d rows, want %d", len(menu.Items), len(rows))
	}
	for i, r := range rows {
		if r.Action == ActionNone {
			continue
		}
		if menu.Items[i].Label != r.Title {
			t.Errorf("row %d lost its label: %q", i, menu.Items[i].Label)
		}
		if menu.Items[i].Icon != nil {
			t.Errorf("row %d has a picture the system could not make", i)
		}
	}
}

// TestTheGlyphIsMadeOnce, because the state binding rebuilds the item's
// picture every second and is one lookup away from this path.
func TestTheGlyphIsMadeOnce(t *testing.T) {
	was := systemSymbol
	t.Cleanup(func() { systemSymbol = was; rowIcons.Clear() })
	rowIcons.Clear()
	made := 0
	systemSymbol = func(_ string, px int) ([]byte, error) {
		made++
		return pngOf(make([]byte, px*px*4), px, px)
	}
	for range 5 {
		if len(rowIcon("gearshape")) == 0 {
			t.Fatal("rowIcon gave nothing back")
		}
	}
	if made != 1 {
		t.Errorf("the window server was asked %d times for one glyph", made)
	}
	if rowIcon("") != nil {
		t.Error("a row with no symbol was given a picture")
	}
}

// TestSquaringCentresTheInkAndKeepsIt.
//
// The padding must not move ink off the edge or change any of it: a glyph is
// the system's, and this only decides where the box around it is.
func TestSquaringCentresTheInkAndKeepsIt(t *testing.T) {
	// A 4x2 picture: a full row of ink, then a row of nothing.
	in := make([]byte, 4*2*4)
	for x := range 4 {
		in[x*4+0], in[x*4+1], in[x*4+2], in[x*4+3] = 10, 20, 30, 255
	}
	b, err := pngOf(in, 4, 2)
	if err != nil {
		t.Fatal(err)
	}
	got, err := squared(b, 6)
	if err != nil {
		t.Fatal(err)
	}
	pix, w, h, err := decodePixels(got)
	if err != nil {
		t.Fatal(err)
	}
	if w != 6 || h != 6 {
		t.Fatalf("squared to %dx%d, want 6x6", w, h)
	}
	// Centred: 4 wide in 6 leaves one column each side, 2 tall in 6 leaves two
	// rows each side.
	ink := 0
	for y := range h {
		for x := range w {
			p := pix[(y*w+x)*4:]
			if p[3] == 0 {
				continue
			}
			ink++
			if y != 2 || x < 1 || x > 4 {
				t.Errorf("ink at %d,%d, outside the centred 4x2", x, y)
			}
			if p[0] != 10 || p[1] != 20 || p[2] != 30 || p[3] != 255 {
				t.Errorf("the colour at %d,%d changed: %v", x, y, p[:4])
			}
		}
	}
	if ink != 4 {
		t.Errorf("%d pixels of ink, want the 4 that went in", ink)
	}
}

// TestAPictureTooBigForTheSquareIsRefused, rather than cropped: ink cut off an
// edge is a glyph that means something else.
func TestAPictureTooBigForTheSquareIsRefused(t *testing.T) {
	b, err := pngOf(make([]byte, 8*8*4), 8, 8)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := squared(b, 6); err == nil {
		t.Error("an 8x8 picture was fitted into a 6-pixel square")
	}
	if _, err := squared([]byte("not a picture"), 6); err == nil {
		t.Error("something that is not a picture was squared")
	}
}

// TestAlreadySquareIsLeftAlone: nothing is decoded and re-encoded for nothing.
func TestAlreadySquareIsLeftAlone(t *testing.T) {
	b, err := pngOf(make([]byte, 6*6*4), 6, 6)
	if err != nil {
		t.Fatal(err)
	}
	got, err := squared(b, 6)
	if err != nil {
		t.Fatal(err)
	}
	if &got[0] != &b[0] {
		t.Error("a square picture was rebuilt")
	}
}

// TestTheMenuSaysWhichKeyDoesTheSameThing.
//
// ⛔ What was GRANTED, not what was asked for: the ladder substitutes when a
// combination is taken, and a menu printing the wanted one would send somebody
// to press a key that does nothing. An action nothing was granted for keeps its
// bare label rather than a lie.
func TestTheMenuSaysWhichKeyDoesTheSameThing(t *testing.T) {
	h := headless(t)
	actions := make(chan Action, TrayQueue)
	item, err := OpenTray(nil, actions)
	if err != nil {
		t.Fatalf("OpenTray = %v", err)
	}
	defer func() { _ = item.Close() }()
	go func() { _ = item.Hold() }()
	waitFor(t, func() bool {
		_, _, menu := h.Snapshot()
		return menu != nil && len(menu.Items) > 0
	}, "the menu to arrive")

	// Before anything is claimed the rows are bare: the item is made before any
	// session and must not print a combination it has not been given.
	_, _, menu := h.Snapshot()
	for i, it := range menu.Items {
		if strings.Contains(it.Label, "⌘") {
			t.Errorf("row %d printed %q before a single key was claimed", i, it.Label)
		}
	}

	item.ShowShortcuts(map[Action]string{
		ActionSettings: "⌃⌥⌘S",
		ActionQuit:     "⌃⌥⌘⎋",
	})
	waitFor(t, func() bool {
		_, _, m := h.Snapshot()
		return m != nil && len(m.Items) > 0 && strings.Contains(m.Items[0].Label, "⌃⌥⌘S")
	}, "the combinations to arrive")

	_, _, menu = h.Snapshot()
	rows := TrayRows()
	if len(menu.Items) != len(rows) {
		t.Fatalf("the menu has %d rows, want %d", len(menu.Items), len(rows))
	}
	for i, r := range rows {
		got := menu.Items[i].Label
		switch r.Action {
		case ActionSettings:
			if !strings.HasPrefix(got, r.Title) || !strings.HasSuffix(got, "⌃⌥⌘S") {
				t.Errorf("row %d is %q, want %q and its combination", i, got, r.Title)
			}
		case ActionNone:
			if !menu.Items[i].Separator {
				t.Errorf("row %d stopped being a separator: %q", i, got)
			}
		default:
			if r.Action != ActionQuit && got != r.Title {
				t.Errorf("row %d is %q; nothing was granted for it, so it should "+
					"still be %q", i, got, r.Title)
			}
		}
	}

	// And the rows still work: the label changed, the action did not.
	menu.Items[0].Activate()
	select {
	case a := <-actions:
		if a != ActionSettings {
			t.Errorf("the first row sent %v", a)
		}
	default:
		t.Error("the first row sent nothing after the menu was rebuilt")
	}
}

// TestShowShortcutsOnNoItemDoesNothing: OpenTray can fail, and main carries on
// with a nil item rather than refusing to run.
func TestShowShortcutsOnNoItemDoesNothing(t *testing.T) {
	var none *Tray
	none.ShowShortcuts(map[Action]string{ActionQuit: "⌃⌥⌘⎋"})
}
