// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-viture/luma"
)

// stand replaces the headset for the duration of a test.
func stand(t *testing.T, in luma.Info, chip []byte, err error) {
	t.Helper()
	was := readGlasses
	readGlasses = func() (luma.Info, []byte, error) { return in, chip, err }
	t.Cleanup(func() { readGlasses = was })
}

// TestTheFirmwareRowIsNeverGuessed.
//
// ⛔⛔ THE MISTAKE THIS GUARDS. The headset answers TWO version numbers: its
// chip's ("02 01 09") and its product's ("12.0.01.101_20260605"). For a day the
// first was shown as the second, and nothing on screen could have revealed it.
// So a value that was not read stays EMPTY and the row says why.
func TestTheFirmwareRowIsNeverGuessed(t *testing.T) {
	t.Run("both read", func(t *testing.T) {
		stand(t, luma.Info{
			FirmwareVersion: "12.0.01.101_20260605",
			BoardSerial:     "P6SPNH54701132",
			PackageSerial:   "S154801101",
		}, []byte{2, 1, 9}, nil)
		rows := glassesRows(ReadGlassesInfo())
		if len(rows) != 3 {
			t.Fatalf("%d row(s): %+v", len(rows), rows)
		}
		// VERBATIM. The vendor's updater trims the leading "12." and this does
		// not, so the two screens can be compared.
		if rows[0].Subtitle != "12.0.01.101_20260605" {
			t.Errorf("firmware row = %q, want what the headset said", rows[0].Subtitle)
		}
		// The PACKAGE serial, which is the one the vendor labels SN -- not the
		// board serial, which is a different number the same read returns.
		if rows[1].Subtitle != "S154801101" {
			t.Errorf("serial row = %q, want the package serial", rows[1].Subtitle)
		}
	})

	t.Run("a partial answer shows what it has and invents nothing", func(t *testing.T) {
		stand(t, luma.Info{FirmwareVersion: "12.0.01.101_20260605"}, nil, errors.New("the serial timed out"))
		rows := glassesRows(ReadGlassesInfo())
		if len(rows) != 1 {
			t.Fatalf("%d row(s), want just the firmware: %+v", len(rows), rows)
		}
		if strings.Contains(rows[0].Title, "Serial") {
			t.Error("a serial row appeared for a serial that was not read")
		}
	})

	t.Run("nothing read says why, and says it in words", func(t *testing.T) {
		for _, c := range []struct {
			err   error
			wants string
		}{
			{luma.ErrUnsupported, "macOS"},
			{luma.ErrNoDevice, "Beast"},
			// The one a person will actually hit: the vendor's own app holds
			// the interface. Naming the remedy beats naming the errno.
			{errors.New("kIOReturnExclusiveAccess"), "SpaceWalker"},
		} {
			stand(t, luma.Info{}, nil, c.err)
			rows := glassesRows(ReadGlassesInfo())
			if len(rows) != 1 || rows[0].Title != "Not available" {
				t.Fatalf("%v gave %+v", c.err, rows)
			}
			if !strings.Contains(rows[0].Subtitle, c.wants) {
				t.Errorf("%v: the row says %q, which does not mention %q",
					c.err, rows[0].Subtitle, c.wants)
			}
		}
	})

	t.Run("a silent headset is not an error but is still empty", func(t *testing.T) {
		stand(t, luma.Info{}, nil, nil)
		rows := glassesRows(ReadGlassesInfo())
		if len(rows) != 1 || rows[0].Subtitle == "" {
			t.Fatalf("got %+v, want one row that says something", rows)
		}
	})
}

// The rows reach the window as the toolkit's own type, and nothing is lost on
// the way.
func TestSettingRowsCarryBothHalves(t *testing.T) {
	got := settingRows([]glassesRow{{Title: "Firmware", Subtitle: "12.0"}})
	if len(got) != 1 || got[0].Title != "Firmware" || got[0].Subtitle != "12.0" {
		t.Errorf("settingRows lost something: %+v", got)
	}
}

// The two shapes ReadGlassesInfo cannot produce but glassesRows must still
// handle, because it is called with whatever it is given.
func TestGlassesRowsOnWhatItIsGiven(t *testing.T) {
	t.Run("a serial without a firmware still shows the serial", func(t *testing.T) {
		rows := glassesRows(GlassesInfo{Serial: "S154801101"})
		if len(rows) != 1 || rows[0].Subtitle != "S154801101" {
			t.Errorf("got %+v, want just the serial", rows)
		}
		if strings.Contains(rows[0].Title, "Firmware") {
			t.Error("a firmware row appeared for a firmware that was not read")
		}
	})

	t.Run("empty with no reason still says something", func(t *testing.T) {
		// ⛔ A BLANK ROW IS THE ONE OUTCOME THAT TELLS A PERSON NOTHING. If a
		// caller hands over nothing and no reason, the row says so rather than
		// leaving a gap somebody has to interpret.
		rows := glassesRows(GlassesInfo{})
		if len(rows) != 1 || rows[0].Subtitle == "" {
			t.Fatalf("got %+v, want one row that says something", rows)
		}
	})
}

// TestTheChipRowIsBytesAndSaysWhoseTheyAre.
//
// ⛔ THE CHIP ANSWERS A NUMBER THAT LOOKS LIKE A VERSION AND IS NOT THE ONE
// ANYBODY MEANS. Showing "2.1.9" beside a product firmware of
// 0.01.101_20260605 is how a day was lost. So it appears as bytes, under its
// own name, with a sentence saying whose they are.
func TestTheChipRowIsBytesAndSaysWhoseTheyAre(t *testing.T) {
	stand(t, luma.Info{FirmwareVersion: "12.0.01.101_20260605"}, []byte{2, 1, 9}, nil)
	rows := glassesRows(ReadGlassesInfo())
	if len(rows) != 2 {
		t.Fatalf("%d row(s): %+v", len(rows), rows)
	}
	chip := rows[1]
	if chip.Title != "Chip firmware" {
		t.Errorf("the chip row is titled %q", chip.Title)
	}
	if !strings.HasPrefix(chip.Subtitle, "02 01 09") {
		t.Errorf("chip row = %q, want the bytes as they arrived", chip.Subtitle)
	}
	if strings.Contains(chip.Subtitle, "2.1.9") {
		t.Error("the bytes were dressed up as a dotted version, which nothing confirms")
	}
	// ⛔ THE WINDOW FONT HAS NO APOSTROPHE AND NO DASH. A sentence carrying
	// either comes out with a hole in the middle of it, which this window
	// learned once already on the words "the glasses own menu bar".
	for _, bad := range []string{"'", "’", "--", "—"} {
		if strings.Contains(chip.Subtitle, bad) {
			t.Errorf("chip row %q contains %q, which the built-in font cannot draw",
				chip.Subtitle, bad)
		}
	}
}

// A chip answer on its own is still worth a card: it is the only thing the
// headset gave, and an empty page would hide that it gave anything.
func TestTheChipAloneIsNotNothing(t *testing.T) {
	stand(t, luma.Info{}, []byte{2, 1, 9}, nil)
	rows := glassesRows(ReadGlassesInfo())
	if len(rows) != 1 || rows[0].Title != "Chip firmware" {
		t.Fatalf("got %+v, want just the chip row", rows)
	}
}

func TestChipBytesAreHexAndSpaced(t *testing.T) {
	if got := chipBytes(nil); got != "" {
		t.Errorf("chipBytes(nil) = %q, want empty", got)
	}
	if got := chipBytes([]byte{0x02, 0x01, 0x09}); got != "02 01 09" {
		t.Errorf("chipBytes = %q", got)
	}
	if got := chipBytes([]byte{0xff, 0x00}); got != "ff 00" {
		t.Errorf("chipBytes = %q: every byte keeps its two digits", got)
	}
}
