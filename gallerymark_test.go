// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"testing"

	"github.com/go-macos/hotkey"
	"github.com/go-xrkit/xrkit/glasses"
	"github.com/go-xrkit/xrkit/ribbon"
)

// beast is the headset most of this was measured on.
func beast() glasses.Display {
	return glasses.Display{Name: "VITURE Beast", Width: 1920, Height: 1200}
}

// galleryOf opens a gallery of n screens and returns the desk and its picture.
func galleryOf(t *testing.T, n int) (*Desk, *Canvas) {
	t.Helper()
	p, err := NewPlan(beast(), Options{Screens: n})
	if err != nil {
		t.Fatal(err)
	}
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	d.Badge(1, nil)
	d.Do(ActionGalleryOpen)
	if d.Nav().Mode() != ribbon.ModeGallery {
		t.Fatalf("the gallery did not open: %v", d.Err())
	}
	return d, d.Render()
}

// TestTheSelectionIsONThePicture.
//
// It was not. The arrows moved a selection held in a variable and NOTHING on
// the picture changed, so six identical desktops sat there while the viewer
// pressed keys and Enter jumped to whichever one they had lost count of. It was
// reported the first time anybody tried to pick a screen with it.
func TestTheSelectionIsONThePicture(t *testing.T) {
	d, first := galleryOf(t, 6)
	defer d.Close()
	before := append([]byte(nil), first.Pix...)

	// Move, and the picture must differ.
	d.Do(ActionNext)
	after := d.Render()
	if len(before) != len(after.Pix) {
		t.Fatalf("the picture changed size")
	}
	differences := 0
	for i := range before {
		if before[i] != after.Pix[i] {
			differences++
		}
	}
	if differences == 0 {
		t.Fatal("moving the selection changed nothing on the picture")
	}

	// And moving back must return to what it was: a selection that leaves a
	// trail is a selection you cannot trust.
	d.Do(ActionPrev)
	back := d.Render()
	for i := range before {
		if before[i] != back.Pix[i] {
			t.Fatalf("moving there and back left the picture different at byte %d", i)
		}
	}
}

// TestEveryCellIsNumbered: the numbers are what a person picks BY, so all of
// them have to be there, and something has to be different about the chosen one.
func TestEveryCellIsNumbered(t *testing.T) {
	for _, n := range []int{1, 3, 6, 9} {
		d, c := galleryOf(t, n)
		// Each cell must carry ink of its own: something is drawn inside every
		// one of them that was not there before the marks.
		plain := NewCanvas(c.W, c.H)
		plain.Compose(d.grid.Frame(nil), d.sources, d.Background)
		for i := 0; i < n; i++ {
			x, y, w, h, ok := d.grid.Cell(i)
			if !ok {
				t.Fatalf("%d screens: no cell %d", n, i)
			}
			if !differsIn(c, plain, x, y, w, h) {
				t.Errorf("%d screens: cell %d carries no number", n, i)
			}
		}
		d.Close()
	}
}

// differsIn reports whether two pictures differ anywhere in a rectangle.
func differsIn(a, b *Canvas, x, y, w, h int) bool {
	for row := y; row < y+h && row < a.H; row++ {
		for col := x; col < x+w && col < a.W; col++ {
			i := (row*a.W + col) * 4
			if a.Pix[i] != b.Pix[i] || a.Pix[i+1] != b.Pix[i+1] || a.Pix[i+2] != b.Pix[i+2] {
				return true
			}
		}
	}
	return false
}

// TestTheGallerySaysWhatEnterWouldDo, in words at the bottom of the view: a
// small accent-coloured pill on one of six tiles is a cue and not an answer.
func TestTheGallerySaysWhatEnterWouldDo(t *testing.T) {
	d, c := galleryOf(t, 6)
	defer d.Close()
	if got := d.marks.says.Text; got != "screen 1 of 6  (Enter to go there)" {
		t.Errorf("the gallery says %q", got)
	}
	d.Do(ActionNext)
	d.Render()
	if got := d.marks.says.Text; got != "screen 2 of 6  (Enter to go there)" {
		t.Errorf("after moving it says %q", got)
	}
	// And it is drawn low, clear of the cells.
	r := d.marks.says.Bounds()
	if r.Y+r.H > c.H || r.Y < c.H/2 {
		t.Errorf("it is at %+v in a picture %d tall", r, c.H)
	}
}

// TestChoosingIsAlsoAGlobalShortcut.
//
// Enter alone is a key in the window, which is no use to somebody who opened
// the gallery from another application: they could move the selection with the
// global arrows and then have nothing to confirm it with.
func TestChoosingIsAlsoAGlobalShortcut(t *testing.T) {
	want := hotkey.Combo{Key: hotkey.KeyReturn,
		Mods: hotkey.Control | hotkey.Option | hotkey.Command}
	for _, s := range DefaultShortcuts() {
		if s.Does == ActionChoose {
			if s.Want != want {
				t.Errorf("choose is on %v, want %v", s.Want, want)
			}
			return
		}
	}
	t.Error("choosing has no system-wide shortcut")
}

// TestTheMarksAreSafeWithNothingToMark.
func TestTheMarksAreSafeWithNothingToMark(t *testing.T) {
	m := newMarks(nil)
	m.draw(nil, nil, 0)
	m.draw(NewCanvas(0, 0), nil, 0)
	d, _ := galleryOf(t, 3)
	defer d.Close()
	m.draw(&Canvas{}, d.grid, 0)
	// A selection outside the grid draws the numbers and says nothing.
	m.draw(NewCanvas(400, 300), d.grid, 99)
	if m.says.Text != "" {
		t.Errorf("a selection that is not there said %q", m.says.Text)
	}
	var nilMarks *marks
	nilMarks.draw(NewCanvas(64, 64), d.grid, 0)
}

// TestTheMouseChoosesAScreen.
//
// One click goes there, rather than one to highlight and another to confirm.
// Somebody who has found the tile they want has already decided, and asking
// them to say so twice is asking them to aim twice at a tile in a headset.
func TestTheMouseChoosesAScreen(t *testing.T) {
	d, _ := galleryOf(t, 6)
	defer d.Close()

	// The middle of cell 4, which is the second row.
	x, y, w, h, ok := d.grid.Cell(4)
	if !ok {
		t.Fatal("no cell 4")
	}
	if !d.Click(x+w/2, y+h/2) {
		t.Fatalf("clicking cell 4 chose nothing: %v", d.Err())
	}
	if d.Nav().Mode() != ribbon.ModeGallery {
		// Choose leaves the gallery.
		if got := d.Nav().Focus(); got != 4 {
			t.Errorf("the band went to screen %d, want the 4 that was clicked", got)
		}
	} else {
		t.Error("clicking a cell left the gallery open")
	}

	// Every cell, by its own middle: an off-by-one in the hit test shows up as
	// one of these landing on its neighbour.
	for i := 0; i < 6; i++ {
		d.Do(ActionGalleryOpen)
		x, y, w, h, _ := d.grid.Cell(i)
		if !d.Click(x+w/2, y+h/2) {
			t.Fatalf("cell %d chose nothing", i)
		}
		if got := d.Nav().Focus(); got != i {
			t.Errorf("clicking cell %d went to screen %d", i, got)
		}
	}
}

// TestClickingTheBackgroundDoesNothing: the gaps between cells and the margin
// round the grid belong to nothing, and a click there should do nothing rather
// than the nearest thing.
func TestClickingTheBackgroundDoesNothing(t *testing.T) {
	d, c := galleryOf(t, 6)
	defer d.Close()
	was := d.grid.Selected()

	// The top-left corner of the view is margin, and the pixel between the
	// first two cells is a gap.
	x, _, w, _, _ := d.grid.Cell(0)
	for name, pt := range map[string][2]int{
		"the corner":         {0, 0},
		"the bottom corner":  {0, c.H - 1},
		"the gap":            {x + w + 1, c.H / 2},
		"outside altogether": {c.W + 10, c.H + 10},
		"a negative point":   {-4, -4},
	} {
		if d.Click(pt[0], pt[1]) {
			t.Errorf("%s chose a screen", name)
		}
	}
	if d.Nav().Mode() != ribbon.ModeGallery {
		t.Error("clicking the background left the gallery")
	}
	if d.grid.Selected() != was {
		t.Error("clicking the background moved the selection")
	}
}

// TestOnTheBandAClickIsNotOurs.
//
// A click on a captured desktop is not this application's to interpret: the
// desktop under the cursor will have its own idea of what was clicked.
func TestOnTheBandAClickIsNotOurs(t *testing.T) {
	p, err := NewPlan(beast(), Options{Screens: 6})
	if err != nil {
		t.Fatal(err)
	}
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if d.Click(100, 100) {
		t.Error("a click on the band chose a screen")
	}
	if d.Nav().Mode() != ribbon.ModeRibbon {
		t.Errorf("a click on the band changed the mode to %v", d.Nav().Mode())
	}
}
