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
func TestBothEyesGetTheSamePicture(t *testing.T) {
	p := stereoPlan(t)
	v, err := newView(p, 3840, 1080)
	if err != nil {
		t.Fatalf("newView = %v", err)
	}
	if len(v.eyes) != 2 {
		t.Fatalf("a side-by-side mode got %d eyes, want 2", len(v.eyes))
	}
	if v.eyes[0].x != 0 || v.eyes[1].x != 1920 {
		t.Errorf("eyes at %v, want the second half a framebuffer along", v.eyes)
	}
	if v.eyes[0].w != 1920 || v.eyes[1].w != 1920 {
		t.Errorf("eyes are %v wide, want half a framebuffer each", v.eyes)
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
	if len(v.eyes) != 1 || v.eyes[0].x != 0 || v.eyes[0].w != 1920 {
		t.Errorf("a mono panel got %d eyes: %v", len(v.eyes), v.eyes)
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
	c := NewCanvas(p.ScreenW, p.ScreenH)
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

// TestSnapshotIsHandedTheFrameOnce is what the -snapshot flag rests on, and it
// covers the branch my own coverage gate caught me shipping untested.
//
// Once, not every frame: once is evidence, every frame is a film. And the
// picture handed over must be the one actually drawn — a snapshot of an empty
// buffer would look like a working feature and prove nothing.
func TestSnapshotIsHandedTheFrameOnce(t *testing.T) {
	p := stereoPlan(t)
	v, err := newView(p, 3840, 1080)
	if err != nil {
		t.Fatalf("newView = %v", err)
	}
	c := NewCanvas(p.ScreenW, p.ScreenH)
	c.Fill([4]byte{0x10, 0x20, 0x30, 0xff})

	calls := 0
	var gotW, gotH int
	var painted bool
	v.Snapshot = func(pix []byte, w, h int) {
		calls++
		gotW, gotH = w, h
		for i := 0; i+4 <= len(pix); i += 4 {
			if pix[i] != 0 || pix[i+1] != 0 || pix[i+2] != 0 {
				painted = true
				break
			}
		}
	}

	v.draw(c)
	v.draw(c)
	v.draw(c)

	if calls != 1 {
		t.Errorf("Snapshot was called %d times, want exactly once", calls)
	}
	if gotW != 3840 || gotH != 1080 {
		t.Errorf("Snapshot got %dx%d, want the framebuffer", gotW, gotH)
	}
	if !painted {
		t.Error("Snapshot was handed an empty buffer; it must be the frame that was drawn")
	}
}

// TestDrawWithoutASnapshotIsFine covers the ordinary path, where nobody asked.
func TestDrawWithoutASnapshotIsFine(t *testing.T) {
	p := stereoPlan(t)
	v, err := newView(p, 1920, 1200)
	if err != nil {
		t.Fatalf("newView = %v", err)
	}
	v.draw(NewCanvas(p.ScreenW, p.ScreenH))
}

// TestNewViewRefusesAPlanItCannotCopyFrom. The picture is the plan's screen, so
// a plan with no screen has no picture to put in front of an eye.
func TestNewViewRefusesAPlanItCannotCopyFrom(t *testing.T) {
	p := stereoPlan(t)
	for name, spoil := range map[string]func(*Plan){
		"screens with no columns": func(p *Plan) { p.ScreenW = 0 },
		"screens with no rows":    func(p *Plan) { p.ScreenH = 0 },
	} {
		q := p
		spoil(&q)
		if _, err := newView(q, 3840, 1080); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
	// A side-by-side mode in a framebuffer one pixel wide leaves nothing for
	// the second eye — or the first.
	if _, err := newView(p, 1, 1080); err == nil {
		t.Error("a one-pixel side-by-side framebuffer was accepted")
	}
}

// TestAClickLandsWhereItsPixelCameFrom.
//
// canvasAt is the inverse of draw, read from the same two tables. If they ever
// disagree, a click in the gallery selects a tile the viewer was not pointing
// at — and by a margin that grows toward the edges, so it would work in the
// middle and fail where it matters.
func TestAClickLandsWhereItsPixelCameFrom(t *testing.T) {
	for _, tc := range []struct {
		name     string
		d        glasses.Display
		fbW, fbH int
	}{
		{"one eye, exactly the plan's size", glasses.Display{Name: "VITURE Beast", Width: 1920, Height: 1200}, 1920, 1200},
		{"one eye, a smaller framebuffer", glasses.Display{Name: "VITURE Beast", Width: 1920, Height: 1200}, 960, 600},
		{"side by side", glasses.Display{Name: "VITURE Beast", Width: 3840, Height: 1080}, 3840, 1080},
	} {
		p, err := NewPlan(tc.d, Options{Screens: 6})
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		v, err := newView(p, tc.fbW, tc.fbH)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		for _, e := range v.eyes {
			for _, fx := range []int{e.x, e.x + e.w/2, e.x + e.w - 1} {
				for _, fy := range []int{0, tc.fbH / 2, tc.fbH - 1} {
					cx, cy, ok := v.canvasAt(fx, fy)
					if !ok {
						t.Errorf("%s: (%d,%d) is in no eye", tc.name, fx, fy)
						continue
					}
					// The very same tables draw uses.
					wantX, wantY := int(v.cols[fx-e.x]), int(v.rows[fy])
					if cx != wantX || cy != wantY {
						t.Errorf("%s: (%d,%d) -> (%d,%d), want (%d,%d)",
							tc.name, fx, fy, cx, cy, wantX, wantY)
					}
					if cx < 0 || cx >= p.ScreenW || cy < 0 || cy >= p.ScreenH {
						t.Errorf("%s: (%d,%d) -> (%d,%d), outside a %dx%d picture",
							tc.name, fx, fy, cx, cy, p.ScreenW, p.ScreenH)
					}
				}
			}
		}
		// Off the picture in either direction is in no eye.
		for _, pt := range [][2]int{{-1, 0}, {0, -1}, {tc.fbW, 0}, {0, tc.fbH}} {
			if _, _, ok := v.canvasAt(pt[0], pt[1]); ok {
				t.Errorf("%s: (%d,%d) was mapped into the picture", tc.name, pt[0], pt[1])
			}
		}
	}
}
