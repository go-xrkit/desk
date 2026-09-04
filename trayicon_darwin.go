// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build darwin

package desk

import (
	"fmt"
	"unsafe"

	"github.com/go-macos/appicon"
	"github.com/go-macos/objc"
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
func platformTrayIcon(px int) ([]byte, error) { return platformSymbolPNG(TraySymbol, px) }

// platformSymbolPNG draws any of the system's symbols, by name.
//
// The menu-bar item is one caller and a menu ROW is the other: go-widgets/tray
// draws a row's icon as a template, so the same black-on-transparency picture
// that the bar wants is what a row wants, at a different size.
//
// It is the platform's symbol rather than a shipped icon pack deliberately.
// A menu row sits among the rows of every other application's menu, so the
// weight, the optical size and the ink all have to be the system's or the row
// reads as foreign -- which is the report the menu-bar icon was already fixed
// for once, in the other direction.
func platformSymbolPNG(name string, px int) ([]byte, error) {
	pix, err := appicon.Symbol(name, px)
	if err != nil {
		return nil, fmt.Errorf("desk: no %q symbol: %w", name, err)
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

// platformLabelInk is the colour macOS paints a TEMPLATE menu-bar icon with.
//
// It matters because an icon carrying a coloured dot is not a template any
// more, so the platform stops recolouring it and draws the pixels as they are
// -- pure black, which is what a system symbol is made of. Measured on a dark
// menu bar: every neighbour is white at 84.7% and ours was black, which is
// exactly as wrong as it sounds. Asked about: "pourquoi l'icon tray est elle
// plus sombre quand on a le point vert que les autres icons dans la barre?".
//
// So the recolouring the platform would have done is done here instead, with
// the colour the platform would have used. labelColor follows the appearance,
// so this is read when an icon is built rather than remembered.
func platformLabelInk() (toolkit.RGBA, bool) {
	if err := objc.Load(objc.AppKit); err != nil {
		return toolkit.RGBA{}, false
	}
	c := objc.ClassID("NSColor").Send(objc.Sel("labelColor"))
	if c == 0 {
		return toolkit.RGBA{}, false
	}
	space := objc.ClassID("NSColorSpace").Send(objc.Sel("sRGBColorSpace"))
	conv := c.Send(objc.Sel("colorUsingColorSpace:"), space)
	if conv == 0 {
		return toolkit.RGBA{}, false
	}
	var r, g, b, a float64
	conv.Send(objc.Sel("getRed:green:blue:alpha:"),
		uintptr(unsafe.Pointer(&r)), uintptr(unsafe.Pointer(&g)),
		uintptr(unsafe.Pointer(&b)), uintptr(unsafe.Pointer(&a)))
	return toolkit.RGBA{
		R: byte(r*255 + 0.5),
		G: byte(g*255 + 0.5),
		B: byte(b*255 + 0.5),
		A: byte(a*255 + 0.5),
	}, true
}
