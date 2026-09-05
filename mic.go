// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import "errors"

// ErrNoMicrophone means there is no microphone this desk can silence.
var ErrNoMicrophone = errors.New("desk: no microphone this can silence")

// Microphone is what the desk needs of one input device.
//
// ⛔ AN INTERFACE, BECAUSE THE ANSWER IS PER DEVICE. A microphone may publish a
// mute switch, or a capture level, or -- measured on the VITURE headset --
// NEITHER, while every other input on the same machine publishes at least one.
// So the desk cannot assume a mechanism; it asks each candidate what it has.
type Microphone interface {
	// Name is what a person reads on the picture.
	Name() string
	// Silenced reads whether the microphone is off, by whichever means this
	// device offers.
	Silenced() (bool, error)
	// Silence turns it off or back on.
	Silence(off bool) error
}

// TheMicrophone finds the input device this desk should silence, or says there
// is none it can.
//
// ⛔ THE ONE IN USE, NOT THE ONE ON THE HEADSET. Asked for as the headset's
// microphone, and the headset's microphone REFUSES: measured 2026-09-05, the
// VITURE input publishes no mute switch and no capture level, so nothing in the
// audio system can turn it off. Silencing the input a person is actually
// speaking into is the nearest thing that works, and it is what a key called
// "mute the microphone" should do anyway.
var TheMicrophone = func() (Microphone, error) { return platformMicrophone() }

// ToggleMic silences the microphone or brings it back, and says which.
//
// ⛔ IT READS FIRST. A key that means "mute" is pressed blind, and the only
// thing that knows whether the microphone is already off is the device -- a
// person may have used a hardware switch, or another application, since
// anything here last looked.
func ToggleMic() (name string, off bool, err error) {
	m, err := TheMicrophone()
	if err != nil {
		return "", false, err
	}
	was, err := m.Silenced()
	if err != nil {
		return m.Name(), false, err
	}
	if err := m.Silence(!was); err != nil {
		return m.Name(), was, err
	}
	return m.Name(), !was, nil
}
