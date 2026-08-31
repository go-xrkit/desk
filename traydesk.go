// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"time"

	"github.com/go-widgets/mvvm"

	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit"
	"github.com/go-widgets/tray"
)

// OpenTray puts the item in the menu bar and sends what is chosen to actions.
//
// THE ITEM IS go-widgets/tray's, NOT THIS PACKAGE'S. It was mine for a while,
// built straight on NSStatusItem, and that was a mistake with a symptom: a
// status item needs an NSApplication whose run loop is RUNNING, and this
// program has none until a display has been chosen and a window opened on it.
// Lending AppKit slices of the main thread drew the icon and never opened its
// menu -- "l'icon est visible dans le tray mais je n'ai aucun menu" -- because
// a menu is not drawn, it is tracked, and tracking needs the loop.
//
// tray owns that distinction: Run when a program has no loop of its own and
// Attach when it does. Both are its backends' business on three platforms
// rather than this package's on one.
//
// The send is NON-BLOCKING, and that is the whole of the design. A menu handler
// runs while the desk may be stopped -- the settings window is up, or the
// ribbon is between sessions -- so nobody is reading. A blocking send would
// leave that handler parked for as long as the window is open and then replay
// every click at once. A dropped choice is logged and forgotten, which is what
// a person clicking a menu with nothing happening expects: the next click, not
// the last five.
func OpenTray(logf func(string, ...any), actions chan<- Action) (*Tray, error) {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	icon, err := trayIcon()
	if err != nil {
		// No item rather than an item with no picture: a blank space in a menu
		// bar is a thing nobody can find or click.
		return nil, fmt.Errorf("desk: no menu-bar icon: %w", err)
	}
	rows := TrayRows()
	menu := tray.NewMenu()
	for _, r := range rows {
		if r.Action == ActionNone {
			menu.Add(tray.Separator())
			continue
		}
		a, name := r.Action, r.Title
		menu.Add(tray.Item(r.Title, func() {
			select {
			case actions <- a:
			default:
				logf("%q was chosen while the desk was not listening; dropped", name)
			}
		}))
	}
	t := tray.New(icon)
	state := mvvm.NewObservable(TrayWaiting)
	if b := trayBackend(); b != nil {
		t = t.WithBackend(b)
	}
	t.SetTooltip(TrayTooltip)
	t.SetMenu(menu)
	logf("the menu bar item is built, with %d rows", len(rows))
	item := &Tray{t: t, logf: logf, state: state}
	item.stop = item.follow()
	return item, nil
}

// trayIcon is the seam for the picture, so a test can take it away: an item
// with no icon must not be made at all.
var trayIcon = func() ([]byte, error) { return TrayIcon(TrayIconPx, false) }

// trayBackend is the seam: nil means the platform's own, which is what a
// program wants and what a test cannot have -- a menu bar is one per machine
// and a test that put an item in somebody's would leave it there.
var trayBackend = func() tray.Backend { return nil }

// Tray is the desk's menu-bar item, and the two ways it can live.
type Tray struct {
	t     *tray.Tray
	logf  func(string, ...any)
	state *mvvm.Observable[TrayState]
	stop  func()
}

// Hold runs the item AND the platform's main loop, and returns when Release is
// called.
//
// It is for the wait: there is no window yet, so nothing else is driving
// AppKit, and without a loop the item is an object nobody can open. It must be
// called on the main thread.
func (t *Tray) Hold() error { return t.t.Run() }

// Release stops the loop Hold is running, so the caller can go on to open a
// window and drive its own.
func (t *Tray) Release() { t.t.Quit() }

// Attach adds the item to a loop somebody else is already running, and returns
// at once. It is for the desk: the window owns the main thread from then on.
func (t *Tray) Attach() error { return t.t.Attach() }

// Close takes the item away and stops the icon following anything.
func (t *Tray) Close() error {
	if t.stop != nil {
		t.stop()
		t.stop = nil
	}
	t.t.Quit()
	return nil
}

// TrayIconPx is the icon's LONGER side, in pixels.
//
// Twice the menu bar's own height, so it is still sharp on a display that draws
// two pixels per point. The other side follows the glyph: a system symbol is
// not square -- visionpro is 21 by 13 points -- and the bar scales what it is
// given by height, so a picture forced square would show the glyph stretched.
const TrayIconPx = 44

// TrayTooltip is what the item says when somebody rests on it.
const TrayTooltip = "XR desk"

// systemIcon is the platform symbol, as a seam.
//
// Above the platform boundary so a test on EITHER platform can take both
// paths: on a Mac the system answers and the fallback is never reached, on a
// runner the reverse, and a branch that only one of them can take is a branch
// the coverage gate can never see closed.
var systemIcon = platformTrayIcon

// TrayIcon renders the menu-bar icon, as PNG bytes.
//
// The SYSTEM's own symbol where there is one, and the toolkit's glasses
// otherwise. It is not a matter of taste: measured at 44 pixels, the toolkit's
// outline inks 7% of the box and a system symbol about 62%, and the difference
// is whether a person finds it among twenty other icons.
//
// The fallback is the toolkit's, not a hand-painted one: the same glyph is on
// the settings window, and an icon painted here would be an icon to maintain
// here.
func TrayIcon(px int, dot bool) ([]byte, error) {
	if px <= 0 {
		return nil, fmt.Errorf("desk: an icon of %d pixels", px)
	}
	if b, err := systemIcon(px); err == nil {
		if !dot {
			return b, nil
		}
		return withDot(b)
	}
	buf := make([]byte, px*px*4)
	p := painter.NewPixelPainterBGRA(buf, px, px)
	toolkit.DrawIconGlasses(p, toolkit.Rect{W: px, H: px}, toolkit.RGB(0, 0, 0))

	// BGRA to NRGBA: the painter writes what a canvas wants and a PNG wants the
	// other order.
	img := image.NewNRGBA(image.Rect(0, 0, px, px))
	for i := 0; i+3 < len(buf); i += 4 {
		img.Pix[i] = buf[i+2]
		img.Pix[i+1] = buf[i+1]
		img.Pix[i+2] = buf[i]
		img.Pix[i+3] = buf[i+3]
	}
	if dot {
		return withDotPixels(img.Pix, px, px)
	}
	return pngOf(img.Pix, px, px)
}

// pngOf encodes straight RGBA pixels as a PNG.
//
// The error is dropped because there is none to have: an NRGBA image with a
// positive size always encodes, and a bytes.Buffer never fails to be written
// to. TestEverySizeOfIconEncodes pins that rather than leaving a branch nothing
// can take.
func pngOf(pix []byte, w, h int) ([]byte, error) {
	if w <= 0 || h <= 0 || len(pix) < w*h*4 {
		return nil, fmt.Errorf("desk: %d bytes for a %dx%d picture", len(pix), w, h)
	}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	copy(img.Pix, pix[:w*h*4])
	var out bytes.Buffer
	_ = png.Encode(&out, img)
	return out.Bytes(), nil
}

// TrayState is what the menu-bar icon says about the desk.
type TrayState int

const (
	// TrayWaiting is the desk with no screens up: waiting for a pair of
	// glasses, or between sessions.
	TrayWaiting TrayState = iota
	// TrayRunning is a desk on a headset, with screens.
	TrayRunning
)

// DotInk is the colour of the dot that says the glasses are on.
//
// A green, and the same green the gallery uses for the screen it has chosen:
// one program, one word for "this one is live".
var DotInk = SelectionInk

// State is what the icon follows.
//
// An Observable rather than a setter, because that is how anything in this
// fleet says "this changed" across a boundary, and because tray.BindIcon takes
// one: the icon then follows the desk without either knowing about the other.
func (t *Tray) State() *mvvm.Observable[TrayState] { return t.state }

// follow makes the icon follow the state, and returns the way to stop.
func (t *Tray) follow() func() {
	icons := tray.Icons[TrayState]{}
	for s, dot := range map[TrayState]bool{TrayWaiting: false, TrayRunning: true} {
		if b, err := TrayIcon(TrayIconPx, dot); err == nil {
			icons[s] = [][]byte{b}
		}
	}
	// One frame each: nothing here animates. The period is what an animation
	// would use and is unused with a single frame; it is given rather than
	// left zero because zero is a ticker that fires as fast as it can.
	return tray.BindIcon(t.t, t.state, icons, time.Second)
}

// withDot puts the running dot on an icon that is already PNG.
func withDot(iconPNG []byte) ([]byte, error) {
	img, err := png.Decode(bytes.NewReader(iconPNG))
	if err != nil {
		return nil, fmt.Errorf("desk: the icon is not a picture: %w", err)
	}
	b := img.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			out.Set(x, y, img.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return withDotPixels(out.Pix, b.Dx(), b.Dy())
}

// withDotPixels draws the running dot over straight RGBA pixels.
//
// THE DOT IS THE STATE, and it is a colour on purpose: a menu bar can say
// "this is live" without anybody opening anything, and a shape change would
// have to be looked at twice. It costs the icon its template treatment -- an
// image carrying colour is not recoloured by the platform, which is what makes
// the green survive -- and that is the trade being made knowingly.
//
// The disc itself comes from the toolkit -- toolkit.DrawIconDot -- like every
// other mark this package puts on screen. Nothing here rasterises a shape by
// hand, and the barrier test in this package enforces that.
func withDotPixels(pix []byte, w, h int) ([]byte, error) {
	if w <= 0 || h <= 0 || len(pix) < w*h*4 {
		return nil, fmt.Errorf("desk: %d bytes for a %dx%d picture", len(pix), w, h)
	}
	// BGRA for the painter, which is the order it writes.
	buf := make([]byte, w*h*4)
	for i := 0; i+3 < w*h*4; i += 4 {
		buf[i], buf[i+1], buf[i+2], buf[i+3] = pix[i+2], pix[i+1], pix[i], pix[i+3]
	}
	p := painter.NewPixelPainterBGRA(buf, w, h)
	// A third of the shorter side, at the right edge and HALF WAY DOWN -- where
	// a headset's temple is.
	//
	// It sat in the bottom corner first, which put it below the frame with
	// nothing behind it: a green disc floating beside a pair of glasses rather
	// than a light ON them. Beside the temple it reads as what it is, the way
	// the indicator on a real headset does.
	//
	// The toolkit leaves its own inset inside the box -- wanted here, it keeps
	// the dot off the very edge -- so the disc that shows is about a QUARTER of
	// the icon: big enough to read at menu-bar size, small enough to leave the
	// glyph legible behind it.
	short := h
	if w < short {
		short = w
	}
	d := short / 3
	if d < 6 {
		// Six is the smallest dot worth drawing, and never wider than the icon
		// itself: a box hanging off the left edge would put the dot over the
		// glyph instead of beside it.
		d = min(6, short)
	}
	toolkit.DrawIconDot(p, toolkit.Rect{X: w - d, Y: (h - d) / 2, W: d, H: d}, DotInk)

	for i := 0; i+3 < w*h*4; i += 4 {
		pix[i], pix[i+1], pix[i+2], pix[i+3] = buf[i+2], buf[i+1], buf[i], buf[i+3]
	}
	return pngOf(pix, w, h)
}

// glassesIcon is the system's glasses symbol as a drawable icon, as a seam.
//
// Nil where the system has none, and above the platform boundary for the same
// reason systemIcon is: a test on either platform can take both paths.
var glassesIcon = platformGlassesIcon
