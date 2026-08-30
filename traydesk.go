// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"bytes"
	"fmt"
	"image"
	"image/png"

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
	if b := trayBackend(); b != nil {
		t = t.WithBackend(b)
	}
	t.SetTooltip(TrayTooltip)
	t.SetMenu(menu)
	logf("the menu bar item is built, with %d rows", len(rows))
	return &Tray{t: t, logf: logf}, nil
}

// trayIcon is the seam for the picture, so a test can take it away: an item
// with no icon must not be made at all.
var trayIcon = func() ([]byte, error) { return TrayIcon(TrayIconPx) }

// trayBackend is the seam: nil means the platform's own, which is what a
// program wants and what a test cannot have -- a menu bar is one per machine
// and a test that put an item in somebody's would leave it there.
var trayBackend = func() tray.Backend { return nil }

// Tray is the desk's menu-bar item, and the two ways it can live.
type Tray struct {
	t    *tray.Tray
	logf func(string, ...any)
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

// Close takes the item away.
func (t *Tray) Close() error { t.t.Quit(); return nil }

// TrayIconPx is how big the icon is rendered, in pixels. Twice the menu bar's
// own height, so it is still sharp on a display that draws two pixels per
// point.
const TrayIconPx = 44

// TrayTooltip is what the item says when somebody rests on it.
const TrayTooltip = "XR desk"

// TrayIcon renders the desk's own glasses, as PNG bytes.
//
// Through the toolkit, which draws them already: an icon painted here would be
// an icon to maintain here, and the same glyph appears on the settings window.
func TrayIcon(px int) ([]byte, error) {
	if px <= 0 {
		return nil, fmt.Errorf("desk: an icon of %d pixels", px)
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
	// The error is dropped because there is none to have: an NRGBA image with a
	// positive size always encodes, and a bytes.Buffer never fails to be
	// written to. TestEverySizeOfIconEncodes pins that rather than leaving a
	// branch nothing can take.
	var out bytes.Buffer
	_ = png.Encode(&out, img)
	return out.Bytes(), nil
}
