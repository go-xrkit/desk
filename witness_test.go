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
	// being able to tell which. Witnesses shorter than four bytes are exempt for
	// the obvious reason, and they are kept on purpose -- they are the ones that
	// test whether the writer touches an object too small to hold four bytes.
	heads := map[string]int{}
	for i := range ws {
		s := WitnessText(i)
		if len(s) < 4 {
			continue
		}
		h := s[:4]
		if other, ok := heads[h]; ok {
			t.Errorf("witnesses %d and %d both begin %q", other, i, h)
		}
		heads[h] = i
	}

	// ⛔⛔ AND EVERY LENGTH FROM 1 TO 64 IS PRESENT, SEVERAL TIMES. This is the
	// whole change: the run of 2026-09-27 damaged strings of 8, 11 and 12 bytes
	// and spared those of 4 and 7, with sixty-four witnesses of thirty bytes
	// beside them seeing nothing. An instrument that holds the one thing
	// separating victims from survivors constant reports "nothing happened" and
	// is worse than none.
	perLen := map[int]int{}
	for i := range ws {
		if got := len(ws[i]); got != WitnessLen(i) {
			t.Fatalf("witness %d is %d bytes, want %d", i, got, WitnessLen(i))
		}
		perLen[WitnessLen(i)]++
	}
	for n := 1; n <= 64; n++ {
		if perLen[n] < 2 {
			t.Errorf("%d witnesses are %d bytes long; one alone cannot tell a "+
				"length class being swept from a single stray write", perLen[n], n)
		}
	}
	if len(perLen) != 64 {
		t.Errorf("the witnesses cover %d lengths, want 64", len(perLen))
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
	// Computed, not written down: a test that hard-codes the count is a test
	// that fails for the wrong reason the day the number of witnesses changes,
	// which is exactly what happened when they went from 64 to 256.
	if !strings.Contains(long, "and "+strconv.Itoa(Witnesses-8)+" more") {
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

// ⛔⛔ THE REPORT HAS TO TELL A LENGTH CLASS FROM A STRAY WRITE, which is the
// whole reason the witnesses stopped being one length.
//
// The run of 2026-09-27 damaged strings of 8, 11 and 12 bytes and spared those
// of 4 and 7. If that is a property of the SHAPE, every witness of those lengths
// goes; if it was chance, one of the four does. Those are different hunts, and a
// report that cannot separate them sends the reader on the wrong one.
func TestTheReportTellsALengthClassFromAStrayWrite(t *testing.T) {
	t.Parallel()

	// The shape: every witness of 8, 11 and 12 bytes.
	var swept []int
	for i := 0; i < Witnesses; i++ {
		switch WitnessLen(i) {
		case 8, 11, 12:
			swept = append(swept, i)
		}
	}
	s := WitnessReport(swept, Witnesses)
	const each = Witnesses / 64
	for _, want := range []string{
		strconv.Itoa(each) + "/" + strconv.Itoa(each) + " at 8",
		strconv.Itoa(each) + "/" + strconv.Itoa(each) + " at 11-12",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("a whole length class does not read as one: missing %q in %q", want, s)
		}
	}

	// The stray write: one witness, of twelve bytes like "VITURE Beast".
	var one int
	for i := 0; i < Witnesses; i++ {
		if WitnessLen(i) == 12 {
			one = i
			break
		}
	}
	s = WitnessReport([]int{one}, Witnesses)
	if want := "1/" + strconv.Itoa(each) + " at 12"; !strings.Contains(s, want) {
		t.Errorf("a single stray write does not read as one: missing %q in %q", want, s)
	}
	// ⛔ AND THE TWO DO NOT READ ALIKE. If they did, the length would be
	// decoration rather than a measurement.
	if strings.Contains(s, strconv.Itoa(each)+"/"+strconv.Itoa(each)+" at 12") {
		t.Errorf("one witness of twelve bytes reads as the whole class: %q", s)
	}
}

// ⚠ AND A REPORT WITH MORE RUNS THAN A SENTENCE HOLDS IS CUT. Damage every
// other length and the runs no longer collapse: thirty-two of them is the
// 3,939-line log again, smaller.
func TestManyScatteredLengthsAreStillOneSentence(t *testing.T) {
	t.Parallel()

	var scattered []int
	for i := 0; i < Witnesses; i++ {
		if WitnessLen(i)%2 == 0 {
			scattered = append(scattered, i)
		}
	}
	s := WitnessReport(scattered, Witnesses)
	if !strings.Contains(s, "more lengths") {
		t.Errorf("thirty-two separate lengths were all printed: %q", s)
	}
	if n := strings.Count(s, ","); n > 20 {
		t.Errorf("the report carries %d commas: %q", n, s)
	}
}
