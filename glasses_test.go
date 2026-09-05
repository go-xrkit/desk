// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"strings"
	"testing"
)

// fakeGlasses stands in for the headset: it remembers what was written and can
// be told to refuse.
type fakeGlasses struct {
	at     map[byte]uint16
	refuse bool
	writes int
}

func (f *fakeGlasses) install(t *testing.T) {
	t.Helper()
	if f.at == nil {
		f.at = map[byte]uint16{}
	}
	wasGet, wasSet := GlassesGet, GlassesSet
	t.Cleanup(func() { GlassesGet, GlassesSet = wasGet, wasSet })
	GlassesGet = func(id byte) (uint16, error) {
		v, ok := f.at[id]
		if !ok {
			return 0, errors.New("no such setting")
		}
		return v, nil
	}
	GlassesSet = func(id byte, v uint16) error {
		f.writes++
		if f.refuse {
			return statusError(4, true)
		}
		f.at[id] = v
		return nil
	}
}

// TestNudgeReadsBeforeItWrites.
//
// ⛔ A key that says "brighter" has to know what it is brighter THAN, and the
// headset is the only thing that knows: the person may have used the buttons on
// the arm since the desk last looked. A step computed from a remembered value
// would fight whoever touched the glasses last.
func TestNudgeReadsBeforeItWrites(t *testing.T) {
	f := &fakeGlasses{at: map[byte]uint16{GlassesBrightness: 3}}
	f.install(t)

	got, err := Nudge(GlassesBrightness, +1, BrightnessMax)
	if err != nil || got != 4 {
		t.Fatalf("Nudge = %d, %v; want 4", got, err)
	}
	// Somebody else moves it. The next step must start from THERE.
	f.at[GlassesBrightness] = 7
	if got, _ := Nudge(GlassesBrightness, +1, BrightnessMax); got != 8 {
		t.Errorf("Nudge = %d after the headset moved to 7; want 8", got)
	}
}

// TestNudgeStopsAtTheEdgesTheHeadsetGave.
//
// ⭐ The range is the headset's own answer: it took 0 to 8 and refused 9 and
// above. Clamping here means a key held down does not turn into a stream of
// refusals -- and the LAST press at the edge writes nothing at all.
func TestNudgeStopsAtTheEdgesTheHeadsetGave(t *testing.T) {
	f := &fakeGlasses{at: map[byte]uint16{GlassesBrightness: BrightnessMax}}
	f.install(t)

	got, err := Nudge(GlassesBrightness, +1, BrightnessMax)
	if err != nil || got != BrightnessMax {
		t.Fatalf("Nudge past the top = %d, %v", got, err)
	}
	if f.writes != 0 {
		t.Errorf("it wrote %d times at the top; there was nothing to change", f.writes)
	}

	f.at[GlassesBrightness] = 0
	if got, _ := Nudge(GlassesBrightness, -1, BrightnessMax); got != 0 {
		t.Errorf("Nudge past the bottom = %d", got)
	}
}

// TestARefusalIsReportedWithWhatTheHeadsetSaid, rather than as a step that
// silently did not happen.
func TestARefusalIsReportedWithWhatTheHeadsetSaid(t *testing.T) {
	f := &fakeGlasses{at: map[byte]uint16{GlassesBrightness: 3}, refuse: true}
	f.install(t)

	at, err := Nudge(GlassesBrightness, +1, BrightnessMax)
	if err == nil {
		t.Fatal("a refusal was reported as a success")
	}
	if !errors.Is(err, ErrNoGlasses3D) {
		t.Errorf("the error is %v", err)
	}
	if at != 3 {
		t.Errorf("it reported %d; the setting did not move", at)
	}
}

// TestWhatEachStatusCodeMeans, because the codes are the instrument: they tell
// "refused" from "not understood" from "never arrived", which a display cannot.
func TestWhatEachStatusCodeMeans(t *testing.T) {
	if err := statusError(0, true); err != nil {
		t.Errorf("code 0 is the one that means taken: %v", err)
	}
	for _, c := range []struct {
		code uint16
		says string
	}{
		{4, "refused"},
		{6, "too short"},
		{9, "code 9"},
	} {
		err := statusError(c.code, true)
		if err == nil {
			t.Fatalf("code %d was reported as a success", c.code)
		}
		if !strings.Contains(err.Error(), c.says) {
			t.Errorf("code %d reads as %q, want it to say %q", c.code, err, c.says)
		}
	}
	// And the refusal names WHICH picture was refused, since that is the thing
	// a person asked for.
	if got := statusError(4, true).Error(); !strings.Contains(got, "side-by-side") {
		t.Errorf("a refused 3D reads as %q", got)
	}
	if got := statusError(4, false).Error(); !strings.Contains(got, "2D") {
		t.Errorf("a refused 2D reads as %q", got)
	}
}

// TestTheGlassesKeysSayWhereTheyLanded.
//
// ⛔ A key that dims something without saying how far is a key pressed blind
// twice: the person cannot tell "one step darker" from "it did nothing". So the
// new step goes on the picture, out of the range the headset gave.
func TestTheGlassesKeysSayWhereTheyLanded(t *testing.T) {
	f := &fakeGlasses{at: map[byte]uint16{
		GlassesBrightness: 4,
		GlassesVolume:     4,
	}}
	f.install(t)

	d := deskAt(t, MinDistance)
	d.Badge(1, nil)

	d.Do(ActionBrighter)
	if text, _, _ := noticeSays(d); !strings.Contains(text, "brightness 5 of 8") {
		t.Errorf("brighter says %q", text)
	}
	d.Do(ActionDimmer)
	if text, _, _ := noticeSays(d); !strings.Contains(text, "brightness 4 of 8") {
		t.Errorf("dimmer says %q", text)
	}
	d.Do(ActionMute)
	if text, _, _ := noticeSays(d); !strings.Contains(text, "muted") {
		t.Errorf("mute says %q", text)
	}
	if f.at[GlassesVolume] != 0 {
		t.Errorf("mute left the volume at %d", f.at[GlassesVolume])
	}
	d.Do(ActionLouder)
	if text, _, _ := noticeSays(d); !strings.Contains(text, "volume 1 of 8") {
		t.Errorf("louder says %q", text)
	}
}

// TestAHeadsetThatIsNotThereSaysSo, rather than a key that does nothing.
func TestAHeadsetThatIsNotThereSaysSo(t *testing.T) {
	f := &fakeGlasses{} // knows no settings at all
	f.install(t)

	d := deskAt(t, MinDistance)
	d.Badge(1, nil)
	d.Do(ActionBrighter)
	text, up, _ := noticeSays(d)
	if !up || text == "" {
		t.Fatal("a headset that answered nothing left the view silent")
	}
	if !strings.Contains(text, "no such setting") {
		t.Errorf("it says %q", text)
	}
}

// TestQuieterAndAnActionThatIsNotOne.
//
// The two branches nothing else reaches: the quieter step, and the guard that
// makes adjustGlasses do nothing for an action it was never meant to see.
func TestQuieterAndAnActionThatIsNotOne(t *testing.T) {
	f := &fakeGlasses{at: map[byte]uint16{GlassesVolume: 5}}
	f.install(t)

	d := deskAt(t, MinDistance)
	d.Badge(1, nil)
	d.Do(ActionQuieter)
	if text, _, _ := noticeSays(d); !strings.Contains(text, "volume 4 of 8") {
		t.Errorf("quieter says %q", text)
	}

	// ⛔ An action that is not one of the five must change nothing and say
	// nothing: adjustGlasses is only ever called for those, and a default that
	// guessed would put a wrong number on the picture.
	before := f.writes
	d.adjustGlasses(ActionQuit)
	if f.writes != before {
		t.Error("an action that is not a glasses setting wrote one")
	}
}
