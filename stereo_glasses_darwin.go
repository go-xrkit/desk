// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build darwin

package desk

import (
	"errors"
	"fmt"

	"github.com/go-macos/iokit/viture"
	headset "github.com/go-viture/beast"
)

// ⛔ THIS FILE USED TO BE THE TRANSPORT, and it was two hundred lines of it:
// building reports by hand, opening the HID interface, starting a listener,
// matching replies, and closing in the one order that does not end the
// process. Every one of those was a lesson paid for here -- listen before you
// ask, the value goes in twice little-endian, cancel then WAIT then close --
// and none of them belongs in a desk.
//
// They live in go-viture/beast now, with their reasons, and this is what is
// left: which setting, and what the desk calls it.

// withGlasses opens the headset, runs fn, and closes it in the right order.
//
// Opened and closed around each exchange: the interface is shared with
// whatever else is talking to the headset, and holding it for a session would
// be holding it against them.
func withGlasses(fn func(*headset.Glasses) error) error {
	g, err := headset.Open()
	if err != nil {
		if errors.Is(err, headset.ErrNoDevice) {
			return fmt.Errorf("%w: no control interface on the bus", ErrNoGlasses3D)
		}
		return err
	}
	defer func() { _ = g.Close() }()
	return fn(g)
}

// platformSet3D asks the glasses to put a different picture in front of each
// eye, or to stop.
//
// ⛔ IT IS THE HEADSET THAT SWITCHES, NOT THE MAC. The conversion needs a
// display with room for two eyes, and the headset only offers one once it has
// been told: its EDID changes with its mode -- model 0x120 with eleven modes to
// 1920x1080, model 0x220 with eleven to 3840x1080 -- so from 2D the wide modes
// are not on offer at all and no display API can reach them.
//
// ⚠ THE DISPLAY GOES AWAY AND COMES BACK, which ends the session that asked.
// See [Set3D].
func platformSet3D(on bool) error {
	mode := viture.Mode1920x1080At60
	if on {
		mode = viture.Mode3840x1080At60
	}
	return withGlasses(func(g *headset.Glasses) error {
		if err := g.Set(viture.MsgDisplayMode, mode); err != nil {
			// ⛔ NAME WHICH PICTURE WAS REFUSED. The library knows the headset
			// said no; only here is it known what was asked for, and "they
			// refused" without saying refused WHAT is a sentence a person
			// cannot act on.
			what := "2D"
			if on {
				what = "side-by-side 3D"
			}
			return fmt.Errorf("%w: %s: %v", ErrNoGlasses3D, what, err)
		}
		return nil
	})
}

// platformGlassesGet reads one setting from the headset.
func platformGlassesGet(id byte) (v uint16, err error) {
	err = withGlasses(func(g *headset.Glasses) error {
		got, err := g.Get(id)
		if err != nil {
			// ⚠ A SETTING CAN BE THERE AND THEN NOT BE. Measured three times
			// on this hardware: 0x22 answered one evening, was gone ninety
			// minutes later, came back the next morning and was gone again by
			// lunchtime -- same cable, no replug, every other id answering.
			// So it is reported, not compiled in.
			if errors.Is(err, headset.ErrNoSetting) {
				return fmt.Errorf("%w: %#02x", ErrNoSetting, id)
			}
			return err
		}
		v = got
		return nil
	})
	return v, err
}

// platformGlassesSet writes one, and reports the headset's own answer.
func platformGlassesSet(id byte, value uint16) error {
	return withGlasses(func(g *headset.Glasses) error { return g.Set(id, value) })
}
