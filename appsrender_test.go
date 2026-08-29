// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-xrkit/xrkit/ribbon"
)

// manyApps is a gallery big enough to have rows, so up and down mean something.
func manyApps() []App {
	return []App{
		{Name: "Code", Windows: 15, On: []int{0}},
		{Name: "Finder", Windows: 1},
		{Name: "Firefox", Windows: 2, On: []int{1, 3}},
		{Name: "KeePassXC", Windows: 1},
		{Name: "Mail", Windows: 1, On: []int{2}},
		{Name: "Music", Windows: 1, Minimized: 1},
		{Name: "Terminal", Windows: 3},
		{Name: "Thunderbird", Windows: 1},
	}
}

// TestTheApplicationGalleryIsDRAWN, and looked at.
//
// A widget tree that has bounds and passes a coverage gate can still be
// unusable: nothing but pixels shows whether a tile is legible at arm's length
// through a pair of glasses. So this renders one and writes it somewhere a
// person can open it — outside every repository, by renderDir's walk-up.
func TestTheApplicationGalleryIsDrawnAndCanBeLookedAt(t *testing.T) {
	const w, h = 1920, 1200
	c := NewCanvas(w, h)
	c.Fill([4]byte{0, 0, 0, 255})

	v := newAppsView(nil)
	v.set(manyApps())
	v.draw(c)

	// Ink, and not one colour: an empty grid draws its surface and its message,
	// which is also ink, so the assertion is that the CELLS are there.
	seen := map[[4]byte]int{}
	for i := 0; i+3 < len(c.Pix); i += 4 {
		seen[[4]byte{c.Pix[i], c.Pix[i+1], c.Pix[i+2], c.Pix[i+3]}]++
	}
	if len(seen) < 3 {
		t.Errorf("the gallery drew %d distinct colours; a grid of tiles with "+
			"labels and one selection field cannot be that flat", len(seen))
	}

	dir := renderDir(t)
	path := filepath.Join(dir, "apps-gallery.png")
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*c.W + x) * 4
			img.Set(x, y, color.RGBA{c.Pix[i+2], c.Pix[i+1], c.Pix[i], 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	t.Logf("the application gallery is at %s — open it and look", path)
}

// TestTheApplicationGallerySelectionWalksTheGrid: left and right WRAP, up and
// down CLAMP, the same rule as the screen gallery, so a person does not have to
// learn two galleries.
func TestTheApplicationGallerySelectionWalksTheGrid(t *testing.T) {
	c := NewCanvas(1920, 1200)
	v := newAppsView(nil)
	v.set(manyApps())
	v.draw(c) // gives the grid its bounds, and therefore its column count
	cols := v.grid.Columns()
	if cols < 2 {
		t.Fatalf("a 1920-wide gallery folded into %d column(s); the rest of this "+
			"test has nothing to walk", cols)
	}

	if got := v.selected(); got != 0 {
		t.Fatalf("a fresh gallery has %d selected, want the first", got)
	}
	// Right wraps round the end.
	for i := 0; i < len(v.apps); i++ {
		if err := v.move(ribbon.Right); err != nil {
			t.Fatalf("move right: %v", err)
		}
	}
	if got := v.selected(); got != 0 {
		t.Errorf("a full lap right ended on %d, want back at 0: right must wrap", got)
	}
	// Left wraps the other way.
	if err := v.move(ribbon.Left); err != nil {
		t.Fatalf("move left: %v", err)
	}
	if got, want := v.selected(), len(v.apps)-1; got != want {
		t.Errorf("left from the first cell went to %d, want %d", got, want)
	}

	// Up from the top row CLAMPS rather than wrapping.
	v.grid.SetSelected(1)
	if err := v.move(ribbon.Up); err != nil {
		t.Fatalf("move up: %v", err)
	}
	if got := v.selected(); got != 1 {
		t.Errorf("up from the top row moved to %d; it must clamp", got)
	}
	// Down moves by a whole row, and clamps at the last one.
	if err := v.move(ribbon.Down); err != nil {
		t.Fatalf("move down: %v", err)
	}
	if got := v.selected(); got != 1+cols {
		t.Errorf("down went to %d, want %d — one row is %d cells", got, 1+cols, cols)
	}
	for i := 0; i < len(v.apps); i++ {
		_ = v.move(ribbon.Down)
	}
	if got := v.selected(); got >= len(v.apps) {
		t.Errorf("down ran off the end to %d of %d", got, len(v.apps))
	}
}

func TestAnEmptyApplicationGalleryHasNothingSelectedAndRefusesToMove(t *testing.T) {
	v := newAppsView(nil)
	v.set(nil)

	if got := v.selected(); got != -1 {
		t.Errorf("selected = %d in an empty gallery, want -1", got)
	}
	if _, ok := v.app(); ok {
		t.Error("an empty gallery offered an application")
	}
	if err := v.move(ribbon.Right); err == nil {
		t.Error("moving in an empty gallery was accepted")
	}
	// And it draws its own message rather than nothing at all.
	c := NewCanvas(400, 300)
	v.draw(c)
}

func TestTheApplicationUnderAPointIsTheOneThatIsClicked(t *testing.T) {
	c := NewCanvas(1920, 1200)
	v := newAppsView(nil)
	v.set(manyApps())
	v.draw(c)

	// The first cell's own area: the grid puts it at the top left, inside its
	// padding, so a point a little way in must be cell zero.
	if i, ok := v.at(30, 40); !ok || i != 0 {
		t.Errorf("at(30,40) = %d,%v, want the first cell", i, ok)
	}
	// Well outside every cell.
	if _, ok := v.at(-5, -5); ok {
		t.Error("a point outside the gallery found an application")
	}
	if _, ok := v.at(10, 1190); ok {
		t.Error("a point below the last row found an application")
	}
}

// TestTheGalleryTextGrowsWithThePicture: the same rule as the badge, because a
// tile read at arm's length through glasses is not a tile read on a monitor.
func TestTheGalleryTextGrowsWithThePicture(t *testing.T) {
	small, big := appsScale(360), appsScale(2160)
	if !(big > small) {
		t.Errorf("appsScale(2160) = %d and appsScale(360) = %d; it must follow the height", big, small)
	}
	if got := appsScale(10); got < 1 {
		t.Errorf("appsScale(10) = %d; a tiny picture still needs a readable font", got)
	}
	if got := appsScale(100000); got > 4 {
		t.Errorf("appsScale(100000) = %d; it must stop somewhere", got)
	}
}

// TestTheGalleryIsOnTheFrameTheDeskDraws, which is the only place it can be
// seen. A view that draws when a test calls draw() and not when the desk renders
// would pass every other test here.
func TestTheGalleryIsOnTheFrameTheDeskDraws(t *testing.T) {
	p := testPlan(t)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatalf("New = %v", err)
	}
	d.OnApps = func() ([]App, error) { return manyApps(), nil }

	before := frameInk(d.Render())
	d.Do(ActionApps)
	after := frameInk(d.Render())

	if !d.inApps {
		t.Fatal("the gallery did not open")
	}
	if after == before {
		t.Error("the frame is identical with the gallery open; it is not being drawn")
	}
}

// frameInk sums a canvas, cheaply, so two frames can be told apart.
func frameInk(c *Canvas) uint64 {
	var sum uint64
	for i := 0; i < len(c.Pix); i += 997 { // a prime stride: a sample, not a hash
		sum += uint64(c.Pix[i])
	}
	return sum
}

func TestTheGalleryDrawsNothingWithoutAPicture(t *testing.T) {
	v := newAppsView(nil)
	v.set(manyApps())
	v.draw(nil)
	v.draw(&Canvas{})
	var none *appsView
	none.draw(NewCanvas(100, 100))
}

func TestTheIconSizeIsBoundedAtBothEnds(t *testing.T) {
	// One application: a whole row to itself, so the quarter-of-the-view ceiling
	// is what stops it being a poster.
	if got, want := appsIconPx(1200, 1), 1200/4; got != want {
		t.Errorf("one application got a %d icon, want the ceiling %d", got, want)
	}
	// None at all: no rows to divide by.
	if got := appsIconPx(1200, 0); got != 1200/4 {
		t.Errorf("an empty gallery got a %d icon", got)
	}
	// Many, in a small picture: the floor.
	if got := appsIconPx(200, 60); got != AppIconPx {
		t.Errorf("a crowded small gallery got %d, want the floor %d", got, AppIconPx)
	}
}

func TestMovingFromNoSelectionStartsAtTheFirst(t *testing.T) {
	v := newAppsView(nil)
	v.set(manyApps())
	v.grid.SetSelected(-1)
	if err := v.move(ribbon.Right); err != nil {
		t.Fatalf("move: %v", err)
	}
	if got := v.selected(); got != 1 {
		t.Errorf("right from nothing selected %d, want 1 — it starts at the first", got)
	}
}

func TestMovingUpARowGoesUpARow(t *testing.T) {
	c := NewCanvas(1920, 1200)
	v := newAppsView(nil)
	v.set(manyApps())
	v.draw(c)
	cols := v.grid.Columns()
	if cols < 2 || len(v.apps) <= cols {
		t.Skipf("a %d-column grid of %d applications has no second row", cols, len(v.apps))
	}

	v.grid.SetSelected(cols) // first cell of the second row
	if err := v.move(ribbon.Up); err != nil {
		t.Fatalf("move up: %v", err)
	}
	if got := v.selected(); got != 0 {
		t.Errorf("up from %d went to %d, want 0 — one row is %d cells", cols, got, cols)
	}
}

// TestChoosingWithNowhereToSendItDoesNotPanic: a desk with no OnPlace is a desk
// nobody wired up, and a key must not take it down.
func TestChoosingWithNowhereToSendItDoesNotPanic(t *testing.T) {
	p := testPlan(t)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatalf("New = %v", err)
	}
	d.OnApps = func() ([]App, error) { return manyApps(), nil }
	d.Do(ActionApps)
	d.Do(ActionChoose)
	d.Do(ActionSpread)
}

// TestATileShowsTheApplicationsOwnIconWhenItHasOne, and the drawn window when
// it does not — including when what it "has" is unusable, which is the case a
// consumer will produce by accident.
func TestATileShowsTheApplicationsOwnIconWhenItHasOne(t *testing.T) {
	red := make([]byte, 8*8*4)
	for i := 0; i < len(red); i += 4 {
		red[i], red[i+3] = 0xFF, 0xFF
	}
	apps := []App{
		{Name: "With", Icon: &Icon{Pix: red, W: 8, H: 8}},
		{Name: "Without"},
		{Name: "Truncated", Icon: &Icon{Pix: red[:16], W: 8, H: 8}}, // fewer bytes than it claims
		{Name: "Empty", Icon: &Icon{}},
	}
	v := newAppsView(nil)
	v.set(apps)

	if v.grid.Cells[0].Image == nil {
		t.Error("the application with an icon is drawing a glyph")
	} else if v.grid.Cells[0].Image.Alt != "With" {
		t.Errorf("the image's Alt is %q, want the application's name", v.grid.Cells[0].Image.Alt)
	}
	for i, name := range []string{"Without", "Truncated", "Empty"} {
		c := v.grid.Cells[i+1]
		if c.Image != nil {
			t.Errorf("%s: an unusable icon was handed to the toolkit anyway", name)
		}
		if c.Icon == nil {
			t.Errorf("%s: no glyph either, so the tile is blank", name)
		}
	}
}

// TestTheScreenGalleryIsDrawnAndCanBeLookedAt, adder included.
//
// The plus was reported as "not properly symmetric" on the glasses, which is
// the kind of thing no assertion finds and one look settles.
func TestTheScreenGalleryIsDrawnAndCanBeLookedAt(t *testing.T) {
	p := testPlan(t)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatalf("New = %v", err)
	}
	d.Badge(0, nil)
	d.Do(ActionGalleryOpen)
	// The adder is the last cell; selecting it is what a person does before
	// pressing Enter, so it is what the picture should show.
	if i, ok := d.grid.Adder(); ok {
		_ = d.grid.Select(i)
	}
	c := d.Render()

	path := filepath.Join(renderDir(t), "screen-gallery.png")
	img := image.NewRGBA(image.Rect(0, 0, c.W, c.H))
	for y := range c.H {
		for x := range c.W {
			i := (y*c.W + x) * 4
			img.Set(x, y, color.RGBA{c.Pix[i+2], c.Pix[i+1], c.Pix[i], 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	t.Logf("the screen gallery is at %s — open it and look", path)
}

// TestTheOverlaysAreDrawnInTheCanvasOwnOrder, which is BGRA.
//
// A Canvas holds what ScreenCaptureKit hands over, and the whole picture is
// swapped once on the way to the window. An overlay drawn RGBA into it arrives
// on the glasses with red and blue exchanged — the orange selection ring was
// blue there, and nobody could see it was wrong because that picture is only
// ever seen through a headset.
func TestTheOverlaysAreDrawnInTheCanvasOwnOrder(t *testing.T) {
	c := NewCanvas(200, 120)
	m := newMarks(nil)
	g, err := NewGridCols(1, 100, 100, 200, 120, 0, 1)
	if err != nil {
		t.Fatalf("NewGridCols: %v", err)
	}
	m.draw(c, g, 0)

	// SelectionInk is orange: red high, blue low. In a BGRA canvas the FIRST
	// byte of an inked pixel is therefore the low one.
	var reds, blues int
	for i := 0; i+3 < len(c.Pix); i += 4 {
		r, b := c.Pix[i+2], c.Pix[i]
		if r == SelectionInk.R && b == SelectionInk.B {
			reds++
		}
		if r == SelectionInk.B && b == SelectionInk.R {
			blues++
		}
	}
	if reds == 0 {
		t.Error("no pixel of the selection ring is orange in BGRA order")
	}
	if blues > 0 {
		t.Errorf("%d pixels of the ring are orange only if the canvas were RGBA; "+
			"they will be BLUE on the glasses", blues)
	}
}
