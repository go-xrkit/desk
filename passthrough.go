// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"fmt"

	"github.com/go-macos/avfoundation"
)

// cameraFeed shows a camera on a ribbon screen.
//
// ⭐ NOTHING IS CONVERTED. AVFoundation delivers BGRA and this desk's canvas
// holds BGRA -- so a camera frame becomes a [Source] by naming its fields. The
// photograph path swaps to RGBA only because PNG wants it that way; the ribbon
// does not.
//
// ⚠ AND THE STRIDE IS NOT THE WIDTH. The decoder pads rows, so a frame indexed
// by W*4 shears progressively down the picture -- which looks like a decode bug
// and is not one. Source carries Stride for exactly this reason.
type cameraFeed struct {
	c    liveCamera
	last *avfoundation.Frame
}

// Frame hands over the newest picture, and whether it is new.
//
// ⛔ NEWNESS BY POINTER IDENTITY, not by comparing pixels or timestamps. The
// capture hands back the same *Frame until it replaces it, so a changed
// pointer IS a new frame -- while a timestamp can repeat on a stalled camera
// and comparing megabytes of pixels every draw would cost more than the draw.
func (f *cameraFeed) Frame() (Source, bool) {
	fr, ok := f.c.Latest()
	if !ok || fr == nil {
		return Source{}, false
	}
	s := Source{Pix: fr.Pix, W: fr.Width, H: fr.Height, Stride: fr.Stride}
	if fr == f.last {
		// The same picture as last time: the caller may still want it, so it
		// is handed over, but nothing has changed.
		return s, false
	}
	f.last = fr
	return s, true
}

// Close stops the camera.
//
// ⚠ AND IT MUST, because the light is on while it lives. On every Mac with a
// camera indicator the hardware wires the light to the sensor's power, so a
// capture left open is a headset watching the room.
func (f *cameraFeed) Close() error {
	if f.c == nil {
		return nil
	}
	c := f.c
	f.c = nil
	return c.Close()
}

// OpenPassthrough starts a camera as a ribbon feed.
//
// ⭐ THERE IS NOTHING TO DECODE. Both VITURE headsets carry an ordinary USB
// video camera -- measured 2026-09-06, a Sonix part that macOS lists with no
// driver and no protocol at all. So passthrough is not a protocol problem; it
// is a camera the desk already knows how to open, pointed the right way.
//
// camera is a [avfoundation.Camera.ID]; empty takes the first the machine
// lists, which on a laptop is the one facing the person. See [HeadsetCamera].
func OpenPassthrough(camera string, logf func(string, ...any)) (Feed, error) {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	c, err := openCamera(avfoundation.CaptureOptions{Camera: camera, Logf: logf})
	if err != nil {
		return nil, fmt.Errorf("desk: passthrough: %w", photoErr(err))
	}
	return &cameraFeed{c: c}, nil
}
