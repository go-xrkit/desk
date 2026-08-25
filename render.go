// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/go-xrkit/xrkit/pose"
	"github.com/go-xrkit/xrkit/projection"
	"github.com/go-xrkit/xrkit/stereo"
	"github.com/go-xrkit/xrkit/warp"
)

// view turns the panorama into the two pictures the glasses show.
//
// There is nothing platform-specific here, and that is the point: the window,
// the toolkit and the warp are all portable, so one render path serves macOS,
// Linux, Windows and the browser. Only getting the screens and their pixels
// differs per system.
type view struct {
	mu sync.Mutex

	// eyeMaps is one lookup table per eye. For a MONO panorama the two are
	// identical, so only one is built and both eyes share it — a build costs
	// 56 ms and there are 16.6 in a frame.
	eyeMaps []*warp.Map
	// eyeOff is where each eye's picture starts in the output, in pixels.
	eyeOff []int

	out   []uint32
	bytes []byte
	w, h  int

	// Coverage is the fraction of the output the panorama actually reaches. A
	// number far below 1 means the field of view and the panorama window
	// disagree, which is invisible in a still picture and obvious here.
	Coverage float64
}

// newView prepares the warp for a framebuffer of the given size.
//
// The panorama is the same for both eyes. Captured screens are flat pictures
// with no depth of their own, so showing each eye a different one would invent
// a parallax that is not in the source — the screens are placed at infinity,
// which is what a viewer's eyes expect of something that far away.
func newView(plan Plan, fbW, fbH int) (*view, error) {
	if fbW <= 0 || fbH <= 0 {
		return nil, fmt.Errorf("desk: the window reported a %dx%d framebuffer", fbW, fbH)
	}
	v := &view{w: fbW, h: fbH, out: make([]uint32, fbW*fbH)}

	eyeW, eyeH := fbW, fbH
	if plan.Stereoscopic {
		eyeW = fbW / 2
	}

	vp := projection.Viewport{Width: eyeW, Height: eyeH, FOVyDeg: plan.VFOVDeg}
	m := warp.Build(vp, plan.Pano.Window, pose.Identity(), warp.Source{
		Width:  plan.Pano.W,
		Height: plan.Pano.H,
		Stride: plan.Pano.W,
		Eye:    stereo.Rect{X: 0, Y: 0, W: plan.Pano.W, H: plan.Pano.H},
	})

	v.eyeMaps = []*warp.Map{m}
	v.eyeOff = []int{0}
	if plan.Stereoscopic {
		// The same table twice: the source rectangle is the whole panorama for
		// both eyes, so a second Build would produce the same numbers at the
		// same price.
		v.eyeMaps = append(v.eyeMaps, m)
		v.eyeOff = append(v.eyeOff, eyeW)
	}
	if n := eyeW * eyeH * len(v.eyeMaps); n > 0 {
		v.Coverage = float64(m.Covered()*len(v.eyeMaps)) / float64(n)
	}
	return v, nil
}

// background is what an output pixel gets when the panorama does not reach it.
const background = 0xff000000

// draw warps the panorama into the output picture, one eye at a time.
//
// The panorama is BGRA, because that is what every capture on every platform
// hands over, and the toolkit wants RGBA — so the swap happens inside the
// gather, where it is free, rather than as a pass of its own.
func (v *view) draw(c *Canvas) {
	v.mu.Lock()
	defer v.mu.Unlock()
	src := asWords(c.Pix)
	for i, m := range v.eyeMaps {
		m.ApplySwapRB(src, v.out, v.w, v.eyeOff[i], background)
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
