// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"strings"
	"testing"
)

// TestNineScreenRowsBecomeOne.
//
// ⛔ This is not tidying. The settings window is not resizable and has no
// scroll view, so every row is height the window must find on the display.
// Nine screen shortcuts made it 1470 pixels tall against 1409 usable, which
// put Save and Close past the bottom edge with no way to reach them --
// measured, from the run that broke.
func TestNineScreenRowsBecomeOne(t *testing.T) {
	in := []string{
		"previous: ⌃⌥⌘←",
		"screen 1: ⌃⌥⌘1",
		"screen 2: ⌃⌥⌘2",
		"screen 3: ⌃⌥⌘3",
		"screen 4: ⌃⌥⌘4",
		"screen 5: ⌃⌥⌘5",
		"screen 6: ⌃⌥⌘6",
		"screen 7: ⌃⌥⌘7",
		"screen 8: ⌃⌥⌘8",
		"screen 9: ⌃⌥⌘9",
		"fit: ⌃⌥⌘0",
	}
	got := foldScreens(in)
	if len(got) != 3 {
		t.Fatalf("folded to %d lines, want 3:\n%s", len(got), strings.Join(got, "\n"))
	}
	if got[0] != "previous: ⌃⌥⌘←" || got[2] != "fit: ⌃⌥⌘0" {
		t.Errorf("the lines around the run were altered: %q", got)
	}
	if want := "go to a screen: ⌃⌥⌘1…⌃⌥⌘9"; got[1] != want {
		t.Errorf("the folded line is %q, want %q", got[1], want)
	}
}

// TestASubstitutionBreaksTheRun.
//
// A combination that had to be substituted is exactly the case somebody needs
// spelled out: it is reported with what was ASKED for in brackets, and folding
// it away would hide the one line that is not routine.
func TestASubstitutionBreaksTheRun(t *testing.T) {
	in := []string{
		"screen 1: ⌃⌥⌘1",
		"screen 2: ⌃⌥⌘2",
		"screen 3: ⇧⌃⌥⌘3 (asked for ⌃⌥⌘3, it was taken)",
		"screen 4: ⌃⌥⌘4",
		"screen 5: ⌃⌥⌘5",
		"screen 6: ⌃⌥⌘6",
	}
	got := foldScreens(in)
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "it was taken") {
		t.Errorf("the substitution was folded away:\n%s", joined)
	}
	// The two before it are a run of two, which is left alone; the three after
	// it are a run of three, which folds.
	if len(got) != 4 {
		t.Errorf("folded to %d lines, want 4:\n%s", len(got), joined)
	}
	if !strings.Contains(joined, "go to a screen: ⌃⌥⌘4…⌃⌥⌘6") {
		t.Errorf("the run after the substitution did not fold:\n%s", joined)
	}
}

// TestAShortRunIsLeftAlone. Two rows are not a list, and replacing them with a
// range would say less in the same space.
func TestAShortRunIsLeftAlone(t *testing.T) {
	in := []string{"screen 1: ⌃⌥⌘1", "screen 2: ⌃⌥⌘2", "quit: ⌃⌥⌘⎋"}
	got := foldScreens(in)
	if len(got) != 3 {
		t.Errorf("a run of two was folded: %q", got)
	}
}

// TestNothingElseIsTouched.
func TestNothingElseIsTouched(t *testing.T) {
	for _, line := range []string{
		"⌃⌥⌘G was refused: another application holds it",
		"screen: ⌃⌥⌘1",    // no number
		"screen 10: ⌃⌥⌘0", // two digits: not one of the nine
		"screens 1: ⌃⌥⌘1", // not the same word
		"",
	} {
		got := foldScreens([]string{line, line, line, line})
		if len(got) != 4 {
			t.Errorf("%q was folded: %q", line, got)
		}
	}
}

// TestTheFoldedRowSurvivesBeingMadeIntoARow, which is what the window
// actually shows.
func TestTheFoldedRowBecomesOneSettingRow(t *testing.T) {
	report := strings.Join([]string{
		"previous: ⌃⌥⌘←",
		"screen 1: ⌃⌥⌘1",
		"screen 2: ⌃⌥⌘2",
		"screen 3: ⌃⌥⌘3",
	}, "\n")
	rows := shortcutRowsFrom(report)
	if len(rows) != 2 {
		t.Fatalf("%d rows, want 2", len(rows))
	}
	if rows[1].Title != "go to a screen" {
		t.Errorf("the second row is titled %q", rows[1].Title)
	}
}
