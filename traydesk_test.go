// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"bytes"
	"errors"
	"image/png"
	"strings"
	"testing"
	"time"

	"github.com/go-widgets/tray"
)

// headless puts the item nowhere for the duration of one test. A menu bar is
// one per machine, and a test that put an item in somebody's would leave it
// there.
func headless(t *testing.T) *tray.Headless {
	t.Helper()
	h := tray.NewHeadless()
	was := trayBackend
	trayBackend = func() tray.Backend { return h }
	t.Cleanup(func() { trayBackend = was })
	return h
}

func TestTheMenuIsTheRowsTheDeskOffers(t *testing.T) {
	h := headless(t)
	actions := make(chan Action, TrayQueue)
	item, err := OpenTray(nil, actions)
	if err != nil {
		t.Fatalf("OpenTray = %v", err)
	}
	defer func() { _ = item.Close() }()

	// The headless backend answers once it is running.
	go func() { _ = item.Hold() }()
	waitFor(t, func() bool {
		_, _, menu := h.Snapshot()
		return menu != nil && len(menu.Items) > 0
	}, "the menu to arrive")

	icon, tip, menu := h.Snapshot()
	if tip != TrayTooltip {
		t.Errorf("tooltip %q, want %q", tip, TrayTooltip)
	}
	if len(icon) == 0 {
		t.Error("the item has no icon")
	}
	rows := TrayRows()
	if len(menu.Items) != len(rows) {
		t.Fatalf("the menu has %d rows, want the %d the desk offers", len(menu.Items), len(rows))
	}
	for i, r := range rows {
		got := menu.Items[i]
		if r.Action == ActionNone {
			if !got.Separator {
				t.Errorf("row %d is %q, want a separator", i, got.Label)
			}
			continue
		}
		if got.Label != r.Title {
			t.Errorf("row %d is %q, want %q", i, got.Label, r.Title)
		}
	}
}

func TestChoosingARowSendsItsAction(t *testing.T) {
	h := headless(t)
	actions := make(chan Action, TrayQueue)
	item, err := OpenTray(nil, actions)
	if err != nil {
		t.Fatalf("OpenTray = %v", err)
	}
	defer func() { _ = item.Close() }()
	go func() { _ = item.Hold() }()
	waitFor(t, func() bool {
		_, _, menu := h.Snapshot()
		return menu != nil && len(menu.Items) > 0
	}, "the menu to arrive")

	_, _, menu := h.Snapshot()
	var chose *tray.MenuItem
	var want Action
	for i, r := range TrayRows() {
		if r.Action != ActionNone {
			chose, want = menu.Items[i], r.Action
			break
		}
	}
	if chose == nil {
		t.Fatal("the desk offers no row to choose")
	}
	chose.OnClick()
	select {
	case got := <-actions:
		if got != want {
			t.Errorf("choosing %q sent %v, want %v", chose.Label, got, want)
		}
	default:
		t.Errorf("choosing %q sent nothing", chose.Label)
	}
}

func TestAChoiceNobodyIsListeningForIsDroppedAndSaidSo(t *testing.T) {
	h := headless(t)
	// A queue with no room and nobody reading: the desk is stopped, the
	// settings window is up. A blocking send would park the handler until
	// somebody came back and then replay every click at once.
	actions := make(chan Action)
	var said []string
	item, err := OpenTray(func(f string, a ...any) { said = append(said, f) }, actions)
	if err != nil {
		t.Fatalf("OpenTray = %v", err)
	}
	defer func() { _ = item.Close() }()
	go func() { _ = item.Hold() }()
	waitFor(t, func() bool {
		_, _, menu := h.Snapshot()
		return menu != nil && len(menu.Items) > 0
	}, "the menu to arrive")

	_, _, menu := h.Snapshot()
	for i, r := range TrayRows() {
		if r.Action != ActionNone {
			menu.Items[i].OnClick() // must not block
			break
		}
	}
	if !strings.Contains(strings.Join(said, "\n"), "dropped") {
		t.Errorf("it said %v, want a line saying the choice was dropped", said)
	}
}

func TestTheIconIsAPictureOfGlasses(t *testing.T) {
	b, err := TrayIcon(TrayIconPx, false)
	if err != nil {
		t.Fatalf("TrayIcon = %v", err)
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("what came back is not a PNG: %v", err)
	}
	// The LONGER side is the one asked for, and neither exceeds it: a system
	// symbol is not square, and a menu bar scales by height, so forcing a
	// square would show the glyph stretched.
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	if w != TrayIconPx && h != TrayIconPx {
		t.Errorf("the icon is %dx%d; one side should be the %d asked for", w, h, TrayIconPx)
	}
	if w > TrayIconPx || h > TrayIconPx {
		t.Errorf("the icon is %dx%d, bigger than the %d asked for", w, h, TrayIconPx)
	}
	// Something was drawn: an icon of nothing is an item nobody can find.
	inked := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a > 0 {
				inked++
			}
		}
	}
	if inked < TrayIconPx {
		t.Errorf("%d pixels of the icon are drawn; it is empty", inked)
	}
	if _, err := TrayIcon(0, false); err == nil {
		t.Error("an icon of no pixels was rendered")
	}
}

// waitFor spins until cond, or fails saying what it was waiting for.
func waitFor(t *testing.T, cond func() bool, what string) {
	t.Helper()
	for i := 0; i < 200; i++ {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("waited for %s and it never happened", what)
}

func TestReleaseAndAttachReachTheBackend(t *testing.T) {
	h := headless(t)
	actions := make(chan Action, TrayQueue)
	item, err := OpenTray(nil, actions)
	if err != nil {
		t.Fatalf("OpenTray = %v", err)
	}
	held := make(chan error, 1)
	go func() { held <- item.Hold() }()
	waitFor(t, func() bool {
		_, _, menu := h.Snapshot()
		return menu != nil && len(menu.Items) > 0
	}, "the menu to arrive")

	// Release stops the loop Hold is running, which is what lets the caller go
	// on to open a window and drive its own.
	item.Release()
	select {
	case <-held:
	case <-time.After(2 * time.Second):
		t.Fatal("Release did not stop the loop")
	}

	// Attach is for a loop somebody else is running. The headless backend does
	// not attach, and saying so is the contract: a caller that thinks it
	// attached and did not has an item nobody can see.
	if err := item.Attach(); err == nil {
		t.Error("Attach said yes on a backend that cannot attach")
	}
	if err := item.Close(); err != nil {
		t.Errorf("Close = %v", err)
	}
}

func TestAnItemWithNoPictureIsNotMade(t *testing.T) {
	headless(t)
	was := trayIcon
	trayIcon = func() ([]byte, error) { return nil, errors.New("no ink") }
	t.Cleanup(func() { trayIcon = was })

	if item, err := OpenTray(nil, make(chan Action, 1)); err == nil {
		_ = item.Close()
		t.Error("an item was made with no picture; a blank space in a menu bar " +
			"is a thing nobody can find")
	}
}

func TestEverySizeOfIconEncodes(t *testing.T) {
	// TrayIcon drops the encoding error because an NRGBA image with a positive
	// size always encodes. This is that assumption, pinned: the size it uses,
	// the smallest that means anything, and one far larger.
	for _, px := range []int{1, 16, TrayIconPx, 256} {
		b, err := TrayIcon(px, false)
		if err != nil {
			t.Fatalf("TrayIcon(%d) = %v", px, err)
		}
		img, err := png.Decode(bytes.NewReader(b))
		if err != nil {
			t.Errorf("TrayIcon(%d) is not a PNG: %v", px, err)
			continue
		}
		if w, h := img.Bounds().Dx(), img.Bounds().Dy(); w != px && h != px {
			t.Errorf("TrayIcon(%d) is %dx%d; one side should be %d", px, w, h, px)
		}
	}
	for _, px := range []int{0, -1} {
		if _, err := TrayIcon(px, false); err == nil {
			t.Errorf("TrayIcon(%d) made an icon", px)
		}
	}
}

func TestPngOfRefusesWhatIsNotAPicture(t *testing.T) {
	// The caller hands in pixels and a size, and the two can disagree: a
	// platform that answered with fewer bytes than it promised would otherwise
	// be encoded as whatever was next in memory.
	for _, c := range []struct {
		name string
		pix  []byte
		w, h int
	}{
		{"no width", make([]byte, 16), 0, 2},
		{"no height", make([]byte, 16), 2, 0},
		{"a negative side", make([]byte, 16), -2, 2},
		{"fewer bytes than pixels", make([]byte, 8), 2, 2},
	} {
		if _, err := pngOf(c.pix, c.w, c.h); err == nil {
			t.Errorf("%s: pngOf said yes", c.name)
		}
	}
	// And enough bytes, with some to spare, is a picture.
	if b, err := pngOf(make([]byte, 2*2*4+7), 2, 2); err != nil || len(b) == 0 {
		t.Errorf("pngOf = %d bytes, %v; want a PNG", len(b), err)
	}
}

// greenish says a colour is the dot rather than the glyph. The dot is drawn
// with antialiasing, so its rim is a blend; what separates it from a black or
// white glyph is that green LEADS.
func greenish(r, g, b uint32) bool { return g > r+0x2000 && g > b+0x2000 }

func TestTheDotSaysTheGlassesAreOn(t *testing.T) {
	plain, err := TrayIcon(TrayIconPx, false)
	if err != nil {
		t.Fatalf("TrayIcon(false) = %v", err)
	}
	lit, err := TrayIcon(TrayIconPx, true)
	if err != nil {
		t.Fatalf("TrayIcon(true) = %v", err)
	}
	if bytes.Equal(plain, lit) {
		t.Fatal("the icon with the glasses on is the same picture as the one without")
	}

	img, err := png.Decode(bytes.NewReader(lit))
	if err != nil {
		t.Fatalf("the lit icon is not a PNG: %v", err)
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	// Where the dot is, and where it is NOT: on the RIGHT, at the height of a
	// headset's temple, and not over the middle of the glyph. Counting green
	// anywhere would pass an icon dyed green.
	//
	// It sat in the bottom corner first, which put it below the frame with
	// nothing behind it -- a green disc BESIDE a pair of glasses rather than a
	// light on them. "que la led soit au niveau des branches des lunettes".
	temple, elsewhere := 0, 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, a := img.At(img.Bounds().Min.X+x, img.Bounds().Min.Y+y).RGBA()
			if a == 0 || !greenish(r, g, b) {
				continue
			}
			// The right third, and the middle half vertically: where a temple
			// is. Anything above or below that is not a temple light.
			if x >= w-w/3 && y >= h/4 && y <= h-h/4 {
				temple++
			} else {
				elsewhere++
			}
		}
	}
	if temple == 0 {
		t.Error("no green beside the temple; there is no dot")
	}
	if elsewhere > 0 {
		t.Errorf("%d green pixels away from the temple; the dot is not where it should be", elsewhere)
	}

	// And the platform must not paint the green away. A menu bar recolours a
	// TEMPLATE image to match the bar, which is exactly what the icon without
	// the dot wants and exactly what would erase the dot.
	if !tray.IsTemplate(plain) {
		t.Error("the plain icon is not a template; it will not follow a dark menu bar")
	}
	if tray.IsTemplate(lit) {
		t.Error("the lit icon reads as a template; the platform would recolour the dot away")
	}
}

func TestTheIconFollowsTheState(t *testing.T) {
	h := headless(t)
	actions := make(chan Action, TrayQueue)
	item, err := OpenTray(nil, actions)
	if err != nil {
		t.Fatalf("OpenTray = %v", err)
	}
	defer func() { _ = item.Close() }()
	go func() { _ = item.Hold() }()
	waitFor(t, func() bool {
		_, _, menu := h.Snapshot()
		return menu != nil && len(menu.Items) > 0
	}, "the menu to arrive")

	lit, err := TrayIcon(TrayIconPx, true)
	if err != nil {
		t.Fatalf("TrayIcon(true) = %v", err)
	}
	// Nothing is told to redraw: the state is set, and the icon follows.
	item.State().Set(TrayRunning)
	waitFor(t, func() bool {
		icon, _, _ := h.Snapshot()
		return bytes.Equal(icon, lit)
	}, "the icon to light up")

	plain, err := TrayIcon(TrayIconPx, false)
	if err != nil {
		t.Fatalf("TrayIcon(false) = %v", err)
	}
	item.State().Set(TrayWaiting)
	waitFor(t, func() bool {
		icon, _, _ := h.Snapshot()
		return bytes.Equal(icon, plain)
	}, "the icon to go dark again")
}

func TestADotOnWhatIsNotAPicture(t *testing.T) {
	if _, err := withDot([]byte("not a PNG")); err == nil {
		t.Error("a dot was drawn on something that is not a picture")
	}
	if _, err := withDotPixels(make([]byte, 4), 8, 8); err == nil {
		t.Error("a dot was drawn on 4 bytes of an 8x8 picture")
	}
	if _, err := withDotPixels(nil, 0, 0); err == nil {
		t.Error("a dot was drawn on a picture of no size")
	}
	// An icon too small to hold the smallest dot still yields a picture rather
	// than a box hanging off its own left edge.
	if _, err := withDotPixels(make([]byte, 4*4*4), 4, 4); err != nil {
		t.Errorf("a 4x4 icon: %v", err)
	}
}

// noSystemIcon takes the system symbol away for one test, which is what a
// machine that is not a Mac looks like from here.
func noSystemIcon(t *testing.T) {
	t.Helper()
	was := systemIcon
	systemIcon = func(int) ([]byte, error) { return nil, errors.New("no symbols here") }
	t.Cleanup(func() { systemIcon = was })
}

func TestTheToolkitGlassesWhenTheSystemHasNoSymbol(t *testing.T) {
	noSystemIcon(t)
	for _, dot := range []bool{false, true} {
		b, err := TrayIcon(TrayIconPx, dot)
		if err != nil {
			t.Fatalf("TrayIcon(dot=%v) = %v", dot, err)
		}
		img, err := png.Decode(bytes.NewReader(b))
		if err != nil {
			t.Fatalf("dot=%v: not a PNG: %v", dot, err)
		}
		// Square, here: the fallback is the toolkit's own glyph, which is drawn
		// to the box it is given rather than to a system symbol's proportions.
		if w, h := img.Bounds().Dx(), img.Bounds().Dy(); w != TrayIconPx || h != TrayIconPx {
			t.Errorf("dot=%v: the icon is %dx%d, want %d square", dot, w, h, TrayIconPx)
		}
		green := 0
		for y := 0; y < TrayIconPx; y++ {
			for x := 0; x < TrayIconPx; x++ {
				if r, g, bl, a := img.At(x, y).RGBA(); a > 0 && greenish(r, g, bl) {
					green++
				}
			}
		}
		if dot != (green > 0) {
			t.Errorf("dot=%v but %d green pixels", dot, green)
		}
	}
	// And the size is still refused before anything is drawn.
	if _, err := TrayIcon(0, true); err == nil {
		t.Error("an icon of no pixels was rendered")
	}
}

func TestTheDotFitsAnIconThatIsNotSquare(t *testing.T) {
	// A system symbol is never square, and the dot is sized off the SHORTER
	// side so it stays a dot instead of a bar down one edge. Tall and wide are
	// both tried: only one of them exercises the side that is picked.
	for _, box := range [][2]int{{12, 40}, {40, 12}} {
		w, h := box[0], box[1]
		b, err := withDotPixels(make([]byte, w*h*4), w, h)
		if err != nil {
			t.Fatalf("%dx%d: %v", w, h, err)
		}
		img, err := png.Decode(bytes.NewReader(b))
		if err != nil {
			t.Fatalf("%dx%d: not a PNG: %v", w, h, err)
		}
		minX, minY, maxX, maxY := w, h, -1, -1
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if _, _, _, a := img.At(x, y).RGBA(); a > 0 {
					minX, minY = min(minX, x), min(minY, y)
					maxX, maxY = max(maxX, x), max(maxY, y)
				}
			}
		}
		if maxX < 0 {
			t.Fatalf("%dx%d: nothing was drawn", w, h)
		}
		short := min(w, h)
		if got := max(maxX-minX+1, maxY-minY+1); got > short {
			t.Errorf("%dx%d: the dot is %d across, wider than the %d it has to fit in",
				w, h, got, short)
		}
		// At the right edge and half way down: where a temple is. Not flush
		// against the rim -- the toolkit leaves its own inset inside the box,
		// which is what keeps the dot off it.
		if minX < w/2 {
			t.Errorf("%dx%d: the dot starts at x=%d, not on the right", w, h, minX)
		}
		if w-1-maxX > short/3 {
			t.Errorf("%dx%d: the dot ends at x=%d, further from the edge than its own box",
				w, h, maxX)
		}
		// And CENTRED vertically, within a pixel or two of the middle: a light
		// that has slid to the top or the bottom is not on a temple.
		mid := (minY + maxY) / 2
		if off := mid - h/2; off > 2 || off < -2 {
			t.Errorf("%dx%d: the dot is centred at y=%d, %d from the middle", w, h, mid, off)
		}
	}
}
