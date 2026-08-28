// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build darwin

package desk

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-macos/appicon"
)

// TestLiveTheApplicationGalleryOfThisMachine renders the gallery from what is
// REALLY running, with the applications' own icons, and writes it somewhere a
// person can open.
//
// The other render test draws synthetic tiles: it proves the layout and can
// prove nothing about the icons, because a made-up App has none. This is the
// one that would have caught a gallery of grey squares.
func TestLiveTheApplicationGalleryOfThisMachine(t *testing.T) {
	if os.Getenv("DESK_LIVE") == "" {
		t.Skip("set DESK_LIVE=1 to render the gallery of this machine")
	}
	b := TheBench()
	if !b.Trusted() {
		t.Skip("no Accessibility grant: this machine cannot list another application's windows")
	}
	list, err := b.Listing()
	if err != nil {
		t.Fatalf("Listing: %v", err)
	}
	apps := AppsFrom(list, nil)
	if len(apps) == 0 {
		t.Skip("nothing with a window is running")
	}

	var withIcon int
	for i := range apps {
		px, err := appicon.ForPID(apps[i].PID, 256)
		if err != nil {
			continue
		}
		apps[i].Icon = &Icon{Pix: px.Pix, W: px.W, H: px.H}
		withIcon++
	}
	t.Logf("%d applications, %d with their own icon", len(apps), withIcon)
	if withIcon == 0 {
		t.Error("not one application gave up its icon; the gallery is grey squares")
	}

	const w, h = 1920, 1200
	c := NewCanvas(w, h)
	c.Fill([4]byte{0, 0, 0, 255})
	v := newAppsView(nil)
	v.set(apps)
	v.draw(c)

	path := filepath.Join(renderDir(t), "apps-gallery-live.png")
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			i := (y*w + x) * 4
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
	t.Logf("the gallery of this machine is at %s — open it and look", path)
}
