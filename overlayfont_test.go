// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"testing"

	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit"
)

// inkRows is how many rows of rendered text carry any coverage at all: what the
// eye measures, as against the box the font reserves above and below it.
func inkRows(t *testing.T, f toolkit.Font, text string) int {
	t.Helper()
	w, h := f.Measure(text)+4*f.Height()+40, 4*f.Height()+40
	pix := make([]byte, w*h*4)
	p := painter.NewPixelPainterBGRA(pix, w, h)
	f.Draw(p, 20, 20, text, toolkit.RGBA{R: 255, G: 255, B: 255, A: 255})
	top, bot := -1, -1
	for y := range h {
		for x := range w {
			if pix[(y*w+x)*4+3] != 0 {
				if top < 0 {
					top = y
				}
				bot = y
				break
			}
		}
	}
	if top < 0 {
		return 0
	}
	return bot - top + 1
}

// ⛔ THE MEASUREMENT [overlayInkRatio] WAS TAKEN FROM, RE-TAKEN. A font's size is
// not what the eye measures: ask a real face for "7 pixels" where a 5x7 bitmap
// was 7 pixels and the text comes out visibly smaller than what it replaced,
// and the whole change reads as a regression instead of as the point.
//
// This is what would catch the bundled face being swapped for one with
// different proportions -- nothing else here would notice, because every other
// test asks whether text was drawn rather than how big it came out.
func TestADigitFillsTheInkItWasAskedFor(t *testing.T) {
	for _, want := range []int{12, 20, 40, 90, 160} {
		f := overlayFont(want)
		got := inkRows(t, f, "8")
		// A tenth either way: the ratio wobbles with hinting at small sizes
		// (0.750 at 16px against 0.695 at 128), and a digit cannot be rendered
		// to a fraction of a row.
		lo, hi := want-want/10-1, want+want/10+1
		if got < lo || got > hi {
			t.Errorf("asked for %d rows of ink, got %d (allowed %d..%d) — "+
				"the bundled face's proportions have moved and overlayInkRatio "+
				"is no longer %.2f", want, got, lo, hi, overlayInkRatio)
		}
	}
}

// ⭐ THE REASON THIS IS WORTH DOING AT ALL, stated as a measurement. A
// proportional face is far narrower than a fixed six-pixel advance, so the
// labels that had to be shrunk or cut to fit a tile have room they did not have.
func TestRealTypeIsNarrowerThanTheBitmapItReplaces(t *testing.T) {
	const ink = 28
	const word = "Firefox (2 windows) on screens 2, 4"
	real := overlayFont(ink).Measure(word)
	bitmap := toolkit.NewBitmapFont(ink / overlayGlyphRows).Measure(word)
	if real >= bitmap {
		t.Errorf("real type measures %d and the bitmap %d at %d rows of ink; "+
			"the tile labels gain nothing", real, bitmap, ink)
	}
}

// A face is built by parsing a TrueType file and the badge draws sixty times a
// second, so the same size must not be built twice.
func TestTheSameSizeIsTheSameFace(t *testing.T) {
	if a, b := overlayFont(24), overlayFont(24); a != b {
		t.Error("two calls for one size built two faces; this is on the frame path")
	}
	if a, b := overlayFont(24), overlayFont(48); a == b {
		t.Error("two sizes came back as one face")
	}
}

// A picture too small for any type at all still gets some: a badge that came
// out zero rows high would be the silence it exists to prevent.
func TestATinyPictureStillGetsType(t *testing.T) {
	for _, ink := range []int{0, -5, 1} {
		if f := overlayFont(ink); f == nil || f.Height() < 1 {
			t.Errorf("overlayFont(%d) gave nothing to draw with", ink)
		}
	}
	if got := overlaySizeFor(0); got < 1 {
		t.Errorf("overlaySizeFor(0) = %d; a face cannot be asked for no size", got)
	}
}

// The three overlays keep the sizes they had, in the units the picture is in:
// this change is the FACE, and a size that moved with it could not be told
// apart from the face looking wrong.
func TestTheOverlaysAskForTheSizesTheyAlwaysDid(t *testing.T) {
	if got := badgeInk(1200); got != 100 {
		t.Errorf("badgeInk(1200) = %d, want a twelfth of the view", got)
	}
	if got := noticeInk(1080); got != 30 {
		t.Errorf("noticeInk(1080) = %d, want a thirty-sixth of the view", got)
	}
	if got := appsInk(1080); got != overlayGlyphRows {
		t.Errorf("appsInk(1080) = %d, want one step of the ladder", got)
	}
	if got := appsInk(2400); got != 4*overlayGlyphRows {
		t.Errorf("appsInk(2400) = %d, want the ladder's ceiling", got)
	}
	// The floors, all three: a view of no height still gets a row.
	for name, got := range map[string]int{
		"badgeInk": badgeInk(1), "noticeInk": noticeInk(10), "appsInk": appsInk(0),
	} {
		if got < 1 {
			t.Errorf("%s on a tiny picture asked for %d rows", name, got)
		}
	}
}

// ⛔ THE PILL IS SIZED BY THE TOOLKIT, AND THIS ASKS IT RATHER THAN REMEMBERING.
// markH was the constant 18 while a Badge left to size itself chose 9, so every
// number in the gallery was placed half a pill too high — and nothing said so,
// because a pill nine pixels out is still a pill in the corner of a cell.
func TestThePillIsAsTallAsTheToolkitMakesIt(t *testing.T) {
	for _, ink := range []int{7, 30, 90} {
		was := toolkit.CurrentFont()
		toolkit.SetFont(overlayFont(ink))
		b := toolkit.NewBadge("3")
		b.SetBounds(toolkit.Rect{X: 0, Y: 0})
		pix := make([]byte, 500*500*4)
		b.Draw(painter.NewPixelPainterBGRA(pix, 500, 500), toolkit.DefaultDark())
		chose, said := b.Bounds().H, markH()
		toolkit.SetFont(was)
		if chose != said {
			t.Errorf("at %d rows of ink a Badge sizes itself to %d and markH says %d",
				ink, chose, said)
		}
	}
}

// ⛔ THE GALLERY HAD NO SIZE OF ITS OWN. It drew in whatever font was current,
// which was the toolkit's package default -- 7 pixels on a panel of 1080, six
// tenths of one per cent of the height. That is the defect this file is about,
// and the test is that the view now asks for a size rather than inheriting one.
func TestTheGalleryAsksForASizeInsteadOfInheritingOne(t *testing.T) {
	const panel = 1080
	if got := markInk(panel); got <= toolkit.NewBitmapFont(1).Height() {
		t.Errorf("markInk(%d) = %d, no bigger than the bitmap default it replaces",
			panel, got)
	}
	if markInk(panel) != noticeInk(panel) {
		t.Errorf("the gallery and the notice are read on the same panel and "+
			"disagree about type size: %d against %d",
			markInk(panel), noticeInk(panel))
	}
	// The inset is what it was on the panel it was chosen on, and follows a
	// bigger one.
	if got := markInset(panel); got != 8 {
		t.Errorf("markInset(1080) = %d, want the 8 pixels this was fixed at", got)
	}
	if markInset(4*panel) <= markInset(panel) {
		t.Error("the inset does not follow the panel")
	}
	if got := markInset(0); got < 1 {
		t.Errorf("markInset(0) = %d; a number has to sit somewhere", got)
	}
}

// ⛔ THE FALLBACK IS THE ONLY TEXT THERE WOULD BE. A face that will not build
// cannot leave the desk with nothing on the panel to say what went wrong, so it
// drops to the bitmap at the nearest scale rather than to no font at all.
//
// The bundled face never fails, which is why this has to be made to.
func TestAFaceThatWillNotBuildStillLeavesSomethingToReadWith(t *testing.T) {
	was := openTypeFont
	openTypeFont = func(int) (toolkit.Font, error) {
		return nil, errors.New("the face would not parse")
	}
	t.Cleanup(func() {
		openTypeFont = was
		clear(overlayFontCache)
	})
	clear(overlayFontCache)

	for _, ink := range []int{1, 7, 30, 90} {
		f := overlayFont(ink)
		if f == nil {
			t.Fatalf("overlayFont(%d) gave nothing at all", ink)
		}
		if f.Height() < 1 {
			t.Errorf("overlayFont(%d) fell back to a font of no height", ink)
		}
		// The nearest bitmap scale, floored at one: a picture too small for a
		// magnified glyph still gets an unmagnified one.
		scale := max(ink/overlayGlyphRows, 1)
		if want := toolkit.NewBitmapFont(scale).Height(); f.Height() != want {
			t.Errorf("overlayFont(%d) fell back to height %d, want %d",
				ink, f.Height(), want)
		}
	}
}
