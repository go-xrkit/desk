// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"fmt"
	"sync"
	"unsafe"
)

// eye is one eye's rectangle in the output.
type eye struct{ x, w int }

// view turns the picture into what the glasses show.
//
// There is nothing platform-specific here, and that is the point: the window,
// the toolkit and the warp are all portable, so one render path serves macOS,
// Linux, Windows and the browser. Only getting the screens and their pixels
// differs per system.
type view struct {
	mu sync.Mutex

	// eyes is where each eye's picture starts in the output, in pixels, and how
	// wide it is. Both eyes are given the SAME picture: captured screens are
	// flat pictures with no depth of their own, so showing each eye a different
	// one would invent a parallax that is not in the source.
	eyes []eye
	// cols maps an output column of one eye to a column of the picture, and
	// rows an output row to a row. They are built once, because the band slides
	// inside the picture rather than the picture moving inside the view.
	cols []int32
	rows []int32

	out   []uint32
	bytes []byte
	w, h  int

	// Snapshot, when set, is called with the output picture once it has been
	// drawn. It is how this proves what it put on the glasses without asking a
	// person to describe what they saw.
	Snapshot func(pix []byte, w, h int)

	// Coverage is the fraction of the output the panorama actually reaches. A
	// number far below 1 means the field of view and the panorama window
	// disagree, which is invisible in a still picture and obvious here.
	Coverage float64
}

// newView prepares the output for a framebuffer of the given size.
//
// There is no projection here any more, and that is the change the glasses
// asked for. When the screens are flat, the picture the compositor produced IS
// what the viewer should see: a screen at rest is exactly the view, at one
// source pixel per output pixel, with nothing to unbend. What is left is
// putting that picture in front of each eye — a copy, and a scale only when
// the framebuffer is not the size the plan asked for.
//
// Both eyes are shown the SAME picture. Captured screens are flat pictures with
// no depth of their own, so giving each eye a different one would invent a
// parallax that is not in the source.
func newView(plan Plan, fbW, fbH int) (*view, error) {
	if fbW <= 0 || fbH <= 0 {
		return nil, fmt.Errorf("desk: the window reported a %dx%d framebuffer", fbW, fbH)
	}
	if plan.ScreenW <= 0 || plan.ScreenH <= 0 {
		return nil, fmt.Errorf("desk: the plan has screens of %dx%d",
			plan.ScreenW, plan.ScreenH)
	}
	v := &view{w: fbW, h: fbH, out: make([]uint32, fbW*fbH)}

	eyeW, eyeH := fbW, fbH
	if plan.Stereoscopic {
		eyeW = fbW / 2
	}
	if eyeW <= 0 {
		return nil, fmt.Errorf("desk: a %d-pixel framebuffer leaves no room for two eyes", fbW)
	}
	v.eyes = []eye{{x: 0, w: eyeW}}
	if plan.Stereoscopic {
		v.eyes = append(v.eyes, eye{x: eyeW, w: fbW - eyeW})
	}

	v.cols = make([]int32, eyeW)
	for x := range v.cols {
		v.cols[x] = int32(int64(x) * int64(plan.ScreenW) / int64(eyeW))
	}
	v.rows = make([]int32, eyeH)
	for y := range v.rows {
		v.rows[y] = int32(int64(y) * int64(plan.ScreenH) / int64(eyeH))
	}
	// Every output pixel is covered, always: the picture is the view. The
	// number is kept because a run that reports anything else has a plan and a
	// framebuffer that disagree, which is invisible in a still picture.
	v.Coverage = 1
	return v, nil
}

// background is what an output pixel gets when the panorama does not reach it.
const background = 0xff000000

// draw copies the picture in front of each eye.
//
// The picture is BGRA, because that is what every capture on every platform
// hands over, and the toolkit wants RGBA — so the swap happens inside the copy,
// where it is free, rather than as a pass of its own.
func (v *view) draw(c *Canvas) {
	v.mu.Lock()
	defer v.mu.Unlock()
	src := asWords(c.Pix)
	for _, e := range v.eyes {
		for y, sy := range v.rows {
			row := int(sy) * c.W
			out := y*v.w + e.x
			for x := 0; x < e.w; x++ {
				sx := int(v.cols[x])
				p := uint32(background)
				if i := row + sx; sx < c.W && i < len(src) {
					p = swapRB(src[i])
				}
				v.out[out+x] = p
			}
		}
	}
	if v.Snapshot != nil {
		snap := v.Snapshot
		v.Snapshot = nil // once is evidence; every frame is a film
		snap(asBytes(v.out), v.w, v.h)
	}
}

// frame is the toolkit Surface's callback: the pixels to show, right now.
func (v *view) frame() ([]byte, int, int) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.bytes == nil {
		v.bytes = asBytes(v.out)
	}
	return v.bytes, v.w, v.h
}

// asWords and asBytes view one pixel buffer as the other's element type. The
// buffers are allocated as one or the other and never both, so nothing is
// copied and nothing is aliased that was not already the same memory.
func asWords(b []byte) []uint32 {
	if len(b) < 4 {
		return nil
	}
	return unsafe.Slice((*uint32)(unsafe.Pointer(&b[0])), len(b)/4)
}

func asBytes(w []uint32) []byte {
	if len(w) == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(&w[0])), len(w)*4)
}

// swapRB turns one BGRA pixel into RGBA, leaving alpha where it is.
func swapRB(p uint32) uint32 {
	return p&0xff00ff00 | (p&0x00ff0000)>>16 | (p&0x000000ff)<<16
}
