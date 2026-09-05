// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build darwin

package desk

import (
	"fmt"

	"github.com/go-macos/coreaudio"
)

// audioMic is one CoreAudio input device and the mechanism it happens to offer.
type audioMic struct {
	dev     coreaudio.Device
	byLevel bool // no mute switch: silence it by taking the level to zero
	// wasAt is the level to put back, remembered because taking a level to
	// zero DESTROYS it -- unmuting to "1" would hand somebody back a
	// microphone louder than they had set it.
	wasAt float32
}

func (m *audioMic) Name() string { return m.dev.Name }

func (m *audioMic) Silenced() (bool, error) {
	if !m.byLevel {
		return m.dev.Muted()
	}
	v, err := m.dev.Volume()
	if err != nil {
		return false, err
	}
	return v == 0, nil
}

func (m *audioMic) Silence(off bool) error {
	if !m.byLevel {
		return m.dev.SetMuted(off)
	}
	if off {
		// ⛔ REMEMBER THE LEVEL BEFORE DESTROYING IT. Zero is not a level a
		// person can be given back: coming out of mute at 1.0 would hand them
		// a microphone louder than the one they had.
		if v, err := m.dev.Volume(); err == nil && v > 0 {
			m.wasAt = v
		}
		return m.dev.SetVolume(0)
	}
	back := m.wasAt
	if back <= 0 {
		// Nothing remembered -- this process did not do the muting. Half is a
		// guess, and it is said to be one on the picture.
		back = 0.5
	}
	return m.dev.SetVolume(back)
}

// platformMicrophone picks the input device to silence.
//
// ⛔ THE ONE IN USE, AND ONLY IF IT OFFERS A CONTROL. Measured 2026-09-05: the
// VITURE headset's microphone publishes no mute switch AND no capture level,
// while every other input on the machine publishes at least one -- so a desk
// that insisted on the headset's own microphone would ship a key that can never
// work. The default input is what a person is speaking into.
func platformMicrophone() (Microphone, error) {
	devs, err := coreaudio.Devices()
	if err != nil {
		return nil, fmt.Errorf("desk: listing audio devices: %w", err)
	}
	var refused []string
	for _, d := range devs {
		if d.Inputs == 0 {
			continue
		}
		switch {
		case d.CanMute():
			return &audioMic{dev: d}, nil
		case d.CanSetVolume():
			return &audioMic{dev: d, byLevel: true}, nil
		default:
			refused = append(refused, d.Name)
		}
	}
	if len(refused) > 0 {
		return nil, fmt.Errorf("%w: %s offers neither a mute switch nor a level",
			ErrNoMicrophone, refused[0])
	}
	return nil, fmt.Errorf("%w: this Mac lists no microphone", ErrNoMicrophone)
}
