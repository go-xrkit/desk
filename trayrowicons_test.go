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

	"github.com/go-macos/hotkey"
	"github.com/go-widgets/tray"
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

	// Before anything is claimed the rows carry NO key equivalent: the item is
	// made before any session and must not name a combination it has not been
	// given.
	_, _, menu := h.Snapshot()
	for i, it := range menu.Items {
		if it.Key != "" || it.Mods != 0 {
			t.Errorf("row %d names %q+%b before a single key was claimed",
				i, it.Key, it.Mods)
		}
		// And never in the LABEL either: appending it there is what this
		// replaced, and it left the combination adrift in the middle of the row
		// instead of in the column a menu is read down.
		if strings.Contains(it.Label, "⌘") {
			t.Errorf("row %d printed %q in its label", i, it.Label)
		}
	}

	const mods = hotkey.Control | hotkey.Option | hotkey.Command
	item.ShowShortcuts(map[Action]hotkey.Combo{
		ActionSettings: {Key: hotkey.KeyS, Mods: mods},
		ActionQuit:     {Key: hotkey.KeyEscape, Mods: mods},
	})
	waitFor(t, func() bool {
		_, _, m := h.Snapshot()
		return m != nil && len(m.Items) > 0 && m.Items[0].Key == "s"
	}, "the combinations to arrive")

	_, _, menu = h.Snapshot()
	rows := TrayRows()
	if len(menu.Items) != len(rows) {
		t.Fatalf("the menu has %d rows, want %d", len(menu.Items), len(rows))
	}
	for i, r := range rows {
		it := menu.Items[i]
		// ⛔ THE LABEL IS ALWAYS JUST THE LABEL. The combination is drawn beside
		// it by the platform, right-aligned in a column with every other row's.
		if r.Action != ActionNone && it.Label != r.Title {
			t.Errorf("row %d is labelled %q, want %q", i, it.Label, r.Title)
		}
		switch r.Action {
		case ActionSettings:
			if it.Key != "s" {
				t.Errorf("row %d names the key %q, want %q", i, it.Key, "s")
			}
			if want := tray.ModControl | tray.ModOption | tray.ModCommand; it.Mods != want {
				t.Errorf("row %d names the modifiers %b, want %b", i, it.Mods, want)
			}
		case ActionQuit:
			if it.Key != tray.KeyEscape {
				t.Errorf("row %d names %q for Escape; a menu draws a glyph there",
					i, it.Key)
			}
		case ActionNone:
			if !it.Separator {
				t.Errorf("row %d stopped being a separator: %q", i, it.Label)
			}
		default:
			if it.Key != "" || it.Mods != 0 {
				t.Errorf("row %d names %q+%b; nothing was granted for it",
					i, it.Key, it.Mods)
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
	none.ShowShortcuts(map[Action]hotkey.Combo{ActionQuit: {Key: hotkey.KeyEscape, Mods: hotkey.Command}})
}

// TestTheKeyEquivalentAMenuRowDraws.
//
// ⛔ Lower case for a letter, and a GLYPH for a key that has no character. Both
// are AppKit conventions with a visible failure: an upper-case key equivalent
// means "with Shift" and draws a ⇧ nobody asked for, and a key whose character
// is a control byte -- the left arrow is U+001C, the file separator the classic
// Mac put on the arrows -- draws nothing at all.
func TestTheKeyEquivalentAMenuRowDraws(t *testing.T) {
	const mods = hotkey.Control | hotkey.Option | hotkey.Command
	all := tray.ModControl | tray.ModOption | tray.ModCommand

	for _, c := range []struct {
		name string
		in   hotkey.Combo
		key  string
		mods tray.Mods
	}{
		{"a letter is lower case", hotkey.Combo{Key: hotkey.KeyS, Mods: mods}, "s", all},
		{"an arrow is a glyph", hotkey.Combo{Key: hotkey.KeyLeftArrow, Mods: mods}, tray.KeyLeft, all},
		{"escape is a glyph", hotkey.Combo{Key: hotkey.KeyEscape, Mods: mods}, tray.KeyEscape, all},
		{"delete is a glyph", hotkey.Combo{Key: hotkey.KeyDelete, Mods: mods}, tray.KeyDelete, all},
		{"shift is carried", hotkey.Combo{Key: hotkey.KeyS, Mods: mods | hotkey.Shift}, "s", all | tray.ModShift},
		// ⛔ The zero value is not a combination. Key code 0 is ANSI's A, which
		// a French keyboard prints as Q -- so a row nothing was granted for
		// would bind a BARE letter.
		{"nothing is nothing", hotkey.Combo{}, "", 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			key, mods := trayKey(c.in)
			if key != c.key {
				t.Errorf("key = %q, want %q", key, c.key)
			}
			if mods != c.mods {
				t.Errorf("mods = %b, want %b", mods, c.mods)
			}
			for _, r := range key {
				if r < 0x20 && r != '\r' && r != '\t' && r != '\b' && r != 0x1b {
					t.Errorf("the key equivalent is %#U, which draws nothing", r)
				}
			}
		})
	}
}

// TestAKeyWithNoNameAndNoCharacterNamesNothing, rather than a modifier mask
// over an empty string: a row saying ⌃⌥⌘ and then nothing is worse than a row
// saying nothing at all.
func TestAKeyWithNoNameAndNoCharacterNamesNothing(t *testing.T) {
	key, mods := trayKey(hotkey.Combo{Key: hotkey.Key(0xFE), Mods: hotkey.Command})
	if key != "" || mods != 0 {
		t.Errorf("an unnamed key gave %q+%b", key, mods)
	}
}

// TestTheSameKeyEquivalentOnEveryPlatform.
//
// ⛔ hotkey has a layout service on macOS and none anywhere else, so Char is
// the keyboard's answer on one platform and "" on the others. A row whose key
// equivalent came out "s" on a Mac and "" on a Linux runner would be a test
// that passes in one place and fails in the other while nothing is wrong -- and
// that is exactly how this was found.
func TestTheSameKeyEquivalentOnEveryPlatform(t *testing.T) {
	was := charOf
	t.Cleanup(func() { charOf = was })

	// With a keyboard that answers, and with one that does not.
	for _, keyboard := range []struct {
		name string
		char func(hotkey.Key) string
	}{
		{"a system that can say what a key prints", func(k hotkey.Key) string { return k.Glyph() }},
		{"a system that cannot", func(hotkey.Key) string { return "" }},
	} {
		t.Run(keyboard.name, func(t *testing.T) {
			charOf = keyboard.char
			if got := equivalentFor(hotkey.KeyS); got != "s" {
				t.Errorf("S = %q, want %q", got, "s")
			}
			if got := equivalentFor(hotkey.KeyN1); got != "1" {
				t.Errorf("1 = %q, want %q", got, "1")
			}
			if got := equivalentFor(hotkey.KeyLeftArrow); got != tray.KeyLeft {
				t.Errorf("the left arrow = %q, want the glyph key", got)
			}
			// A key whose name is a WORD is no key equivalent: half of "F1"
			// would be worse than none of it.
			if got := equivalentFor(hotkey.KeyF1); got != "" {
				t.Errorf("F1 = %q, want nothing", got)
			}
		})
	}
}

// TestOneRowForThreeDWithATick.
//
// ⛔ IT WAS TWO ROWS, and that is right for a KEY: a shortcut is pressed blind,
// so one meaning "on" from outside and "off" from inside does the wrong thing
// every time somebody has lost track. A MENU is the opposite case -- the state
// is in front of the person as they choose -- so two rows are two rows where
// one always does nothing, and the tick says which.
func TestOneRowForThreeDWithATick(t *testing.T) {
	rows := TrayRows()
	n := 0
	for _, r := range rows {
		switch r.Action {
		case ActionStereo3DOn, ActionStereo3DOff:
			t.Errorf("%q is still a row of its own", r.Title)
		case ActionStereo3D:
			n++
			if !r.Toggle {
				t.Errorf("%q is not a checkbox", r.Title)
			}
		}
	}
	if n != 1 {
		t.Errorf("%d rows toggle 3D, want one", n)
	}
}

// TestTheTickSaysWhatThePictureIs.
//
// ⛔ What HAPPENED, not what was asked. Turning 3D on can be refused by a
// display that shows one eye or by a depth model that will not load, and a tick
// that followed the request would say the desk is in a state it is not -- so a
// person would press it again to fix something that was never on.
func TestTheTickSaysWhatThePictureIs(t *testing.T) {
	h := headless(t)
	actions := make(chan Action, TrayQueue)
	item, err := OpenTray(nil, actions)
	if err != nil {
		t.Fatalf("OpenTray = %v", err)
	}
	defer func() { _ = item.Close() }()
	go func() { _ = item.Hold() }()
	waitFor(t, func() bool {
		_, _, m := h.Snapshot()
		return m != nil && len(m.Items) > 0
	}, "the menu to arrive")

	tick := func() bool {
		_, _, m := h.Snapshot()
		for i, r := range TrayRows() {
			if r.Action == ActionStereo3D {
				return m.Items[i].Checked
			}
		}
		t.Fatal("no 3D row")
		return false
	}
	if tick() {
		t.Error("3D is ticked before anything turned it on")
	}

	item.Show3D(Stereo3D{On: true})
	waitFor(t, tick, "the tick to arrive")

	// ⛔ AND THE COMBINATIONS SURVIVE IT. The other thing that rebuilds this
	// menu is the shortcut report, and a rebuild that dropped it would take
	// every key equivalent off the menu the first time somebody pressed 3D.
	const mods = hotkey.Control | hotkey.Option | hotkey.Command
	item.ShowShortcuts(map[Action]hotkey.Combo{
		ActionSettings: {Key: hotkey.KeyS, Mods: mods},
	})
	item.Show3D(Stereo3D{})
	waitFor(t, func() bool { return !tick() }, "the tick to go away")

	_, _, m := h.Snapshot()
	if m.Items[0].Key != "s" {
		t.Errorf("the first row's key equivalent is %q after a 3D rebuild: "+
			"the combinations were thrown away", m.Items[0].Key)
	}
}

// TestShow3DOnNoItemDoesNothing: OpenTray can fail, and main carries on.
func TestShow3DOnNoItemDoesNothing(t *testing.T) {
	var none *Tray
	none.Show3D(Stereo3D{On: true})
}

// TestTellingTheItemSomethingItAlreadyKnowsRebuildsNothing.
//
// A rebuild replaces every NSMenuItem in the bar. Doing it for a state that has
// not changed is work for nothing on a path the frame loop can reach -- and the
// 3D state is reported on EVERY press, including the ones that were refused and
// changed nothing.
func TestTellingTheItemSomethingItAlreadyKnowsRebuildsNothing(t *testing.T) {
	h := headless(t)
	actions := make(chan Action, TrayQueue)
	item, err := OpenTray(nil, actions)
	if err != nil {
		t.Fatalf("OpenTray = %v", err)
	}
	defer func() { _ = item.Close() }()
	go func() { _ = item.Hold() }()
	waitFor(t, func() bool {
		_, _, m := h.Snapshot()
		return m != nil && len(m.Items) > 0
	}, "the menu to arrive")

	item.Show3D(Stereo3D{On: true})
	waitFor(t, func() bool {
		_, _, m := h.Snapshot()
		return m.Items[threeDRow(t)].Checked
	}, "the tick to arrive")

	// ⛔ THE MENU'S IDENTITY, READ UNDER THE BACKEND'S OWN LOCK. Reading
	// h.Refreshes directly is a DATA RACE and the race detector said so, once
	// in several runs: this item binds its icon to an observable and refreshes
	// from the animator's goroutine every second, so the counter moves while a
	// test looks at it. Snapshot is the accessor that takes the lock, and a
	// rebuild is visible in it anyway -- buildMenu makes a NEW menu, so an
	// unchanged pointer is a menu that was not rebuilt.
	_, _, before := h.Snapshot()
	for range 5 {
		item.Show3D(Stereo3D{On: true})
	}
	if _, _, after := h.Snapshot(); after != before {
		t.Error("saying the same thing five times rebuilt the menu")
	}
}

// TestOnlyTheThreeDRowIsATick: stateFor answers for one action and no other, so
// a row that was never meant to carry a tick does not sprout one.
func TestOnlyTheThreeDRowIsATick(t *testing.T) {
	if on, _ := stateFor(ActionStereo3D, Stereo3D{On: true}); !on {
		t.Error("the 3D row does not follow the state")
	}
	if on, _ := stateFor(ActionStereo3D, Stereo3D{}); on {
		t.Error("the 3D row is ticked with 3D off")
	}
	if _, why := stateFor(ActionStereo3D, Stereo3D{Why: "one eye"}); why != "one eye" {
		t.Errorf("the reason came back %q", why)
	}
	for _, a := range []Action{ActionQuit, ActionSettings, ActionPhoto, ActionNone} {
		on, why := stateFor(a, Stereo3D{On: true, Why: "one eye"})
		if on || why != "" {
			t.Errorf("%v carries a state: %v %q", a, on, why)
		}
	}
}

// TestARowThatCannotDoAnythingSaysSoAndIsGreyed.
//
// ⛔ THE REPORT, exactly. On the headset it was written for, the log said
// "3D asked for, but this display shows one eye" and the menu said NOTHING
// WHATEVER -- so the row looked pressable and did nothing, every time. A menu
// that cannot do a thing has to say it cannot, and why.
func TestARowThatCannotDoAnythingSaysSoAndIsGreyed(t *testing.T) {
	h := headless(t)
	actions := make(chan Action, TrayQueue)
	item, err := OpenTray(nil, actions)
	if err != nil {
		t.Fatalf("OpenTray = %v", err)
	}
	defer func() { _ = item.Close() }()
	go func() { _ = item.Hold() }()
	waitFor(t, func() bool {
		_, _, m := h.Snapshot()
		return m != nil && len(m.Items) > 0
	}, "the menu to arrive")

	const why = "this display shows one eye"
	item.Show3D(Stereo3D{Why: why})
	waitFor(t, func() bool {
		_, _, m := h.Snapshot()
		return m.Items[threeDRow(t)].Disabled
	}, "the row to be greyed")

	_, _, m := h.Snapshot()
	row := m.Items[threeDRow(t)]
	if !strings.Contains(row.Label, why) {
		t.Errorf("the row reads %q; it should say why it cannot", row.Label)
	}
	if row.Checked {
		t.Error("a conversion that cannot run is ticked")
	}

	// And when it becomes possible again, the row comes back.
	item.Show3D(Stereo3D{})
	waitFor(t, func() bool {
		_, _, m := h.Snapshot()
		return !m.Items[threeDRow(t)].Disabled
	}, "the row to come back")
	if _, _, m := h.Snapshot(); m.Items[threeDRow(t)].Label != "3D" {
		t.Errorf("the row reads %q once it works again", m.Items[threeDRow(t)].Label)
	}
}

// TestTheSymbolSaysTheStateToo.
//
// ⛔ A TICK SAYS "ON" AND NOTHING SAYS "OFF": macOS draws a checkmark for a
// menu item that is on and NOTHING AT ALL for one that is off, so an unticked
// checkbox is indistinguishable from an ordinary row. The symbol is what
// answers in both directions, and it is the pair the system itself uses.
func TestTheSymbolSaysTheStateToo(t *testing.T) {
	h := headless(t)
	actions := make(chan Action, TrayQueue)
	item, err := OpenTray(nil, actions)
	if err != nil {
		t.Fatalf("OpenTray = %v", err)
	}
	defer func() { _ = item.Close() }()
	go func() { _ = item.Hold() }()
	waitFor(t, func() bool {
		_, _, m := h.Snapshot()
		return m != nil && len(m.Items) > 0
	}, "the menu to arrive")

	icon := func() []byte {
		_, _, m := h.Snapshot()
		return m.Items[threeDRow(t)].Icon
	}
	off := icon()
	if len(off) == 0 {
		t.Skip("this machine draws no symbols, so there is nothing to compare")
	}
	item.Show3D(Stereo3D{On: true})
	waitFor(t, func() bool {
		_, _, m := h.Snapshot()
		return m.Items[threeDRow(t)].Checked
	}, "the tick to arrive")
	if on := icon(); string(on) == string(off) {
		t.Error("the row draws the same symbol with 3D on and off, so the " +
			"picture says nothing that the tick did not")
	}
}

// threeDRow is where the 3D row sits in the menu.
func threeDRow(t *testing.T) int {
	t.Helper()
	for i, r := range TrayRows() {
		if r.Action == ActionStereo3D {
			return i
		}
	}
	t.Fatal("no 3D row")
	return -1
}

// TestChoosingTheThreeDRowTogglesIt.
//
// ⛔ A checkbox row is an ordinary row that also shows a state. Its tick is
// flipped by the menu before the callback runs, and the ACTION it sends must
// still be the toggle -- a row that changed its own tick and told nobody would
// be a menu that lies until the next rebuild.
func TestChoosingTheThreeDRowTogglesIt(t *testing.T) {
	h := headless(t)
	actions := make(chan Action, TrayQueue)
	item, err := OpenTray(nil, actions)
	if err != nil {
		t.Fatalf("OpenTray = %v", err)
	}
	defer func() { _ = item.Close() }()
	go func() { _ = item.Hold() }()
	waitFor(t, func() bool {
		_, _, m := h.Snapshot()
		return m != nil && len(m.Items) > 0
	}, "the menu to arrive")

	_, _, menu := h.Snapshot()
	menu.Items[threeDRow(t)].Activate()
	select {
	case a := <-actions:
		if a != ActionStereo3D {
			t.Errorf("the 3D row sent %v", a)
		}
	default:
		t.Error("the 3D row sent nothing")
	}
}
