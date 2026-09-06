// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"strings"
	"testing"
)

// ⭐ THE STRINGS ARE THE ONES THAT WERE ACTUALLY OBSERVED, four times on
// 2026-09-06, on a real headset. Not invented shapes.
const (
	damagedBeast = "\x00\x00\x00\x00RE Beast"
	wholeBeast   = "VITURE Beast"
)

func TestADamagedNameFindsItsDisplay(t *testing.T) {
	attached := []string{"Built-in Retina Display", wholeBeast, "XR desk 1", "XR desk 2"}
	i, ok := undamage(damagedBeast, attached)
	if !ok {
		t.Fatalf("%q was not recognised as %q", damagedBeast, wholeBeast)
	}
	if attached[i] != wholeBeast {
		t.Errorf("recovered %q, want %q", attached[i], wholeBeast)
	}
}

// ⛔ AN UNDAMAGED NAME IS NEVER GUESSED AT. A display genuinely unplugged has to
// go on reporting itself unplugged, or unplugging the glasses would leave the
// desk running on somebody else's screen.
func TestAnIntactNameIsNeverRecovered(t *testing.T) {
	if _, ok := undamage(wholeBeast, []string{"Built-in Retina Display"}); ok {
		t.Error("a name with no NUL in it was treated as damaged")
	}
	if _, ok := undamage("XR desk 9", []string{"XR desk 1", "XR desk 2"}); ok {
		t.Error("a name that is simply absent was recovered")
	}
}

// ⛔ TWO CANDIDATES IS NOT A RECOVERY, IT IS A GUESS. Putting a desk on the
// wrong display is worse than saying the right one is gone.
func TestAnAmbiguousTailIsRefused(t *testing.T) {
	// Two attached names of the same length ending the same way.
	if _, ok := undamage("\x00\x00\x00\x00 desk 1", []string{"the desk 1", "one desk 1"}); ok {
		t.Error("an ambiguous tail was resolved anyway")
	}
}

// The length has to match too: a name that merely ENDS the same way is a
// different display, and "desk 1" ends "XR desk 1".
func TestALongerNameEndingTheSameWayIsNotIt(t *testing.T) {
	if _, ok := undamage("\x00\x00desk 1", []string{"XR desk 1"}); ok {
		t.Error("a longer name ending the same way was taken for this one")
	}
}

func TestNothingLeftAfterTheDamageIsNotRecovered(t *testing.T) {
	if _, ok := undamage("\x00\x00\x00\x00", []string{"VITURE Beast"}); ok {
		t.Error("a name that is nothing but damage was recovered")
	}
}

// ⚠ IT SAYS SO, LOUDLY, AND THAT IS THE POINT. Recovering silently would turn a
// session-ending fault into an invisible one: nobody would ever collect another
// instance, and the defect would stop being fixable.
func TestTheReportNamesBothStringsAndCallsItADefect(t *testing.T) {
	got := damagedNameReport(damagedBeast, wholeBeast)
	for _, want := range []string{"DAMAGED", wholeBeast, `\x00`, "defect", "Carrying on"} {
		if !strings.Contains(got, want) {
			t.Errorf("the report does not contain %q:\n%s", want, got)
		}
	}
}

// And the error a caller sees carries the same sentence, so nothing has to be
// reconstructed at the other end.
func TestTheErrorSaysWhatTheReportSays(t *testing.T) {
	e := errDamagedName{want: damagedBeast, got: wholeBeast}
	if e.Error() != damagedNameReport(damagedBeast, wholeBeast) {
		t.Error("the error and the report disagree")
	}
}
