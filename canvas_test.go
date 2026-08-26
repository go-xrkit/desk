// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"bytes"
	"testing"

	"github.com/go-xrkit/xrkit/glasses"
	"github.com/go-xrkit/xrkit/ribbon"
	"github.com/go-xrkit/xrkit/stereo"
)

// makeSource builds a screen whose every pixel encodes its own coordinates, so
// a pixel landing in the wrong place is not merely a wrong colour but says
// exactly where it came from.
func makeSource(w, h, pad int) Source {
	stride := w*4 + pad
	pix := make([]byte, stride*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			o := y*stride + x*4
			pix[o] = byte(x)
			pix[o+1] = byte(x >> 8)
			pix[o+2] = byte(y)
			pix[o+3] = byte(y >> 8)
		}
		// Poison the padding, so a blit that walks into it is caught rather than
		// quietly reading zeroes that happen to look like black.
		for o := y*stride + w*4; o < (y+1)*stride; o++ {
			pix[o] = 0xEE
		}
	}
	return Source{Pix: pix, W: w, H: h, Stride: stride}
}

func identityBlit(w, h int) ribbon.Blit {
	rows := make([]int32, h)
	for i := range rows {
		rows[i] = int32(i)
	}
	return ribbon.Blit{
		Screen: 0,
		Dst:    stereo.Rect{X: 0, Y: 0, W: w, H: h},
		SrcX:   0, SrcXStep: one,
		SrcY: rows,
	}
}

// naiveBlit is an independent reference: it gathers one pixel at a time, the
// obvious way, and reuses nothing of the run decomposition under test.
//
// It is what Blit used to do, kept as the thing the faster version must agree
// with. A speed-up is only a speed-up if it draws the same picture.
func naiveBlit(c *Canvas, b ribbon.Blit, src Source) {
	d := b.Dst
	for j := 0; j < d.H && j < len(b.SrcY); j++ {
		srow := src.row(int(b.SrcY[j]))
		if srow == nil {
			continue
		}
		x := b.SrcX
		for i := 0; i < d.W; i++ {
			sx := int(x >> fracBits)
			x += b.SrcXStep
			if sx < 0 || sx >= src.W {
				continue
			}
			o := ((d.Y+j)*c.W + d.X + i) * 4
			copy(c.Pix[o:o+4], srow[sx*4:sx*4+4])
		}
	}
}

// TestRunsAgreeWithGatheringOnePixelAtATime is what earns the run decomposition.
//
// Blitting by runs replaced a per-pixel gather and made a frame thirteen times
// cheaper. The whole risk of that is drawing something subtly different, so it
// is checked against the naive version it replaced: at a step of exactly one, at
// the 0.99982 a real panorama actually produces, at steps that stretch and
// shrink, at offsets off a pixel boundary, and at a start before the source
// where some columns have nothing to read.
func TestRunsAgreeWithGatheringOnePixelAtATime(t *testing.T) {
	const w, h = 96, 24
	src := makeSource(w, h, 12)

	for _, tc := range []struct {
		name  string
		step  int64
		start int64
		dstW  int
	}{
		{"exactly one", one, 0, w},
		{"one, half a pixel along", one, one / 2, w - 1},
		{"the 0.99982 a real panorama gives", 4294204894, 0, w - 1},
		{"the same, unaligned", 4294204894, one/3 + 7, w - 2},
		{"shrinking by a third", one * 3 / 2, 0, w * 2 / 3},
		{"stretching by half", one * 2 / 3, 0, w},
		{"a start before the source", one, -5 * one, w},
		{"running off the end", one, int64(w-4) * one, w},
		{"barely moving", one / 64, 0, w},
	} {
		rows := make([]int32, h)
		for i := range rows {
			rows[i] = int32(i)
		}
		b := ribbon.Blit{
			Dst:  stereo.Rect{X: 0, Y: 0, W: tc.dstW, H: h},
			SrcX: tc.start, SrcXStep: tc.step, SrcY: rows,
		}

		fast := NewCanvas(w, h)
		slow := NewCanvas(w, h)
		fast.Blit(b, src)
		naiveBlit(slow, b, src)

		if !bytes.Equal(fast.Pix, slow.Pix) {
			for i := range fast.Pix {
				if fast.Pix[i] != slow.Pix[i] {
					t.Errorf("%s: runs and gathering differ at byte %d (%d vs %d)",
						tc.name, i, fast.Pix[i], slow.Pix[i])
					break
				}
			}
			continue
		}
		// And something must actually have been drawn, or the two agree on
		// nothing at all.
		if tc.name != "a start before the source" && bytes.Equal(fast.Pix, make([]byte, len(fast.Pix))) {
			t.Errorf("%s: nothing was drawn by either", tc.name)
		}
	}
}

// TestRunsAreLongWhenTheStepIsNearOne pins down WHY this is fast. If a change
// ever makes the decomposition produce a run per pixel it will still be
// correct — and thirteen times slower, with no test failing to say so.
func TestRunsAreLongWhenTheStepIsNearOne(t *testing.T) {
	c := NewCanvas(2000, 4)
	b := ribbon.Blit{
		Dst:  stereo.Rect{W: 1250, H: 4},
		SrcX: 0, SrcXStep: 4294204894, // the real panorama's step
		SrcY: []int32{0, 1, 2, 3},
	}
	runs := c.decompose(b, 1920)
	if len(runs) > 2 {
		t.Errorf("a step of 0.99982 was split into %d runs over 1250 columns; "+
			"the block copy has stopped being a block copy", len(runs))
	}
	total := 0
	for _, r := range runs {
		total += r.n
	}
	if total != 1250 {
		t.Errorf("the runs cover %d of 1250 columns", total)
	}
}

// TestPaddedStrideDoesNotShear: a source whose rows are padded is the normal
// case from ScreenCaptureKit, and reading it as width*4 shears the picture
// progressively down the screen — a fault that looks like a skew, not like a
// crash.
func TestPaddedStrideDoesNotShear(t *testing.T) {
	const w, h = 37, 21
	for _, pad := range []int{0, 4, 64, 1264} {
		src := makeSource(w, h, pad)
		c := NewCanvas(w, h)
		c.Blit(identityBlit(w, h), src)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				o := (y*w + x) * 4
				if c.Pix[o] != byte(x) || c.Pix[o+2] != byte(y) {
					t.Fatalf("pad=%d: pixel (%d,%d) carries (%d,%d); the rows have sheared",
						pad, x, y, c.Pix[o], c.Pix[o+2])
				}
				if c.Pix[o] == 0xEE && c.Pix[o+1] == 0xEE {
					t.Fatalf("pad=%d: padding was read as picture at (%d,%d)", pad, x, y)
				}
			}
		}
	}
}

func TestFill(t *testing.T) {
	for _, n := range []int{1, 2, 3, 7, 16, 100} {
		c := NewCanvas(n, 3)
		c.Fill([4]byte{9, 8, 7, 255})
		for i := 0; i+4 <= len(c.Pix); i += 4 {
			if c.Pix[i] != 9 || c.Pix[i+1] != 8 || c.Pix[i+2] != 7 || c.Pix[i+3] != 255 {
				t.Fatalf("W=%d: byte %d was not filled", n, i)
			}
		}
	}
	// A canvas too small to hold a pixel must not panic.
	(&Canvas{Pix: []byte{1, 2}, W: 0, H: 0}).Fill([4]byte{1, 1, 1, 1})
}

// TestBlitRefusesToWriteOutsideItself. The ribbon guarantees its rectangles fit,
// but this is the last thing between a bad number and a memory fault, and it
// must decline rather than clip halfway.
func TestBlitRefusesToWriteOutsideItself(t *testing.T) {
	src := makeSource(8, 8, 0)
	base := identityBlit(8, 8)
	for _, tc := range []struct {
		name string
		dst  stereo.Rect
	}{
		{"past the right edge", stereo.Rect{X: 5, Y: 0, W: 8, H: 8}},
		{"past the bottom", stereo.Rect{X: 0, Y: 5, W: 8, H: 8}},
		{"negative x", stereo.Rect{X: -1, Y: 0, W: 8, H: 8}},
		{"negative y", stereo.Rect{X: 0, Y: -1, W: 8, H: 8}},
		{"no width", stereo.Rect{X: 0, Y: 0, W: 0, H: 8}},
		{"no height", stereo.Rect{X: 0, Y: 0, W: 8, H: 0}},
	} {
		c := NewCanvas(8, 8)
		b := base
		b.Dst = tc.dst
		c.Blit(b, src)
		if !bytes.Equal(c.Pix, make([]byte, len(c.Pix))) {
			t.Errorf("%s: something was drawn", tc.name)
		}
	}

	// A rectangle whose every column falls outside the source draws nothing at
	// all, rather than a stripe of whatever the decomposition happened to leave.
	c0 := NewCanvas(8, 8)
	bFar := base
	bFar.SrcX = -200 * one
	c0.Blit(bFar, src)
	if !bytes.Equal(c0.Pix, make([]byte, len(c0.Pix))) {
		t.Error("a rectangle entirely outside the source drew something")
	}

	// An empty source draws nothing rather than reading from nowhere.
	c := NewCanvas(8, 8)
	c.Blit(base, Source{})
	if !bytes.Equal(c.Pix, make([]byte, len(c.Pix))) {
		t.Error("an empty source drew something")
	}
}

// TestBlitSurvivesAShortRowTable and a source row that is not there. Both mean
// a caller got something wrong; neither may take the process down.
func TestBlitSurvivesAShortRowTable(t *testing.T) {
	src := makeSource(8, 8, 0)
	c := NewCanvas(8, 8)
	b := identityBlit(8, 8)
	b.SrcY = b.SrcY[:3] // fewer rows than the rectangle is tall
	c.Blit(b, src)
	// The first three rows are drawn, the rest untouched.
	for y := 3; y < 8; y++ {
		for x := 0; x < 8*4; x++ {
			if c.Pix[y*8*4+x] != 0 {
				t.Fatalf("row %d was drawn from a table that does not reach it", y)
			}
		}
	}

	// A row index outside the source is skipped, not read.
	c = NewCanvas(8, 8)
	b = identityBlit(8, 8)
	b.SrcY[4] = 99
	c.Blit(b, src)
	for x := 0; x < 8*4; x++ {
		if c.Pix[4*8*4+x] != 0 {
			t.Fatal("a row outside the source was drawn")
		}
	}

	// A stride too short to hold the pixels is refused rather than read.
	c = NewCanvas(8, 8)
	c.Blit(identityBlit(8, 8), Source{Pix: make([]byte, 8*8*4), W: 8, H: 8, Stride: 4})
	if !bytes.Equal(c.Pix, make([]byte, len(c.Pix))) {
		t.Error("a source with an impossible stride was drawn")
	}

	// A buffer shorter than the stride says it is.
	c = NewCanvas(8, 8)
	c.Blit(identityBlit(8, 8), Source{Pix: make([]byte, 8), W: 8, H: 8, Stride: 32})
	if !bytes.Equal(c.Pix, make([]byte, len(c.Pix))) {
		t.Error("a truncated source was drawn")
	}
}

// TestGeneralPathSkipsColumnsOutsideTheSource covers a stepper that walks off
// the end — the ribbon says it cannot happen, and this is what happens if it
// does.
func TestGeneralPathSkipsColumnsOutsideTheSource(t *testing.T) {
	src := makeSource(4, 4, 0)
	c := NewCanvas(8, 4)
	b := identityBlit(8, 4)
	b.SrcX = -2*one + 1 // starts before the source, and takes the general path
	c.Blit(b, src)
	// The first two columns had no source and must be untouched.
	for y := 0; y < 4; y++ {
		for x := 0; x < 2; x++ {
			o := (y*8 + x) * 4
			if c.Pix[o] != 0 || c.Pix[o+3] != 0 {
				t.Fatalf("column %d was drawn from before the source", x)
			}
		}
	}
}

func TestComposeLeavesAScreenWithNoSourceAsBackground(t *testing.T) {
	src := makeSource(4, 4, 0)
	c := NewCanvas(8, 4)

	b0 := identityBlit(4, 4)
	b1 := identityBlit(4, 4)
	b1.Screen = 1
	b1.Dst.X = 4
	b2 := identityBlit(4, 4)
	b2.Screen = 7 // no such source
	b3 := identityBlit(4, 4)
	b3.Screen = -1

	bg := [4]byte{20, 30, 40, 255}
	c.Compose([]ribbon.Blit{b0, b1, b2, b3}, []Source{src, {}}, bg)

	// Screen 0 drew, screen 1 has an empty source, and the out-of-range ones
	// were ignored — so the right half is still background.
	for y := 0; y < 4; y++ {
		if o := (y*8 + 0) * 4; c.Pix[o+2] != byte(y) {
			t.Fatalf("screen 0 did not draw at row %d", y)
		}
		if o := (y*8 + 5) * 4; c.Pix[o] != bg[0] || c.Pix[o+1] != bg[1] {
			t.Fatalf("a screen with no source drew something at row %d", y)
		}
	}
}

func BenchmarkComposeBeastRibbon(b *testing.B) {
	// The real shape: a Beast's view, with a screen and a piece of its
	// neighbour in it.
	plan, err := planForBench()
	if err != nil {
		b.Fatal(err)
	}
	c := NewCanvas(plan.ScreenW, plan.ScreenH)
	src := makeSource(plan.ScreenW, plan.ScreenH, 0)
	sources := make([]Source, plan.Count())
	for i := range sources {
		sources[i] = src
	}
	r, err := ribbon.Place(plan.Screens(), plan.Layout)
	if err != nil {
		b.Fatal(err)
	}
	strip, err := NewStrip(placedOf(r), plan.Count()*(plan.ScreenW+DefaultGapPx),
		plan.ScreenW, plan.ScreenH, plan.ScreenW, plan.ScreenH)
	if err != nil {
		b.Fatal(err)
	}
	// Half a screen along, so the frame carries two blits and a seam between
	// them — the shape a turning band actually composes.
	blits := strip.Frame(make([]ribbon.Blit, 0, 8), plan.ScreenW/2)
	bg := [4]byte{12, 14, 18, 255}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Compose(blits, sources, bg)
	}
	b.StopTimer()
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/1e6, "ms/frame")
}

func planForBench() (Plan, error) {
	return NewPlan(glassesDisplay("VITURE Beast", 3840, 1080), Options{})
}

func glassesDisplay(name string, w, h int) glasses.Display {
	return glasses.Display{Name: name, Width: w, Height: h}
}
