// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"bytes"
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// bgra makes a picture whose pixels are all one colour, with a stride wider
// than the picture: a capture buffer pads its rows, and a test that used
// W*4 would never see the bug that padding causes.
func bgra(w, h int, b, g, r, a byte) Picture {
	stride := w*4 + 32
	pix := make([]byte, stride*h)
	for y := range h {
		row := pix[y*stride:]
		for x := range w {
			row[x*4+0], row[x*4+1], row[x*4+2], row[x*4+3] = b, g, r, a
		}
		// The padding is deliberately NOT the colour: if it were read as
		// picture, the test could not tell.
		for i := w * 4; i < stride; i++ {
			row[i] = 0x7f
		}
	}
	return Picture{Pix: pix, W: w, H: h, Stride: stride}
}

// TestAPhotographKeepsItsColours.
//
// ⛔ BGRA READ AS RGBA GIVES A PERSON A BLUE FACE. It is unmistakable at a
// glance and completely silent to every check that only asks whether a file was
// written, so it is asserted on the actual pixels of the actual PNG.
func TestAPhotographKeepsItsColours(t *testing.T) {
	// A colour with all three channels different, so a swap of any two shows.
	const wantR, wantG, wantB = 0x10, 0x80, 0xf0
	data, err := encodePhoto(bgra(8, 4, wantB, wantG, wantR, 0xff))
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 8 || b.Dy() != 4 {
		t.Fatalf("the photograph is %v", b)
	}
	for _, at := range [][2]int{{0, 0}, {7, 3}, {4, 2}} {
		r, g, b, a := img.At(at[0], at[1]).RGBA()
		got := [4]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
		want := [4]uint8{wantR, wantG, wantB, 0xff}
		if got != want {
			t.Errorf("at %v the pixel is %v, want %v -- the channels are swapped",
				at, got, want)
		}
	}
}

// TestTheStrideIsHonoured.
//
// ⛔ A capture buffer pads its rows for alignment. Indexing by W*4 instead
// produces a picture that shears progressively down the frame -- it looks like
// a broken camera and is not one. Here the padding is a different colour from
// the picture, so a photograph that read it would be caught.
func TestTheStrideIsHonoured(t *testing.T) {
	p := bgra(6, 5, 0x11, 0x22, 0x33, 0xff)
	data, err := encodePhoto(p)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	for y := range p.H {
		for x := range p.W {
			r, g, b, _ := img.At(x, y).RGBA()
			if uint8(r>>8) != 0x33 || uint8(g>>8) != 0x22 || uint8(b>>8) != 0x11 {
				t.Fatalf("at %d,%d the pixel is %02x%02x%02x: the padding was read as picture",
					x, y, uint8(r>>8), uint8(g>>8), uint8(b>>8))
			}
		}
	}
}

// TestAPictureThatIsNotOneIsRefused, rather than written as a file nobody can
// open.
func TestAPictureThatIsNotOneIsRefused(t *testing.T) {
	for _, c := range []struct {
		name string
		p    Picture
	}{
		{"no width", Picture{Pix: make([]byte, 16), H: 2}},
		{"no height", Picture{Pix: make([]byte, 16), W: 2}},
		{"a stride narrower than the picture", Picture{Pix: make([]byte, 64), W: 4, H: 4, Stride: 8}},
		{"not enough bytes", Picture{Pix: make([]byte, 4), W: 4, H: 4}},
	} {
		if _, err := encodePhoto(c.p); err == nil {
			t.Errorf("%s was accepted", c.name)
		}
	}
	// A stride of zero means "no padding", which is what a caller with a plain
	// buffer has.
	if _, err := encodePhoto(Picture{Pix: make([]byte, 4*4*4), W: 4, H: 4}); err != nil {
		t.Errorf("a picture with no padding was refused: %v", err)
	}
}

// TestAPhotographIsNeverWrittenWhereItCouldBeCommitted.
//
// ⛔ THE MISTAKE HAS ALREADY BEEN MADE IN THIS FLEET: a live test wrote a
// capture of a whole desktop into a public repository's testdata/, untracked --
// one `git add -A` from publication. A photograph from a headset is a picture
// of whatever the person wearing it was looking at.
//
// And the person's OWN choice is checked, because that is exactly the mistake
// a person makes.
func TestAPhotographIsNeverWrittenWhereItCouldBeCommitted(t *testing.T) {
	tree := t.TempDir()
	if err := os.Mkdir(filepath.Join(tree, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(PhotoDirEnv, filepath.Join(tree, "photos"))

	_, err := PhotoPath(time.Now())
	if err == nil {
		t.Fatal("a photograph was allowed inside a git work tree")
	}
	if !strings.Contains(err.Error(), "work tree") {
		t.Errorf("the refusal does not say why: %v", err)
	}
	// And nothing was made on the way to refusing.
	if _, err := os.Stat(filepath.Join(tree, "photos")); !os.IsNotExist(err) {
		t.Error("the directory was created before being refused")
	}
}

// TestWhereAPhotographGoes, and that two in one session are two files.
func TestWhereAPhotographGoes(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(PhotoDirEnv, dir)

	at := time.Date(2026, 9, 4, 23, 30, 15, 0, time.UTC)
	path, err := WritePhoto(bgra(4, 3, 1, 2, 3, 255), at)
	if err != nil {
		t.Fatal(err)
	}
	if got := filepath.Base(path); got != "2026-09-04-233015.png" {
		t.Errorf("the photograph is called %q", got)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the photograph is not there: %v", err)
	}
	// A second, a second later, is a second file.
	other, err := WritePhoto(bgra(4, 3, 1, 2, 3, 255), at.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if other == path {
		t.Error("two photographs became one file")
	}

	// And what was written really is a picture, not an empty file.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := png.Decode(bytes.NewReader(data)); err != nil {
		t.Errorf("what was written is not a PNG: %v", err)
	}
}

// TestAPictureThatCannotBeEncodedIsNotWritten, so a refusal leaves no file
// behind for somebody to find and wonder about.
func TestAPictureThatCannotBeEncodedIsNotWritten(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(PhotoDirEnv, dir)
	if _, err := WritePhoto(Picture{}, time.Now()); err == nil {
		t.Fatal("an empty picture was written")
	}
	entries, err := os.ReadDir(filepath.Join(dir, "photos"))
	if err == nil && len(entries) > 0 {
		t.Errorf("%d files were left behind", len(entries))
	}
}

// TestADeskWithNoCameraSaysSo, rather than doing nothing.
//
// ⛔ A key that is granted and changes nothing is indistinguishable from a key
// that was never granted -- the thing "fit" was reported for. A desk with no
// OnPhoto is exactly that case, so it must say what happened.
func TestADeskWithNoCameraSaysSo(t *testing.T) {
	d := deskAt(t, MinDistance)
	d.Badge(1, nil)
	d.Do(ActionPhoto)
	text, up, _ := noticeSays(d)
	if !up {
		t.Fatal("asking for a photograph with no camera said nothing at all")
	}
	if !strings.Contains(text, "no photograph") && !strings.Contains(text, "camera") {
		t.Errorf("the notice says %q", text)
	}
}

// TestThePathIsSaidOutLoud.
//
// A photograph a program took and did not name is a photograph nobody can
// find, and the whole reason it goes somewhere durable rather than a temporary
// directory is that somebody comes back to it.
func TestThePathIsSaidOutLoud(t *testing.T) {
	d := deskAt(t, MinDistance)
	d.Badge(1, nil)
	d.OnPhoto = func() (string, error) {
		return "/somewhere/durable/2026-09-04-233015.png", nil
	}
	d.Do(ActionPhoto)
	text, up, _ := noticeSays(d)
	if !up || !strings.Contains(text, "2026-09-04-233015.png") {
		t.Errorf("the notice says %q; it should name the file", text)
	}
}

// TestAPhotographThatFailsSaysWhy, with the reason the camera gave rather than
// a sentence of our own that loses it.
func TestAPhotographThatFailsSaysWhy(t *testing.T) {
	d := deskAt(t, MinDistance)
	d.Badge(1, nil)
	d.OnPhoto = func() (string, error) {
		return "", errors.New("another program has the camera")
	}
	d.Do(ActionPhoto)
	text, _, _ := noticeSays(d)
	if !strings.Contains(text, "another program has the camera") {
		t.Errorf("the notice says %q; the camera's own reason was lost", text)
	}
}
