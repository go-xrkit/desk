// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build !darwin && !linux && !android && !windows

package desk

import (
	"context"
	"errors"
	"runtime"
)

// ErrNoPlatform means this build has no way to get screens or their pixels.
//
// The geometry, the plan, the ribbon and the compositor are portable and are
// compiled here in full — what is missing is only the two ends: creating
// displays and capturing them. Keeping the package buildable everywhere is not
// courtesy, it is what lets the portable half be TESTED everywhere, including on
// the machines that run this project's CI.
var ErrNoPlatform = errors.New("desk: no screen capture on this platform yet")

// Screens is what the ribbon will show, and how it was obtained.
type Screens struct {
	// IDs identify the displays, in ribbon order.
	IDs []uint64
	// Virtual reports whether these were created by us.
	Virtual bool
	// Why explains what happened, and is worth showing to a person.
	Why string
}

// Close removes any display this created. There are none here.
func (s *Screens) Close() error { return nil }

// Release is Close here: nothing was created, so nothing has to go.
func (s *Screens) Release() error { return nil }

// Provide gets the ribbon its screens. Not on this platform.
func Provide(_ context.Context, _ Plan, logf func(string, ...any)) (*Screens, error) {
	if logf != nil {
		logf("no display provider for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	return nil, ErrNoPlatform
}

// Capture opens a stream on every screen. Not on this platform.
func Capture(_ context.Context, _ Plan, _ *Screens, logf func(string, ...any)) ([]Feed, error) {
	if logf != nil {
		logf("no screen capture for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	return nil, ErrNoPlatform
}

// Sources lists what a ribbon position could show. Not on this platform.
func Sources(context.Context, *Screens) ([]Offer, error) { return nil, ErrNoPlatform }

// OpenOffer starts capturing one source. Not on this platform.
func OpenOffer(context.Context, Plan, Offer) (Feed, error) { return nil, ErrNoPlatform }

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
