// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"testing"

	"github.com/go-viture/luma"
	"github.com/go-widgets/toolkit"
	"github.com/go-xrkit/xrkit/glasses"
)

// TestEveryWordInTheSettingsWindowCanBeDrawn.
//
// ⛔⛔ THIS REPLACES A RULE NOBODY COULD ENFORCE. settings.go carried, in a
// comment, "No apostrophe and no dashes anywhere in this window's text -- the
// built-in font has neither, and 'the glasses' own menu bar' came out as a hole
// in the middle of a sentence."
//
// The rule was wrong in both directions, and measuring the font settled it:
//
//	'  and  ’   ABSENT      -- the rule was right about these
//	-           DRAWN       -- the rule was wrong: a plain hyphen is fine
//	—  and  é   ABSENT
//
// And it was unenforced: this very window shipped "Turn this Mac's screen off".
// A rule in a comment is a rule somebody has to remember; this is a test.
//
// ⚠ IT IS ABOUT THE FALLBACK FONT. On macOS the window uses the system face,
// which has all of these. The built-in 5x7 font is what a platform with no
// system face falls back TO, and there a missing glyph is a blank cell in the
// middle of a word -- which reads as a typo, not as a missing font.
func TestEveryWordInTheSettingsWindowCanBeDrawn(t *testing.T) {
	stand(t, luma.Info{
		FirmwareVersion: "12.0.01.101_20260605",
		PackageSerial:   "S154801101",
	}, []byte{2, 1, 9}, nil)

	cfg := &Config{}
	attached := []glasses.USB{oneS, lumaUltra}
	root, _ := settingsRoot(cfg, attached, 0, nil, func() {})
	w, h := settingsSize(*cfg, attached)
	root.SetBounds(toolkit.Rect{X: 0, Y: 0, W: w, H: h})

	seen := 0
	check := func(where, s string) {
		if s == "" {
			return
		}
		seen++
		if missing := toolkit.BitmapMissing(s); len(missing) > 0 {
			t.Errorf("%s: %q cannot be drawn in the fallback font -- %q would each "+
				"come out as a blank cell", where, s, missing)
		}
	}
	for _, r := range found[*toolkit.SettingRow](root) {
		check("a settings row title", r.Title)
		check("a settings row subtitle", r.Subtitle)
	}
	for _, l := range found[*toolkit.Label](root) {
		check("a label", l.Text().Get())
	}
	for _, b := range found[*toolkit.Button](root) {
		check("a button", b.Label().Get())
	}
	// ⛔ A SWEEP THAT FOUND NOTHING TO LOOK AT IS NOT A PASS. If the window ever
	// stops exposing its text this way, this test must fail rather than go
	// quietly green.
	if seen < 8 {
		t.Fatalf("only %d strings were examined; the window has more than that, so "+
			"this test is no longer reaching them", seen)
	}
	t.Logf("%d strings examined", seen)
}
