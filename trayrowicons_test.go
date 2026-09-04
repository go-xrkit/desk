// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"bytes"
	"errors"
	"image/png"
	"runtime"
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
		if cfg.Width > TrayRowPx || cfg.Height > TrayRowPx {
			t.Errorf("%q came back %dx%d, past the %d asked for",
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
	systemSymbol = func(string, int) ([]byte, error) {
		made++
		return []byte("a picture"), nil
	}
	for range 5 {
		if got := string(rowIcon("gearshape")); got != "a picture" {
			t.Fatalf("rowIcon = %q", got)
		}
	}
	if made != 1 {
		t.Errorf("the window server was asked %d times for one glyph", made)
	}
	if rowIcon("") != nil {
		t.Error("a row with no symbol was given a picture")
	}
}
