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

	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit"
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
	// Where the light is, and where it is NOT: ACROSS THE LENS, in the middle
	// band of the icon. Counting green anywhere would pass an icon dyed green.
	//
	// It was a dot beside the temple first and it could not be seen: five or
	// six pixels of colour outside a bright glyph, next to twenty other icons.
	// "on ne voit plus le point vert. et si on mettait une ligne verte sur le
	// verre des lunettes?" -- which is the biggest mark this shape can carry
	// without stopping being a pair of glasses.
	lens, elsewhere := 0, 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, a := img.At(img.Bounds().Min.X+x, img.Bounds().Min.Y+y).RGBA()
			if a == 0 || !greenish(r, g, b) {
				continue
			}
			// The middle band, horizontally and vertically: on the glass. Any
			// green outside it is on the frame or off the glasses entirely.
			if x >= w/6 && x <= w-w/6 && y >= h/4 && y <= h-h/4 {
				lens++
			} else {
				elsewhere++
			}
		}
	}
	if lens == 0 {
		t.Error("no green on the lens; there is no light")
	}
	if elsewhere > 0 {
		t.Errorf("%d green pixels off the lens; the light is not where it should be", elsewhere)
	}
	// And BIG. The whole complaint about the dot was that it could not be seen,
	// so a light that shrank back to a speck would be the same defect again.
	if lens < w*h/40 {
		t.Errorf("the light is %d pixels of a %dx%d icon; that is a speck, not a light", lens, w, h)
	}

	// NEITHER icon is a template now, and that is the whole design: both carry
	// a coloured light, so the platform stops recolouring both -- which is why
	// the recolouring is done here instead.
	if tray.IsTemplate(lit) || tray.IsTemplate(plain) {
		t.Error("an icon carrying a light reads as a template; the platform would paint the light away")
	}

	// AND THE GLYPH WEARS THE SYSTEM'S COLOUR. This is the defect that
	// prompted it: "pourquoi l'icon tray est elle plus sombre quand on a le
	// point vert que les autres icons dans la barre?" -- because a
	// non-template is drawn as it is, and a system symbol is pure black, while
	// every neighbour had been painted in labelColor. Measured on the machine
	// that asked: white at 84.7%%, on a dark bar.
	want, ok := labelInk()
	if !ok {
		t.Skip("this machine will not say what colour it paints a template")
	}
	glyph, black := 0, 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, a := img.At(img.Bounds().Min.X+x, img.Bounds().Min.Y+y).RGBA()
			// Substantially opaque only. The rim of a glyph is anti-aliased,
			// and a barely-there edge pixel composited over transparent black
			// is dark whatever colour it was asked to be -- counting those
			// called a correctly tinted icon black in two thirds of its pixels.
			if a>>8 < 0x80 || greenish(r, g, b) {
				continue
			}
			glyph++
			if r>>8 < 0x20 && g>>8 < 0x20 && b>>8 < 0x20 && want.R > 0x20 {
				black++
			}
		}
	}
	if glyph == 0 {
		t.Fatal("the icon has no glyph, only a light")
	}
	if black > glyph/10 {
		t.Errorf("%d of %d glyph pixels are still black while the system paints templates %v; "+
			"the icon will look darker than its neighbours", black, glyph, want)
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

func TestAnIconBuiltFromWhatIsNotAPicture(t *testing.T) {
	// A system symbol that comes back as something other than a picture is an
	// error rather than an icon of noise.
	was := systemIcon
	t.Cleanup(func() { systemIcon = was })
	systemIcon = func(int) ([]byte, error) { return []byte("not a PNG"), nil }
	if _, err := TrayIcon(TrayIconPx, true); err == nil {
		t.Error("an icon was built out of something that is not a picture")
	}
}

func TestAnIconWithNoRoomForALight(t *testing.T) {
	// Four pixels square: too small for the smallest light, and it must still
	// yield a picture rather than a box hanging off its own left edge.
	if _, err := litIcon(nil, 4, 4, toolkit.RGB(0, 0, 0), DotInk, false); err != nil {
		t.Errorf("a 4x4 icon: %v", err)
	}
	if _, err := litIcon(nil, 0, 0, toolkit.RGB(0, 0, 0), DotInk, false); err == nil {
		t.Error("an icon of no size was accepted")
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

func TestTheLightFitsAnIconThatIsNotSquare(t *testing.T) {
	// A system symbol is never square. The light is a bar across the middle,
	// half the width and a sixth of the height, so a tall icon and a wide one
	// both get a line rather than a block or a hairline.
	for _, box := range [][2]int{{12, 40}, {40, 12}, {44, 27}} {
		w, h := box[0], box[1]
		buf := make([]byte, w*h*4)
		drawTheLight(painter.NewPixelPainterBGRA(buf, w, h), w, h, DotInk)

		minX, minY, maxX, maxY := w, h, -1, -1
		for y := range h {
			for x := range w {
				if buf[(y*w+x)*4+3] > 0 {
					minX, minY = min(minX, x), min(minY, y)
					maxX, maxY = max(maxX, x), max(maxY, y)
				}
			}
		}
		if maxX < 0 {
			t.Fatalf("%dx%d: nothing was drawn", w, h)
		}
		gotW, gotH := maxX-minX+1, maxY-minY+1
		// WIDER THAN TALL, always: a light that came out square or upright is
		// not a line across a lens.
		if gotW <= gotH {
			t.Errorf("%dx%d: the light is %dx%d; a bar is wider than it is tall", w, h, gotW, gotH)
		}
		// Centred, within a pixel, both ways.
		if off := (minX+maxX)/2 - w/2; off > 1 || off < -1 {
			t.Errorf("%dx%d: the light is centred at x=%d, %d off", w, h, (minX+maxX)/2, off)
		}
		if off := (minY+maxY)/2 - h/2; off > 1 || off < -1 {
			t.Errorf("%dx%d: the light is centred at y=%d, %d off", w, h, (minY+maxY)/2, off)
		}
		// And it stays INSIDE: a bar wider than the icon would run off the glass
		// and stop looking like a light behind it.
		if minX < 1 || maxX > w-2 {
			t.Errorf("%dx%d: the light spans %d..%d, out to the very edge", w, h, minX, maxX)
		}
	}
}
