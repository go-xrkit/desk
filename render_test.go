// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"testing"

	"github.com/go-xrkit/xrkit/glasses"
)

func stereoPlan(t *testing.T) Plan {
	t.Helper()
	p, err := NewPlan(glasses.Display{Name: "VITURE Beast", Width: 3840, Height: 1080}, Options{Screens: 4})
	if err != nil {
		t.Fatalf("NewPlan = %v", err)
	}
	return p
}

func TestNewViewRefusesAFramebufferThatIsNotOne(t *testing.T) {
	p := stereoPlan(t)
	for _, wh := range [][2]int{{0, 1080}, {1920, 0}, {-1, -1}} {
		if _, err := newView(p, wh[0], wh[1]); err == nil {
			t.Errorf("a %dx%d framebuffer was accepted", wh[0], wh[1])
		}
	}
}

// TestBothEyesShareOneTable is what keeps start-up quick and honest.
//
// Captured screens are flat pictures with no depth of their own, so both eyes
// look at the same panorama; a second lookup table would hold the same numbers
// and cost another 56 ms to build. The two eyes must therefore differ ONLY in
// where they write.
func TestBothEyesShareOneTable(t *testing.T) {
	p := stereoPlan(t)
	v, err := newView(p, 3840, 1080)
	if err != nil {
		t.Fatalf("newView = %v", err)
	}
	if len(v.eyeMaps) != 2 {
		t.Fatalf("a side-by-side mode got %d eyes, want 2", len(v.eyeMaps))
	}
	if v.eyeMaps[0] != v.eyeMaps[1] {
		t.Error("the two eyes have different tables; one was built twice for the same numbers")
	}
	if v.eyeOff[0] != 0 || v.eyeOff[1] != 1920 {
		t.Errorf("eye offsets %v, want the second eye half a framebuffer along", v.eyeOff)
	}
	if v.Coverage <= 0 || v.Coverage > 1 {
		t.Errorf("Coverage = %g, which is not a fraction", v.Coverage)
	}

	// A mono panel gets one eye across the whole framebuffer.
	mono, err := NewPlan(glasses.Display{Name: "VITURE Beast", Width: 1920, Height: 1200}, Options{Screens: 4})
	if err != nil {
		t.Fatalf("NewPlan = %v", err)
	}
	v, err = newView(mono, 1920, 1200)
	if err != nil {
		t.Fatalf("newView = %v", err)
	}
	if len(v.eyeMaps) != 1 || v.eyeOff[0] != 0 {
		t.Errorf("a mono panel got %d eyes at %v", len(v.eyeMaps), v.eyeOff)
	}
}

// TestDrawSwapsRedAndBlue is the assertion that would otherwise be found by a
// person noticing the sky is orange.
//
// Every capture on every platform hands over BGRA and the toolkit wants RGBA, so
// the swap happens inside the gather. A panorama painted one distinctive colour
// must come out as that colour with its red and blue exchanged — and it must
// come out in BOTH eyes.
func TestDrawSwapsRedAndBlue(t *testing.T) {
	p := stereoPlan(t)
	v, err := newView(p, 3840, 1080)
	if err != nil {
		t.Fatalf("newView = %v", err)
	}
	c := NewCanvas(p.Pano)
	// B=0x10 G=0x20 R=0x30 A=0xff in memory order.
	c.Fill([4]byte{0x10, 0x20, 0x30, 0xff})
	v.draw(c)

	pix, w, h := v.frame()
	if w != 3840 || h != 1080 {
		t.Fatalf("frame is %dx%d, want the framebuffer", w, h)
	}
	// After the swap the bytes in memory are R=0x10 G=0x20 B=0x30, so the panorama's
	// blue byte has become the output's red.
	const wantR, wantG, wantB = 0x30, 0x20, 0x10
	perEye := map[string]int{"left": 0, "right": 0}
	for _, eye := range []struct {
		name string
		x0   int
	}{{"left", 0}, {"right", 1920}} {
		for y := 0; y < h; y += 37 {
			for x := eye.x0; x < eye.x0+1920; x += 41 {
				o := (y*w + x) * 4
				r, g, b := pix[o+2], pix[o+1], pix[o]
				if r == 0 && g == 0 && b == 0 {
					continue // outside the panorama's arc: background
				}
				if r != wantB || g != wantG || b != wantR {
					t.Fatalf("%s eye at (%d,%d): got r=%#x g=%#x b=%#x", eye.name, x, y, r, g, b)
				}
				perEye[eye.name]++
			}
		}
	}
	for name, n := range perEye {
		if n == 0 {
			t.Errorf("the %s eye is entirely background; the warp reached none of it", name)
		}
	}
}

func TestFrameIsStableAcrossCalls(t *testing.T) {
	p := stereoPlan(t)
	v, err := newView(p, 3840, 1080)
	if err != nil {
		t.Fatalf("newView = %v", err)
	}
	a, w1, h1 := v.frame()
	b, w2, h2 := v.frame()
	if w1 != w2 || h1 != h2 {
		t.Error("the frame changed size between calls")
	}
	if len(a) == 0 || &a[0] != &b[0] {
		t.Error("frame handed back a different buffer; it must lend, not copy")
	}
}

func TestWordsAndBytesRefuseNothing(t *testing.T) {
	if asWords(nil) != nil || asWords([]byte{1, 2, 3}) != nil {
		t.Error("asWords built a slice from fewer than four bytes")
	}
	if asBytes(nil) != nil {
		t.Error("asBytes built a slice from nothing")
	}
	// And a real buffer round-trips to the same memory rather than a copy.
	b := make([]byte, 16)
	w := asWords(b)
	if len(w) != 4 {
		t.Fatalf("asWords gave %d words for 16 bytes", len(w))
	}
	w[0] = 0xdeadbeef
	if got := asBytes(w); &got[0] != &b[0] {
		t.Error("asBytes returned different memory")
	}
	if b[0] == 0 && b[1] == 0 && b[2] == 0 && b[3] == 0 {
		t.Error("writing through the word view did not reach the bytes")
	}
}
