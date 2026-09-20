// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"strconv"
	"strings"
	"testing"
	"unsafe"
)

// TestTheWitnessesAreOnTheHeapAndEachItsOwn.
//
// ⛔⛔ A CONSTANT WOULD MEASURE NOTHING. A package-level string literal lives in
// the binary's read-only data, where nothing can write to it -- the instrument
// would report a clean run for ever, which is indistinguishable from a run that
// was clean. They are built at run time, on the heap, like the display name and
// the line from the settings file that the corruption has actually eaten.
//
// ⛔ AND EACH IS ITS OWN ALLOCATION. Slicing one backing array would make a
// single stray write look like a sweep across every witness that shares it --
// turning the one measurement this exists to make into a lie.
func TestTheWitnessesAreOnTheHeapAndEachItsOwn(t *testing.T) {
	ws := NewWitnesses()
	if len(ws) != Witnesses {
		t.Fatalf("there are %d witnesses, want %d", len(ws), Witnesses)
	}
	seen := map[uintptr]int{}
	for i, s := range ws {
		if s != WitnessText(i) {
			t.Fatalf("witness %d says %q on the day it was made", i, s)
		}
		at := uintptr(unsafe.Pointer(unsafe.StringData(s)))
		if other, ok := seen[at]; ok {
			t.Errorf("witnesses %d and %d share one allocation", other, i)
		}
		seen[at] = i
	}

	// ⛔ AND THEIR FIRST FOUR BYTES DIFFER, because the damage zeroes exactly
	// those: two witnesses with the same head could be damaged without anyone
	// being able to tell which.
	heads := map[string]int{}
	for i := range ws {
		h := WitnessText(i)[:4]
		if other, ok := heads[h]; ok {
			t.Errorf("witnesses %d and %d both begin %q", other, i, h)
		}
		heads[h] = i
	}
}

// TestDamagedWitnessesNamesWhichOnes, because which ones is the measurement: a
// run of neighbours is a sweep over a region and one alone is a stray write.
func TestDamagedWitnessesNamesWhichOnes(t *testing.T) {
	ws := NewWitnesses()
	if hit := DamagedWitnesses(ws); len(hit) != 0 {
		t.Fatalf("fresh witnesses are already damaged at %v", hit)
	}
	// The damage this looks for, reproduced: the first four bytes zeroed.
	bite := func(i int) {
		b := []byte(ws[i])
		b[0], b[1], b[2], b[3] = 0, 0, 0, 0
		ws[i] = string(b)
	}
	bite(3)
	bite(4)
	hit := DamagedWitnesses(ws)
	if len(hit) != 2 || hit[0] != 3 || hit[1] != 4 {
		t.Errorf("DamagedWitnesses = %v, want [3 4]", hit)
	}
}

// TestTheWitnessReportSaysHowManyAndWhere, and stays readable when many are hit.
func TestTheWitnessReportSaysHowManyAndWhere(t *testing.T) {
	if got := WitnessReport(nil, Witnesses); got != "" {
		t.Errorf("nothing damaged reported %q", got)
	}
	one := WitnessReport([]int{7}, 64)
	if !strings.Contains(one, "1 of 64") || !strings.Contains(one, "at 7") {
		t.Errorf("one witness: %q", one)
	}
	// ⛔ A REPORT NOBODY CAN READ IS A REPORT NOBODY KEEPS. Sixty-four indexes
	// on one line is the 3,939-line log all over again, smaller.
	many := make([]int, Witnesses)
	for i := range many {
		many[i] = i
	}
	long := WitnessReport(many, Witnesses)
	if !strings.Contains(long, strconv.Itoa(Witnesses)+" of "+strconv.Itoa(Witnesses)) {
		t.Errorf("all damaged: %q", long)
	}
	if !strings.Contains(long, "and 56 more") {
		t.Errorf("a long list was not shortened: %q", long)
	}
	if n := strings.Count(long, ","); n > 10 {
		t.Errorf("the report carries %d commas: %q", n, long)
	}
}

// ⛔⛔ A DETECTOR NOBODY HAS SEEN FIRE IS NOT A DETECTOR, and the log that goes
// with it cannot be read: silence about the witnesses means either "nothing was
// damaged" or "the check never ran", which are opposite conclusions.
func TestTheSelfCheckArmsTheInstrument(t *testing.T) {
	t.Parallel()

	if bad := SelfCheck(); bad != "" {
		t.Fatalf("the instrument does not see a head set to zero: %s", bad)
	}
}

// AND IT REFUSES WHEN THE DETECTOR IS BROKEN, which is the half that makes it
// a control. Each of these is a way the instrument could be wrong while looking
// exactly as healthy from outside.
func TestTheSelfCheckRefusesABrokenDetector(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name    string
		damaged func([]string) []int
	}{
		// The comparison that never fires: a detector reading the text it just
		// built rather than the witness in front of it would look like this.
		{"sees nothing", func([]string) []int { return nil }},
		// The one that counts wrong, which matters MORE than it looks: how many
		// are hit at once is the whole question this instrument was built to
		// answer -- one is a stray write, several a sweep over a region.
		{"counts everyone", func(ws []string) []int {
			all := make([]int, len(ws))
			for i := range all {
				all[i] = i
			}
			return all
		}},
		// And the one that finds damage in the wrong place: indexes are what
		// say whether the victims are neighbours.
		{"names the wrong one", func([]string) []int { return []int{7} }},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			if selfCheck(c.damaged) == "" {
				t.Error("a broken detector passed the self-check; it proves nothing")
			}
		})
	}
}
