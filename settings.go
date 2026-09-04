// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"fmt"
	"strings"
	"sync"

	"github.com/go-widgets/toolkit"
	"github.com/go-xrkit/xrkit/glasses"
)

// Sizes of the settings window and its controls, in LOGICAL pixels: everything
// here goes through [toolkit.Scaled] at use, so one knob magnifies the whole
// window and nothing is a device-pixel constant.
const (
	// SettingsWidth is how wide the window is. Its HEIGHT is measured, not
	// declared -- see settingsSize.
	SettingsWidth = 560
	// SettingsPadX and SettingsPadY are the window's margins.
	SettingsPadX = 16
	SettingsPadY = 12
	// ButtonBarH is the band the Save/Close row keeps at the bottom. It is a
	// BorderLayout region, so the content above it can never reach into it --
	// which is what a fixed-height run of fields used to do when the window was
	// made smaller: the buttons ended up drawn over the text.
	ButtonBarH = 40
	// ButtonW is one button.
	ButtonW = 96
	// ControlW and ControlH size the trailing control of a
	// settings row. A SettingRow right-aligns a control at the size the control
	// carries, so these are given to the widget rather than to the row.
	ControlW = 220
	ControlH = 28
	// SwitchW and SwitchH size a switch, which is wider than it is tall.
	SwitchW = 44
	SwitchH = 24
	// PageSpacing is the gap between the cards on the page.
	PageSpacing = 12
	// GlassesIconPx is the drawn headset in a glasses tile. Bigger than a
	// toolbar icon, because the tile is the thing being chosen rather than a
	// decoration beside a word.
	GlassesIconPx = 40
	// GlassesTileW is the floor on a tile's width, in logical pixels: enough for
	// the longest model name in the catalogue rather than for the icon.
	GlassesTileW = 150
)

// settingsPage builds the measurable half of the window: the cards, the controls
// in them, and the function that reads the controls back into the settings.
//
// It is separate from [settingsRoot] because it is the part that can say how big
// it is. A page of [toolkit.SettingsGroup] cards measures itself -- each row
// reports its own height, the group sums them -- so the window's height is asked
// of the tree rather than computed from a table of pixel constants. That table
// was the defect: it is right at one window size and wrong at every other.
func settingsPage(cfg *Config, attached []glasses.USB) (*toolkit.Container, func()) {
	// Which glasses, when several are attached: a tile each, not a list.
	//
	// A list of two names says which two are on the bus and nothing else, and
	// picking hardware from a line of text is the one job a picture is actually
	// better at. So each headset is a tile with a drawn pair of glasses and its
	// model underneath, and the chosen one is the selected cell.
	//
	// DRAWN, not photographed. A photograph of a manufacturer's product is the
	// manufacturer's picture, and this application has nowhere to fetch one from:
	// it runs with no network and ships no artwork.
	// toolkit.DrawIconGlasses is in the stock icon set for exactly this, and
	// the SYSTEM's own symbol is used instead where there is one -- the same
	// glyph the menu bar carries, so the window and the bar do not disagree
	// about what a pair of glasses looks like. Measured at 44 pixels, the drawn
	// outline inks 7% of its box and the system symbol 62%, which is the
	// difference between a shape you notice and one you do not.
	names := headsetNames(attached)
	icon := toolkit.DrawIconGlasses
	if sym := glassesIcon(toolkit.Scaled(GlassesIconPx)); sym != nil {
		icon = sym
	}
	cells := make([]toolkit.IconCell, 0, len(names))
	for _, n := range names {
		cells = append(cells, toolkit.IconCell{
			Icon:  icon,
			Label: n,
			Key:   n,
		})
	}
	tiles := toolkit.NewIconGrid(cells...)
	tiles.SetIconSize(toolkit.Scaled(GlassesIconPx))
	// Wide enough for a model name: a tile whose label is elided to "VITURE ..."
	// has lost the one thing it exists to say.
	tiles.MinCellW = GlassesTileW
	tiles.Empty = "none on the bus; name one with -screen"
	chosen := 0
	for i, n := range names {
		if n == cfg.Model() {
			chosen = i
		}
	}
	if len(names) > 0 {
		// Nothing chosen yet selects the first, so what the window SHOWS and
		// what it would save are the same thing. A grid with no highlight and a
		// Save that then records nothing is a window disagreeing with itself.
		tiles.SetSelected(chosen)
	}
	glassesCard := toolkit.NewFrame(tiles)
	glassesCard.Title = "Glasses: which headset when several are attached"

	// How many screens is NOT here.
	//
	// It was a drop-down, and it is the gallery's now: the adder tile puts a
	// screen on the band where a person can see the band, which is the moment
	// they know whether they want another one. A number chosen in a dialogue
	// before any of it is on screen is a guess, and two ways to set one thing is
	// one way too many. `-screens` and the settings file still carry it, for a
	// script and for somebody who has decided.

	// The menu bar, which is the thing everyone asks about.
	//
	// A switch rather than a check button: it is a setting that is on or off,
	// and it sits in the trailing slot of a row whose words are the label. No
	// apostrophe and no dashes anywhere in this window's text -- the built-in
	// font has neither, and "the glasses' own menu bar" came out as a hole in
	// the middle of a sentence. The system face has them, but the window falls
	// back to the built-in one on a platform that offers no face at all.
	immersive := toolkit.NewSwitch(cfg.Immersive())
	immersive.SetBounds(toolkit.Rect{
		W: toolkit.Scaled(SwitchW), H: toolkit.Scaled(SwitchH)})

	deskCard := toolkit.NewSettingsGroup("The desk",
		&toolkit.SettingRow{
			Title:    "Cover the local menu bar and Dock",
			Subtitle: "the glasses own the whole panel",
			Control:  immersive,
		},
	)

	// ONLY WHAT WENT WRONG.
	//
	// Every combination used to be listed here, one row per action, and there
	// are twenty-nine of them: a page too tall for a laptop screen, saying the
	// same thing the menu-bar menu now says beside each row it belongs to. A
	// person looking for "which key opens the gallery" is better served by the
	// menu, where the key sits next to the thing it does.
	//
	// What a menu CANNOT say is what it did not get. A combination another
	// application holds is substituted or refused, and of the three ways one can
	// be taken only two are detectable at all -- so the exceptions are the whole
	// reason this card exists, and now they are all it contains.
	keysCard := toolkit.NewSettingsGroup(
		"Shortcuts this machine would not grant",
		shortcutRows(cfg)...)

	l := toolkit.NewBoxLayout()
	l.Vertical = true
	l.Spacing = toolkit.Scaled(PageSpacing)
	page := toolkit.NewContainer(l)
	// Natural, not a size: each card is as tall as its own rows say, re-measured
	// every time the window changes shape.
	page.Add(toolkit.Item{Widget: glassesCard, Natural: true})
	page.Add(toolkit.Item{Widget: deskCard, Natural: true})
	page.Add(toolkit.Item{Widget: keysCard, Natural: true})

	read := func() {
		if i := tiles.Selected().Get(); len(names) > 0 && i >= 0 && i < len(names) {
			cfg.Glasses = &ConfigGlasses{Model: &names[i]}
		}
		if cfg.Ribbon == nil {
			cfg.Ribbon = &ConfigRibbon{}
		}
		on := immersive.On().Get()
		cfg.Ribbon.Immersive = &on
	}
	return page, read
}

// shortcutRows is one row per shortcut the machine granted, its combination in
// the trailing slot.
func shortcutRows(cfg *Config) []*toolkit.SettingRow {
	return shortcutRowsFrom(whatWeGet(cfg))
}

// shortcutRowsFrom is the same thing from a report already in hand: whatWeGet
// has to CLAIM the shortcuts to find out what it would get, which needs a real
// window server, so the shaping of the answer is separated from the asking.
func shortcutRowsFrom(report string) []*toolkit.SettingRow {
	var out []*toolkit.SettingRow
	// No blank-line skip: troubleOnly keeps only the lines carrying one of two
	// markers, and a blank line carries neither.
	for _, line := range troubleOnly(strings.Split(strings.TrimSpace(report), "\n")) {
		title, keys, ok := strings.Cut(line, ":")
		if !ok {
			// A refusal rather than a grant: it has no combination to put in the
			// trailing slot, so the sentence is the label -- across BOTH of the
			// row's text lines, because a refusal is three times the length of a
			// grant and a row draws each line without wrapping it.
			head, tail := splitToFit(strings.TrimSpace(line), cardRoom())
			out = append(out, &toolkit.SettingRow{Title: head, Subtitle: tail})
			continue
		}
		// The combination goes in the trailing slot; anything the machine adds
		// in brackets -- what was ASKED for, when it had to be substituted --
		// goes under the name instead.
		//
		// It used to be one string, and the gallery's line is "the granted
		// combination (asked for the other one, it was taken)": three times the
		// width of the slot, so the row drew it straight over its own title and
		// neither could be read. A row places its control at the size the
		// control carries -- it does not clip it.
		keys = strings.TrimSpace(keys)
		aside := ""
		if i := strings.Index(keys, " ("); i >= 0 {
			aside = strings.Trim(strings.TrimSpace(keys[i:]), "()")
			keys = strings.TrimSpace(keys[:i])
		}
		row := &toolkit.SettingRow{Title: strings.TrimSpace(title), Subtitle: aside}
		if w := toolkit.TextWidth(keys) + toolkit.Scaled(8); w <= valueRoom() {
			value := toolkit.NewLabel(keys)
			value.SetBounds(toolkit.Rect{W: w, H: toolkit.Scaled(ControlH)})
			row.Control = value
		} else {
			// Still too wide for half a card: it goes under the name, where
			// there is a whole line for it, rather than over the name.
			row.Subtitle = strings.TrimSpace(keys + " " + aside)
		}
		out = append(out, row)
	}
	if len(out) == 0 {
		// A card with no rows in it is a heading over nothing, which reads as
		// something that failed to load. Nothing going wrong is worth one line.
		out = append(out, &toolkit.SettingRow{
			Title:    "Every combination was granted",
			Subtitle: "they are in the menu-bar menu, beside what each one does",
		})
	}
	return out
}

// settingsRoot builds the widget tree and returns it with the function that
// reads the controls back into the settings.
//
// It is separate from [RunSettings] so that what the window DOES is not tangled
// with opening one: this half needs no display, and a caller with a headless
// machine can still ask what the controls would produce.
//
// The buttons own a BorderLayout band at the bottom, which the content cannot
// reach into. That was the defect: a stack of fixed-height fields overflowed
// when the window was made smaller and the buttons were drawn over the text.
// The window is not resizable any more (window.Config.FixedSize) because it is
// exactly as big as what it has to say, so the band and the measured page are
// enough and nothing has to scroll.
func settingsRoot(cfg *Config, attached []glasses.USB, roomH int,
	logf func(string, ...any), closeWindow func()) (toolkit.Widget, func()) {

	// A nil Logf says nothing, here as everywhere else. It matters more here
	// than usual: this one is only ever called from a button, so a nil left
	// unguarded is a window that works perfectly until somebody presses Save.
	if logf == nil {
		logf = func(string, ...any) {}
	}

	// A scroll view ONLY when the page does not fit, which is the answer to why
	// there was none at all.
	//
	// There was one while the window was resizable, and its gutter took a strip
	// of width from every row to hold a scrollbar that could not appear -- so it
	// was removed, on the reasoning that a window opening at the size its page
	// measures never has anything out of view. That reasoning has a hole: the
	// page measures what it measures, and the SCREEN decides what there is room
	// for. A desk with nine screen shortcuts wanted 1470 pixels where 1409 were
	// usable, and the Save and Close buttons went past the bottom edge with no
	// way to reach them.
	//
	// FitScale shrinks the magnification to fit and stops at 1, deliberately:
	// type smaller than that buys nothing a person can read. So when the page
	// still will not fit at 1, something has to give, and it is better that the
	// content scrolls than that the buttons leave the screen.
	//
	// roomH of zero means "no limit", which is what every caller who is not
	// opening a window passes: measuring a page needs no screen.
	page, read := settingsPage(cfg, attached)
	content := toolkit.Widget(page)
	if roomH > 0 {
		inner := toolkit.Scaled(SettingsWidth) - 2*toolkit.Scaled(SettingsPadX)
		if _, ph := page.Measure(inner, 0); ph > roomH {
			logf("the page is %d pixels tall and there is room for %d: it scrolls", ph, roomH)
			content = toolkit.NewScrollView(page)
		}
	}

	row := toolkit.NewBoxLayout()
	row.Spacing = toolkit.Scaled(8)
	row.Pack = toolkit.PackEnd
	buttons := toolkit.NewContainer(row)
	buttons.Add(toolkit.Item{Size: toolkit.Scaled(ButtonW), Widget: toolkit.NewButton("Save", func() {
		read()
		path, err := cfg.Save()
		if err != nil {
			logf("%v", err)
			return
		}
		logf("saved to %s", path)
		closeWindow()
	})})
	buttons.Add(toolkit.Item{Size: toolkit.Scaled(ButtonW),
		Widget: toolkit.NewButton("Close", func() { closeWindow() })})

	frame := toolkit.NewContainer(toolkit.BorderLayout{})
	frame.Add(toolkit.Item{Widget: buttons, Region: toolkit.RegionSouth,
		Size: toolkit.Scaled(ButtonBarH)})
	frame.Add(toolkit.Item{Widget: content, Region: toolkit.RegionCenter})

	pad := toolkit.NewPadding(frame, 0)
	pad.Left, pad.Right = SettingsPadX, SettingsPadX
	pad.Top, pad.Bottom = SettingsPadY, SettingsPadY
	return pad, read
}

// settingsSize is how big the window wants to be, in the pixels the tree is laid
// out in: the page's own measurement, plus the two things around it.
//
// The page is ASKED. What is added is the margin and the button band, which are
// two constants rather than a table of row heights -- and if the answer is too
// tall for the display, FitScale shrinks the whole window through the metric
// scale rather than clipping anything.
func settingsSize(cfg Config, attached []glasses.USB) (int, int) {
	w := toolkit.Scaled(SettingsWidth)
	page, _ := settingsPage(&cfg, attached)
	inner := w - 2*toolkit.Scaled(SettingsPadX)
	_, h := page.Measure(inner, 0)
	return w, h + toolkit.Scaled(ButtonBarH) + 2*toolkit.Scaled(SettingsPadY) +
		toolkit.Scaled(PageSpacing)
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
	want := cfg.ShortcutsOr(DefaultShortcuts())

	// ⛔ ASKED ONCE PER SET, AND REMEMBERED. Opening this window measures the
	// page and then builds it, so the machine was claimed TWICE for one window
	// -- twenty-nine system-wide combinations each time -- and the two answers
	// need not agree. They did not: a combination another application took
	// between the two calls is a row in one and not the other, so the window was
	// sized for a page it then did not draw. It showed up as a test that failed
	// about one run in three, with nothing wrong in it.
	//
	// Keyed on the SET, so changing a shortcut and reopening asks again. Held
	// under a mutex because the settings window and a session can both be
	// running while the desk is stopping.
	key := shortcutKey(want)
	reportMu.Lock()
	defer reportMu.Unlock()
	if key == lastReportKey {
		return lastReport
	}

	h := ClaimGlobal(want, cfg.HotkeyOptions())
	defer h.Close()
	// Spelled out, not in glyphs: the toolkit's font has no ⌥ ⌘ ⇧ ⌃ ← →, and a
	// line saying which combination was granted rendered as "previous:" and
	// nothing else.
	lastReportKey, lastReport = key, h.DescribeNames()
	return lastReport
}

var (
	reportMu      sync.Mutex
	lastReportKey string
	lastReport    string
)

// shortcutKey names a set of shortcuts, so two asks for the same set can be
// told from two asks for different ones.
func shortcutKey(ss []Shortcut) string {
	var b strings.Builder
	for _, s := range ss {
		fmt.Fprintf(&b, "%s=%s;", s.Want, s.Does)
	}
	return b.String()
}

// SettingsFontPx is the settings window's type size in LOGICAL pixels, before
// the metric scale. Thirteen is a comfortable interface size; on a 2160-row
// panel the scale takes it to thirty-nine.
const SettingsFontPx = 13

// SettingsScale is how much to magnify the settings window on a display of this
// height.
//
// It is a LEGIBILITY choice and not a HiDPI derivation, which is worth being
// clear about: the back-end allocates one framebuffer pixel per logical point,
// so a widget tree is already the right size in the platform's own terms on
// every display. Nothing here has to compensate for anything.
//
// What it compensates for is a person sitting in front of a very large panel. On
// the 8K screen this was reported against, the platform's own answer is a
// backing factor of one, so interface type comes out at its nominal size on a
// panel four times the area of a laptop's. A modest magnification reads better
// there; a proportional one does not -- scaling by pixel height gave a factor of
// three, and a dialogue magnified three times is a poster.
//
// So: three steps, and the largest is a third again -- reported as still a
// little too big at half again, on the panel it was measured on. [FitScale]
// shrinks whatever
// this asks for until the window fits the display, so the worst case of getting
// this wrong is a window that is smaller than it might have been.
func SettingsScale(displayH int) float64 {
	switch {
	case displayH >= 1440:
		return 1.35
	case displayH >= 1000:
		return 1.2
	default:
		return 1
	}
}

// ShouldChoose reports whether to put the settings window in front of somebody
// before starting.
//
// Only when nobody has decided and the machine cannot: several headsets on the
// bus, no model in the settings, and no display named on the command line.
// Naming one with -screen is the answer for a script, and writing one in the
// settings is the answer for a person who has already chosen — this is for the
// case where neither has happened and picking one would be a guess.
func ShouldChoose(cfg Config, screen string, attached []glasses.USB) bool {
	if strings.TrimSpace(screen) != "" || cfg.Model() != "" {
		return false
	}
	return len(headsetNames(attached)) > 1
}

// FitScale is the largest magnification, at or below want, whose window still
// fits in maxW x maxH pixels. size reports the window's pixel size at a given
// scale.
//
// Legibility and fitting are two different questions and they disagree. What
// makes the type comfortable on a 2160-row panel is a scale of three; what fits
// on the 1200-row display macOS may well put the window on is nothing like
// that. A window magnified past its screen is worse than a small one: the
// buttons are outside the frame, so the settings cannot be saved at all, and
// nothing on screen says why.
//
// The measurement is handed in rather than computed here because the size
// depends on the FONT installed at that scale -- a row is a glyph high plus
// padding -- and installing a face is the display half's business. That also
// makes this the testable half: a synthetic size function proves the search
// without a window, a display or a font.
//
// The step down is proportional, not a fixed decrement: if the window came out
// twice too tall, the useful next candidate is half the scale, not one notch
// less. It always shrinks -- the factor is a size over a smaller limit -- and it
// is bounded anyway, because this runs while a person waits for a window.
func FitScale(want float64, maxW, maxH int, size func(float64) (int, int)) float64 {
	if want < 1 {
		want = 1
	}
	scale := want
	for range fitTries {
		w, h := size(scale)
		if (maxW <= 0 || w <= maxW) && (maxH <= 0 || h <= maxH) {
			return scale
		}
		if scale <= 1 {
			// Already at the bottom: an unmagnified window that still does not
			// fit is what the machine has, and shrinking the type further would
			// buy nothing a person could read.
			return 1
		}
		next := scale
		if maxH > 0 && h > maxH {
			next = min(next, scale*float64(maxH)/float64(h))
		}
		if maxW > 0 && w > maxW {
			next = min(next, scale*float64(maxW)/float64(w))
		}
		if next < 1 {
			next = 1
		}
		scale = next
	}
	return scale
}

// fitTries bounds the search. Four rounds is generous for a proportional step:
// the first guess is normally right and the second corrects for rounding.
const fitTries = 4

// SettingsRoom is the fraction of a display's usable area the settings window
// may occupy. A dialogue that reaches the very edges of the screen reads as a
// mistake, and on macOS the bottom edge is where the Dock lives.
const SettingsRoom = 0.9

// settingsSurface is what the window is actually given: the form, wrapped in the
// toolkit's popover host.
//
// It is one line, and it is here rather than inline in the display half so a
// test can prove the drop-down works through the SAME wrapper the window uses.
// Without the wrapper the control opens onto nothing -- a defect no bounds test
// and no render of the closed form can see, which is how it shipped once.
func settingsSurface(root toolkit.Widget) toolkit.Widget {
	return toolkit.NewPopoverHost(root)
}

// valueRoom is the widest a settings row's trailing value may be: a little under
// half the card.
//
// A SettingRow right-aligns its control at whatever size the control carries and
// does not clip it, which is the right behaviour for a switch or a drop-down and
// the wrong one for a line of text that turns out to be longer than the row. So
// the length is checked here, where there is somewhere else to put it.
func valueRoom() int {
	return toolkit.Scaled(SettingsWidth-2*SettingsPadX) * 45 / 100
}

// cardRoom is the width a settings row has for one line of text.
func cardRoom() int {
	return toolkit.Scaled(SettingsWidth - 2*SettingsPadX - 2*toolkit.SettingRowPadX)
}

// splitToFit breaks a sentence into the two text lines a settings row has, at
// the word boundary nearest the middle, and returns it whole with an empty
// second line when it already fits.
//
// The nearest boundary to the MIDDLE rather than the last one that fits, because
// two lines of similar length read as a sentence that was laid out and a long
// line with one word under it reads as a mistake. A single word too wide for the
// row is returned as it stands: cutting a combination in half is worse than
// letting it overhang, and a row is not the place to hyphenate.
func splitToFit(s string, room int) (string, string) {
	if room <= 0 || toolkit.TextWidth(s) <= room {
		return s, ""
	}
	best, bestGap := -1, 0
	mid := len(s) / 2
	for i, r := range s {
		if r != ' ' {
			continue
		}
		gap := i - mid
		if gap < 0 {
			gap = -gap
		}
		if best < 0 || gap < bestGap {
			best, bestGap = i, gap
		}
	}
	if best < 0 {
		return s, ""
	}
	return s[:best], strings.TrimSpace(s[best+1:])
}

// troubleOnly keeps the lines of a shortcut report that a person has to act on.
//
// ⛔ THE ROUTINE GRANTS ARE NOT HERE ANY MORE, and it is not tidying. There
// are twenty-nine of them; listed one per row they made a page taller than a
// laptop screen, and they now sit in the menu-bar menu beside the row each one
// does the same thing as -- which is where somebody asking "what opens the
// gallery" was going to look anyway.
//
// What a menu cannot say is what it did not GET. A combination another
// application already holds is substituted or refused, and of the three ways
// one can be taken only two are detectable at all. Those two lines are the
// reason this card exists, so they are all that is left in it.
//
// Two shapes, both from Hotkeys.describe: a substitution carries "(asked for
// ..., it was taken)" after the combination it got, and a refusal is a whole
// sentence beginning "no global shortcut for".
func troubleOnly(lines []string) []string {
	var out []string
	for _, line := range lines {
		l := strings.TrimSpace(line)
		if strings.Contains(l, "asked for ") || strings.HasPrefix(l, "no global shortcut for") {
			out = append(out, line)
		}
	}
	return out
}
