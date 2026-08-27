// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

// Drawing the window as an 8K panel actually gets it: magnified, and in the
// host's own face rather than the built-in bitmap.
//
// This is a separate render from the one beside it on purpose. That one is the
// unmagnified default, portable and identical everywhere, which is what a
// regression is measured against. This one is what the person in front of the
// machine sees, and the two differ in every metric that comes from the font.
package desk

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit"
	"github.com/go-widgets/window"
	"github.com/go-xrkit/xrkit/glasses"
)

// eightK is the row count of the display this was reported against.
const eightK = 2160

// TestRenderTheSettingsWindowForAnEightKPanel draws it at the magnification and
// in the face RunSettings would use there, and — the part a rectangle cannot
// show — leaves the picture somewhere a person can open it.
//
// The assertion is about clipping, and it is only possible in this order. The
// window's height is computed from the font (a label row is a glyph high plus
// padding), so it MUST be installed before the height is asked for. Getting
// that backwards sizes the window for the bitmap font and then draws the system
// face into it, and the buttons go under the bottom edge — which is how this
// window was already wrong once, on a Linux runner, for the same reason.
func TestRenderTheSettingsWindowForAnEightKPanel(t *testing.T) {
	was := toolkit.MetricScale()
	t.Cleanup(func() { toolkit.SetMetricScale(was) })
	toolkit.SetMetricScale(SettingsScale(eightK))
	if got := toolkit.MetricScale(); got < 2 {
		t.Fatalf("an %d-row panel asked for a scale of %.2f; nothing is magnified",
			eightK, got)
	}

	face := "the built-in bitmap"
	if ttf, err := window.SystemFontTTF(); err == nil {
		f, err := toolkit.NewTrueTypeFont(ttf, toolkit.Scaled(SettingsFontPx))
		if err != nil {
			t.Fatalf("the system face is %d bytes and would not parse: %v", len(ttf), err)
		}
		wasFont := toolkit.CurrentFont()
		t.Cleanup(func() { toolkit.SetFont(wasFont) })
		toolkit.SetFont(f)
		face = "the system face"
	} else {
		// Linux names a family and leaves finding it to fontconfig, so there is
		// nothing to read there. The magnification is still worth drawing.
		t.Logf("no system face here, drawing in the built-in one: %v", err)
	}

	cfg := &Config{}
	attached := []glasses.USB{oneS, luma}
	w, h := toolkit.Scaled(SettingsWidth), settingsHeight(*cfg, attached)
	form, _ := settingsRoot(cfg, attached, nil, func() {})
	root := settingsSurface(form)
	root.SetBounds(toolkit.Rect{X: 0, Y: 0, W: w, H: h})

	// With the screen-count list OPEN, because that is the state a person
	// reported as broken and the one no earlier picture ever showed.
	var count *toolkit.DropDown
	var find func(toolkit.Widget)
	find = func(x toolkit.Widget) {
		if d, ok := x.(*toolkit.DropDown); ok {
			count = d
		}
		if p, ok := x.(interface{ Children() []toolkit.Widget }); ok {
			for _, kid := range p.Children() {
				find(kid)
			}
		}
	}
	find(root)
	if count == nil {
		t.Fatal("no drop-down in the window")
	}
	count.Open().Set(true)

	// Nothing is drawn under the bottom edge of the window it was sized for.
	if deep := deepest(root); deep > h {
		t.Errorf("the tree reaches %d pixels down, past the %d-pixel window: "+
			"in %s at %d pixels, a row is taller than the height was told",
			deep, h, face, toolkit.Scaled(SettingsFontPx))
	}

	buf := make([]byte, w*h*4)
	p := painter.NewPixelPainter(buf, w, h)
	theme := toolkit.DefaultDark()
	p.FillRect(painter.Rect{X: 0, Y: 0, W: w, H: h}, theme.Background)
	root.Draw(p, theme)

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	copy(img.Pix, buf)
	out := filepath.Join(renderDir(t), "settings-8k.png")
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	t.Logf("%dx%d in %s at scale %.2f: %s", w, h, face, toolkit.MetricScale(), out)
}

// deepest is the bottom edge of the lowest thing in the tree.
func deepest(w toolkit.Widget) int {
	r := w.Bounds()
	deep := r.Y + r.H
	if p, ok := w.(interface{ Children() []toolkit.Widget }); ok {
		for _, kid := range p.Children() {
			if d := deepest(kid); d > deep {
				deep = d
			}
		}
	}
	return deep
}
