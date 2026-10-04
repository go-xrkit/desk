// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// readmeLegend is the key each glyph the README prints stands for, in the
// spelled ANSI names [hotkey.Combo.Names] uses.
//
// ⛔⛔ THE TEST BELOW FIRST COMPARED THE README AGAINST [hotkey.Combo.Glyphs],
// AND THAT WAS A JUDGE THAT COUNTED DIFFERENTLY FROM ITS SUBJECT. Glyphs
// renders a key through THIS MACHINE'S LAYOUT: on the AZERTY keyboard this was
// written on, ANSI's Minus prints ")", Equal prints "-", M prints "," and A
// prints "Q". So the check reported six correct rows as wrong and would have
// reported different ones on a different keyboard.
//
// A README is read on every layout, so it names the LEGEND -- and the legend is
// what a reader gets, because ClaimGlobal asks for OnThisKeyboard: a claim
// moves to the key whose legend matches rather than keeping the ANSI position.
// This table is the translation, written out, so what the check believes is
// visible instead of inferred.
var readmeLegend = map[string]string{
	"←": "Left", "→": "Right", "↑": "Up", "↓": "Down",
	"↩": "Return", "⎋": "Escape", "⌫": "Delete", "⇥": "Tab",
	"-": "Minus", "=": "Equal", "[": "LeftBracket", "]": "RightBracket",
}

// ⛔⛔ THE README SAID KEYS THAT WERE NOT BOUND, FOR LONGER THAN THEY WERE.
// Its table of system-wide shortcuts put the two galleries on ⌃⌥⌘↑ and ⌃⌥⌘↓;
// they have been on ⌃⌥⌘F3 and ⌃⌥⌘F4 since the arrows turned out to be the pair
// a settings file most often wants for the distance. Four of its fourteen rows
// were wrong when this was written -- the two galleries, ⌃⌥⌘A described as
// "choose" when it shows the applications, and ⌃⌥⌘⇥ for cycling, which is
// ⌃⌥⌘C -- and sixteen shortcuts were missing altogether, including the one that
// follows your head.
//
// Nothing could have caught that. The table was prose, and prose about a
// mapping kept in another file is a copy that rots in silence: the keys went on
// working, the document went on describing a program that no longer existed,
// and the only reader who found out was one who pressed the key.
//
// ⚠ IT READS THE TABLE ROWS, NOT THE PROSE. A table row is a claim about what
// is bound today; the prose around it discusses keys that MOVED, and has to be
// able to name them -- "⌃⌥⌘↑ pushed the band away sixteen times" is a true
// sentence about a session, not a mapping.
//
// ⚠ AND IT CHECKS ONE DIRECTION, the one that rots: the README may not name a
// combination the code does not grant. It deliberately does not demand that
// every shortcut appear, because the nine screen keys are one row and several
// pairs share a row, and a document forced to list each one separately would be
// a worse document.
func TestEveryShortcutTheReadmeNamesIsBound(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	bound := map[string]Action{}
	for _, s := range DefaultShortcuts() {
		n := s.Want.Names()
		bound[n[strings.LastIndex(n, "-")+1:]] = s.Does
	}

	combo := regexp.MustCompile("`⌃⌥⌘([^`]+)`")
	seen, named := map[string]bool{}, 0
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "| ") {
			continue
		}
		for _, m := range combo.FindAllStringSubmatch(line, -1) {
			glyph := strings.TrimSuffix(m[1], "…")
			if seen[glyph] {
				continue
			}
			seen[glyph] = true
			named++
			want := glyph
			if n, ok := readmeLegend[glyph]; ok {
				want = n
			}
			if _, ok := bound[want]; !ok {
				t.Errorf("a table row names ⌃⌥⌘%s (the key ANSI calls %q) and "+
					"desk.DefaultShortcuts binds nothing to it", glyph, want)
			}
		}
	}
	// ⛔ AND THE TABLE MUST ACTUALLY HAVE BEEN READ. A regexp that stopped
	// matching, or a table that lost its leading pipes, would otherwise turn
	// this into a check that passes by looking at nothing -- the scan that could
	// not read and reported zero.
	if named < 20 {
		t.Errorf("only %d system-wide shortcuts were found in the README's "+
			"tables; it lists over twenty, so this is no longer reading them", named)
	}
}

// ⛔⛔ AND EVERY CONSTANT THE README PUTS A NUMBER ON. The document says
// "`desk.MinReachDeg` = 5" and "`desk.MaxDistance` = 4"; a constant that moves
// leaves those sentences behind, and a reader has no way to tell. The expected
// side comes from the CONSTANT, so this compares the document against the code
// rather than against a number somebody typed twice.
//
// ⚠ The names are listed rather than discovered. Go has no reflection over
// constants, so a new one is not gated the day it is written -- which is the
// weakness of this check and the reason it is worth having anyway: the four it
// does hold are the four a person types on a command line.
func TestEveryConstantTheReadmeQuotesIsItsValue(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	var quoted int
	for _, c := range []struct {
		name string
		is   float64
	}{
		{"MaxScreens", MaxScreens},
		{"MinDistance", MinDistance},
		{"MaxDistance", MaxDistance},
		{"MinReachDeg", MinReachDeg},
		{"ComfortableYawDeg", ComfortableYawDeg},
	} {
		// "`desk.MinReachDeg` = 5", in whatever spacing the prose uses.
		at := regexp.MustCompile("`desk\\." + c.name + "` *= *([0-9.]+)")
		ms := at.FindAllStringSubmatch(string(data), -1)
		if len(ms) == 0 {
			continue // the README does not quote this one, which is allowed
		}
		quoted += len(ms)
		for _, m := range ms {
			got, err := strconv.ParseFloat(m[1], 64)
			if err != nil {
				t.Errorf("the README quotes desk.%s as %q, which is not a number",
					c.name, m[1])
				continue
			}
			if got != c.is {
				t.Errorf("the README says desk.%s = %v; it is %v",
					c.name, got, c.is)
			}
		}
	}
	// ⛔ AND IT MUST HAVE READ SOME. Every name above is allowed to be absent,
	// so a regexp that stopped matching -- a change of spacing, of backticks,
	// of the `desk.` prefix -- would make this a check that passes by looking at
	// nothing at all.
	if quoted < 4 {
		t.Errorf("only %d constants were found quoted in the README; it quotes "+
			"at least four, so this is no longer reading it", quoted)
	}
}
