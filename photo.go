// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/go-fsctl/outdir"
)

// PhotoDirEnv moves where photographs are written. It is checked like any other
// choice: see [PhotoPath].
const PhotoDirEnv = "XRDESK_PHOTO_DIR"

// Picture is one frame to be written, in the shape both a camera and a canvas
// already have.
//
// BGRA, because that is what everything upstream of it carries: AVFoundation
// delivers BGRA and the desk's own canvas is painted through a BGRA painter.
// Converting on the way in would be a second copy of every frame to save one
// byte-swap on the few that are ever written.
type Picture struct {
	Pix    []byte
	W, H   int
	Stride int
}

// PhotoPath is where a photograph taken now may be written.
//
// ⛔ NEVER INSIDE A GIT WORK TREE, and that is not a convention -- it is the
// mistake this fleet has already made: a live test wrote a capture of a whole
// desktop into a public repository's testdata/, one `git add -A` from
// publication. A photograph from a headset is a picture of whatever the person
// wearing it was looking at, which is a stronger version of the same thing.
//
// A .gitignore entry would be the wrong fix: ignoring is a safety net, not a
// barrier -- `git add -f`, a fresh clone, or any tool that does not consult it
// publishes the file anyway. go-fsctl/outdir is the barrier, and it REFUSES
// rather than choosing somewhere else, including when a person points
// XRDESK_PHOTO_DIR at their own work tree.
//
// The name carries the moment, to the second, so two photographs taken in one
// session do not become one file.
func PhotoPath(at time.Time) (string, error) {
	dir, err := outdir.Ensure(outdir.Spec{
		App: "xrdesk",
		Env: PhotoDirEnv,
		Sub: "photos",
	})
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, at.Format("2006-01-02-150405")+".png"), nil
}

// WritePhoto writes one picture where [PhotoPath] says it may go, and reports
// the path so a person can be told where to look.
//
// ⛔ IT SAYS WHERE. A photograph a program took and did not name is a
// photograph nobody can find, and the whole point of writing it somewhere
// durable rather than a temporary directory is that somebody comes back to it
// later.
func WritePhoto(p Picture, at time.Time) (string, error) {
	path, err := PhotoPath(at)
	if err != nil {
		return "", err
	}
	data, err := encodePhoto(p)
	if err != nil {
		return "", err
	}
	if err := writeFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("desk: writing the photograph: %w", err)
	}
	return path, nil
}

// encodePhoto turns BGRA pixels into a PNG.
//
// ⛔ THE CHANNELS ARE SWAPPED HERE, once, on the way out. BGRA read as RGBA
// gives a picture whose reds and blues are exchanged -- a photograph of a
// person with a blue face, which is unmistakable at a glance and completely
// silent to every check that only asks whether a file was written.
//
// The stride is honoured rather than assumed. A capture buffer pads its rows
// for alignment, and indexing by W*4 instead produces a picture that shears
// progressively down the frame: it looks like a broken camera and is not one.
func encodePhoto(p Picture) ([]byte, error) {
	if p.W <= 0 || p.H <= 0 {
		return nil, fmt.Errorf("desk: a photograph %dx%d", p.W, p.H)
	}
	stride := p.Stride
	if stride == 0 {
		stride = p.W * 4
	}
	if stride < p.W*4 || len(p.Pix) < stride*p.H {
		return nil, fmt.Errorf("desk: %d bytes for a %dx%d picture with a stride of %d",
			len(p.Pix), p.W, p.H, stride)
	}
	img := image.NewNRGBA(image.Rect(0, 0, p.W, p.H))
	for y := range p.H {
		src := p.Pix[y*stride:]
		dst := img.Pix[y*img.Stride:]
		for x := range p.W {
			s, d := x*4, x*4
			dst[d+0] = src[s+2] // R <- the third byte of a BGRA pixel
			dst[d+1] = src[s+1]
			dst[d+2] = src[s+0]
			dst[d+3] = src[s+3]
		}
	}
	var out bytes.Buffer
	if err := encodePNG(&out, img); err != nil {
		return nil, fmt.Errorf("desk: encoding the photograph: %w", err)
	}
	return out.Bytes(), nil
}

// The platform calls, replaced in tests.
//
// A disk that is full and an encoder that refuses are both places this could
// report a photograph it did not write, and neither can be reached by asking
// the real filesystem for a picture four pixels wide.
var (
	writeFile = os.WriteFile
	encodePNG = png.Encode
)
