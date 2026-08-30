// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"context"
	"errors"
	"fmt"
	"time"

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

// RemovalBudget is how long Close waits for macOS to stop listing the displays
// it just released. Six 1920x1080 displays measured 1.9 s on macOS 26.6.2, so the
// budget is several times that: it is a ceiling on a wait that normally ends
// early, not a delay anyone pays in full.
const RemovalBudget = 8 * time.Second

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

// Close removes any display this created and WAITS for the removal to be
// observable, which is not the same as releasing it. Use it when something is
// about to look at the display list — the settings phase gives the screens back
// before opening its window. It is safe to call twice.
//
// ⚠ On the way OUT, use [Screens.Release] instead. Waiting there was measured
// costing eight full seconds and then printing a warning nobody could act on:
// once AppKit has reconfigured the displays in this process — which opening the
// desk's own window does — releasing a CGVirtualDisplay object no longer
// removes the display, and it goes when the process does. virtualdisplay.Close
// documents that as a property of the private API. A fresh process with the
// same six displays, captures running and windows moved onto them, releases
// them in under a tenth of a second, so the wait is right where it can work and
// wrong where it cannot.
func (s *Screens) Close() error {
	ids := make([]uint32, 0, len(s.virtual))
	for _, d := range s.virtual {
		ids = append(ids, d.ID())
	}
	first := s.Release()
	if err := virtualdisplay.WaitGone(RemovalBudget, ids...); err != nil && first == nil {
		first = err
	}
	return first
}

// Release removes any display this created without waiting for macOS to stop
// listing it, and is safe to call twice.
//
// It matters more than most teardowns: a virtual display that outlives its
// process is a display the person has to remove by hand. What it does NOT do is
// promise the desktop is already back — see [Screens.Close] for that, and for
// why waiting is a bad idea on the way out.
func (s *Screens) Release() error {
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

	// One after another, and that is not an oversight. A display costs about
	// 375 ms, nearly all of it the window server bringing it up, so six screens
	// are 2.6 s of a startup — the largest single piece of it. Measured on macOS
	// 26.6.2 with four displays: 1.50 s one after another, 1.53 s all at once
	// from four goroutines. The window server serialises the work whoever asks,
	// so concurrency here buys NOTHING, and it would cost the simple failure
	// path below, which closes what it made when one is refused.
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
	// Say where they can be SEEN. A person who goes looking for these in System
	// Settings needs to know what they are called there, and that they last
	// exactly as long as this process: the question "why do I not see the
	// virtual screens in the display list?" is answered by both halves.
	logf("macOS lists them as \"XR desk 1\"..\"XR desk %d\", for as long as this runs", len(s.IDs))
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
	ids, err := settledDisplayIDs()
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
	byID := make(map[uint32]screencapture.Display, len(ds))
	for _, d := range ds {
		byID[d.ID] = d
	}

	// THE DESK'S OWN SCREENS COME BACK IN THE BAND'S ORDER, not in the window
	// server's.
	//
	// This is the whole reason this function does not simply walk ds. The band
	// draws position i from Screens.IDs[i]; the inventory says what position i
	// is SHOWING; and everything that has to agree with the picture -- which
	// screen the pointer is held to, where the pointer is drawn -- goes through
	// the inventory. MEASURED, with four screens made left to right:
	//
	//	position 1 draws display 107, and the inventory said 108
	//	position 2 draws display 108, and the inventory said 107
	//
	// ScreenCaptureKit does not list displays in the order they were created,
	// so the pointer was being held to a screen the band was not showing. The
	// report was "je ne peux plus interragir avec les app".
	ours := make(map[uint32]bool, len(s.IDs))
	var mine []Offer
	if s != nil && s.Virtual {
		for _, id := range s.IDs {
			ours[uint32(id)] = true
			d, ok := byID[uint32(id)]
			if !ok {
				// Made, but not offered for capture: it is not a source, and
				// putting it on the ribbon would be putting up a black screen.
				continue
			}
			mine = append(mine, Offer{
				ID:   offerID(d.ID),
				Name: fmt.Sprintf("XR screen %d", len(mine)+1),
				Kind: KindDisplay,
				W:    d.PixelWidth,
				H:    d.PixelHeight,
			})
		}
	}

	// And then the machine's own screens, in whatever order it lists them --
	// nothing here depends on that one.
	var theirs []Offer
	for _, d := range ds {
		if ours[d.ID] {
			continue
		}
		name := fmt.Sprintf("display %d", d.ID)
		if d.Main {
			name += " (main)"
		}
		theirs = append(theirs, Offer{
			ID:   offerID(d.ID),
			Name: name,
			Kind: KindDisplay,
			W:    d.PixelWidth,
			H:    d.PixelHeight,
			Main: d.Main,
		})
	}
	return append(mine, theirs...), nil
}

// OpenOffer starts capturing one source, ready to be put on a position.
func OpenOffer(ctx context.Context, plan Plan, o Offer) (Feed, error) {
	var id uint32
	if _, err := fmt.Sscanf(o.ID, "display-%d", &id); err != nil {
		return nil, fmt.Errorf("%w: %q is not a display on this platform", ErrNoSuchOffer, o.ID)
	}
	// AT ITS OWN SHAPE, at the band's height.
	//
	// Asked for the band's width as well, ScreenCaptureKit letterboxes into it
	// -- measured: 124 flat columns down each side of a 1920x1080 frame of this
	// Mac's 2056x1329 panel -- and those bands are then part of the picture, on
	// a screen that cannot be told to drop them. Captured at its own shape
	// instead, the frame is all screen, and the desk gives the SCREEN that
	// shape: see Desk.fit and Plan.WithScreenWidth.
	//
	// The panel has no 16:9 mode to be put into either, which was the other way
	// round: its sixty modes are 1.547 and 1.600 and nothing else.
	w := plan.ScreenW
	if o.W > 0 && o.H > 0 {
		w = o.W * plan.ScreenH / o.H
	}
	st, err := screencapture.CaptureDisplay(ctx,
		screencapture.Display{ID: id, Width: w, Height: plan.ScreenH},
		screencapture.Options{
			Width: w, Height: plan.ScreenH, FPS: 60, ShowsCursor: true,
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

// settledDisplayIDs is the display list once it has stopped changing.
//
// Closing four virtual displays and asking CoreGraphics what is left, in the
// same breath, got NOTHING back on a machine with two displays plainly attached
// — and the desk gave up with "no displays at all" while the person was looking
// at their screens. Removal is asynchronous, and a list read during a
// reconfiguration is a list of whatever the window server has got to so far.
//
// Two identical readings in a row is the whole rule. It costs one poll when
// nothing is happening, which is the normal case.
func settledDisplayIDs() ([]uint32, error) {
	deadline := time.Now().Add(RemovalBudget)
	last := ""
	for {
		ids, err := virtualdisplay.ActiveDisplayIDs()
		if err != nil {
			return nil, err
		}
		now := fmt.Sprint(ids)
		if now == last {
			return ids, nil
		}
		if time.Now().After(deadline) {
			// Whatever it says now: a list that will not settle is still better
			// than none, and the caller reports what it found either way.
			return ids, nil
		}
		last = now
		time.Sleep(250 * time.Millisecond)
	}
}

// Remove gives back the virtual display at pos, closing it.
//
// It refuses when these screens are not ours, exactly as [Screens.Add] does: a
// desk that fell back to the machine's real displays cannot take one away, and
// switching somebody's monitor off is not something to attempt on their behalf.
func (s *Screens) Remove(pos int) error {
	if !s.Virtual {
		return fmt.Errorf("desk: these are real displays (%s), so one cannot be removed", s.Why)
	}
	if pos < 0 || pos >= len(s.virtual) {
		return fmt.Errorf("desk: no screen %d of %d", pos, len(s.virtual))
	}
	d := s.virtual[pos]
	s.virtual = append(s.virtual[:pos], s.virtual[pos+1:]...)
	s.IDs = append(s.IDs[:pos], s.IDs[pos+1:]...)
	return d.Close()
}
