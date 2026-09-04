// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"image"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-macos/avfoundation"
)

// fakeCamera is a camera a test can decide the behaviour of.
type fakeCamera struct {
	frame  *avfoundation.Frame
	closes int
}

func (f *fakeCamera) Latest() (*avfoundation.Frame, bool) { return f.frame, f.frame != nil }
func (f *fakeCamera) Camera() avfoundation.Camera {
	return avfoundation.Camera{ID: "fake", Name: "a camera that does what this test says"}
}
func (f *fakeCamera) Close() error { f.closes++; return nil }

// TestAPhotographIsTakenAndTheCameraIsCLOSED.
//
// ⛔ The close is the assertion that matters. A camera held open is a camera
// left ON, light and all -- a headset that watches the room all afternoon so
// that one key press can be quick. It must be shut whatever happens, which is
// why it is checked on the way out of success AND of failure.
func TestAPhotographIsTakenAndTheCameraIsClosed(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(PhotoDirEnv, dir)

	p := bgra(4, 3, 0x11, 0x22, 0x33, 0xff)
	cam := &fakeCamera{frame: &avfoundation.Frame{
		Width: p.W, Height: p.H, Stride: p.Stride, Format: avfoundation.BGRA, Pix: p.Pix,
	}}
	swapCamera(t, cam, nil)
	swap(t, &photoWarmUp, time.Millisecond)

	said := []string{}
	path, err := TakePhoto("", func(f string, a ...any) { said = append(said, f) })
	if err != nil {
		t.Fatalf("TakePhoto: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("the photograph is not there: %v", err)
	}
	if cam.closes == 0 {
		t.Error("the camera was left open, and its light with it")
	}
	if len(said) == 0 {
		t.Error("nothing was logged about the picture that was taken")
	}
}

// TestACameraThatDeliversNothingIsGivenUpOn, and says both reasons.
//
// ⛔ A session that runs and delivers nothing is EXACTLY what a refusal at the
// system prompt looks like from in here, and also what a camera another program
// holds looks like. Neither is visible in any error, so both are named -- and
// the camera is still closed.
func TestACameraThatDeliversNothingIsGivenUpOn(t *testing.T) {
	cam := &fakeCamera{}
	swapCamera(t, cam, nil)
	swap(t, &photoWait, 30*time.Millisecond)

	start := time.Now()
	_, err := TakePhoto("", nil)
	if err == nil {
		t.Fatal("a camera that delivered nothing was treated as a success")
	}
	if took := time.Since(start); took > time.Second {
		t.Errorf("it waited %v; the deadline was %v", took, photoWait)
	}
	for _, want := range []string{"refused", "another program"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not mention %q: %v", want, err)
		}
	}
	if cam.closes == 0 {
		t.Error("a camera that gave nothing was left open")
	}
}

// TestWhatEachRefusalTellsAPersonToDo.
//
// ⛔ "No usage description" is a sentence about an Info.plist. What the person
// in front of the machine needs to know is that THIS copy of the program cannot
// do it and which one can -- and for a denial, where the switch is.
func TestWhatEachRefusalTellsAPersonToDo(t *testing.T) {
	for _, c := range []struct {
		from error
		want string
	}{
		{avfoundation.ErrNoUsageDescription, "internal/macapp"},
		{avfoundation.ErrCameraDenied, "Privacy & Security"},
		{avfoundation.ErrNoCamera, "no camera"},
		{errors.New("something else entirely"), "something else entirely"},
	} {
		swapCamera(t, nil, c.from)
		_, err := TakePhoto("", nil)
		if err == nil {
			t.Fatalf("%v was treated as a success", c.from)
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%v reads as %q; it should say %q", c.from, err, c.want)
		}
		// And the original is still there for a caller testing with errors.Is.
		if !errors.Is(err, c.from) {
			t.Errorf("%v was wrapped away: %v", c.from, err)
		}
	}
}

// TestAPhotographThatCannotBeWrittenIsReported, rather than a path to a file
// that is not there.
func TestAPhotographThatCannotBeWrittenIsReported(t *testing.T) {
	t.Setenv(PhotoDirEnv, t.TempDir())
	boom := errors.New("the disk is full")
	swap(t, &writeFile, func(string, []byte, os.FileMode) error { return boom })

	if _, err := WritePhoto(bgra(2, 2, 1, 2, 3, 255), time.Now()); !errors.Is(err, boom) {
		t.Errorf("WritePhoto = %v, want the disk's own error", err)
	}
}

// TestAPictureThatWillNotEncodeIsReported.
//
// It cannot happen with an NRGBA image and a bytes.Buffer, which is exactly why
// it needs a seam: a branch nothing can take is a branch nothing has taken.
func TestAPictureThatWillNotEncodeIsReported(t *testing.T) {
	boom := errors.New("the encoder gave up")
	swap(t, &encodePNG, func(io.Writer, image.Image) error { return boom })

	if _, err := encodePhoto(bgra(2, 2, 1, 2, 3, 255)); !errors.Is(err, boom) {
		t.Errorf("encodePhoto = %v, want the encoder's own error", err)
	}
}

// swapCamera installs a camera, or a refusal, for one test.
func swapCamera(t *testing.T, cam liveCamera, err error) {
	t.Helper()
	swap(t, &openCamera, func(avfoundation.CaptureOptions) (liveCamera, error) {
		if err != nil {
			return nil, err
		}
		return cam, nil
	})
}

// swap installs a replacement for the length of one test.
func swap[T any](t *testing.T, p *T, v T) {
	t.Helper()
	was := *p
	t.Cleanup(func() { *p = was })
	*p = v
}

// TestTheExposureIsLetToSettle.
//
// A sensor powers up with its exposure at nothing and ramps, so the first
// frames are darker than what the camera can see -- and nothing about a frame
// can say whether it has settled, so only time can. The first frame must
// therefore START a wait rather than end it, which is what this says: a camera
// delivering from the first instant is still not photographed until the warm-up
// has passed.
//
// ⚠ The warm-up is a precaution and not a measured fix; see PhotoWarmUp for
// what was actually seen and what the control said about it.
func TestTheExposureIsLetToSettle(t *testing.T) {
	t.Setenv(PhotoDirEnv, t.TempDir())
	p := bgra(2, 2, 1, 2, 3, 255)
	cam := &fakeCamera{frame: &avfoundation.Frame{
		Width: p.W, Height: p.H, Stride: p.Stride, Pix: p.Pix,
	}}
	swapCamera(t, cam, nil)
	const warm = 120 * time.Millisecond
	swap(t, &photoWarmUp, warm)

	start := time.Now()
	if _, err := TakePhoto("", nil); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took < warm {
		t.Errorf("the photograph was taken after %v, before the %v warm-up: "+
			"the first frame ended the wait instead of starting it", took, warm)
	}
}

// TestAWarmUpLongerThanTheWaitStillGivesUp, rather than sitting with the light
// on for ever: the deadline governs both.
func TestAWarmUpLongerThanTheWaitStillGivesUp(t *testing.T) {
	p := bgra(2, 2, 1, 2, 3, 255)
	cam := &fakeCamera{frame: &avfoundation.Frame{
		Width: p.W, Height: p.H, Stride: p.Stride, Pix: p.Pix,
	}}
	swapCamera(t, cam, nil)
	swap(t, &photoWarmUp, time.Hour)
	swap(t, &photoWait, 50*time.Millisecond)

	start := time.Now()
	if _, err := TakePhoto("", nil); err == nil {
		t.Fatal("a warm-up longer than the wait produced a photograph")
	}
	if took := time.Since(start); took > time.Second {
		t.Errorf("it waited %v with the light on", took)
	}
	if cam.closes == 0 {
		t.Error("the camera was left open")
	}
}
