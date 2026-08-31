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

// TrayIcon renders the menu-bar icon, as PNG bytes, with the light lit for the
// state given: green while a desk is up, red while there is none.
//
// The SYSTEM's own symbol where there is one, and the toolkit's glasses
// otherwise. It is not a matter of taste: measured at 44 pixels, the toolkit's
// outline inks 7% of the box and a system symbol about 62%, and the difference
// is whether a person finds it among twenty other icons.
//
// THE GLYPH IS RECOLOURED HERE because the platform has stopped doing it. An
// image carrying a colour is not a template, so macOS draws its pixels as they
// are -- and a system symbol is pure black, which on a dark menu bar sits
// among neighbours the platform has painted white at 85%. So the recolouring
// the platform would have done is done with the colour the platform would have
// used, read from the system when the icon is built rather than remembered:
// labelColor follows the appearance, and an icon built once would be wrong
// after somebody switches to dark.
func TrayIcon(px int, live bool) ([]byte, error) {
	if px <= 0 {
		return nil, fmt.Errorf("desk: an icon of %d pixels", px)
	}
	ink := DotInk
	if !live {
		ink = WaitingInk
	}
	glyph := toolkit.RGB(0, 0, 0) // what a template is drawn as on a light bar
	if c, ok := labelInk(); ok {
		// The COLOUR, at full opacity, not the colour at its own alpha.
		// labelColor is 85% transparent because the platform composites it
		// onto the bar; painting it that way into a transparent picture
		// composites it onto BLACK instead, and 85% white over black is grey.
		// The shape's alpha comes from the stencil, so what is wanted here is
		// the hue and nothing else. The icon ends a shade brighter than its
		// neighbours rather than a shade darker, which is the right way round
		// to be wrong.
		glyph = toolkit.RGB(c.R, c.G, c.B)
	}

	if b, err := systemIcon(px); err == nil {
		stencil, w, h, err := decodePixels(b)
		if err != nil {
			return nil, err
		}
		return litIcon(stencil, w, h, glyph, ink, true)
	}
	return litIcon(nil, px, px, glyph, ink, false)
}

// litIcon draws the glyph and its light into one picture.
//
// stencil, when given, is a picture whose ALPHA is the shape -- what a system
// symbol is -- and it is drawn through the toolkit so the colour comes from
// here. Without one the toolkit's own glasses are drawn instead, and then the
// picture is square because that glyph is drawn to the box it is given rather
// than to a symbol's proportions.
//
// Nothing here paints a pixel by hand: the glyph is toolkit.StencilIcon and the
// light is toolkit.DrawIconDot. The barrier test in this package enforces that.
func litIcon(stencil []byte, w, h int, glyph, light toolkit.RGBA, fromStencil bool) ([]byte, error) {
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("desk: an icon of %dx%d", w, h)
	}
	buf := make([]byte, w*h*4)
	p := painter.NewPixelPainterBGRA(buf, w, h)
	box := toolkit.Rect{W: w, H: h}
	if fromStencil {
		toolkit.StencilIcon(stencil, w, h)(p, box, glyph)
	} else {
		toolkit.DrawIconGlasses(p, box, glyph)
	}
	drawTheLight(p, w, h, light)

	// BGRA to NRGBA: the painter writes what a canvas wants and a PNG wants the
	// other order.
	pix := make([]byte, w*h*4)
	for i := 0; i+3 < len(buf); i += 4 {
		pix[i], pix[i+1], pix[i+2], pix[i+3] = buf[i+2], buf[i+1], buf[i], buf[i+3]
	}
	return pngOf(pix, w, h)
}

// decodePixels turns a PNG into straight RGBA bytes.
func decodePixels(b []byte) ([]byte, int, int, error) {
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("desk: the icon is not a picture: %w", err)
	}
	r := img.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	for y := 0; y < r.Dy(); y++ {
		for x := 0; x < r.Dx(); x++ {
			out.Set(x, y, img.At(r.Min.X+x, r.Min.Y+y))
		}
	}
	return out.Pix, r.Dx(), r.Dy(), nil
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

// DotInk is the colour of the light when a desk is up.
//
// A green, and the same green the gallery uses for the screen it has chosen:
// one program, one word for "this one is live".
var DotInk = SelectionInk

// WaitingInk is the colour of the light when there is no desk.
//
// RED, because the menu bar should answer the question without being opened,
// and "no light at all" is not an answer -- an icon with nothing on it reads as
// an icon, not as a state. Asked for: "on pourrait avoir un point rouge quand
// les lunettes ne sont pas en action".
//
// Muted rather than a signal red: it is a resting state, not a fault, and a
// menu bar full of alarm colours teaches a person to stop looking.
var WaitingInk = toolkit.RGB(0xE0, 0x6C, 0x6C)

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

// glassesIcon is the system's glasses symbol as a drawable icon, as a seam.
//
// Nil where the system has none, and above the platform boundary for the same
// reason systemIcon is: a test on either platform can take both paths.
var glassesIcon = platformGlassesIcon

// drawTheLight puts the state light where a headset's temple is.
//
// A third of the shorter side, at the right edge and half way down. It sat in
// the bottom corner first, which put it BELOW the frame with nothing behind it
// -- a coloured disc beside a pair of glasses rather than a light on them.
//
// The toolkit leaves its own inset inside the box, which is wanted: it keeps
// the light off the very rim. So what shows is about a quarter of the icon --
// big enough to read at menu-bar size, small enough to leave the glyph legible.
func drawTheLight(p painter.Painter, w, h int, ink toolkit.RGBA) {
	short := h
	if w < short {
		short = w
	}
	d := short / 3
	if d < 6 {
		// Six is the smallest light worth drawing, and never wider than the
		// icon itself: a box hanging off the left edge would put it over the
		// glyph instead of beside it.
		d = min(6, short)
	}
	toolkit.DrawIconDot(p, toolkit.Rect{X: w - d, Y: (h - d) / 2, W: d, H: d}, ink)
}

// labelInk is the colour the platform paints a template menu-bar icon with, as
// a seam. Above the platform boundary so a test on either platform can take
// both paths: an icon built where the system will not say is a branch the
// coverage gate could otherwise never see closed.
var labelInk = platformLabelInk
