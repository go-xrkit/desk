// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"strings"

	"github.com/go-macos/hotkey"
	"github.com/go-widgets/tray"
)

// trayKey turns a claimed combination into the key equivalent a menu row draws.
//
// ⛔ THE PLATFORM DRAWS IT, which is the whole reason this exists rather than
// a string appended to the label. A key equivalent is laid out right-aligned in
// a column with every other row's, in the system's own glyphs and at the
// system's own spacing; text in the label is left-aligned in the middle of the
// row, in whatever run of glyphs we chose. A menu is read by scanning that
// column, and appending is what gives it up.
//
// An empty combination -- an action nothing was granted for -- comes back with
// no key at all, so the row draws as it always did rather than claiming a
// combination that does nothing.
func trayKey(c hotkey.Combo) (string, tray.Mods) {
	// ⛔ THE ZERO COMBINATION IS NOT A COMBINATION, and it does not look like
	// one: key code 0 is ANSI's A, which a French keyboard prints as Q. A caller
	// that reached here with a Combo it never filled in would bind a bare letter
	// on the row -- so the empty value is refused here as well as at the call
	// site, because one guard is a guard and two are an invariant.
	if c.Key == 0 && c.Mods == 0 {
		return "", 0
	}
	key := equivalentFor(c.Key)
	if key == "" {
		// No character and no name this package knows: no key equivalent. The
		// modifiers alone would be a row that says ⌃⌥⌘ over nothing.
		return "", 0
	}
	var mods tray.Mods
	if c.Mods&hotkey.Command != 0 {
		mods |= tray.ModCommand
	}
	if c.Mods&hotkey.Shift != 0 {
		mods |= tray.ModShift
	}
	if c.Mods&hotkey.Option != 0 {
		mods |= tray.ModOption
	}
	if c.Mods&hotkey.Control != 0 {
		mods |= tray.ModControl
	}
	return key, mods
}

// equivalentFor is the one character AppKit wants for this key.
//
// ⚠ LOWER CASE for a letter. An UPPER-CASE key equivalent means "with Shift" to
// AppKit, and it draws the ⇧ to prove it -- so "S" on a row bound to ⌃⌥⌘S would
// come out as ⇧⌃⌥⌘S and send a person to press a fourth modifier. hotkey.Char
// answers in upper case because that is how a menu prints a letter, and this is
// where the two conventions meet.
//
// The keys with no character of their own are named, not translated: a menu
// draws an arrow there, and the constant is the Unicode private-use code AppKit
// has read as one since NeXT.
func equivalentFor(k hotkey.Key) string {
	if s, ok := trayGlyphKeys[k]; ok {
		return s
	}
	// What the keyboard prints, and the ANSI name when there is no keyboard to
	// ask -- which is every platform but macOS, where hotkey has no layout
	// service. The row is drawn there by a backend that ignores the key anyway,
	// but a value that is empty on one platform and "s" on another is a test
	// that passes in one place and not the other, and the difference would say
	// nothing about the row.
	ch := charOf(k)
	if ch == "" {
		ch = k.Glyph()
	}
	if len([]rune(ch)) != 1 {
		// A word -- "Space", "F1" -- or nothing. A menu draws neither as a key
		// equivalent, and half a name would be worse than no name.
		return ""
	}
	return strings.ToLower(ch)
}

// trayGlyphKeys are the keys a menu draws as a picture.
//
// Char answers a control character for these -- the left arrow is U+001C, the
// file separator the classic Mac put on the arrow keys -- and a menu row whose
// shortcut is an unprintable byte draws nothing at all.
var trayGlyphKeys = map[hotkey.Key]string{
	hotkey.KeyUpArrow:    tray.KeyUp,
	hotkey.KeyDownArrow:  tray.KeyDown,
	hotkey.KeyLeftArrow:  tray.KeyLeft,
	hotkey.KeyRightArrow: tray.KeyRight,
	hotkey.KeyReturn:     tray.KeyReturn,
	hotkey.KeyEscape:     tray.KeyEscape,
	hotkey.KeyDelete:     tray.KeyDelete,
	hotkey.KeyTab:        tray.KeyTab,
	hotkey.KeySpace:      tray.KeySpace,
	// The function keys, which have no character to print.
	//
	// ⛔ WITHOUT THESE A ROW BOUND TO F3 DRAWS NOTHING, and it draws nothing in
	// exactly the way a row nothing was granted for does -- so moving the two
	// galleries onto F3 and F4 quietly took their shortcuts out of the menu and
	// said nothing. Asked about in those words: the shortcuts were not there.
	hotkey.KeyF1:  tray.KeyF1,
	hotkey.KeyF2:  tray.KeyF2,
	hotkey.KeyF3:  tray.KeyF3,
	hotkey.KeyF4:  tray.KeyF4,
	hotkey.KeyF5:  tray.KeyF5,
	hotkey.KeyF6:  tray.KeyF6,
	hotkey.KeyF7:  tray.KeyF7,
	hotkey.KeyF8:  tray.KeyF8,
	hotkey.KeyF9:  tray.KeyF9,
	hotkey.KeyF10: tray.KeyF10,
	hotkey.KeyF11: tray.KeyF11,
	hotkey.KeyF12: tray.KeyF12,
	hotkey.KeyF13: tray.KeyF13,
	hotkey.KeyF14: tray.KeyF14,
	hotkey.KeyF15: tray.KeyF15,
}

// charOf is what a key prints, as a seam: hotkey has a layout service on macOS
// and none anywhere else, so the branch that matters can only be taken on one
// platform unless a test can install a keyboard.
var charOf = hotkey.Key.Char
