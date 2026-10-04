// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/go-xrkit/desk"
)

// comboIn finds a shortcut named inside a user-facing string: the modifier
// glyphs, then one key.
var comboIn = regexp.MustCompile("[⌃⇧]*⌥⌘([^\\s,;.)/\"']+)")

// legend is the key each glyph stands for, in the spelled ANSI names
// [hotkey.Combo.Names] uses -- the same table as the desk package's README
// check, and for the same reason: a string shown to a person names the LEGEND
// they read, not the ANSI position, because ClaimGlobal asks for OnThisKeyboard.
var legend = map[string]string{
	"←": "Left", "→": "Right", "↑": "Up", "↓": "Down",
	"↩": "Return", "⎋": "Escape", "⌫": "Delete", "⇥": "Tab",
	"-": "Minus", "=": "Equal", "[": "LeftBracket", "]": "RightBracket",
}

// modifier is the word each glyph stands for, in the order Names writes them.
var modifier = []struct {
	glyph, word string
}{
	{"⌃", "Control"}, {"⌥", "Option"}, {"⇧", "Shift"}, {"⌘", "Command"},
}

// spell turns "⌃⌥⌘←" into "Control-Option-Command-Left".
//
// ⛔⛔ THE WHOLE COMBINATION, AND THE FIRST VERSION COMPARED THE KEY ALONE. That
// let "⌥⌘←" through, because ⌃⌥⌘← is bound and the key is the same -- so the
// check missed half of the very defect it was written for, which was a help
// string naming the band's OLD two-modifier shortcut. A judge has to count what
// its subject counts.
func spell(combo string) string {
	key := comboIn.FindStringSubmatch(combo)
	if key == nil {
		return ""
	}
	var parts []string
	for _, m := range modifier {
		if strings.Contains(combo, m.glyph) {
			parts = append(parts, m.word)
		}
	}
	k := strings.TrimSuffix(strings.TrimSuffix(key[1], "…"), "'")
	if n, ok := legend[k]; ok {
		k = n
	}
	return strings.Join(append(parts, k), "-")
}

// ⛔⛔ A FLAG'S HELP NAMED TWO SHORTCUTS THAT HAD NOT EXISTED FOR MONTHS.
// -no-global said it declined "⌥⌘←/→ and ⌥⌘Space": the band has been on ⌃⌥⌘←
// and ⌃⌥⌘→ since one prefix for everything beat two keys saved, and Space is
// bound to NO ACTION AT ALL -- it survives only in traykey.go's translation
// table. The same stale pair had been copied into four places: this help, the
// README's table, the documentation site, and a comment in hotkeys.go.
//
// A flag's help is the one piece of documentation a person reads with their
// hands on the keyboard, which makes it the worst place for it.
//
// ⚠ IT READS STRING LITERALS, THROUGH go/ast, AND NOT THE FILE AS TEXT. The
// comments in this package discuss keys that MOVED -- "the band was on ⌥⌘←" is
// a true sentence about last month -- and a grep cannot tell that from a claim
// about today. The parser can: a literal is addressed to the person running the
// program, a comment to whoever reads the code.
//
// ⚠ AND IT SKIPS THE TESTS, which is not an exemption but a correction: this
// file's own regexp contains the glyphs, and the first version reported itself.
func TestNoUserFacingStringNamesAnUnboundShortcut(t *testing.T) {
	t.Parallel()

	bound := map[string]bool{}
	for _, s := range desk.DefaultShortcuts() {
		bound[s.Want.Names()] = true
	}

	fset := token.NewFileSet()
	pkg, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	var literals, checked int
	for _, p := range pkg {
		ast.Inspect(p, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			s, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			literals++
			for _, m := range comboIn.FindAllString(s, -1) {
				checked++
				if want := spell(m); want != "" && !bound[want] {
					t.Errorf("%s: a string shown to the person running this "+
						"program names %s (%s), and desk.DefaultShortcuts "+
						"binds nothing to it", fset.Position(lit.Pos()), m, want)
				}
			}
			return true
		})
	}
	// ⛔⛔ AND THE GUARD IS ON WHAT WAS READ, NOT ON WHAT WAS FOUND. It first
	// demanded that at least two shortcuts be named -- and then the fix removed
	// the last one, so a correct package failed its own blind-scan check. A
	// guard against "this stopped reading" cannot require the defect to still
	// be there. Literals are what proves the parser ran: this package has
	// hundreds, and zero means the directory or the filter is wrong -- the scan
	// that could not read and reported zero.
	t.Logf("%d string literals read, %d shortcuts named in them", literals, checked)
	if literals < 100 {
		t.Errorf("only %d string literals were parsed; this package has hundreds, "+
			"so the parser is not reading it", literals)
	}
}
