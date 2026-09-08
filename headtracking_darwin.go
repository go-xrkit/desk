// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build darwin

package desk

import (
	"context"
	"fmt"
	"image"
	"sync"
	"time"

	"github.com/go-macos/avfoundation"
	"github.com/go-xrkit/xrkit/headflow"
)

// CameraHead reads a head's yaw from the headset's own camera.
//
// ⭐ IT EXISTS BECAUSE THE HEADSET WILL NOT SAY. A VITURE Beast tracks its own
// 3DOF and anchors the picture it is given with it, but publishes no orientation
// at all -- measured three ways and none of them produced a number. The camera
// is an ordinary UVC device, and a turn of the head is plainly visible in it.
//
// ⚠ THE LIGHT IS ON WHILE THIS RUNS. On every Mac with a camera indicator the
// hardware ties it to the sensor's power, so following somebody's head means a
// lit camera light for as long as they want it followed. That is a thing to tell
// them, not to work around.
type CameraHead struct {
	cap    *avfoundation.Capture
	cancel context.CancelFunc
	done   chan struct{}

	mu   sync.Mutex
	tr   headflow.Tracker
	yaw  float64
	ok   bool
	seen int
}

// OpenCameraHead starts reading the camera that belongs to display's headset.
//
// ⛔ IT NAMES WHAT IS MISSING RATHER THAN RETURNING A DEAD TRACKER. A headset
// attached for its picture only -- over a display cable -- presents no camera at
// all, and a head tracker that silently never moves is indistinguishable from
// one whose owner is sitting still.
func OpenCameraHead(display string) (*CameraHead, error) {
	id := HeadsetCamera(display)
	if id == "" {
		which := display
		if which == "" {
			which = "the headset"
		}
		return nil, fmt.Errorf("%w: %s has none, so its head cannot be followed",
			ErrNoRoomCamera, which)
	}
	cap, err := avfoundation.OpenCamera(avfoundation.CaptureOptions{Camera: id})
	if err != nil {
		return nil, fmt.Errorf("desk: the headset camera would not open: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	h := &CameraHead{cap: cap, cancel: cancel, done: make(chan struct{})}
	go h.run(ctx)
	return h, nil
}

// run reads frames until the context is cancelled.
//
// ⛔⛔ IT READS ON ITS OWN GOROUTINE, NOT IN Advance. A frame costs under a
// millisecond, but the camera delivers on its own clock -- 46 to 107ms from
// capture to availability, measured -- and a render loop that waited on it would
// take the camera's jitter as its own.
func (h *CameraHead) run(ctx context.Context) {
	defer close(h.done)
	var last *avfoundation.Frame
	var rgba *image.RGBA
	for ctx.Err() == nil {
		f, ok := h.cap.Latest()
		if !ok || f == last {
			// ⛔ NEWEST, NOT NEXT: the same frame comes back until the camera
			// delivers another, and feeding it twice scores a shift of zero for
			// a head that was moving.
			select {
			case <-ctx.Done():
			case <-time.After(2 * time.Millisecond):
			}
			continue
		}
		last = f
		// ⛔ AND NOTHING IS RELEASED. The frame owns its memory, copied out of
		// the capture buffer before the callback returned; releasing it kills
		// the capture's current frame and the next read comes back nil.
		im := f.ToRGBA(rgba)
		if im == nil {
			continue
		}
		rgba = im
		yaw, used := h.tr.Feed(im)
		h.mu.Lock()
		h.yaw, h.ok, h.seen = yaw, used, h.tr.Unusable()
		h.mu.Unlock()
	}
}

// Yaw is radians since the last Recenter, and whether the latest frame could be
// used at all.
func (h *CameraHead) Yaw() (float64, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.yaw, h.ok
}

// Blind is how many frames in a row could not be used.
func (h *CameraHead) Blind() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.seen
}

// Recenter makes the current view the origin.
func (h *CameraHead) Recenter() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.tr.Recenter()
	h.yaw = 0
}

// Close stops the camera and turns its light off. It is safe to call twice.
func (h *CameraHead) Close() error {
	h.cancel()
	<-h.done
	return h.cap.Close()
}
