// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// ⛔⛔ AN EMPTY LIST IS A FAILED READ, NOT AN UNPLUGGED DISPLAY, and this is the
// test that would have saved two sessions. Measured on 2026-09-08, on a desk
// somebody was wearing:
//
//	desk: "VITURE Beast" is not attached any more; there is  -- stopping
//
// Nothing after "there is". The message names every attached display and it
// named none -- so the desk put the arrangement back and quit, twice, on a
// machine whose displays were all present.
func TestAnEmptyDisplayListIsNotAnUnpluggedDisplay(t *testing.T) {
	i, err := lookUpScreen(wholeBeast, nil)
	if !errors.Is(err, errDisplaysUnreadable) {
		t.Fatalf("an empty list gave %v, want errDisplaysUnreadable", err)
	}
	if i >= 0 {
		t.Errorf("an empty list handed back screen %d; there is no screen to hand back", i)
	}
	// ⛔ AND IT MUST NOT READ AS "NOT ATTACHED", which is what quit the session.
	if strings.Contains(err.Error(), "not attached") {
		t.Errorf("the message still says the display is unplugged:\n%s", err)
	}
	// The same for an empty non-nil slice: a caller that built a list and found
	// nothing in it is in exactly the same position.
	if _, err := lookUpScreen(wholeBeast, []string{}); !errors.Is(err, errDisplaysUnreadable) {
		t.Errorf("an empty (non-nil) list gave %v", err)
	}
}

// A display genuinely gone, among displays that ARE there, still reports itself
// gone -- or unplugging the glasses would leave the desk running on somebody
// else's screen.
func TestADisplayMissingFromARealListIsStillGone(t *testing.T) {
	attached := []string{"Color LCD", "Odyssey G95NC"}
	i, err := lookUpScreen(wholeBeast, attached)
	if err == nil {
		t.Fatal("a genuinely absent display was reported present")
	}
	if errors.Is(err, errDisplaysUnreadable) {
		t.Error("a populated list was treated as an unreadable one")
	}
	if i >= 0 {
		t.Errorf("handed back screen %d for a display that is not there", i)
	}
	if !strings.Contains(err.Error(), "not attached") ||
		!strings.Contains(err.Error(), "Odyssey") {
		t.Errorf("the message must say what IS attached:\n%s", err)
	}
}

func TestAnAttachedDisplayIsFoundWithNoComplaint(t *testing.T) {
	i, err := lookUpScreen(wholeBeast, []string{"Color LCD", wholeBeast})
	if err != nil {
		t.Fatalf("an attached display reported %v", err)
	}
	if i != 1 {
		t.Errorf("found at %d, want 1", i)
	}
}

// And the damaged-name recovery still works through the same door.
func TestADamagedNameIsStillRecoveredHere(t *testing.T) {
	i, err := lookUpScreen(damagedBeast, []string{"Color LCD", wholeBeast})
	var damaged errDamagedName
	if !errors.As(err, &damaged) {
		t.Fatalf("a damaged name gave %v, want errDamagedName", err)
	}
	if i != 1 {
		t.Errorf("recovered screen %d, want 1", i)
	}
}

// ⭐ AND THE EMPTY LIST IS SAID ONCE, not once a second: the lookup runs every
// second, and a damaged name already showed what that does to a log -- 3 939
// identical lines out of 4 797.
func TestTheEmptyListReportIsThrottledToo(t *testing.T) {
	var d damagedNames
	t0 := time.Now()
	first := d.reportOnce(t0, "an empty display list", errDisplaysUnreadable.Error())
	if !strings.Contains(first, "EMPTY") {
		t.Fatalf("the first report was %q", first)
	}
	for i := 1; i < 600; i++ {
		if s := d.reportOnce(t0.Add(time.Duration(i)*time.Second),
			"an empty display list", errDisplaysUnreadable.Error()); s != "" {
			t.Fatalf("occurrence %d spoke inside the quiet period: %q", i, s)
		}
	}
	again := d.reportOnce(t0.Add(damagedNameQuiet+time.Second),
		"an empty display list", errDisplaysUnreadable.Error())
	if !strings.Contains(again, "STILL") || !strings.Contains(again, "601") {
		t.Errorf("the repeat was %q, want STILL and the count 601", again)
	}
}
