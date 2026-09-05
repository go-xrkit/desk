// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"testing"

	"github.com/go-macos/hotkey"
	"github.com/go-widgets/tray"
)

// TestAFunctionKeyDrawsItsRow.
//
// ⛔ THE DEFECT WAS SILENT AND IT LOOKED LIKE NOTHING. A function key has no
// character, so equivalentFor answered "" -- which is also what it answers for
// a row nothing was granted for. Moving the two galleries onto ⌃⌥⌘F3 and
// ⌃⌥⌘F4 therefore took their shortcuts out of the menu with no error anywhere,
// and the only report was a person saying the shortcuts were not shown.
func TestAFunctionKeyDrawsItsRow(t *testing.T) {
	for _, c := range []struct {
		k    hotkey.Key
		want string
	}{
		{hotkey.KeyF1, tray.KeyF1},
		{hotkey.KeyF3, tray.KeyF3},
		{hotkey.KeyF10, tray.KeyF10},
		{hotkey.KeyF12, tray.KeyF12},
	} {
		key, mods := trayKey(hotkey.Combo{Key: c.k, Mods: hotkey.Control | hotkey.Option | hotkey.Command})
		if key != c.want {
			t.Errorf("%v draws %q, want %q", c.k, key, c.want)
		}
		if mods != tray.ModControl|tray.ModOption|tray.ModCommand {
			t.Errorf("%v carries the wrong modifiers: %v", c.k, mods)
		}
	}
}

// TestTheAtKeyDrawsWhatIsPrintedOnIt: the key that puts the glasses down is
// the ISO one, and a menu has to say so with the character the person sees.
func TestTheAtKeyDrawsWhatIsPrintedOnIt(t *testing.T) {
	was := charOf
	t.Cleanup(func() { charOf = was })
	charOf = func(k hotkey.Key) string {
		if k == hotkey.KeyISOSection {
			return "@"
		}
		return ""
	}
	key, _ := trayKey(hotkey.Combo{Key: hotkey.KeyISOSection, Mods: hotkey.Command})
	if key != "@" {
		t.Errorf("the ISO key draws %q, want the legend on it", key)
	}
}
