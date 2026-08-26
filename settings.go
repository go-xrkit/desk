// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"strconv"
	"strings"

	"github.com/go-widgets/toolkit"
	"github.com/go-xrkit/xrkit/glasses"
)

// settingsRoot builds the widget tree and returns it with the function that
// reads the controls back into the settings.
//
// It is separate from [RunSettings] so that what the window DOES is not tangled
// with opening one: this half needs no display, and a caller with a headless
// machine can still ask what the controls would produce.
func settingsRoot(cfg *Config, attached []glasses.USB, logf func(string, ...any),
	closeWindow func()) (toolkit.Widget, func()) {

	// A nil Logf says nothing, here as everywhere else. It matters more here
	// than usual: this one is only ever called from a button, so a nil left
	// unguarded is a window that works perfectly until somebody presses Save.
	if logf == nil {
		logf = func(string, ...any) {}
	}

	// Vertical, and every row sized. A BoxLayout is HORIZONTAL by default and
	// gives an item with no config an EQUAL FLEX SHARE — so the first version of
	// this put eleven controls side by side, each a tenth of the window wide.
	// It was reported, accurately, as looking like nothing.
	col := toolkit.NewBoxLayout()
	col.Vertical = true
	col.Spacing = 12
	box := toolkit.NewContainer(col)

	// Which glasses, when several are attached. A list rather than a drop-down:
	// the whole point is to SEE that there are two and which one is which.
	names := headsetNames(attached)
	list := toolkit.NewListBox(names)
	chosen := -1
	for i, n := range names {
		if n == cfg.Model() {
			chosen = i
		}
	}
	// Nothing chosen yet selects the first, so that what the window SHOWS and
	// what it would save are the same thing. A list with a highlight on nothing
	// and a Save that then records nothing is a window that quietly disagrees
	// with itself.
	if chosen < 0 && len(names) > 0 {
		chosen = 0
	}
	if chosen >= 0 {
		list.Selected().Set(chosen)
	}
	// Sized to its rows, not given the slack. A list of two headsets stretched
	// over half the window is a large empty box with its caption stranded at
	// the bottom, which is what this looked like the first time.
	if len(names) == 0 {
		box.Add(toolkit.Item{Size: fieldH(1), Widget: toolkit.NewFormField(
			"Glasses on the bus: none; name one with -screen",
			toolkit.NewLabel("nothing attached"))})
	} else {
		box.Add(toolkit.Item{Size: fieldH(len(names)), Widget: toolkit.NewFormField(
			"Glasses: which headset when several are attached", list)})
	}

	// How many screens.
	counts := make([]string, len(screenCounts))
	pick := 0
	for i, n := range screenCounts {
		counts[i] = strconv.Itoa(n)
		if n == cfg.Screens() {
			pick = i
		}
	}
	count := toolkit.NewDropDown(counts, pick)
	box.Add(toolkit.Item{Size: fieldH(1) + 8, Widget: toolkit.NewFormField(
		"Screens on the band: 3, 6 and 9 fold into a gallery three columns wide",
		count)})

	// The menu bar, which is the thing everyone asks about.
	// No apostrophe and no dashes anywhere in this window's words. The
	// toolkit's font has neither an apostrophe nor an em dash, so "the glasses'
	// own menu bar" rendered as "the glasses  own menu bar" — a hole in the
	// middle of a sentence, which is what "hard to read" was made of.
	immersive := toolkit.NewCheckButton(
		"Cover the local menu bar and Dock on the glasses", cfg.Immersive())
	box.Add(toolkit.Item{Widget: immersive, Size: 24})

	// What the machine will actually grant. Shown, not logged: of the three
	// ways a shortcut can already be taken, only two are detectable.
	shortcuts := toolkit.NewContainer(func() *toolkit.BoxLayout {
		l := toolkit.NewBoxLayout()
		l.Vertical = true
		l.Spacing = 2
		return l
	}())
	lines := 0
	for _, line := range wrapShortcuts(whatWeGet(cfg)) {
		shortcuts.Add(toolkit.Item{Widget: toolkit.NewLabel(line), Size: rowH})
		lines++
	}
	// No Help caption anywhere: the toolkit draws one in theme.Border, which on
	// the dark theme is very nearly the background. Whatever it said could not
	// be read, so it is said in the label instead.
	box.Add(toolkit.Item{Size: fieldH(lines), Widget: toolkit.NewFormField(
		"Shortcuts this machine granted, not what was asked for",
		shortcuts)})

	read := func() {
		if i := list.Selected().Get(); len(names) > 0 && i >= 0 && i < len(names) {
			cfg.Glasses = &ConfigGlasses{Model: &names[i]}
		}
		if i := count.Selected().Get(); i >= 0 && i < len(screenCounts) {
			n := screenCounts[i]
			if cfg.Ribbon == nil {
				cfg.Ribbon = &ConfigRibbon{}
			}
			cfg.Ribbon.Screens = &n
		}
		// The block exists by now: the screen count above always makes one, and
		// the drop-down always has a selection.
		on := immersive.Checked().Get()
		cfg.Ribbon.Immersive = &on
	}

	row := toolkit.NewBoxLayout()
	row.Spacing = 8
	row.Pack = toolkit.PackEnd
	buttons := toolkit.NewContainer(row)
	buttons.Add(toolkit.Item{Size: 96, Widget: toolkit.NewButton("Save", func() {
		read()
		path, err := cfg.Save()
		if err != nil {
			logf("%v", err)
			return
		}
		logf("saved to %s", path)
		closeWindow()
	})})
	buttons.Add(toolkit.Item{Widget: toolkit.NewButton("Close", func() { closeWindow() }), Size: 96})
	// The buttons are pinned to the bottom; the fields take what is above them.
	frame := toolkit.NewContainer(toolkit.BorderLayout{})
	frame.Add(toolkit.Item{Widget: box, Region: toolkit.RegionCenter})
	frame.Add(toolkit.Item{Widget: buttons, Region: toolkit.RegionSouth, Size: 40})

	return pad(frame, 20, 16), read
}

// headsetNames is what the catalogue calls each headset on the bus.
func headsetNames(attached []glasses.USB) []string {
	out := make([]string, 0, len(attached))
	for i := range attached {
		p, how := glasses.IdentifyDevice("", &attached[i])
		if how != glasses.ByUSBProduct {
			continue
		}
		out = append(out, p.Model)
	}
	return out
}

// whatWeGet asks the machine what it will give, and gives it straight back.
//
// It claims and releases: there is no way to find out what a shortcut would be
// except to take it, and holding one while a settings window is open would take
// it from whatever the person is about to go back to.
func whatWeGet(cfg *Config) string {
	h := ClaimGlobal(cfg.ShortcutsOr(DefaultShortcuts()), cfg.HotkeyOptions())
	defer h.Close()
	// Spelled out, not in glyphs: the toolkit's font has no ⌥ ⌘ ⇧ ⌃ ← →, and a
	// line saying which combination was granted rendered as "previous:" and
	// nothing else.
	return h.DescribeNames()
}

// SettingsWidth is how wide the settings window is. Its HEIGHT depends on what
// the machine has to say; see settingsHeight.
const SettingsWidth = 560

// rowH is one line of a list or a caption, and fieldH is a form field holding n
// of them: its label, its child, and the padding the toolkit puts between.
const rowH = 20

// settingsHeight is how tall the window has to be for THIS machine's settings.
//
// It is computed rather than fixed. How much this window has to say depends on
// the machine: one that grants no global shortcut at all reports three long
// refusals instead of three short grants, and a window sized for the short
// version puts its Save button below the frame — measured on Linux, where
// exactly that happened. A window whose buttons have fallen off the end is
// worse than an ugly one.
func settingsHeight(cfg Config, attached []glasses.USB) int {
	const spacing, buttons, margins = 12, 40, 32
	rows := len(headsetNames(attached))
	if rows == 0 {
		rows = 1
	}
	h := fieldH(rows) + fieldH(1) + 8 + 24 +
		fieldH(len(wrapShortcuts(whatWeGet(&cfg))))
	return h + 4*spacing + buttons + margins
}

func fieldH(n int) int {
	if n < 1 {
		n = 1
	}
	return 26 + n*rowH + 6
}

// pad wraps a widget in a margin, because a window whose text starts at its
// own left edge reads as spilled rather than laid out.
func pad(child toolkit.Widget, x, y int) toolkit.Widget {
	c := toolkit.NewContainer(toolkit.BorderLayout{})
	c.Add(toolkit.Item{Widget: toolkit.NewContainer(nil), Region: toolkit.RegionNorth, Size: y})
	c.Add(toolkit.Item{Widget: toolkit.NewContainer(nil), Region: toolkit.RegionSouth, Size: y})
	c.Add(toolkit.Item{Widget: toolkit.NewContainer(nil), Region: toolkit.RegionWest, Size: x})
	c.Add(toolkit.Item{Widget: toolkit.NewContainer(nil), Region: toolkit.RegionEast, Size: x})
	c.Add(toolkit.Item{Widget: child, Region: toolkit.RegionCenter})
	return c
}

// shortcutCols is how many characters of a shortcut line fit the window.
//
// Measured from the render rather than guessed: the line saying which
// combination the gallery got ran off the right edge at "it was taken)", so
// the reader was told everything except the part that mattered.
const shortcutCols = 58

// wrapShortcuts breaks the shortcut report into lines that fit.
//
// It breaks at the parenthesis, because that is where the sentence divides:
// what was granted, then what was asked for. Anywhere else would split a
// combination in half, and half a combination is worse than none.
func wrapShortcuts(report string) []string {
	var out []string
	for _, line := range strings.Split(strings.TrimRight(report, "\n"), "\n") {
		if line == "" {
			continue
		}
		if len(line) <= shortcutCols {
			out = append(out, line)
			continue
		}
		if i := strings.Index(line, " ("); i > 0 {
			out = append(out, line[:i], "    "+line[i+1:])
			continue
		}
		out = append(out, line)
	}
	return out
}
