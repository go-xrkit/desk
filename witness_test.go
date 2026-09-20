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
