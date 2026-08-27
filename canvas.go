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

// Canvas is the picture the screens are drawn into, and the window shows.
//
// It used to be a panorama that a projection then read from. There is no
// projection any more: the screens are flat, so the buffer they are composited
// into IS what the glasses are given.
type Canvas struct {
	Pix  []byte
	W, H int

	// scratch holds the run decomposition of the blit being drawn. It lives here
	// so that drawing a frame allocates nothing.
	scratch []run
}

// NewCanvas allocates a picture of the given size.
func NewCanvas(w, h int) *Canvas {
	return &Canvas{Pix: make([]byte, w*h*4), W: w, H: h}
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

// Slant draws one turned screen: a column at a time, each from its own source
// row range.
//
// It is the slow path and it says so. [Canvas.Blit] copies whole runs of a row
// because every row of a rectangle reads the same columns; a trapezoid's columns
// each have their own height, so there is no run to copy and the pixels are
// gathered one at a time. That is the four-byte-copy cost the fast path exists to
// avoid, which is why the screen in FRONT of the viewer -- the one turned by
// nothing, and the only one whose pixels a person is reading -- goes through Blit
// and only its neighbours come here.
//
// Rows outside, columns inside: the writes then run along the canvas in order,
// and it is the reads that jump. One of the two has to, and a sequential write is
// worth more than a sequential read on every machine this has been measured on.
func (c *Canvas) Slant(s Slant, src Source) {
	d := s.Dst
	if d.W <= 0 || d.H <= 0 || src.W <= 0 || src.H <= 0 || len(s.Cols) != d.W {
		return
	}
	if d.X < 0 || d.Y < 0 || d.X+d.W > c.W || d.Y+d.H > c.H {
		return
	}
	for y := d.Y; y < d.Y+d.H; y++ {
		drow := c.Pix[(y*c.W)*4 : (y*c.W+c.W)*4]
		for i, col := range s.Cols {
			if col.Src < 0 || int32(y) < col.Y0 || int32(y) >= col.Y1 {
				continue
			}
			// Which source row this destination row shows, from the column's own
			// height. The division is per pixel and it is the price of the shape:
			// a column of a turned panel is a different length from its
			// neighbour, so there is no per-blit table to precompute.
			sy := int(int64(int32(y)-col.Y0) * int64(src.H) / int64(col.Y1-col.Y0))
			srow := src.row(sy)
			if srow == nil {
				continue
			}
			o := (d.X + i) * 4
			copy(drow[o:o+4], srow[int(col.Src)*4:int(col.Src)*4+4])
		}
	}
}
