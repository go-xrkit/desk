// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"fmt"

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
	// Firmware is the application firmware, VERBATIM. VITURE's own updater
	// trims its leading component -- "12.0.01.101_20260605" there reads
	// "0.01.101_20260605" -- and this does not, so that what is on this screen
	// can be compared with what is on theirs.
	Firmware string
	// Serial is the one the vendor labels SN.
	Serial string
	// Why is empty when both were read, and a sentence when they were not.
	Why string
}

// readGlasses is the seam: the real one claims a USB interface, which a test
// cannot arrange and must not need to.
var readGlasses = func() (luma.Info, error) {
	g, err := luma.Open()
	if err != nil {
		return luma.Info{}, err
	}
	defer func() { _ = g.Close() }()
	return g.Info()
}

// ReadGlassesInfo asks the headset who it is.
//
// ⚠ IT IS ONLY WIRED FOR THE LUMA. The Beast answers a different envelope
// again, and a row that quietly showed one headset's numbers while another was
// plugged in would be worse than an empty row. So a headset this cannot ask
// says so.
func ReadGlassesInfo() GlassesInfo {
	in, err := readGlasses()
	got := GlassesInfo{Firmware: in.FirmwareVersion, Serial: in.PackageSerial}
	switch {
	case got.Firmware != "" || got.Serial != "":
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
	if in.Firmware == "" && in.Serial == "" {
		why := in.Why
		if why == "" {
			why = "not read"
		}
		return []glassesRow{{Title: "Not available", Subtitle: why}}
	}
	rows := make([]glassesRow, 0, 2)
	if in.Firmware != "" {
		rows = append(rows, glassesRow{
			Title: "Firmware", Subtitle: in.Firmware,
		})
	}
	if in.Serial != "" {
		rows = append(rows, glassesRow{Title: "Serial number", Subtitle: in.Serial})
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
