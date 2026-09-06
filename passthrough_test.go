// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-macos/avfoundation"
)

// stubCamera hands out frames a test chooses.
type stubCamera struct {
	frames []*avfoundation.Frame
	closes int
}

func (s *stubCamera) Latest() (*avfoundation.Frame, bool) {
	if len(s.frames) == 0 {
		return nil, false
	}
	f := s.frames[0]
	if len(s.frames) > 1 {
		s.frames = s.frames[1:]
	}
	return f, true
}
func (s *stubCamera) Close() error                { s.closes++; return nil }
func (s *stubCamera) Camera() avfoundation.Camera { return avfoundation.Camera{} }

// TestACameraFrameNeedsNoConversion.
//
// ⭐ AVFoundation DELIVERS BGRA AND THIS DESK'S CANVAS HOLDS BGRA, so a frame
// becomes a Source by naming its fields. The photograph path swaps to RGBA
// only because PNG wants it that way; the ribbon does not.
//
// ⚠ AND THE STRIDE IS NOT THE WIDTH. A frame indexed by W*4 shears
// progressively down the picture, which looks like a decode bug and is not
// one -- so Source must carry the frame's own stride, not a computed one.
func TestACameraFrameNeedsNoConversion(t *testing.T) {
	fr := &avfoundation.Frame{Width: 3, Height: 2, Stride: 16, Pix: make([]byte, 32)}
	f := &cameraFeed{c: &stubCamera{frames: []*avfoundation.Frame{fr}}}

	s, fresh := f.Frame()
	if !fresh {
		t.Error("the first frame is not reported as new")
	}
	if s.W != 3 || s.H != 2 {
		t.Errorf("the picture is %dx%d", s.W, s.H)
	}
	if s.Stride != 16 {
		t.Errorf("stride %d, want the frame's own 16 -- a computed 12 would shear it", s.Stride)
	}
	if &s.Pix[0] != &fr.Pix[0] {
		t.Error("the pixels were copied; they should be handed over")
	}
}

// TestTheSameFrameIsNotNewsTwice.
//
// ⛔ NEWNESS BY POINTER IDENTITY. The capture hands back the same *Frame until
// it replaces one, so a changed pointer IS a new frame -- while a timestamp can
// repeat on a stalled camera, and comparing megabytes of pixels every draw
// would cost more than the draw.
func TestTheSameFrameIsNotNewsTwice(t *testing.T) {
	fr := &avfoundation.Frame{Width: 1, Height: 1, Stride: 4, Pix: make([]byte, 4)}
	next := &avfoundation.Frame{Width: 1, Height: 1, Stride: 4, Pix: make([]byte, 4)}
	cam := &stubCamera{frames: []*avfoundation.Frame{fr, next}}
	f := &cameraFeed{c: cam}

	if _, fresh := f.Frame(); !fresh {
		t.Error("the first frame is not new")
	}
	// The stub keeps handing out the last one once it runs down.
	if _, fresh := f.Frame(); !fresh {
		t.Error("the second, different frame is not new")
	}
	if _, fresh := f.Frame(); fresh {
		t.Error("the same frame was reported as new a second time")
	}

	// A camera with nothing yet.
	empty := &cameraFeed{c: &stubCamera{}}
	if s, fresh := empty.Frame(); fresh || s.Pix != nil {
		t.Errorf("a camera with no frame gave %v, %v", s, fresh)
	}
}

// TestClosingIsSafeTwice, because the light is on while it lives.
func TestClosingIsSafeTwice(t *testing.T) {
	cam := &stubCamera{}
	f := &cameraFeed{c: cam}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if cam.closes != 1 {
		t.Errorf("the camera was closed %d times, want 1", cam.closes)
	}
}

// TestTheDisplacedFeedIsKeptAndGivenBack.
//
// ⛔⛔ THIS IS THE ONE THAT MATTERS. A screen showing an application still has
// that application behind it. A desk that closed its capture to show the room
// would hand back a BLACK RECTANGLE when the person pressed again -- and it
// would have thrown away something it cannot recreate.
func TestTheDisplacedFeedIsKeptAndGivenBack(t *testing.T) {
	p := testPlan(t)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	d.Badge(1, nil)

	was, _ := d.SetFeed(0, nil)
	originalCam := &stubCamera{}
	original := &cameraFeed{c: originalCam}
	if _, err := d.SetFeed(0, original); err != nil {
		t.Fatal(err)
	}
	_ = was

	roomCam := &stubCamera{}
	room := &cameraFeed{c: roomCam}
	d.OnPassthrough = func() (Feed, error) { return room, nil }

	d.togglePassthrough(0)
	if got, _, _ := noticeSays(d); !strings.Contains(got, "shows the room") {
		t.Errorf("turning it on says %q", got)
	}
	// The application's feed must NOT have been closed.
	if c := originalCam.closes; c != 0 {
		t.Errorf("the displaced feed was closed %d times", c)
	}

	d.togglePassthrough(0)
	if got, _, _ := noticeSays(d); !strings.Contains(got, "is back") {
		t.Errorf("turning it off says %q", got)
	}
	// ⭐ AND THE CAMERA IS CLOSED, because the light is on while it runs.
	if c := roomCam.closes; c != 1 {
		t.Errorf("the camera was closed %d times, want 1", c)
	}
	if c := originalCam.closes; c != 0 {
		t.Errorf("the restored feed was closed %d times", c)
	}
}

// TestADeskWithNoCameraSaysSoAboutTheRoom rather than doing nothing.
func TestADeskWithNoCameraSaysSoAboutTheRoom(t *testing.T) {
	p := testPlan(t)
	d, _ := New(p, feedsFor(p))
	defer d.Close()
	d.Badge(1, nil)

	d.togglePassthrough(0)
	if got, _, _ := noticeSays(d); !strings.Contains(got, "no camera") {
		t.Errorf("a desk with no camera says %q", got)
	}

	// And a camera that will not open is reported in its own words.
	boom := errors.New("the light is busy")
	d.OnPassthrough = func() (Feed, error) { return nil, boom }
	d.togglePassthrough(0)
	if got, _, _ := noticeSays(d); !strings.Contains(got, "busy") {
		t.Errorf("a refused camera says %q", got)
	}
}

// TestOpenPassthroughReportsWhatTheCameraSaid.
//
// ⭐ THE SAME SEAM THE PHOTOGRAPH USES, so a machine with no camera -- or one
// where the person said no -- gives the same words in both places.
func TestOpenPassthroughReportsWhatTheCameraSaid(t *testing.T) {
	was := openCamera
	t.Cleanup(func() { openCamera = was })

	cam := &stubCamera{}
	openCamera = func(o avfoundation.CaptureOptions) (liveCamera, error) {
		if o.Camera != "0x1200000c45636b" {
			t.Errorf("it opened %q rather than the camera it was given", o.Camera)
		}
		return cam, nil
	}
	f, err := OpenPassthrough("0x1200000c45636b", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if cam.closes != 1 {
		t.Errorf("the camera was closed %d times", cam.closes)
	}

	boom := errors.New("no camera here")
	openCamera = func(avfoundation.CaptureOptions) (liveCamera, error) { return nil, boom }
	if _, err := OpenPassthrough("", nil); err == nil || !strings.Contains(err.Error(), "passthrough") {
		t.Errorf("a refusal reads as %v", err)
	}
}

// TestPassthroughSurvivesAScreenThatWillNotTakeIt.
//
// ⛔ AND IT CLOSES THE CAMERA WHEN IT FAILS. A camera opened for a screen that
// refused it is a light left on for nothing -- and on this hardware that is a
// headset watching the room.
func TestPassthroughSurvivesAScreenThatWillNotTakeIt(t *testing.T) {
	p := testPlan(t)
	d, _ := New(p, feedsFor(p))
	defer d.Close()
	d.Badge(1, nil)

	cam := &stubCamera{}
	d.OnPassthrough = func() (Feed, error) { return &cameraFeed{c: cam}, nil }
	// A position the ribbon does not have.
	d.togglePassthrough(999)
	if got, _, _ := noticeSays(d); got == "" {
		t.Error("a refused screen said nothing")
	}
	if cam.closes != 1 {
		t.Errorf("the camera was left open %d closes after a refusal", cam.closes)
	}
}

// TestTurningTheRoomOffOnAScreenThatWentAway is reported, not silent.
func TestTurningTheRoomOffOnAScreenThatWentAway(t *testing.T) {
	p := testPlan(t)
	d, _ := New(p, feedsFor(p))
	defer d.Close()
	d.Badge(1, nil)

	cam := &stubCamera{}
	d.roomAt = map[int]Feed{999: &cameraFeed{c: cam}}
	d.togglePassthrough(999)
	if got, _, _ := noticeSays(d); got == "" {
		t.Error("it said nothing about a screen that is gone")
	}
}

// TestTheRoomKeyWorksFromTheRibbon, which is the case Do routes.
//
// ⛔ EVERY VIEW HAS ITS OWN SWITCH in this desk -- the ribbon, the screen
// gallery, the applications -- and a key that works from one and silently does
// nothing from the others is worse than no key: it works when you first try it
// and fails when you need it.
func TestTheRoomKeyWorksFromTheRibbon(t *testing.T) {
	p := testPlan(t)
	d, _ := New(p, feedsFor(p))
	defer d.Close()
	d.Badge(1, nil)

	cam := &stubCamera{}
	d.OnPassthrough = func() (Feed, error) { return &cameraFeed{c: cam}, nil }

	d.Do(ActionPassthrough)
	if got, _, _ := noticeSays(d); !strings.Contains(got, "shows the room") {
		t.Errorf("the key from the ribbon says %q", got)
	}
	d.Do(ActionPassthrough)
	if got, _, _ := noticeSays(d); !strings.Contains(got, "is back") {
		t.Errorf("pressing again says %q", got)
	}
	if cam.closes != 1 {
		t.Errorf("the camera was closed %d times", cam.closes)
	}
	// And the action names itself, for the menu row and the report.
	if got := ActionPassthrough.String(); got != "show the room" {
		t.Errorf("the action is called %q", got)
	}
}
