// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build !darwin

package desk

import (
	"errors"

	"github.com/go-widgets/toolkit"
)

// platformTrayIcon has no system symbols to draw here, so the toolkit's own
// glasses are the icon.
func platformTrayIcon(px int) ([]byte, error) { return platformSymbolPNG(TraySymbol, px) }

// platformSymbolPNG has no system symbols to draw here either. A row whose
// icon cannot be made draws as it always did -- text alone -- which is also
// what the Windows and Linux tray backends do with an icon they are given.
func platformSymbolPNG(string, int) ([]byte, error) {
	return nil, errors.New("desk: system symbols are macOS's")
}

// platformGlassesIcon has no system symbol to offer, so the caller keeps the
// toolkit's own drawn glasses.
func platformGlassesIcon(int) toolkit.IconFunc { return nil }

// platformLabelInk has no system colour to read here, so a caller uses its own.
func platformLabelInk() (toolkit.RGBA, bool) { return toolkit.RGBA{}, false }
