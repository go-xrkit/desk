// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"testing"

	"github.com/go-xrkit/xrkit/glasses"
)

// TestAScreenTakesTheShapeOfWhatItShows.
//
// The measured case: this Mac's panel is 2056x1329, a ratio of 1.547 against
// the band's 1.778, and it has no 16:9 mode to be put into -- its sixty display
// modes are 1.547 and 1.600 and nothing else. Fitted into a screen of the
// band's shape it left an empty band 124 pixels wide down each side, which is
// what was reported. So the SCREEN takes the shape.
func TestAScreenTakesTheShapeOfWhatItShows(t *testing.T) {
	p := testPlan(t)
	// The Mac's panel, at the band's height: 1329 -> 1080 scales 2056 to 1670.
	const mirrored = 1670
	feeds := feedsFor(p)
	d, err := New(p, feeds)
	if err != nil {
		t.Fatalf("New = %v", err)
	}
	if got := d.Plan().ScreenWidth(1); got != p.ScreenW {
		t.Fatalf("screen 2 starts %d wide, want the band's %d", got, p.ScreenW)
	}

	d.SetFeed(1, &shapedFeed{w: mirrored, h: p.ScreenH})
	d.Render()

	if got := d.Plan().ScreenWidth(1); got != mirrored {
		t.Errorf("screen 2 is %d wide, want the %d its pixels are", got, mirrored)
	}
	// And its neighbours are untouched: one screen changing shape must not
	// reshape the band.
	for _, i := range []int{0, 2, 3} {
		if got := d.Plan().ScreenWidth(i); got != p.ScreenW {
			t.Errorf("screen %d is %d wide, want the band's %d", i+1, got, p.ScreenW)
		}
	}

	// Put a band-shaped feed back and the screen goes back with it.
	d.SetFeed(1, &shapedFeed{w: p.ScreenW, h: p.ScreenH})
	d.Render()
	if got := d.Plan().ScreenWidth(1); got != p.ScreenW {
		t.Errorf("screen 2 stayed %d wide after a band-shaped feed, want %d", got, p.ScreenW)
	}
}

func TestAFrameThatIsNotTheBandsHeightChangesNothing(t *testing.T) {
	p := testPlan(t)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatalf("New = %v", err)
	}
	// A frame from a capture that has not settled: the negative control for the
	// test above, and the reason fit only trusts a source of the right height.
	d.SetFeed(1, &shapedFeed{w: 640, h: 360})
	d.Render()
	if got := d.Plan().ScreenWidth(1); got != p.ScreenW {
		t.Errorf("screen 2 is %d wide, want the band's %d", got, p.ScreenW)
	}
}

func TestAShapeNobodyCouldHaveMeantIsRefused(t *testing.T) {
	p := testPlan(t)
	for _, w := range []int{1, p.ScreenH * 9} { // far too narrow, far too wide
		d, err := New(p, feedsFor(p))
		if err != nil {
			t.Fatalf("New = %v", err)
		}
		d.SetFeed(1, &shapedFeed{w: w, h: p.ScreenH})
		d.Render()
		if got := d.Plan().ScreenWidth(1); got != p.ScreenW {
			t.Errorf("a source %d wide gave screen 2 a width of %d, want the band's %d",
				w, got, p.ScreenW)
		}
	}
}

// shapedFeed is a feed of a given shape, filled with one colour.
type shapedFeed struct{ w, h int }

func (f *shapedFeed) Frame() (Source, bool) {
	return Source{Pix: make([]byte, f.w*f.h*4), W: f.w, H: f.h, Stride: f.w * 4}, true
}

func (f *shapedFeed) Close() error { return nil }

func TestEveryAllowedShapeBuilds(t *testing.T) {
	p := testPlan(t)
	// The widths a mirror actually produces: a screen NARROWER than the band's
	// own, because a display that is not 16:9 is fitted to the band's height.
	// Wider is a different matter and often does not fit at all -- four screens
	// of the eye's own shape already take 343 of the 360 degrees -- which is
	// what TestAShapeTheBandCannotHoldIsRefused is about.
	for _, w := range []int{
		p.ScreenH * MinAspectNum / MinAspectDen,
		p.ScreenW,
		1670,
	} {
		q := p.WithScreenWidth(1, w)
		if got := q.ScreenWidth(1); got != w {
			t.Fatalf("WithScreenWidth(1, %d) gave %d", w, got)
		}
		if _, _, _, _, err := build(q); err != nil {
			t.Errorf("a screen %d wide does not build: %v", w, err)
		}
	}
}

func TestWithScreenWidthRefusesWhatIsNotAShape(t *testing.T) {
	p := testPlan(t)
	for _, c := range []struct {
		i, w int
		why  string
	}{
		{1, 0, "no width at all"},
		{1, -100, "a negative width"},
		{1, p.ScreenH*MinAspectNum/MinAspectDen - 1, "narrower than a panel on its side"},
		{1, p.ScreenH*MaxAspectNum/MaxAspectDen + 1, "wider than an ultrawide"},
	} {
		if got := p.WithScreenWidth(c.i, c.w).ScreenWidth(c.i); got != p.ScreenW {
			t.Errorf("%s (%d) gave a screen %d wide, want the band's %d",
				c.why, c.w, got, p.ScreenW)
		}
	}
	// And a screen the band does not have leaves the plan alone.
	for _, i := range []int{-1, p.Count()} {
		q := p.WithScreenWidth(i, 1670)
		if !q.sameShapes(p) {
			t.Errorf("WithScreenWidth on screen %d of %d changed the plan", i, p.Count())
		}
	}
}

func TestSameShapesComparesTheBandAndNotJustTheWidths(t *testing.T) {
	p := testPlan(t)
	if !p.sameShapes(p) {
		t.Error("a plan is not the same shape as itself")
	}
	if p.sameShapes(p.WithScreenWidth(0, 1670)) {
		t.Error("a screen given its own shape compares equal to one that has not")
	}
	// A different number of screens, and a different screen size, are both
	// different shapes whatever the widths say.
	if p.sameShapes(p.WithScreens(p.Count() + 1)) {
		t.Error("a band with another screen on it compares equal")
	}
	wider := p
	wider.ScreenW = p.ScreenW + 1
	if p.sameShapes(wider) {
		t.Error("a band of wider screens compares equal")
	}
}

func TestAGalleryCellKeepsAScreensShapeInsideIt(t *testing.T) {
	g, err := NewGrid(4, 1920, 1080, 1920, 1080, 8)
	if err != nil {
		t.Fatalf("NewGrid = %v", err)
	}
	x, _, w, _, _ := g.Cell(1)

	// Nothing set: the cell is filled.
	if gx, gw := g.inset(1, x, w); gx != x || gw != w {
		t.Errorf("inset with no widths = %d,%d, want %d,%d", gx, gw, x, w)
	}
	// A narrower screen sits centred, narrower.
	g.SetSourceWidths([]int{0, 1670, 0, 0})
	gx, gw := g.inset(1, x, w)
	if gw >= w {
		t.Errorf("a 1670-wide screen fills a cell meant for 1920: %d of %d", gw, w)
	}
	if gx-x != (w-gw)/2 {
		t.Errorf("it sits at %d in a cell at %d..%d, want it centred", gx, x, x+w)
	}
	// A screen of the band's own width, and one wider than its cell, both fill
	// it: there is nothing to inset and nothing to shrink into.
	g.SetSourceWidths([]int{0, 1920, 3840, 0})
	for _, i := range []int{1, 2} {
		if gx, gw := g.inset(i, x, w); gx != x || gw != w {
			t.Errorf("cell %d inset = %d,%d, want the whole cell %d,%d", i, gx, gw, x, w)
		}
	}
}

func TestAShapeTheBandCannotHoldIsRefused(t *testing.T) {
	p := testPlan(t)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatalf("New = %v", err)
	}
	// Wide enough that four screens no longer fit in 360 degrees. It is inside
	// what a screen may be -- an ultrawide is a real thing -- so no fixed limit
	// on one screen rules it out, and fit has to be able to say no.
	d.SetFeed(1, &shapedFeed{w: p.ScreenH * MaxAspectNum / MaxAspectDen, h: p.ScreenH})
	d.Render()

	if d.Err() == nil {
		t.Error("a band that cannot hold the shape said nothing")
	}
	// And the band is exactly as it was, rather than half rebuilt.
	for i := 0; i < p.Count(); i++ {
		if got := d.Plan().ScreenWidth(i); got != p.ScreenW {
			t.Errorf("screen %d is %d wide after a refused shape, want %d", i+1, got, p.ScreenW)
		}
	}
}

// TestATurnedScreenShowingANarrowerSourceDoesNotCrash.
//
// The crash this is here for, measured with the glasses on and the app
// running: a position mirroring this Mac's panel captured at 1670x1080, a fan
// still gathering columns at the band's 1920, and
//
//	panic: slice bounds out of range [:6684] with capacity 6680
//
// where 6680 is 1670 pixels of BGRA. Two things were wrong and both are fixed:
// the fan now reads each screen's own width, and Slant treats a column outside
// the source as background rather than as a place to read from.
func TestATurnedScreenShowingANarrowerSourceDoesNotCrash(t *testing.T) {
	p, err := NewPlan(glasses.Display{Name: "VITURE Beast", Width: 3840, Height: 1080},
		Options{Screens: 4, SplayDeg: 8})
	if err != nil {
		t.Fatalf("NewPlan = %v", err)
	}
	if p.SplayDeg() <= 0 {
		t.Fatal("this test needs the turned band, which is the one that crashed")
	}
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatalf("New = %v", err)
	}
	d.SetFeed(1, &shapedFeed{w: 1670, h: p.ScreenH})
	d.Render() // takes the shape
	d.Render() // and draws it

	if got := d.Plan().ScreenWidth(1); got != 1670 {
		t.Errorf("screen 2 is %d wide, want 1670", got)
	}

	// And the floor under it, on its own: a fan that was never told, drawing a
	// source narrower than it expects.
	fan, err := NewFan(p)
	if err != nil {
		t.Fatalf("NewFan = %v", err)
	}
	sources := make([]Source, p.Count())
	for i := range sources {
		sources[i] = Source{Pix: make([]byte, p.ScreenW*p.ScreenH*4), W: p.ScreenW, H: p.ScreenH,
			Stride: p.ScreenW * 4}
	}
	sources[1] = Source{Pix: make([]byte, 1670*p.ScreenH*4), W: 1670, H: p.ScreenH, Stride: 1670 * 4}
	c := NewCanvas(p.ScreenW, p.ScreenH)
	c.ComposeSlants(fan.Frame(nil, 1, 0), sources, [4]byte{})
}
