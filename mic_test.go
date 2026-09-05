// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"testing"
)

// fakeMic is a microphone that remembers whether it is off.
type fakeMic struct {
	name    string
	off     bool
	reads   int
	writes  int
	readErr error
	setErr  error
}

func (m *fakeMic) Name() string { return m.name }
func (m *fakeMic) Silenced() (bool, error) {
	m.reads++
	return m.off, m.readErr
}
func (m *fakeMic) Silence(off bool) error {
	m.writes++
	if m.setErr != nil {
		return m.setErr
	}
	m.off = off
	return nil
}

// installMic puts one in for the duration of a test.
func installMic(t *testing.T, m Microphone, err error) {
	t.Helper()
	was := TheMicrophone
	t.Cleanup(func() { TheMicrophone = was })
	TheMicrophone = func() (Microphone, error) { return m, err }
}

// TestTheMicKeyReadsBeforeItWrites.
//
// ⛔ A KEY THAT MEANS "MUTE" IS PRESSED BLIND. The only thing that knows
// whether the microphone is already off is the device -- a person may have used
// a hardware switch, or another application, since anything here last looked --
// so a key that toggled a remembered value would be wrong exactly when somebody
// most needs it to be right.
func TestTheMicKeyReadsBeforeItWrites(t *testing.T) {
	m := &fakeMic{name: "MacBook Pro Microphone"}
	installMic(t, m, nil)

	name, off, err := ToggleMic()
	if err != nil {
		t.Fatal(err)
	}
	if m.reads != 1 {
		t.Errorf("it wrote without asking first: %d reads", m.reads)
	}
	if !off || !m.off {
		t.Errorf("the microphone is %v, want muted", m.off)
	}
	if name != "MacBook Pro Microphone" {
		t.Errorf("it named %q", name)
	}

	// And back, from what the device says rather than from what we did.
	m.off = true
	if _, off, _ = ToggleMic(); off {
		t.Error("a muted microphone did not come back")
	}
}

// TestTheMicKeySaysWhichMicrophone.
//
// ⛔ THE HEADSET'S OWN CANNOT BE SILENCED. Measured 2026-09-05: the VITURE
// microphone publishes no mute switch and no capture level, while every other
// input on the machine publishes at least one. So what this key turns off is
// NOT the one the person asked for by name, and muting a different microphone
// without saying which is worse than refusing.
func TestTheMicKeySaysWhichMicrophone(t *testing.T) {
	m := &fakeMic{name: "MacBook Pro Microphone"}
	installMic(t, m, nil)

	d := deskAt(t, MinDistance)
	d.Badge(1, nil)

	d.Do(ActionMic)
	if got, _, _ := noticeSays(d); got != "MacBook Pro Microphone is muted" {
		t.Errorf("the notice reads %q", got)
	}
	d.Do(ActionMic)
	if got, _, _ := noticeSays(d); got != "MacBook Pro Microphone is live" {
		t.Errorf("the notice reads %q", got)
	}
}

// TestAMicrophoneThatWillNotBeSilencedSaysSo, rather than looking like it
// worked.
func TestAMicrophoneThatWillNotBeSilencedSaysSo(t *testing.T) {
	installMic(t, nil, ErrNoMicrophone)

	d := deskAt(t, MinDistance)
	d.Badge(1, nil)
	d.Do(ActionMic)
	if got, _, _ := noticeSays(d); got != ErrNoMicrophone.Error() {
		t.Errorf("the notice reads %q", got)
	}
	if !errors.Is(d.Err(), ErrNoMicrophone) {
		t.Errorf("the desk reports %v", d.Err())
	}

	// A device that lists but will not answer, and one that will not be
	// written: both are reported, and neither leaves the desk claiming a state.
	sulky := &fakeMic{name: "a stubborn microphone", readErr: errors.New("no")}
	installMic(t, sulky, nil)
	if _, _, err := ToggleMic(); err == nil {
		t.Error("a microphone that will not be read looked fine")
	}
	if sulky.writes != 0 {
		t.Error("it wrote to a microphone it could not read")
	}
	stuck := &fakeMic{name: "a stuck microphone", setErr: errors.New("no")}
	installMic(t, stuck, nil)
	if _, off, err := ToggleMic(); err == nil || off {
		t.Errorf("a refused write read as off=%v err=%v", off, err)
	}
}
