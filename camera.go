// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"fmt"
	"time"

	"github.com/go-macos/avfoundation"
)

// PhotoWait is how long to wait for the first frame after a camera is opened.
//
// A camera is not instant: the session starts, the sensor powers up, the light
// comes on, and the first frame arrives a moment later -- measured at a few
// hundred milliseconds on the machine this was written on, and a headset over
// USB is slower than a built-in one. Long enough not to give up on a slow
// camera, short enough that somebody who pressed the key knows it has failed.
const PhotoWait = 5 * time.Second

// PhotoWarmUp is how long the camera runs before the picture is taken.
//
// A sensor powers up with its exposure at nothing and ramps, so the first
// frames a camera delivers are darker than what it can see. Half a second is
// what a camera application usually discards, and it is what this waits.
//
// ⚠ NOT MEASURED HERE, and it is worth being exact about why. A photograph
// taken from the first frame did come back 1920x1080 with a mean luminance of
// ZERO -- and so did the raw capture probe run against the same camera a minute
// later, which is the CONTROL that says the room was dark and not that the
// first frame was. The effect this guards against is real in general and was
// not the cause of what was seen, so this is a precaution and not a fix.
//
// It is a WAIT rather than a test of the pixels, deliberately: a photograph of
// something genuinely dark is a photograph, and refusing it would be worse than
// taking it.
const PhotoWarmUp = 500 * time.Millisecond

// TakePhoto opens a camera, waits for a picture, writes it and reports where.
//
// ⛔ IT OPENS AND CLOSES AROUND ONE PHOTOGRAPH. A camera held open is a camera
// left ON -- the light with it -- and a desk that kept one for the length of a
// session would be a headset that watches the room all afternoon so that a key
// press can be quick. Opening costs a second; that is the right price.
//
// camera is a [avfoundation.Camera.ID], or empty for the first the machine
// lists. On a headset with several, which is the case this is written for, the
// caller chooses.
func TakePhoto(camera string, logf func(string, ...any)) (string, error) {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	c, err := openCamera(avfoundation.CaptureOptions{
		Camera: camera,
		Logf:   logf,
	})
	if err != nil {
		return "", photoErr(err)
	}
	defer c.Close()

	deadline := time.Now().Add(photoWait)
	var settled time.Time
	for {
		if f, ok := c.Latest(); ok {
			// The first frame STARTS a clock rather than ending the wait: see
			// PhotoWarmUp. Nothing about a frame can say whether the sensor
			// has settled, so only time can.
			if settled.IsZero() {
				settled = time.Now().Add(photoWarmUp)
				logf("the camera is awake; letting its exposure settle for %v", photoWarmUp)
			}
			if time.Now().After(settled) {
				logf("a %dx%d picture from %q", f.Width, f.Height, c.Camera().Name)
				return WritePhoto(Picture{
					Pix: f.Pix, W: f.Width, H: f.Height, Stride: f.Stride,
				}, time.Now())
			}
		}
		if time.Now().After(deadline) {
			// ⛔ SAY THE TWO THINGS THAT CAUSE THIS. A camera that opens and
			// delivers nothing is what a refusal at the system prompt looks
			// like from in here -- the session runs, and no frame ever comes --
			// and it is also what a camera in use by something else looks like.
			// Neither is visible from the error, so both are named.
			return "", fmt.Errorf("desk: %q gave no picture in %v; "+
				"either the camera prompt was refused, or another program has it",
				c.Camera().Name, photoWait)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// photoErr says what to do about the three ways a camera refuses.
//
// The build problem is the one worth translating: "no usage description" is a
// sentence about an Info.plist, and what the person in front of the machine
// needs to know is that this copy of the program cannot do it and which one can.
func photoErr(err error) error {
	switch {
	case errors.Is(err, avfoundation.ErrNoUsageDescription):
		return fmt.Errorf("desk: this copy of xrdesk cannot open a camera -- "+
			"macOS only allows it from an application bundle. Build one with "+
			"`go run ./cmd/macapp` and run that: %w", err)
	case errors.Is(err, avfoundation.ErrCameraDenied):
		return fmt.Errorf("desk: the camera was refused; it is turned on again in "+
			"System Settings > Privacy & Security > Camera: %w", err)
	default:
		return fmt.Errorf("desk: no camera: %w", err)
	}
}

// liveCamera is the part of a running camera this package uses.
//
// An interface and not the concrete type, so a test can hand over a camera that
// never delivers -- which is what a refusal at the system prompt looks like
// from in here, and the branch that says so. There is no other way to reach it:
// a real camera on a machine that has one always answers.
type liveCamera interface {
	Latest() (*avfoundation.Frame, bool)
	Camera() avfoundation.Camera
	Close() error
}

// The platform call and the clock, replaced in tests.
var (
	openCamera = func(o avfoundation.CaptureOptions) (liveCamera, error) {
		return avfoundation.OpenCamera(o)
	}
	photoWait   = PhotoWait
	photoWarmUp = PhotoWarmUp
)
