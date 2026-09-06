// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"sync"

	"github.com/go-widgets/toolkit"
)

// ⛔ THE TOOLKIT'S DEFAULT FONT IS A 5x7 BITMAP, AND MAGNIFYING IT IS DRAWING BY
// HAND. Every glyph is a table of lit bits, and NewBitmapFont(8) turns each one
// into an 8x8 block: a letter built out of squares, with no curve, no stem
// weight, no antialiasing and no kerning, on a panel a hand's width from
// somebody's eye. That is what the desk's own text was -- the screen's number,
// its messages, the names in the application gallery -- while the settings
// window beside it drew in a real face.
//
// go-widgets/toolkit bundles Atkinson Hyperlegible, designed by the Braille
// Institute for maximum character distinction, and rasterises it anti-aliased
// and shaped through the painter's mask capability. There is nothing to build
// and nothing to hand-roll: it is one call.
//
// ⭐ AND IT IS NARROWER. "Firefox" is 84 pixels of bitmap at 14 pixels of ink
// and 48 pixels of real type at 20 -- a proportional face against a fixed
// six-pixel advance -- so a label that had to be shrunk or cut to fit a tile now
// has room it did not have.

// overlayInkRatio is how much of a face's nominal size the INK of a digit fills.
//
// A font's size is not what the eye measures. The 5x7 bitmap fills its box
// exactly -- ink and height are the same number -- and a real face reserves room
// above and below for ascenders and descenders that a digit does not use. So
// asking for a face "7 pixels tall" where a bitmap was 7 pixels tall makes text
// that is visibly SMALLER than what it replaced, and the change reads as a
// regression rather than as the point.
//
// Measured on the bundled face by rendering "8" and counting the rows that
// carry any coverage at all: 0.700 at 10px, 0.750 at 16, 0.714 at 28, 0.725 at
// 40, 0.696 at 56, 0.700 at 90, 0.695 at 128 -- so 0.70, and the wobble is
// hinting at small sizes. [TestADigitFillsTheInkItWasAskedFor] re-measures it,
// which is what would catch the bundled face being changed underneath this.
const overlayInkRatio = 0.70

// overlayGlyphRows is the height of the built-in bitmap's glyph box, and hence
// of its ink: the size the three overlays used to ask for, in the units they
// asked in.
const overlayGlyphRows = 7

var (
	overlayFontMu    sync.Mutex
	overlayFontCache = map[int]toolkit.Font{}

	// openTypeFont is the toolkit's bundled face, named rather than called
	// directly so that a test can make it fail.
	//
	// The bundled face never does, which is exactly the problem: the fallback
	// below is the code that runs on the day this stops being true, and it
	// would otherwise be the only line here nobody has ever executed.
	openTypeFont = toolkit.DefaultOpenTypeFont
)

// overlayFont is the face the desk's own text is drawn in, sized so that ink
// lands where inkPx says.
//
// ⛔ CACHED BECAUSE THIS IS ON THE FRAME PATH. The badge and the notice draw
// sixty times a second, and building a face parses a TrueType file; the bitmap
// font it replaces was a struct with an int in it. A handful of sizes are ever
// asked for -- one per panel height per overlay -- so the map never grows.
//
// A face that will not build falls back to the bitmap at the nearest scale.
// That cannot happen with the bundled face, and a fallback is not decoration:
// this is the only text the desk has, and no text at all is a picture with
// nothing on it to say what went wrong.
func overlayFont(inkPx int) toolkit.Font {
	if inkPx < 1 {
		inkPx = 1
	}
	overlayFontMu.Lock()
	defer overlayFontMu.Unlock()
	if f, ok := overlayFontCache[inkPx]; ok {
		return f
	}
	f, err := openTypeFont(overlaySizeFor(inkPx))
	if err != nil {
		scale := inkPx / overlayGlyphRows
		if scale < 1 {
			scale = 1
		}
		f = toolkit.NewBitmapFont(scale)
	}
	overlayFontCache[inkPx] = f
	return f
}

// overlaySizeFor is the nominal size to ask a face for so that a digit's ink is
// inkPx tall.
func overlaySizeFor(inkPx int) int {
	px := int(float64(inkPx)/overlayInkRatio + 0.5)
	if px < 1 {
		px = 1
	}
	return px
}
