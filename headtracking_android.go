// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-xrkit/android"
	"github.com/go-xrkit/xrkit/headflow"
)

// CameraHead reads a head's yaw from the headset's own camera, through usbfs.
//
// ⭐⭐ ON THIS PLATFORM IT IS THE WHOLE INTERFACE, not a convenience. Android's
// own desktop mode already puts one screen on the glasses; the ribbon is about
// 309° wide against roughly 50° of view, and every other way of turning it —
// a global shortcut, a menu bar, three fingers on a trackpad — is macOS. So
// without this the band exists and five sixths of it is unreachable.
//
// ⭐ IT WORKS AT ALL BECAUSE usbfs DOES. Neither Android camera API offers the
// headset's camera: there is no external camera2 device on this phone, and the
// Java USB API cannot submit isochronous transfers. go-xrkit/android reaches it
// through usbfs on an UNROOTED phone, which is what makes this file possible
// rather than a wish.
//
// ⛔ AND IT COSTS A REPLUG, ONCE PER APPLICATION. Claiming the streaming
// interface takes it from uvcvideo and the reconnect does not restore the
// binding here; and the USB permission is granted per UID, so a newly installed
// APK has none until the headset is unplugged and plugged back with its own
// offer accepted. Both are said out loud rather than discovered.
type CameraHead struct {
	cam    *android.UVCCamera
	cancel context.CancelFunc
	done   chan struct{}

	mu    sync.Mutex
	tr    headflow.Tracker
	yaw   float64
	sight Sight
}

// ErrNoCameraHead is returned where no camera can be opened at all.
//
// ⛔ IT REFUSES RATHER THAN RETURNING SOMETHING THAT NEVER MOVES. A head tracker
// that silently reports stillness is indistinguishable from one whose owner is
// sitting still, and the difference is the whole of what a caller needs.
var ErrNoCameraHead = errors.New("desk: following the head needs the headset's camera, and it could not be opened")

// OpenCameraHead starts reading the headset's camera.
//
// ⚠ display is accepted and not used. On macOS it picks the camera that belongs
// to that headset off the USB bus; here there is one UVC camera on the phone at
// a time — the headset is the only thing plugged into the single USB-C port, and
// the port is how the glasses are a display in the first place.
func OpenCameraHead(display string) (*CameraHead, error) {
	ctx, cancel := context.WithCancel(context.Background())
	// ⛔ NO WAIT. The desk is already running and the glasses are already
	// attached — they are the display it is drawing on. A camera that is not
	// there now is a refusal to report, not something to block the ribbon for.
	cam, err := android.OpenUVCCamera(ctx, android.UVCCameraOptions{}, nil)
	if err != nil {
		cancel()
		which := display
		if which == "" {
			which = "the headset"
		}
		return nil, fmt.Errorf("%w: %s: %v", ErrNoCameraHead, which, err)
	}
	h := &CameraHead{cam: cam, cancel: cancel, done: make(chan struct{})}
	go h.run(ctx)
	return h, nil
}

// run reads frames until the context is cancelled.
//
// ⛔⛔ IT READS ON ITS OWN GOROUTINE, NOT IN Advance. This camera delivers about
// ten frames a second — measured, and a ceiling of its own across a fourfold
// change in pixel count — while the ribbon redraws every 16 ms. A render loop
// that waited on the camera would take the camera's rate as its own and the
// picture would stutter at ten frames a second to track a head.
func (h *CameraHead) run(ctx context.Context) {
	defer close(h.done)
	for ctx.Err() == nil {
		img, err := h.cam.Frame(ctx)
		if err != nil {
			// ⛔ A STOPPED STREAM IS NOT STILLNESS. The sight goes LOST so the
			// caller stops turning the band on a yaw that is no longer being
			// measured, which is exactly what a frozen picture would look like.
			h.mu.Lock()
			h.sight = SightLost
			h.mu.Unlock()
			if ctx.Err() != nil {
				return
			}
			// The stream can refuse a frame and carry on — a lost interval, a
			// short read — so this is a pause and not an exit.
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Millisecond):
			}
			continue
		}
		yaw, used := h.tr.Feed(img)
		h.mu.Lock()
		h.yaw, h.sight = yaw, sightOf(used, h.tr.Unusable())
		h.mu.Unlock()
	}
}

// Yaw is radians since the last Recenter, and what the camera could see of the
// frame it read.
func (h *CameraHead) Yaw() (float64, Sight) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.yaw, h.sight
}

// Recenter makes the current view the origin.
func (h *CameraHead) Recenter() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.tr.Recenter()
	h.yaw = 0
}

// Close stops the camera. It is safe to call twice.
func (h *CameraHead) Close() error {
	h.cancel()
	<-h.done
	return h.cam.Close()
}

// WhyTheHeadCannotBeFollowed says why this process could not follow a head at
// all, or nil when nothing is known to stand in the way.
//
// ⚠ NOTHING IS KNOWN IN ADVANCE HERE, and that is an honest answer rather than
// a missing one. On macOS a bare binary cannot open a camera whatever the grant
// says, so the refusal is certain before anything is touched. On Android the
// two things that can stand in the way — a USB permission granted per UID, and
// a streaming interface still held by uvcvideo — are only answered by asking,
// and asking is opening. Reporting a guess would make the key look broken on a
// phone where it works.
func WhyTheHeadCannotBeFollowed() error { return nil }
