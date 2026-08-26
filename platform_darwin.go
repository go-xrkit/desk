// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-macos/screencapture"
	"github.com/go-macos/virtualdisplay"
)

// Screens is what the ribbon will show, and how it was obtained.
//
// The two are reported together because a viewer needs to know which they got:
// screens that this program CREATED and can put anything on, or the displays the
// machine already had. They look the same on the ribbon and they are not the
// same thing at all.
type Screens struct {
	// IDs are CGDirectDisplayIDs, in ribbon order.
	IDs []uint64
	// Virtual reports whether these were created by us.
	Virtual bool
	// Why explains what happened, and is worth showing to a person: a fallback
	// to real displays is not a failure, but it IS a different feature.
	Why string

	virtual []*virtualdisplay.Display
}

// Add creates one more virtual display and appends it, returning its id.
//
// It refuses when these screens are not ours: a desk that fell back to the real
// displays attached to the machine cannot add one, and inventing a seventh
// monitor is not something to attempt on somebody's behalf.
func (s *Screens) Add(w, h int) (uint64, error) {
	if !s.Virtual {
		return 0, fmt.Errorf("desk: these are real displays (%s), so one cannot be added", s.Why)
	}
	d, err := virtualdisplay.Open(virtualdisplay.Spec{
		Name:   fmt.Sprintf("XR desk %d", len(s.virtual)+1),
		Width:  uint32(w),
		Height: uint32(h),
	})
	if err != nil {
		return 0, fmt.Errorf("desk: another virtual display was refused: %w", err)
	}
	s.virtual = append(s.virtual, d)
	s.IDs = append(s.IDs, uint64(d.ID()))
	return uint64(d.ID()), nil
}

// Close removes any display this created, and is safe to call twice.
//
// It matters more than most Close methods: a virtual display that outlives its
// process is a display the person has to remove by hand.
func (s *Screens) Close() error {
	var first error
	for _, d := range s.virtual {
		if err := d.Close(); err != nil && first == nil {
			first = err
		}
	}
	s.virtual = nil
	return first
}

// Provide gets the ribbon its screens.
//
// It asks for count virtual displays at exactly the size the plan wants — one
// eye's worth of pixels, which is the most the glasses can show — and falls back
// to the displays that already exist when it cannot have them. Both outcomes are
// useful; only one of them is what the application is for.
//
// A virtual display's size is fixed AT CREATION and can never be changed
// afterwards: setting a mode on one leaves it on the desktop until the process
// exits, however carefully it is released. So the size is asked for once, here,
// and a display that comes up wrong is refused rather than corrected.
func Provide(ctx context.Context, plan Plan, logf func(string, ...any)) (*Screens, error) {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	s := &Screens{}

	if err := virtualdisplay.Available(); err != nil {
		logf("virtual displays are not available: %v", err)
		return realScreens(ctx, s, fmt.Sprintf("virtual displays unavailable (%v)", err))
	}

	for i := 0; i < plan.Count(); i++ {
		d, err := virtualdisplay.Open(virtualdisplay.Spec{
			Name:   fmt.Sprintf("XR desk %d", i+1),
			Width:  uint32(plan.ScreenW),
			Height: uint32(plan.ScreenH),
			// OnTerminate is deliberately nil: the block can fire while the Go
			// runtime is shutting down, and it has been seen to crash a process
			// on the way out.
		})
		if err != nil {
			// Whatever was made so far must go, or the person is left with
			// displays nobody asked for.
			_ = s.Close()
			logf("could not create virtual display %d of %d: %v", i+1, plan.Count(), err)
			return realScreens(ctx, &Screens{}, fmt.Sprintf("virtual display %d refused (%v)", i+1, err))
		}
		s.virtual = append(s.virtual, d)
		s.IDs = append(s.IDs, uint64(d.ID()))
	}
	s.Virtual = true
	s.Why = fmt.Sprintf("%d virtual displays of %dx%d", len(s.IDs), plan.ScreenW, plan.ScreenH)
	logf("%s", s.Why)
	return s, nil
}

// realScreens falls back to the displays the machine already has.
//
// It asks CoreGraphics, NOT the capture library. The first version asked
// screencapture, which needs the screen-recording permission — so the fallback
// that exists to work around a missing capability required a permission of its
// own, and a machine without virtual displays reported a TCC error that had
// nothing to do with why it got there. Listing displays needs no permission;
// only reading their pixels does.
func realScreens(_ context.Context, s *Screens, why string) (*Screens, error) {
	ids, err := virtualdisplay.ActiveDisplayIDs()
	if err != nil {
		return nil, fmt.Errorf("desk: no virtual displays and cannot list real ones: %w", err)
	}
	if len(ids) == 0 {
		return nil, errors.New("desk: no displays at all")
	}
	for _, id := range ids {
		s.IDs = append(s.IDs, uint64(id))
	}
	s.Virtual = false
	s.Why = why + fmt.Sprintf("; showing the %d display(s) this machine already has", len(s.IDs))
	return s, nil
}

// captureFeed adapts one capture stream to the portable Feed the desk consumes.
//
// It is thin on purpose: the stream already hands back borrowed pixels with the
// stride carried, which is exactly what the compositor wants, so there is
// nothing here to copy and nothing to allocate.
type captureFeed struct{ s *screencapture.Stream }

func (f *captureFeed) Frame() (Source, bool) {
	fr, fresh := f.s.Frame()
	if !fr.Valid() {
		return Source{}, false
	}
	return Source{Pix: fr.Pix, W: fr.Width, H: fr.Height, Stride: fr.Stride}, fresh
}

func (f *captureFeed) Close() error { return f.s.Close() }

// Capture opens a stream on every screen, in ribbon order.
//
// A screen whose capture cannot be opened becomes a NIL feed rather than an
// error: one display refusing is not a reason to show the viewer nothing, and a
// desk with a gap in it says more about what went wrong than a failure to start.
// What went wrong is reported through logf.
func Capture(ctx context.Context, plan Plan, s *Screens, logf func(string, ...any)) ([]Feed, error) {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	if !screencapture.Available() {
		return nil, errors.New("desk: ScreenCaptureKit is not available on this system")
	}
	if !screencapture.Authorized() {
		return nil, errPermission()
	}

	feeds := make([]Feed, plan.Count())
	for i := 0; i < plan.Count() && i < len(s.IDs); i++ {
		st, err := screencapture.CaptureDisplay(ctx,
			screencapture.Display{ID: uint32(s.IDs[i]), Width: plan.ScreenW, Height: plan.ScreenH},
			screencapture.Options{
				Width:  plan.ScreenW,
				Height: plan.ScreenH,
				FPS:    60,
				// The cursor belongs on the screen it is on, and a viewer looking
				// at six of them wants to see where it is.
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
// arbitrary so that the same display keeps the same identity across a restart,
// which is what lets an arrangement be written down and restored.
func offerID(id uint32) string { return fmt.Sprintf("display-%d", id) }

// DisplayOfferID is the [Offer] id for a display, for a caller that has just
// created one and wants it opened.
func DisplayOfferID(id uint64) string { return offerID(uint32(id)) }

// Sources lists everything on this machine that a ribbon position could show.
//
// The displays this program created come first, because they are what it is
// for; the machine's own screens follow, because a viewer may well want their
// laptop on the ribbon too.
func Sources(ctx context.Context, s *Screens) ([]Offer, error) {
	if !screencapture.Available() {
		return nil, errors.New("desk: ScreenCaptureKit is not available on this system")
	}
	if !screencapture.Authorized() {
		return nil, errPermission()
	}
	ds, err := screencapture.Displays(ctx)
	if err != nil {
		return nil, fmt.Errorf("desk: cannot list displays: %w", err)
	}

	ours := make(map[uint32]bool, len(s.IDs))
	if s != nil && s.Virtual {
		for _, id := range s.IDs {
			ours[uint32(id)] = true
		}
	}
	var mine, theirs []Offer
	for _, d := range ds {
		o := Offer{
			ID:   offerID(d.ID),
			Kind: KindDisplay,
			W:    d.PixelWidth,
			H:    d.PixelHeight,
		}
		if ours[d.ID] {
			o.Name = fmt.Sprintf("XR screen %d", len(mine)+1)
			mine = append(mine, o)
			continue
		}
		o.Name = fmt.Sprintf("display %d", d.ID)
		if d.Main {
			o.Name += " (main)"
		}
		theirs = append(theirs, o)
	}
	return append(mine, theirs...), nil
}

// OpenOffer starts capturing one source, ready to be put on a position.
func OpenOffer(ctx context.Context, plan Plan, o Offer) (Feed, error) {
	var id uint32
	if _, err := fmt.Sscanf(o.ID, "display-%d", &id); err != nil {
		return nil, fmt.Errorf("%w: %q is not a display on this platform", ErrNoSuchOffer, o.ID)
	}
	st, err := screencapture.CaptureDisplay(ctx,
		screencapture.Display{ID: id, Width: plan.ScreenW, Height: plan.ScreenH},
		screencapture.Options{
			Width: plan.ScreenW, Height: plan.ScreenH, FPS: 60, ShowsCursor: true,
		})
	if err != nil {
		return nil, fmt.Errorf("desk: cannot capture %s: %w", o, err)
	}
	return &captureFeed{s: st}, nil
}

// errPermission is the one message about the screen-recording grant, said once.
//
// It names what to do rather than what failed, because "permission denied" sends
// a person to the wrong place: the grant belongs to whatever LAUNCHED this, not
// to this.
func errPermission() error {
	return errors.New("desk: screen recording is not permitted. " +
		"Grant it in System Settings > Privacy & Security > Screen & System Audio Recording " +
		"to the application that launched this program — for a program started from a shell " +
		"that is the terminal or editor, not the program itself — then restart it")
}
