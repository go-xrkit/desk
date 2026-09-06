// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build darwin

package desk

import (
	"fmt"

	headset "github.com/go-viture/beast"
)

// SetTracking asks the glasses to hold the picture one of three ways, or to
// recentre an anchored one.
//
// ⭐ THE GLASSES DO THE TRACKING. Nothing here computes a pose and no camera is
// opened -- which is what makes this a command rather than a project, and what
// separates it from the Luma Ultra's 6DOF, which is visual-inertial odometry on
// the host from the cameras.
func SetTracking(mode int, recentre bool) error {
	return withGlasses(func(g *headset.Glasses) error {
		if recentre {
			if err := g.Recenter(); err != nil {
				return fmt.Errorf("the picture could not be put back: %w", err)
			}
			return nil
		}
		// ⛔ SAID BEFORE IT IS ASKED FOR, because a headset showing the host's
		// video as it arrives has nothing to anchor and will refuse with an
		// error about a mode nobody mentioned.
		if native, err := g.Native(); err == nil && !native {
			return fmt.Errorf("these glasses are passing the picture through " +
				"rather than placing it, so there is nothing to anchor")
		}
		if err := g.SetDOF(headset.DOF(mode)); err != nil {
			return fmt.Errorf("the glasses would not hold the picture that way: %w", err)
		}
		return nil
	})
}

// ReadTracking is which of the three modes the glasses are in, for the tick in
// the menu.
//
// ⛔ ASKED OF THE HEADSET RATHER THAN REMEMBERED. It has its own button, and
// somebody pressing it has changed the mode without going near the menu -- so a
// menu that remembered its own clicks would be wrong from the first press.
func ReadTracking() Tracking {
	var t Tracking
	err := withGlasses(func(g *headset.Glasses) error {
		native, err := g.Native()
		if err != nil {
			return err
		}
		if !native {
			t.Why = "these glasses are passing the picture through"
			return nil
		}
		d, err := g.DOF()
		if err != nil {
			return err
		}
		t.Mode = int(d)
		return nil
	})
	if err != nil {
		t.Why = "no glasses to ask"
	}
	return t
}
