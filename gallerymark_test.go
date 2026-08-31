// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"bytes"
	"errors"
	"os"
	"strings"
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

// TestTheGalleryOffersOneMoreScreen.
//
// A gallery is where somebody looks at the desk they have, so it is where they
// will want another one. The alternative is a settings file, which is not where
// anybody is when they run out of room.
func TestTheGalleryOffersOneMoreScreen(t *testing.T) {
	d, _ := galleryOf(t, 6)
	defer d.Close()

	if got, want := d.grid.Cells(), 7; got != want {
		t.Errorf("six screens give %d cells, want %d", got, want)
	}
	if got := d.grid.Screens(); got != 6 {
		t.Errorf("the grid says %d screens", got)
	}
	a, ok := d.grid.Adder()
	if !ok || a != 6 {
		t.Fatalf("Adder() = %d, %v; want the last cell", a, ok)
	}
	// Len is what a navigator asks, and it must be the SCREENS: a gallery of
	// seven cells on a band of six would be refused as another ribbon's.
	if got := d.grid.Len(); got != 6 {
		t.Errorf("Len() = %d, want the six screens", got)
	}

	// Enter on it grows the desk, through the callback the platform fills in.
	asked := 0
	d.OnAdd = func() (Feed, error) {
		asked++
		return newFakeFeed(d.Plan().ScreenW, d.Plan().ScreenH, 99), nil
	}
	if err := d.grid.Select(a); err != nil {
		t.Fatal(err)
	}
	d.Do(ActionChoose)
	if asked != 1 {
		t.Fatalf("choosing the adder asked for %d screens", asked)
	}
	if got := d.Plan().Count(); got != 7 {
		t.Errorf("the desk has %d screens, want 7", got)
	}
	if err := d.Err(); err != nil {
		t.Errorf("growing: %v", err)
	}
	// And the new one is drawable: the band and the gallery were rebuilt for it.
	d.Do(ActionGalleryOpen)
	if got := d.grid.Cells(); got != 8 {
		t.Errorf("after growing, the gallery has %d cells", got)
	}
	if c := d.Render(); c == nil {
		t.Error("the desk stopped drawing")
	}
}

// TestGrowingKeepsTheScreenTheViewerIsFacing.
//
// Adding a seventh screen must not move the band: somebody who added one to put
// something on it would otherwise find themselves somewhere else.
func TestGrowingKeepsTheScreenTheViewerIsFacing(t *testing.T) {
	p, err := NewPlan(beast(), Options{Screens: 6})
	if err != nil {
		t.Fatal(err)
	}
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	d.Do(ActionNext)
	d.Do(ActionNext)
	d.Advance(10)
	was := d.Nav().Focus()

	pos, err := d.Grow(newFakeFeed(p.ScreenW, p.ScreenH, 42))
	if err != nil {
		t.Fatalf("Grow = %v", err)
	}
	if pos != 6 {
		t.Errorf("the new screen is at %d, want the end", pos)
	}
	if got := d.Nav().Focus(); got != was {
		t.Errorf("growing moved the viewer from screen %d to %d", was, got)
	}
	// Every screen is still exactly one view wide, the new one included.
	for i := 0; i < 7; i++ {
		if got := d.strip.width[i]; got != p.ScreenW {
			t.Errorf("screen %d is %d wide, want %d", i, got, p.ScreenW)
		}
	}
}

// TestAddingWhatCannotBeAddedLeavesTheDeskAlone.
func TestAddingWhatCannotBeAddedLeavesTheDeskAlone(t *testing.T) {
	d, _ := galleryOf(t, 6)
	defer d.Close()
	a, _ := d.grid.Adder()
	if err := d.grid.Select(a); err != nil {
		t.Fatal(err)
	}

	// A platform that cannot.
	d.OnAdd = func() (Feed, error) { return nil, errors.New("no more displays") }
	d.Do(ActionChoose)
	if d.Plan().Count() != 6 {
		t.Errorf("the desk grew to %d anyway", d.Plan().Count())
	}
	if err := d.Err(); err == nil {
		t.Error("a refusal was not reported")
	}

	// And no callback at all: pressing it does nothing rather than crashing.
	d.OnAdd = nil
	d.Do(ActionChoose)
	if d.Plan().Count() != 6 {
		t.Errorf("the desk grew to %d with no way to add one", d.Plan().Count())
	}
}

// TestClickingThePlusAddsAScreen.
func TestClickingThePlusAddsAScreen(t *testing.T) {
	d, _ := galleryOf(t, 3)
	defer d.Close()
	d.OnAdd = func() (Feed, error) {
		return newFakeFeed(d.Plan().ScreenW, d.Plan().ScreenH, 7), nil
	}
	a, ok := d.grid.Adder()
	if !ok {
		t.Fatal("no adder")
	}
	x, y, w, h, _ := d.grid.Cell(a)
	if !d.Click(x+w/2, y+h/2) {
		t.Fatalf("clicking the plus added nothing: %v", d.Err())
	}
	if got := d.Plan().Count(); got != 4 {
		t.Errorf("the desk has %d screens, want 4", got)
	}
}

// TestTheClampsAndTheEdges covers what is left: the smallest a magnified glyph
// is allowed to get, a grid built without an adder, a plan asked for fewer than
// one screen, and the paths a refusal takes.
func TestTheClampsAndTheEdges(t *testing.T) {
	// A picture too small for the number to be magnified at all still gets one.
	if got := badgeScale(1); got != 1 {
		t.Errorf("badgeScale(1) = %d, want 1", got)
	}

	// A plan cannot have fewer than one screen.
	p, err := NewPlan(beast(), Options{Screens: 3})
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []int{0, -5} {
		if got := p.WithScreens(n).Count(); got != 1 {
			t.Errorf("WithScreens(%d) gave %d screens", n, got)
		}
	}

	// A grid built through NewGridCols has no adder: every cell is a screen.
	g, err := NewGridCols(4, 1920, 1200, 1920, 1200, DefaultGapPx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got := g.Screens(); got != 4 {
		t.Errorf("Screens() = %d, want all four cells", got)
	}
	if _, ok := g.Adder(); ok {
		t.Error("a grid built without an adder has one")
	}
	if g.IsAdder(3) {
		t.Error("the last cell of an adder-less grid is an adder")
	}
	// And the marks draw for it without one, saying which screen is chosen.
	m := newMarks(nil)
	c := NewCanvas(1920, 1200)
	m.draw(c, g, 1)
	if want := "screen 2 of 4  (Enter to go there)"; m.says.Text != want {
		t.Errorf("it says %q, want %q", m.says.Text, want)
	}
	// A selection that is no cell at all says nothing rather than something
	// wrong.
	m.says.Text = ""
	m.draw(c, g, 99)
	if m.says.Text != "" {
		t.Errorf("a selection outside the grid said %q", m.says.Text)
	}

	// Neither constructor takes no screens at all. NewGrid guards before it
	// adds the adder cell, and NewGridCols guards for a caller that reaches it
	// directly.
	if _, err := NewGrid(0, 1920, 1200, 1920, 1200, 0); !errors.Is(err, ErrNoScreens) {
		t.Errorf("NewGrid(0) = %v, want an ErrNoScreens", err)
	}
	if _, err := NewGridCols(0, 1920, 1200, 1920, 1200, 0, 1); !errors.Is(err, ErrNoScreens) {
		t.Errorf("NewGridCols(0) = %v, want an ErrNoScreens", err)
	}

	// And with the adder chosen, the words say what Enter would make rather
	// than where it would go.
	with, err := NewGrid(4, 1920, 1200, 1920, 1200, DefaultGapPx)
	if err != nil {
		t.Fatal(err)
	}
	a, ok := with.Adder()
	if !ok {
		t.Fatal("a grid from NewGrid has no adder")
	}
	m.draw(c, with, a)
	if want := "add a screen  (Enter)"; m.says.Text != want {
		t.Errorf("with the adder chosen it says %q, want %q", m.says.Text, want)
	}
}

// TestADeskThatCannotGrowSaysSo, and closes the feed it was handed: it was
// opened for a screen that does not exist now.
func TestADeskThatCannotGrowSaysSo(t *testing.T) {
	// Screens so small that a gallery holds exactly three of them and no more —
	// measured, not guessed: at ninety pixels a side, build succeeds at three
	// and refuses at four.
	p := testPlan(t)
	p.ScreenW, p.ScreenH = 90, 90
	p = p.WithScreens(3)
	d, err := New(p, make([]Feed, 3))
	if err != nil {
		t.Fatalf("a desk of three 90-pixel screens will not build: %v", err)
	}
	defer d.Close()

	f := newFakeFeed(90, 90, 1)
	if _, err := d.Grow(f); err == nil {
		t.Fatal("a desk that cannot fold another screen into its gallery grew")
	}
	if d.Plan().Count() != 3 {
		t.Errorf("it grew to %d anyway", d.Plan().Count())
	}

	// Through the callback, the feed is closed on the way out.
	d.OnAdd = func() (Feed, error) { return f, nil }
	d.grow(d.OnAdd)
	if f.closes == 0 {
		t.Error("the feed opened for a screen that was never added was left open")
	}
	if d.Err() == nil {
		t.Error("the refusal was not kept")
	}
}

// TestClickingThePlusWithNoWayToAddIt.
func TestClickingThePlusWithNoWayToAddIt(t *testing.T) {
	d, _ := galleryOf(t, 3)
	defer d.Close()
	d.OnAdd = nil
	a, _ := d.grid.Adder()
	x, y, w, h, _ := d.grid.Cell(a)
	if d.Click(x+w/2, y+h/2) {
		t.Error("clicking the plus with no way to add one reported success")
	}
	if d.Plan().Count() != 3 {
		t.Errorf("the desk grew to %d", d.Plan().Count())
	}
}

// TestNothingInThisPackageDrawsAPixel.
//
// "did you add widgets to the toolkit, or are you drawing by hand?" -- asked on
// 2026-08-27, and the honest answer was: hand-drawn, two painter.StrokeRect
// calls, and nothing added to the toolkit. The answer is now a widget,
// toolkit.SelectionBox, and this is the barrier that keeps it that way.
//
// The capture path is excluded: Canvas composites captured desktops, which is
// pixels by definition and not interface. Everything else — the numbers, the
// words, the borders, the plus — has to come from a widget, so that it scales
// with the metric scale, follows the theme, and reaches the accessibility tree
// like the rest of the interface.
func TestNothingInThisPackageDrawsAPixel(t *testing.T) {
	primitives := []string{".StrokeRect(", ".FillRect(", ".FillRoundRect(",
		".StrokeRoundRect(", ".PutPixel(", ".Text("}
	// canvas.go is the compositor: its whole job is to move captured pixels.
	// render.go maps that picture into the framebuffer, likewise.
	allowed := map[string]bool{"canvas.go": true, "render.go": true}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") ||
			strings.HasSuffix(name, "_test.go") || allowed[name] {
			continue
		}
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range primitives {
			if bytes.Contains(b, []byte(p)) {
				t.Errorf("%s calls %s — draw it with a toolkit widget, or add one "+
					"to the toolkit if there is none that fits", name, p)
			}
		}
	}
}

// TestThePlusHasAFloor: a gallery of many screens makes small cells, and a
// plus a quarter of nothing is nothing. Eight pixels is the floor.
func TestThePlusHasAFloor(t *testing.T) {
	c := NewCanvas(60, 40)
	m := newMarks(nil)
	g, err := NewGrid(8, 100, 100, 60, 40, 0)
	if err != nil {
		t.Fatalf("NewGrid: %v", err)
	}
	i, ok := g.Adder()
	if !ok {
		t.Fatal("a grid built with NewGrid has no adder")
	}
	if err := g.Select(i); err != nil {
		t.Fatalf("Select: %v", err)
	}
	m.draw(c, g, i)

	var ink int
	for j := 0; j+3 < len(c.Pix); j += 4 {
		if c.Pix[j+2] == SelectionInk.R && c.Pix[j] == SelectionInk.B {
			ink++
		}
	}
	if ink == 0 {
		t.Error("nothing orange was drawn in a tiny gallery: the plus vanished")
	}
}
