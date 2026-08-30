// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"testing"

	"github.com/go-xrkit/xrkit/ribbon"
	"github.com/go-xrkit/xrkit/stereo"
)

// blitOf builds the blit Strip.append would for a screen w wide at left.
func blitOf(screen, left, w, h, srcW int) ribbon.Blit {
	return ribbon.Blit{
		Screen:   screen,
		Dst:      stereo.Rect{X: left, Y: 0, W: w, H: h},
		SrcX:     0,
		SrcXStep: int64(srcW) << fracBits / int64(w),
	}
}

func TestCursorAtPutsThePointerWhereItsScreenWasDrawn(t *testing.T) {
	// A screen 1920 wide drawn at half size, 400 pixels along the picture.
	b := blitOf(2, 400, 960, 540, 1920)

	for _, c := range []struct {
		px, py       int
		wantX, wantY int
		why          string
	}{
		{0, 0, 400, 0, "the top left corner of the source is the top left of the quad"},
		{1918, 1078, 400 + 959, 539, "and the bottom right is the bottom right"},
		{960, 540, 400 + 480, 270, "the middle is the middle"},
	} {
		x, y, ok := CursorAt([]ribbon.Blit{b}, nil, 2, c.px, c.py, 1920, 1080)
		if !ok {
			t.Fatalf("%s: CursorAt said no", c.why)
		}
		if x != c.wantX || y != c.wantY {
			t.Errorf("%s: %d,%d -> %d,%d, want %d,%d", c.why, c.px, c.py, x, y, c.wantX, c.wantY)
		}
	}
}

func TestCursorAtAnswersForThePieceThePointerIsIn(t *testing.T) {
	// A screen straddling the seam of the band comes back as two blits. The
	// pointer is in one of them, and the answer has to be that one -- the first
	// piece holds the LEFT of the source and would otherwise swallow it.
	left := ribbon.Blit{Screen: 0, Dst: stereo.Rect{X: 1500, Y: 0, W: 420, H: 540},
		SrcX: 0, SrcXStep: int64(1920) << fracBits / int64(960)}
	right := ribbon.Blit{Screen: 0, Dst: stereo.Rect{X: 0, Y: 0, W: 540, H: 540},
		SrcX: int64(840) << fracBits, SrcXStep: int64(1920) << fracBits / int64(960)}

	if x, _, ok := CursorAt([]ribbon.Blit{left, right}, nil, 0, 100, 100, 1920, 1080); !ok || x < 1500 {
		t.Errorf("a pointer near the left of the source landed at x=%d,%v, want the piece at 1500", x, ok)
	}
	if x, _, ok := CursorAt([]ribbon.Blit{left, right}, nil, 0, 1800, 100, 1920, 1080); !ok || x > 540 {
		t.Errorf("a pointer near the right of the source landed at x=%d,%v, want the piece at 0", x, ok)
	}
}

func TestCursorAtOnATurnedPanelFollowsItsColumns(t *testing.T) {
	// A slant's columns are not evenly spaced: that is the whole reason the
	// answer is a search through them and not a division.
	s := Slant{Screen: 1, Dst: stereo.Rect{X: 200, Y: 0, W: 4, H: 400}, Cols: []SlantCol{
		{Src: -1, Y0: 0, Y1: 0}, // turned past the edge of the source
		{Src: 100, Y0: 10, Y1: 210},
		{Src: 900, Y0: 20, Y1: 220},
		{Src: 1800, Y0: 30, Y1: 230},
	}}
	x, y, ok := CursorAt(nil, []Slant{s}, 1, 880, 540, 1920, 1080)
	if !ok {
		t.Fatal("CursorAt said no for a turned panel")
	}
	if x != 202 {
		t.Errorf("x = %d, want the column whose source is nearest 880", x)
	}
	// Half way down the source is half way down THAT column, not half way down
	// the box: every column of a turned panel has its own two rows.
	if y != 20+100 {
		t.Errorf("y = %d, want half way down the column at 20..220", y)
	}
}

func TestCursorAtRefusesWhatItCannotPlace(t *testing.T) {
	b := blitOf(0, 0, 960, 540, 1920)
	for _, c := range []struct {
		name           string
		screen, px, py int
		srcW, srcH     int
		blits          []ribbon.Blit
	}{
		{"a screen nothing drew", 3, 10, 10, 1920, 1080, []ribbon.Blit{b}},
		{"a source of no width", 0, 10, 10, 0, 1080, []ribbon.Blit{b}},
		{"a source of no height", 0, 10, 10, 1920, 0, []ribbon.Blit{b}},
		{"a pointer off the right of the source", 0, 1920, 10, 1920, 1080, []ribbon.Blit{b}},
		{"a pointer off the bottom", 0, 10, 1080, 1920, 1080, []ribbon.Blit{b}},
		{"a pointer before the source", 0, -1, 10, 1920, 1080, []ribbon.Blit{b}},
		{"a pointer above it", 0, 10, -1, 1920, 1080, []ribbon.Blit{b}},
		{"nothing drawn at all", 0, 10, 10, 1920, 1080, nil},
		{"a quad of no size", 0, 10, 10, 1920, 1080,
			[]ribbon.Blit{{Screen: 0, Dst: stereo.Rect{}, SrcXStep: 1}}},
		{"a quad with no step", 0, 10, 10, 1920, 1080,
			[]ribbon.Blit{{Screen: 0, Dst: stereo.Rect{W: 10, H: 10}}}},
	} {
		if x, y, ok := CursorAt(c.blits, nil, c.screen, c.px, c.py, c.srcW, c.srcH); ok {
			t.Errorf("%s: CursorAt = %d,%d,true, want false", c.name, x, y)
		}
	}
	// And the turned panels' own refusals.
	for _, c := range []struct {
		name string
		s    Slant
	}{
		{"a slant of another screen", Slant{Screen: 9, Cols: []SlantCol{{Src: 0, Y0: 0, Y1: 10}}}},
		{"a slant with no source anywhere", Slant{Screen: 0, Cols: []SlantCol{{Src: -1}}}},
		{"a column of no height", Slant{Screen: 0, Cols: []SlantCol{{Src: 10, Y0: 5, Y1: 5}}}},
	} {
		if x, y, ok := CursorAt(nil, []Slant{c.s}, 0, 10, 10, 1920, 1080); ok {
			t.Errorf("%s: CursorAt = %d,%d,true, want false", c.name, x, y)
		}
	}
}

// TestTheDeskDrawsTheMouseWhereItSaysItIs is the one that looks at PIXELS.
//
// macOS does not draw a cursor on a display this program made -- measured, 2003
// pixels change when the pointer crosses this Mac's own panel and 14 when it
// crosses one of the desk's screens -- so this is the only cursor there is.
func TestTheDeskDrawsTheMouseWhereItSaysItIs(t *testing.T) {
	p := testPlan(t)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatalf("New = %v", err)
	}
	d.Badge(0, nil)

	before := snapshot(d.Render())
	// A quarter of the way across the focused screen, half way down.
	d.SetPointer(0, PointerScale/4, PointerScale/2, true)
	after := snapshot(d.Render())

	x0, y0, x1, y1, n := changedBox(before, after, d.Render().W)
	if n == 0 {
		t.Fatal("nothing was drawn for the pointer")
	}
	if w, h := x1-x0+1, y1-y0+1; w > CursorPx+2 || h > CursorPx+2 {
		t.Errorf("what was drawn is %dx%d, want no bigger than the %d-pixel cursor", w, h, CursorPx)
	}
	// It hangs down and to the right of the pointer, the way a real cursor's
	// hotspot does, so the box STARTS at where the pointer is.
	wantX, _, ok := CursorAt(d.blits, d.slants, 0, p.ScreenW/4, p.ScreenH/2, p.ScreenW, p.ScreenH)
	if !ok {
		t.Fatal("CursorAt could not place the pointer the desk just drew")
	}
	if x0 < wantX || x0 > wantX+4 {
		t.Errorf("the drawn pointer starts at x=%d, want it at %d", x0, wantX)
	}

	// And with no pointer on the band, nothing is drawn at all.
	d.SetPointer(0, 0, 0, false)
	if _, _, _, _, n := changedBox(before, snapshot(d.Render()), d.Render().W); n != 0 {
		t.Errorf("%d pixels were drawn for a pointer that is not on the band", n)
	}
}

func snapshot(c *Canvas) []byte {
	out := make([]byte, len(c.Pix))
	copy(out, c.Pix)
	return out
}

func changedBox(a, b []byte, w int) (x0, y0, x1, y1, n int) {
	x0, y0, x1, y1 = 1<<30, 1<<30, -1, -1
	for i := 0; i+3 < len(a) && i+3 < len(b); i += 4 {
		if a[i] == b[i] && a[i+1] == b[i+1] && a[i+2] == b[i+2] {
			continue
		}
		n++
		px := i / 4
		x, y := px%w, px/w
		if x < x0 {
			x0 = x
		}
		if y < y0 {
			y0 = y
		}
		if x > x1 {
			x1 = x
		}
		if y > y1 {
			y1 = y
		}
	}
	return
}

func TestDrawingThePointerOnNoCanvasDrawsNothing(t *testing.T) {
	// The floor: every caller of drawCursor has a canvas, and a canvas with no
	// pixels is what a desk looks like before its first frame.
	drawCursor(nil, 10, 10)
	drawCursor(&Canvas{}, 10, 10)
}

func TestTheDeskDrawsNoMouseForAScreenTheBandDidNotDraw(t *testing.T) {
	p := testPlan(t)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatalf("New = %v", err)
	}
	d.Badge(0, nil)
	before := snapshot(d.Render())
	// A position no blit of this frame carries: the band draws what is in
	// front of the viewer and its neighbours, not the whole ring.
	d.SetPointer(p.Count()+3, PointerScale/2, PointerScale/2, true)
	if _, _, _, _, n := changedBox(before, snapshot(d.Render()), d.Render().W); n != 0 {
		t.Errorf("%d pixels were drawn for a screen that is not in the picture", n)
	}
}

func TestInkStartFindsTheCornerOfWhatWasDrawn(t *testing.T) {
	const w, h = 8, 6
	pix := make([]byte, w*h*4)
	set := func(x, y int) { pix[(y*w+x)*4+3] = 0xFF }
	set(3, 2)
	set(5, 4)
	if dx, dy := inkStart(pix, w, h); dx != 3 || dy != 2 {
		t.Errorf("inkStart = %d,%d, want 3,2", dx, dy)
	}
	// A glyph with nothing in it: drawn where it was asked for rather than a
	// whole box away from it.
	if dx, dy := inkStart(make([]byte, w*h*4), w, h); dx != 0 || dy != 0 {
		t.Errorf("inkStart of an empty buffer = %d,%d, want 0,0", dx, dy)
	}
	// And a buffer smaller than it is said to be is not a panic.
	if dx, dy := inkStart(make([]byte, 8), w, h); dx != 0 || dy != 0 {
		t.Errorf("inkStart of a short buffer = %d,%d, want 0,0", dx, dy)
	}
}
