// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-xrkit/android"
)

// Screens is what the ribbon will show, and how it was obtained.
//
// On Android it is the phone's own screen, and that is not a shortcoming of this
// code — it is settled. An ordinary application may create a virtual display,
// but the display comes back WITHOUT the trusted flag and launching anything
// onto it, even the application's own activity, is refused. Asking for the
// trusted flag wants a permission whose protection level is `signature`, and the
// flag is not in the public SDK at all. So the ribbon carries a mirror of the
// phone plus whatever this application draws itself, which is also what the
// manufacturer's own software does here.
type Screens struct {
	// IDs identify the displays, in ribbon order.
	IDs []uint64
	// Virtual reports whether these were created by us. Never true here.
	Virtual bool
	// Why explains what happened, and is worth showing to a person.
	Why string
}

// Close releases anything this created. There is nothing to release.
func (s *Screens) Close() error { return nil }

// Provide gets the ribbon its screens.
//
// Consent is asked for HERE rather than at the first capture. MediaProjection
// puts a system dialog in front of the person, once per session and never
// remembered, and a dialog that appears in the middle of setting up a desk reads
// as a fault. Asking while nothing is on screen yet reads as a question.
func Provide(ctx context.Context, plan Plan, logf func(string, ...any)) (*Screens, error) {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	if !android.Available() {
		return nil, errors.New("desk: this process was not started by the Android host")
	}
	if !android.Authorized() {
		logf("asking for permission to capture the screen")
		ok, err := android.RequestAuthorization(ctx)
		if err != nil {
			return nil, fmt.Errorf("desk: asking to capture the screen: %w", err)
		}
		if !ok {
			return nil, errors.New("desk: permission to capture the screen was declined")
		}
	}

	d, err := android.DefaultDisplay(ctx)
	if err != nil {
		return nil, fmt.Errorf("desk: cannot find the screen to capture: %w", err)
	}
	s := &Screens{IDs: []uint64{uint64(d.ID)}}
	s.Why = fmt.Sprintf("Android does not let an ordinary application put other apps on a "+
		"display it creates, so the ribbon shows this phone's own screen: %q %dx%d",
		d.Name, d.Width, d.Height)
	logf("%s", s.Why)
	return s, nil
}

// captureFeed adapts one capture stream to the portable Feed the desk consumes.
type captureFeed struct{ s *android.Stream }

func (f *captureFeed) Frame() (Source, bool) {
	fr, fresh := f.s.Frame()
	if fr.Width <= 0 || fr.Height <= 0 || len(fr.Pix) == 0 {
		return Source{}, false
	}
	return Source{Pix: fr.Pix, W: fr.Width, H: fr.Height, Stride: fr.Stride}, fresh
}

func (f *captureFeed) Close() error { return f.s.Close() }

// Capture opens a stream on the screen.
//
// There is exactly one to open. Every ribbon position beyond the first gets a
// NIL feed and shows background — the arrangement is still a ribbon, it simply
// has one screen on it, and pretending otherwise by repeating the same picture
// would tell the viewer something untrue about their phone.
func Capture(ctx context.Context, plan Plan, s *Screens, logf func(string, ...any)) ([]Feed, error) {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	if len(s.IDs) == 0 {
		return nil, errors.New("desk: no screen to capture")
	}
	d, err := android.DefaultDisplay(ctx)
	if err != nil {
		return nil, fmt.Errorf("desk: cannot find the screen to capture: %w", err)
	}
	st, err := android.CaptureDisplay(ctx, d, android.Options{
		Width:  plan.ScreenW,
		Height: plan.ScreenH,
		// A ceiling, not a rate: MediaProjection is change-driven, and a still
		// screen delivers nothing at all until it moves.
		FPS: 60,
	})
	if err != nil {
		return nil, fmt.Errorf("desk: cannot capture the screen: %w", err)
	}

	feeds := make([]Feed, plan.Count())
	feeds[0] = &captureFeed{s: st}
	if plan.Count() > 1 {
		logf("%d ribbon positions have nothing to show: this platform offers one screen",
			plan.Count()-1)
	}
	return feeds, nil
}
