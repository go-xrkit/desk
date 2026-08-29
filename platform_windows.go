// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-mswin/screencapture"
)

// Screens is what the ribbon will show, and how it was obtained.
//
// On Windows it is the displays the machine already has. Adding one means an
// indirect display driver, which must be signed and installed — not something a
// program can do for itself while it runs.
type Screens struct {
	// IDs identify the displays, in ribbon order. These are HMONITORs, which is
	// why the field is wide enough for a handle rather than for macOS's
	// 32-bit display id.
	IDs []uint64
	// Virtual reports whether these were created by us. Never true here.
	Virtual bool
	// Why explains what happened, and is worth showing to a person.
	Why string
}

// Close releases anything this created. There is nothing to release.
func (s *Screens) Close() error { return nil }

// Release is Close here: nothing was created, so nothing has to go.
func (s *Screens) Release() error { return nil }

// Provide gets the ribbon its screens.
func Provide(ctx context.Context, plan Plan, logf func(string, ...any)) (*Screens, error) {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	if !screencapture.Available() {
		return nil, errors.New("desk: the Windows display libraries did not load")
	}
	ds, err := screencapture.Displays(ctx)
	if err != nil {
		return nil, fmt.Errorf("desk: cannot list displays: %w", err)
	}
	if len(ds) == 0 {
		return nil, errors.New("desk: no displays at all")
	}
	s := &Screens{}
	for _, d := range ds {
		s.IDs = append(s.IDs, d.ID)
	}
	s.Why = fmt.Sprintf("adding a display on Windows needs a signed driver, so the ribbon "+
		"shows the %d display(s) this machine already has", len(ds))
	logf("%s", s.Why)
	return s, nil
}

// captureFeed adapts one capture stream to the portable Feed the desk consumes.
type captureFeed struct{ s *screencapture.Stream }

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
// error: duplication is torn down by a mode change, a secure desktop or a
// session switch, and one display refusing is not a reason to show the viewer
// nothing.
func Capture(ctx context.Context, plan Plan, s *Screens, logf func(string, ...any)) ([]Feed, error) {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	feeds := make([]Feed, plan.Count())
	for i := 0; i < plan.Count() && i < len(s.IDs); i++ {
		st, err := screencapture.CaptureDisplay(ctx,
			screencapture.Display{ID: s.IDs[i], Width: plan.ScreenW, Height: plan.ScreenH},
			screencapture.Options{
				Width:  plan.ScreenW,
				Height: plan.ScreenH,
				FPS:    60,
				// The cursor belongs on the screen it is on, and a viewer
				// looking at several of them wants to see where it is.
				ShowsCursor: true,
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
func offerID(id uint64) string { return fmt.Sprintf("display-%d", id) }

// Sources lists everything on this machine that a ribbon position could show.
func Sources(ctx context.Context, _ *Screens) ([]Offer, error) {
	if !screencapture.Available() {
		return nil, errors.New("desk: the Windows display libraries did not load")
	}
	ds, err := screencapture.Displays(ctx)
	if err != nil {
		return nil, fmt.Errorf("desk: cannot list displays: %w", err)
	}
	out := make([]Offer, 0, len(ds))
	for _, d := range ds {
		name := d.DeviceName
		if name == "" {
			name = fmt.Sprintf("display %d", d.ID)
		}
		if d.Primary {
			name += " (primary)"
		}
		out = append(out, Offer{ID: offerID(d.ID), Name: name, Kind: KindDisplay,
			W: d.PixelWidth, H: d.PixelHeight})
	}
	return out, nil
}

// OpenOffer starts capturing one source, ready to be put on a position.
func OpenOffer(ctx context.Context, plan Plan, o Offer) (Feed, error) {
	var id uint64
	if _, err := fmt.Sscanf(o.ID, "display-%d", &id); err != nil {
		return nil, fmt.Errorf("%w: %q is not a display on this platform", ErrNoSuchOffer, o.ID)
	}
	st, err := screencapture.CaptureDisplay(ctx,
		screencapture.Display{ID: id, Width: plan.ScreenW, Height: plan.ScreenH},
		screencapture.Options{Width: plan.ScreenW, Height: plan.ScreenH, FPS: 60, ShowsCursor: true})
	if err != nil {
		return nil, fmt.Errorf("desk: cannot capture %s: %w", o, err)
	}
	return &captureFeed{s: st}, nil
}

// Add creates one more display and appends it.
//
// Nothing here can: creating a display is the one part of this that has no
// portable answer, and only macOS is wired up. A person is told rather than
// left pressing a key that does nothing.
func (s *Screens) Add(w, h int) (uint64, error) {
	return 0, fmt.Errorf("desk: adding a screen is not supported on this platform")
}

// DisplayOfferID is the [Offer] id for a display, for a caller that has just
// created one and wants it opened.
func DisplayOfferID(id uint64) string { return fmt.Sprintf("display-%d", id) }

// Remove gives back a display. Nothing here can: only macOS is wired up, and a
// person is told rather than left pressing a key that does nothing.
func (s *Screens) Remove(int) error {
	return fmt.Errorf("desk: removing a screen is not supported on this platform")
}
