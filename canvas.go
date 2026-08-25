// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import "github.com/go-xrkit/xrkit/ribbon"

// fracBits is the fixed-point scale ribbon uses for its horizontal stepper.
const fracBits = 32

// one is a step of exactly one source column per destination column.
const one = int64(1) << fracBits

// Source is one captured screen's pixels.
//
// Stride is carried rather than assumed because it is NOT width*4 in general:
// a 400-pixel-wide capture came back from ScreenCaptureKit with rows padded to
// 1664 bytes, while a 3840-pixel-wide one had no padding at all. Code that
// assumes either is wrong half the time, and wrong here means a picture that
// shears progressively down the screen.
type Source struct {
	Pix    []byte
	W, H   int
	Stride int
}

// row returns one row of the source, or nil when it is not there.
func (s Source) row(y int) []byte {
	if y < 0 || y >= s.H || s.Stride < s.W*4 {
		return nil
	}
	off := y * s.Stride
	if off+s.W*4 > len(s.Pix) {
		return nil
	}
	return s.Pix[off : off+s.W*4 : off+s.W*4]
}

// run is a stretch of destination columns whose source columns are consecutive,
// so the whole stretch is one copy rather than one gather per pixel.
type run struct{ dst, src, n int }

// Canvas is the panorama the screens are drawn into, and the warp reads from.
type Canvas struct {
	Pix  []byte
	W, H int

	// scratch holds the run decomposition of the blit being drawn. It lives here
	// so that drawing a frame allocates nothing.
	scratch []run
}

// NewCanvas allocates a panorama of the planned size.
func NewCanvas(p ribbon.Pano) *Canvas {
	return &Canvas{Pix: make([]byte, p.W*p.H*4), W: p.W, H: p.H}
}

// Fill paints the whole canvas one colour, which is what the gaps between
// screens show.
func (c *Canvas) Fill(px [4]byte) {
	if len(c.Pix) < 4 {
		return
	}
	copy(c.Pix[:4], px[:])
	// Doubling is how a row-oriented painter fills: each copy moves twice as
	// much as the last, so a buffer of n bytes takes log2(n) copies rather than
	// n writes.
	for filled := 4; filled < len(c.Pix); filled *= 2 {
		copy(c.Pix[filled:], c.Pix[:filled])
	}
}

// Blit draws one screen into the canvas, where the ribbon said it goes.
//
// It does not gather pixel by pixel. The horizontal mapping is a linear stepper
// whose step, for a panorama sized to show a screen at its own resolution, is
// very near one — measured at 0.99982 for a VITURE Beast. A step that near one
// means the source column advances by exactly one for long stretches, breaking
// only when the accumulated fraction wraps: over a thousand columns at a time.
//
// So the columns are decomposed into RUNS of consecutive source pixels, once per
// blit, and each run is copied as a block. Every row of the rectangle reuses the
// same decomposition, because on a cylinder a screen's longitudes do not vary
// with height.
//
// Gathering one pixel at a time cost 8.7 ms a frame. It was not the arithmetic:
// it was doing four-byte copies two and a half million times.
func (c *Canvas) Blit(b ribbon.Blit, src Source) {
	d := b.Dst
	if d.W <= 0 || d.H <= 0 || src.W <= 0 || src.H <= 0 {
		return
	}
	if d.X < 0 || d.Y < 0 || d.X+d.W > c.W || d.Y+d.H > c.H {
		return
	}

	runs := c.decompose(b, src.W)
	if len(runs) == 0 {
		return
	}
	for j := 0; j < d.H; j++ {
		if j >= len(b.SrcY) {
			return
		}
		srow := src.row(int(b.SrcY[j]))
		if srow == nil {
			continue
		}
		doff := ((d.Y+j)*c.W + d.X) * 4
		drow := c.Pix[doff : doff+d.W*4 : doff+d.W*4]
		for _, r := range runs {
			copy(drow[r.dst*4:(r.dst+r.n)*4], srow[r.src*4:(r.src+r.n)*4])
		}
	}
}

// decompose splits the destination columns into runs whose source columns are
// consecutive and inside the source. Columns with no source at all are simply
// absent from the result, which is how they stay background.
func (c *Canvas) decompose(b ribbon.Blit, srcW int) []run {
	runs := c.scratch[:0]
	cur := run{dst: -1}
	x := b.SrcX
	for i := 0; i < b.Dst.W; i++ {
		sx := int(x >> fracBits)
		x += b.SrcXStep
		if sx < 0 || sx >= srcW {
			if cur.n > 0 {
				runs = append(runs, cur)
				cur = run{dst: -1}
			}
			continue
		}
		if cur.n > 0 && sx == cur.src+cur.n {
			cur.n++
			continue
		}
		if cur.n > 0 {
			runs = append(runs, cur)
		}
		cur = run{dst: i, src: sx, n: 1}
	}
	if cur.n > 0 {
		runs = append(runs, cur)
	}
	c.scratch = runs
	return runs
}

// Compose fills the canvas and draws every blit, which is one frame's worth of
// panorama. sources is indexed by ribbon screen number; a screen with no source
// yet — a display just created, a capture not started — is simply left as
// background rather than drawn as garbage.
func (c *Canvas) Compose(blits []ribbon.Blit, sources []Source, background [4]byte) {
	c.Fill(background)
	for _, b := range blits {
		if b.Screen < 0 || b.Screen >= len(sources) {
			continue
		}
		c.Blit(b, sources[b.Screen])
	}
}
