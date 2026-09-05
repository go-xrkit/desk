// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"fmt"
	"time"
)

// ErrNoGlasses3D means the headset could not be told to change its mode.
var ErrNoGlasses3D = errors.New("desk: the glasses would not change mode")

// The device this speaks to, and how long it is given to answer.
//
// ⭐ 35ca:1201 on the vendor usage page is the headset's control interface, and
// it is the ONLY one of the three the glasses publish that carries the
// protocol: the other two are called "VITURE Microphone" and are a Consumer
// page audio set. The name misleads and the usage page does not.
const (
	vitureVendor uint16 = 0x35ca
	vendorPage   uint16 = 0xff00

	// glasses3DWait is generous for a device that answers in milliseconds, and
	// short enough that a menu row does not appear to hang.
	glasses3DWait = 2 * time.Second
)

// Set3D asks the glasses to put a different picture in front of each eye.
//
// ⛔ IT IS THE HEADSET THAT SWITCHES, NOT THE MAC. The 3D conversion needs a
// display with room for two eyes, and the headset only offers one once it has
// been told: its EDID changes with its mode, so from 2D the wide modes are not
// on offer at all and no display API can reach them.
//
// ⚠ THE DISPLAY GOES AWAY AND COMES BACK. Changing the mode tears the screen
// down and re-negotiates it, which ends the session the desk is running -- so a
// caller must expect to be restarted, and must remember what was asked for
// across that gap. That is what [Config.Stereo3D] is for.
var Set3D = func(on bool) error { return platformSet3D(on) }

// statusError turns the headset's own answer into one a person can act on.
//
// ⭐ THE HEADSET ANSWERS EVERY COMMAND, and that is worth more than watching the
// screen: a display that does not change cannot tell a refusal from a command
// that never arrived, which is what made this take nineteen attempts to find.
func statusError(status uint16, on bool) error {
	switch status {
	case 0: // taken
		return nil
	case 4:
		what := "2D"
		if on {
			what = "side-by-side 3D"
		}
		return fmt.Errorf("%w: they refused %s", ErrNoGlasses3D, what)
	case 6:
		return fmt.Errorf("%w: they called the command too short, which is a bug here",
			ErrNoGlasses3D)
	default:
		return fmt.Errorf("%w: they answered with the code %d", ErrNoGlasses3D, status)
	}
}

// What the glasses hold, and the range each takes.
//
// ⭐ THE RANGES ARE THE HEADSET'S OWN ANSWER, not a guess. It refuses a value it
// cannot take with a code, so a sweep finds the edges without watching anything:
// brightness answered "taken" from 0 to 8 and "refused" from 9 up.
const (
	// GlassesBrightness is the display's brightness, 0 to 8.
	GlassesBrightness byte = 0x22
	// GlassesFilm is the electrochromic film's opacity: 0 clear, 2 dark.
	GlassesFilm byte = 0x43
	// GlassesVolume is the sound.
	//
	// ⚠ ACCEPTED BUT UNVERIFIED. A write is answered "taken", and the glasses
	// then announce NOTHING -- where pressing their own volume button announces
	// the new step. So the command is well-formed and reaches them; whether it
	// moves the sound has not been established, and only a person listening can
	// say. It is wired because it costs nothing to offer and the failure is
	// harmless; it is documented because pretending otherwise would not be.
	GlassesVolume byte = 0x30

	// BrightnessSteps is how many the headset accepts, 0 to BrightnessMax.
	BrightnessMax uint16 = 8
	// VolumeMax is what the button was seen to reach.
	VolumeMax uint16 = 8
)

// Glasses reads one of the headset's settings.
var GlassesGet = func(id byte) (uint16, error) { return platformGlassesGet(id) }

// GlassesSet writes one, and reports what the headset answered.
var GlassesSet = func(id byte, value uint16) error { return platformGlassesSet(id, value) }

// Nudge moves a setting by one step, clamped, and reports where it ended up.
//
// ⛔ IT READS FIRST. A key that says "brighter" has to know what it is brighter
// than, and the headset is the only thing that knows: the person may have
// changed it with the buttons on the arm since the desk last looked.
func Nudge(id byte, by int, max uint16) (uint16, error) {
	at, err := GlassesGet(id)
	if err != nil {
		return 0, err
	}
	next := int(at) + by
	if next < 0 {
		next = 0
	}
	if next > int(max) {
		next = int(max)
	}
	if uint16(next) == at {
		return at, nil
	}
	if err := GlassesSet(id, uint16(next)); err != nil {
		return at, err
	}
	return uint16(next), nil
}
