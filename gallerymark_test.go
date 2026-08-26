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
