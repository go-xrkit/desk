// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

// GOOS=android also matches the _linux suffix, so this file must say it is not
// for Android; the phone has its own adapter and a very different answer about
// virtual displays.
//go:build linux && !android

package desk

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-freedesktop/screencast"
)

// Screens is what the ribbon will show, and how it was obtained.
//
// On Linux it is always the displays the machine already has. Creating one needs
// a compositor to agree, and no protocol an ordinary program can speak offers
// it — so unlike macOS, where a private CoreGraphics call makes real ones, here
// the ribbon carries what is already there. Fewer screens, everything else the
// same.
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
func Provide(ctx context.Context, plan Plan, logf func(string, ...any)) (*Screens, error) {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	if err := screencast.Probe(); err != nil {
		return nil, fmt.Errorf("desk: cannot reach a display server: %w", err)
	}
	ds, err := screencast.Displays(ctx)
	if err != nil {
		return nil, fmt.Errorf("desk: cannot list displays: %w", err)
	}
	if len(ds) == 0 {
		return nil, errors.New("desk: no displays at all")
	}
	s := &Screens{}
	for _, d := range ds {
		s.IDs = append(s.IDs, uint64(d.ID))
	}
	s.Why = fmt.Sprintf("no virtual displays on this platform; showing the %d display(s) "+
		"this machine already has", len(ds))
	logf("%s", s.Why)
	return s, nil
}

// captureFeed adapts one capture stream to the portable Feed the desk consumes.
type captureFeed struct{ s *screencast.Stream }

func (f *captureFeed) Frame() (Source, bool) {
	fr, fresh := f.s.Frame()
	if fr.Width <= 0 || fr.Height <= 0 || len(fr.Pix) == 0 {
		return Source{}, false
	}
	return Source{Pix: fr.Pix, W: fr.Width, H: fr.Height, Stride: fr.Stride}, fresh
}

func (f *captureFeed) Close() error { return f.s.Close() }

// Capture opens a stream on every screen, in ribbon order.
//
// A screen whose capture cannot be opened becomes a NIL feed rather than an
// error: one display refusing is not a reason to show the viewer nothing.
func Capture(ctx context.Context, plan Plan, s *Screens, logf func(string, ...any)) ([]Feed, error) {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	if err := screencast.Probe(); err != nil {
		return nil, fmt.Errorf("desk: cannot capture: %w", err)
	}

	feeds := make([]Feed, plan.Count())
	for i := 0; i < plan.Count() && i < len(s.IDs); i++ {
		st, err := screencast.CaptureDisplay(ctx,
			screencast.Display{ID: uint32(s.IDs[i]), Width: plan.ScreenW, Height: plan.ScreenH},
			screencast.Options{
				Width:  plan.ScreenW,
				Height: plan.ScreenH,
				FPS:    60,
				// The cursor belongs on the screen it is on, and a viewer
				// looking at several of them wants to see where it is.
				ShowsCursor: true,
				// The bytes arriving are already BGRA, so forcing the fourth one
				// opaque is the ONLY per-frame work — and at 4K that single pass
				// measures 18 of 24 ms. The compositor never reads alpha, so
				// this hands the 18 ms back.
				RawAlpha: true,
			})
		if err != nil {
			logf("screen %d (display %d) will stay blank: %v", i+1, s.IDs[i], err)
			continue
		}
		feeds[i] = &captureFeed{s: st}
	}
	return feeds, nil
}

// offerID is how a display is named in an inventory. It is derived rather than
// arbitrary so the same display keeps its identity across a restart.
func offerID(id uint32) string { return fmt.Sprintf("display-%d", id) }

// Sources lists everything on this machine that a ribbon position could show.
func Sources(ctx context.Context, _ *Screens) ([]Offer, error) {
	if err := screencast.Probe(); err != nil {
		return nil, fmt.Errorf("desk: cannot reach a display server: %w", err)
	}
	ds, err := screencast.Displays(ctx)
	if err != nil {
		return nil, fmt.Errorf("desk: cannot list displays: %w", err)
	}
	out := make([]Offer, 0, len(ds))
	for _, d := range ds {
		name := d.Name
		if name == "" {
			name = fmt.Sprintf("display %d", d.ID)
		}
		out = append(out, Offer{ID: offerID(d.ID), Name: name, Kind: KindDisplay, W: d.Width, H: d.Height})
	}
	return out, nil
}

// OpenOffer starts capturing one source, ready to be put on a position.
func OpenOffer(ctx context.Context, plan Plan, o Offer) (Feed, error) {
	var id uint32
	if _, err := fmt.Sscanf(o.ID, "display-%d", &id); err != nil {
		return nil, fmt.Errorf("%w: %q is not a display on this platform", ErrNoSuchOffer, o.ID)
	}
	st, err := screencast.CaptureDisplay(ctx,
		screencast.Display{ID: id, Width: plan.ScreenW, Height: plan.ScreenH},
		screencast.Options{
			Width: plan.ScreenW, Height: plan.ScreenH, FPS: 60, ShowsCursor: true,
			// See Capture: at 4K, forcing the fourth byte opaque is 18 of 24 ms
			// and the compositor never reads it.
			RawAlpha: true,
		})
	if err != nil {
		return nil, fmt.Errorf("desk: cannot capture %s: %w", o, err)
	}
	return &captureFeed{s: st}, nil
}
