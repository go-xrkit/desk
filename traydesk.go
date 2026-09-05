// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"sync"
	"time"

	"github.com/go-macos/hotkey"
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
	t := tray.New(icon)
	state := mvvm.NewObservable(TrayWaiting)
	if b := trayBackend(); b != nil {
		t = t.WithBackend(b)
	}
	t.SetTooltip(TrayTooltip)
	item := &Tray{t: t, logf: logf, state: state, actions: actions}
	t.SetMenu(item.buildMenu(nil))
	logf("the menu bar item is built, with %d rows", len(rows))
	item.stop = item.follow()
	return item, nil
}

// TrayRowPx is a menu ROW's glyph, in pixels.
//
// go-widgets/tray draws a row's icon at 16 points, so 32 pixels is that at 2x
// and a Retina display gets one image pixel per device pixel with nothing
// resampled. It is NOT TrayIconPx: a menu-bar icon sits in a 22-point bar and a
// row's icon sits beside the row's text, and a symbol rasterised for one and
// drawn at the other has strokes of the wrong weight.
const TrayRowPx = 32

// rowIcons holds the row glyphs, made once each.
//
// A menu is built once per session today, but the state binding rebuilds the
// item's picture every second and this is one symbol lookup away from being
// asked for on that path. A picture that cannot have changed is not worth
// asking the window server for twice.
var rowIcons sync.Map // symbol name -> []byte, nil for one that would not draw

// rowIcon is the glyph a row carries, or nil.
//
// NIL RATHER THAN AN ERROR, deliberately: a symbol this system does not have
// must cost a row its picture, not its menu. The row still says "Quit the desk"
// and still quits. TestEveryMenuRowCarriesASymbol is what keeps that silence
// from covering a typo -- the degradation is a safety net, not the way this
// normally runs.
func rowIcon(symbol string) []byte {
	if symbol == "" {
		return nil
	}
	if b, ok := rowIcons.Load(symbol); ok {
		b, _ := b.([]byte)
		return b
	}
	b, err := systemSymbol(symbol, TrayRowPx)
	if err == nil {
		b, err = squared(b, TrayRowPx)
	}
	if err != nil {
		b = nil
	}
	rowIcons.Store(symbol, b)
	return b
}

// systemSymbol is the platform's symbol by name, as a seam -- the same reason
// systemIcon is one: a test on either platform can take both paths.
var systemSymbol = platformSymbolPNG

// trayIcon is the seam for the picture, so a test can take it away: an item
// with no icon must not be made at all.
var trayIcon = func() ([]byte, error) { return TrayIcon(TrayIconPx, false) }

// trayBackend is the seam: nil means the platform's own, which is what a
// program wants and what a test cannot have -- a menu bar is one per machine
// and a test that put an item in somebody's would leave it there.
var trayBackend = func() tray.Backend { return nil }

// Tray is the desk's menu-bar item, and the two ways it can live.
type Tray struct {
	t       *tray.Tray
	logf    func(string, ...any)
	state   *mvvm.Observable[TrayState]
	stop    func()
	actions chan<- Action

	// mu guards what the MENU says about the world: the combinations the
	// machine granted, and whether the 3D conversion is on. Both arrive from
	// somewhere else while the item is already in the menu bar -- a session
	// claiming keys, a converter opening or being refused.
	mu     sync.Mutex
	keys   map[Action]hotkey.Combo
	threeD Stereo3D
	// running mirrors the state observable, for the "use the glasses" tick.
	//
	// ⛔ A MIRROR RATHER THAN A READ. mvvm.Observable has no lock -- it is
	// built for one thread, the one that draws -- and this menu is rebuilt
	// from whichever goroutine noticed a change: the session, for the
	// shortcuts, and the 3D conversion, for its tick. So the value is copied
	// under THIS lock when it changes, and buildMenu reads the copy.
	running bool
}

// ShowShortcuts puts the combination that was GRANTED on each row it belongs
// to, and rebuilds the menu.
//
// Granted rather than asked for, which is why this is a method called later
// instead of an argument to [OpenTray]. The item is made once for the whole
// process and outlives every session, while the shortcuts are claimed when a
// session starts -- and a claim is not a grant: the ladder substitutes when a
// combination is taken, so a menu built from what was ASKED for would print a
// combination that does nothing. A row whose action was never granted keeps its
// bare label rather than a lie.
//
// The rows a person can reach with the keyboard say so here rather than in the
// settings window, which is where they used to be listed: a menu row and the
// key that does the same thing belong on the same line, and moving them takes
// most of a page out of a window that had grown too tall for a laptop screen.
func (t *Tray) ShowShortcuts(keys map[Action]hotkey.Combo) {
	if t == nil {
		return
	}
	// Remembered, because the OTHER thing that rebuilds this menu -- the 3D
	// tick -- must not throw the combinations away to do it.
	t.mu.Lock()
	t.keys = keys
	t.mu.Unlock()
	t.t.SetMenu(t.buildMenu(keys))
}

// Show3D tells the item whether the 3D conversion is on, and rebuilds.
//
// ⛔ WHAT HAPPENED, NOT WHAT WAS ASKED. Turning 3D on can be refused -- by a
// display that shows one eye, or by a depth model that will not load -- and a
// tick that followed the request would then say the desk is in a state it is
// not. The application calls this from the place that knows: after the
// conversion has been made or refused.
func (t *Tray) Show3D(s Stereo3D) {
	if t == nil {
		return
	}
	t.mu.Lock()
	if t.threeD == s {
		t.mu.Unlock()
		return
	}
	t.threeD = s
	keys := t.keys
	t.mu.Unlock()
	t.t.SetMenu(t.buildMenu(keys))
}

// stateFor reports what a toggling row should show: whether it is on, and why
// it cannot be turned on at all.
//
// A function of the state rather than a field on TrayRow: a row is a
// DESCRIPTION, written once, and what it describes changes while the menu is on
// screen.
func stateFor(a Action, threeD Stereo3D, running bool) (on bool, why string) {
	switch a {
	case ActionStereo3D:
		return threeD.On, threeD.Why
	case ActionPause:
		// Ticked while a desk IS up, because the row says "use the glasses"
		// and a tick is the platform's word for "this is on". The tick is
		// also the only thing that says which way the row will go: macOS
		// draws nothing at all for an unticked item, so a row whose title
		// changed with the state would read as one word with no state.
		return running, ""
	default:
		return false, ""
	}
}

// buildMenu is the rows, with whatever combinations are known.
func (t *Tray) buildMenu(keys map[Action]hotkey.Combo) *tray.Menu {
	// Read ONCE, under the lock. A menu half built before a change and half
	// after would show two answers to one question.
	t.mu.Lock()
	threeD, running := t.threeD, t.running
	t.mu.Unlock()

	menu := tray.NewMenu()
	for _, r := range TrayRows() {
		if r.Action == ActionNone {
			menu.Add(tray.Separator())
			continue
		}
		a, name := r.Action, r.Title
		// ⛔ ONLY WHAT IS IN THE MAP. A missing entry gives the zero Combo, whose
		// key code is 0 -- and code 0 is a real key: ANSI calls it A and a French
		// keyboard prints Q on it. Taken as a combination that would bind a BARE
		// letter on every row nothing was granted for. Found by the test that
		// says a row nothing was granted for names no key at all.
		var key string
		var mods tray.Mods
		if c, ok := keys[a]; ok {
			key, mods = trayKey(c)
		}
		choose := func() {
			select {
			case t.actions <- a:
			default:
				t.logf("%q was chosen while the desk was not listening; dropped", name)
			}
		}
		var it *tray.MenuItem
		if r.Toggle {
			// ⛔ THE ROW SAYS ITS STATE THREE WAYS, because one is not enough.
			// macOS draws a checkmark for a menu item that is on and NOTHING
			// AT ALL for one that is off, so a tick alone answers "is it on?"
			// with silence whenever the answer is no -- which is how somebody
			// came to ask how they were supposed to tell.
			//
			// So: the TICK is the platform's own convention for on; the SYMBOL
			// changes between the pair the system uses for 2D and 3D, which
			// answers in BOTH directions; and a conversion that cannot be
			// turned on here at all is DISABLED and says why, because a row
			// that looks pressable and does nothing is a row somebody presses
			// again and again.
			//
			// The Checked field is flipped by tray before the callback runs,
			// and overwritten on the next rebuild by what actually happened --
			// which matters, because turning 3D on can be refused.
			on, why := stateFor(a, threeD, running)
			sym := r.Symbol
			if on && r.SymbolOn != "" {
				sym = r.SymbolOn
			}
			label := r.Title
			if why != "" {
				label = r.Title + " — " + why
			}
			it = tray.Checkbox(label, on, func(bool) { choose() })
			it.Icon = rowIcon(sym)
			it.Key, it.Mods = key, mods
			it.Disabled = why != ""
		} else {
			it = tray.KeyItem(r.Title, rowIcon(r.Symbol), key, mods, choose)
		}
		menu.Add(it)
	}
	return menu
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
	stop := tray.BindIcon(t.t, t.state, icons, time.Second)
	// And the MENU follows it too, for the "use the glasses" tick.
	//
	// A menu is built once and kept, so a row whose tick describes the session
	// has to be rebuilt when the session starts or stops -- otherwise the row
	// that puts the glasses down still reads as ticked afterwards, which is the
	// one moment a person looks at it.
	unwatch := t.state.Subscribe(func(s TrayState) {
		t.mu.Lock()
		t.running = s == TrayRunning
		keys := t.keys
		t.mu.Unlock()
		t.t.SetMenu(t.buildMenu(keys))
	})
	return func() { unwatch(); stop() }
}

// glassesIcon is the system's glasses symbol as a drawable icon, as a seam.
//
// Nil where the system has none, and above the platform boundary for the same
// reason systemIcon is: a test on either platform can take both paths.
var glassesIcon = platformGlassesIcon

// drawTheLight puts the state light across the LENS.
//
// It was a dot beside the temple first, and it could not be seen: five or six
// pixels of colour outside a bright glyph, on a menu bar, next to twenty other
// icons. "on ne voit plus le point vert" -- and the suggestion that followed is
// the right one, because a line on the lens is the biggest mark this shape can
// carry without stopping being a pair of glasses.
//
// Half the width, a sixth of the height, centred: on the lens rather than on
// the frame, which is what makes it read as something lit up BEHIND the glass
// instead of a sticker on the rim.
func drawTheLight(p painter.Painter, w, h int, ink toolkit.RGBA) {
	bar := toolkit.Rect{W: w / 2}
	if bar.W < 4 {
		bar.W = min(4, w)
	}
	// The thickness is measured against the BAR, not against the icon: a sixth
	// of a tall narrow icon is taller than the bar is wide, and what comes out
	// is a square rather than a line.
	bar.H = min(h/6, bar.W/3)
	if bar.H < 2 {
		bar.H = min(2, h)
	}
	bar.X = (w - bar.W) / 2
	bar.Y = (h - bar.H) / 2
	// DrawIconBar and not DrawIconDot: the dot insets its box by two pixels a
	// side, which eats a four-pixel bar entirely and draws nothing at all.
	toolkit.DrawIconBar(p, bar, ink)
}

// labelInk is the colour the platform paints a template menu-bar icon with, as
// a seam. Above the platform boundary so a test on either platform can take
// both paths: an icon built where the system will not say is a branch the
// coverage gate could otherwise never see closed.
var labelInk = platformLabelInk

// squared centres a picture in a transparent square of the given side.
//
// ⛔ WITHOUT THIS THE ROWS ARE NOT THE SAME SIZE, and it is not a matter of
// taste: go-widgets/tray normalises a row icon's HEIGHT to 16 points and lets
// the width follow the aspect ratio -- right for a caller shipping a wordmark,
// wrong for a set of glyphs. A symbol comes back at the size of its own ink, so
// "eyeglasses" is 32x14 and "power" is 30x32; drawn at a common height the
// first is 37 points wide and the second is 15, and a menu of them reads as a
// mistake. "les icons utilisées ne semble avoir une taille uniforme."
//
// A square is also what the system itself lays symbols out in: appicon renders
// each one so its LONGER side is the size asked for, which is the symbol's
// bounding box. Centring that box in a square of the same side normalises every
// glyph to it and changes not one pixel of ink.
//
// A picture already that square is returned untouched, so nothing is decoded
// and re-encoded for nothing.
func squared(b []byte, side int) ([]byte, error) {
	pix, w, h, err := decodePixels(b)
	if err != nil {
		return nil, err
	}
	if w == side && h == side {
		return b, nil
	}
	if w > side || h > side {
		return nil, fmt.Errorf("desk: a %dx%d picture does not fit a square of %d", w, h, side)
	}
	out := make([]byte, side*side*4)
	x0, y0 := (side-w)/2, (side-h)/2
	for y := range h {
		copy(out[((y0+y)*side+x0)*4:], pix[y*w*4:(y+1)*w*4])
	}
	return pngOf(out, side, side)
}
