// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build darwin

package desk

import (
	"fmt"

	"github.com/go-macos/appicon"
	"github.com/go-widgets/toolkit"
)

// platformTrayIcon draws the system's own symbol.
//
// MEASURED, at 44 pixels: the toolkit's glasses outline inks 140 pixels of
// 1936 -- 7% of the box -- and a system symbol inks about 1200, which is 62%.
// The difference is what a person sees in a menu bar full of other people's
// icons, and it is why this exists rather than the portable drawing alone:
// "la precedente etait plus visible", about the moment I replaced one with the
// other without measuring.
//
// A system that has no such symbol falls back rather than showing nothing.
func platformTrayIcon(px int) ([]byte, error) {
	pix, err := appicon.Symbol(TraySymbol, px)
	if err != nil {
		return nil, fmt.Errorf("desk: no %q symbol: %w", TraySymbol, err)
	}
	// appicon hands back straight RGBA, which is the order a PNG wants.
	return pngOf(pix.Pix, pix.W, pix.H)
}

// platformGlassesIcon is the system's symbol as an icon a widget can draw.
//
// The SAME glyph the menu bar carries, so the settings window and the bar
// agree about what a pair of glasses looks like. A stencil rather than a
// picture: the symbol comes back as black with an alpha channel, and drawn as
// pixels it would be a black glyph on whatever the theme's card happens to be,
// with no highlight when its tile is the chosen one.
//
// A system with no such symbol answers nil, and the caller keeps the toolkit's
// own drawn glasses.
func platformGlassesIcon(px int) toolkit.IconFunc {
	pix, err := appicon.Symbol(TraySymbol, px)
	if err != nil {
		return nil
	}
	return toolkit.StencilIcon(pix.Pix, pix.W, pix.H)
}
