// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-viture/beast"
	"github.com/go-viture/luma"
	"github.com/go-widgets/toolkit"
)

// GlassesInfo is what the headset says about itself, for the settings page.
//
// ⭐ WHY A PERSON WANTS THIS ON SCREEN. A firmware version and a serial are what
// a support conversation asks for first, and until now the only way to see them
// was to quit this application and open the vendor's -- which claims the USB
// interface exclusively, so the two cannot even be open at once.
//
// ⛔ AND NOTHING HERE IS INVENTED. A field this could not read stays EMPTY and
// Why says what stopped it. The alternative -- filling the firmware row with the
// CHIP's version, which is a different number the headset also answers -- is
// exactly the mistake that put "2.1.9" in front of a headset running
// 0.01.101_20260605 for a day, and nobody could have caught it from the screen.
type GlassesInfo struct {
	// Model names the headset these values came FROM. Both can be attached at
	// once, so a card without it would be numbers nobody could attribute.
	Model string
	// Firmware is the application firmware, VERBATIM. VITURE's own updater
	// trims its leading component -- "12.0.01.101_20260605" there reads
	// "0.01.101_20260605" -- and this does not, so that what is on this screen
	// can be compared with what is on theirs.
	Firmware string
	// Serial is the one the vendor labels SN.
	Serial string
	// Chip is the CHIP's firmware, as the bytes the headset answers.
	//
	// ⛔ AS BYTES, AND UNDER ITS OWN NAME. It reads "02 01 09" on a headset
	// whose product firmware is 0.01.101_20260605, and calling those three
	// bytes "the firmware version" is precisely the mistake that stood for a
	// day with nothing on screen able to reveal it. Nothing says they are to be
	// shown as 2.1.9 either -- not their order, not that all three are numbers
	// -- so they are not dressed up.
	//
	// The vendor's own tools do not show this at all. It is here because a
	// support conversation that has got past the product version has nowhere
	// else to go.
	Chip string
	// Why is empty when anything was read, and a sentence when nothing was.
	Why string
}

// readGlasses is the seam: the real one claims a USB interface, which a test
// cannot arrange and must not need to.
var readGlasses = func() (luma.Info, []byte, error) {
	g, err := luma.Open()
	if err != nil {
		return luma.Info{}, nil, err
	}
	defer func() { _ = g.Close() }()
	in, err := g.Info()
	// ⚠ ASKED SECOND, AND ITS FAILURE IS NOT THE OTHERS'. The chip version
	// travels on the other envelope entirely -- the MCU pair rather than the
	// data pair -- so it can be refused while the product values arrive. One
	// row missing beats a page emptied by the least important of three reads.
	chip, chipErr := g.ChipVersion()
	if chipErr != nil {
		chip = nil
	}
	return in, chip, err
}

// readBeastGlasses is the other seam. The Beast answers a different envelope
// on a different bus, so it is a separate call rather than a branch inside one.
var readBeastGlasses = func() (beast.Info, error) {
	g, err := beast.Open()
	if err != nil {
		return beast.Info{}, err
	}
	defer func() { _ = g.Close() }()
	return g.Info()
}

// ReadGlassesInfo asks a headset who it is: the Luma first, then the Beast.
//
// ⛔ AND IT SAYS WHICH ONE ANSWERED. Both can be attached at once -- that is
// what the picker beside this card exists for -- so numbers with no name on
// them would be the same confusion this card was built to end, one level up.
func ReadGlassesInfo() GlassesInfo {
	if got, ok := readLuma(); ok {
		return got
	}
	if got, ok := readBeast(); ok {
		return got
	}
	// Neither answered. The Luma's reason is the more useful one to show,
	// because it is the headset this can ask the most of.
	return lumaInfo()
}

// readBeast asks the Beast, and reports whether it said anything.
func readBeast() (GlassesInfo, bool) {
	// ⚠ THE ERROR IS DROPPED ON PURPOSE. When neither headset answers it is the
	// LUMA reason that gets shown, because it is the one this can ask the most
	// of; a second explanation here would only compete with it.
	in, _ := readBeastGlasses()
	got := GlassesInfo{
		Model:    "VITURE Beast",
		Firmware: in.FirmwareVersion,
		Serial:   in.PackageSerial,
	}
	// ⭐ THE BOARD SERIAL STANDS IN WHEN THERE IS NO PRODUCT ONE. This headset
	// answers 0xff for its package serial, which means it has none -- and the
	// vendor's own tool falls back the same way rather than showing nothing.
	if got.Serial == "" {
		got.Serial = in.BoardSerial
	}
	if got.Firmware == "" && got.Serial == "" {
		return GlassesInfo{}, false
	}
	return got, true
}

// readLuma asks the Luma, and reports whether it said anything.
func readLuma() (GlassesInfo, bool) {
	got := lumaInfo()
	if got.Firmware == "" && got.Serial == "" && got.Chip == "" {
		return GlassesInfo{}, false
	}
	return got, true
}

func lumaInfo() GlassesInfo {
	in, chip, err := readGlasses()
	got := GlassesInfo{
		Model:    "VITURE Luma Ultra",
		Firmware: in.FirmwareVersion,
		Serial:   in.PackageSerial,
		Chip:     chipBytes(chip),
	}
	switch {
	case got.Firmware != "" || got.Serial != "" || got.Chip != "":
		// Something came back. A partial answer is still an answer, and the
		// empty field speaks for itself.
		return got
	case errors.Is(err, luma.ErrUnsupported):
		got.Why = "only macOS can ask the glasses this"
	case errors.Is(err, luma.ErrNoDevice):
		got.Why = "no Luma Ultra is attached; a Beast answers a different protocol"
	case err != nil:
		// The exclusive-access case is the one a person will actually hit, and
		// naming the remedy beats naming the errno.
		got.Why = fmt.Sprintf("the glasses would not answer: %v "+
			"(SpaceWalker and the firmware page hold the interface while they are open)", err)
	default:
		got.Why = "the glasses answered with nothing"
	}
	return got
}

// glassesRows is what the settings page shows for it: one row per value, or one
// row saying why there is none.
func glassesRows(in GlassesInfo) []glassesRow {
	if in.Firmware == "" && in.Serial == "" && in.Chip == "" {
		why := in.Why
		if why == "" {
			why = "not read"
		}
		return []glassesRow{{Title: "Not available", Subtitle: why}}
	}
	rows := make([]glassesRow, 0, 4)
	if in.Model != "" {
		rows = append(rows, glassesRow{Title: "Headset", Subtitle: in.Model})
	}
	if in.Firmware != "" {
		rows = append(rows, glassesRow{
			Title: "Firmware", Subtitle: in.Firmware,
		})
	}
	if in.Serial != "" {
		rows = append(rows, glassesRow{Title: "Serial number", Subtitle: in.Serial})
	}
	if in.Chip != "" {
		// ⛔ ITS OWN NAME AND ITS OWN SENTENCE. Two version numbers on one card,
		// one of which is not the one anybody means, is how the confusion this
		// row exists to end got started.
		rows = append(rows, glassesRow{
			Title:    "Chip firmware",
			Subtitle: in.Chip + ", from the chip itself, not the version above",
		})
	}
	return rows
}

// glassesRow is a title and its value, kept separate from the toolkit's type so
// that the choice of what to show is testable without building a window.
type glassesRow struct{ Title, Subtitle string }

// settingRows turns the choice of what to show into the toolkit's rows.
//
// Split from [glassesRows] so that WHAT is shown can be tested without a
// window, and so that this file is the only place that knows the widget type.
func settingRows(rows []glassesRow) []*toolkit.SettingRow {
	out := make([]*toolkit.SettingRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, &toolkit.SettingRow{Title: r.Title, Subtitle: r.Subtitle})
	}
	return out
}

// chipBytes renders the chip's answer the way it arrived: bytes, in hex.
//
// ⛔ NOT "2.1.9". Three bytes look like a dotted version and nothing says they
// are one -- not their order, not that all three are numbers. A screen that
// guessed would be a screen nobody could correct against anything.
func chipBytes(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	parts := make([]string, 0, len(b))
	for _, c := range b {
		parts = append(parts, fmt.Sprintf("%02x", c))
	}
	return strings.Join(parts, " ")
}
