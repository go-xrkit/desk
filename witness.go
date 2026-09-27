// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Witnesses is how many strings the desk allocates to catch the heap
// corruption in the act.
//
// ⛔⛔ A GO STRING IN THIS PROGRAM LOSES ITS FIRST FOUR BYTES, now and then, and
// nine occurrences have not named the writer. Static analysis is EXHAUSTED: the
// twelve purego calls that take a Go pointer all write synchronously into live
// memory with matched sizes, and every other candidate turned out to be a READ
// hazard. See the memory note; this is the runtime witness that replaces it.
//
// ⭐ AND THE QUESTION IT ANSWERS IS THE ONE THE EVIDENCE MADE PRECISE. Two
// preserved logs -- run90 and run92 -- show a display name from the window
// server AND a string from desk.hcl damaged in the SAME run, both already
// damaged within the first second. One stray pointer writing once cannot do
// that. So the question is no longer "is it damaged" but HOW MANY objects are
// hit at once: one says a stray write, many says a sweep over a region, and the
// two want different hunts.
//
// ⛔ FOUR AT EVERY LENGTH FROM 1 TO 64, because length is the only thing that
// separates the three victims of 2026-09-27 from the two survivors, and an
// instrument that holds it constant can only report "nothing happened". Four
// rather than one so a single stray write cannot be mistaken for a whole length
// class being hit, and 256 short strings still cost under eight kilobytes.
const Witnesses = 256

// WitnessLen is how long witness i is, in bytes.
//
// ⛔⛔ EVERY WITNESS WAS THE SAME LENGTH, AND THAT IS WHY THEY SAW NOTHING. On
// 2026-09-27 an ordinary run damaged three strings and spared two, all read from
// the same settings file in the same parse:
//
//	Wave          4 bytes   intact
//	Firefox       7 bytes   intact
//	Activity      8 bytes   DAMAGED
//	Thunderbird  11 bytes   DAMAGED
//	VITURE Beast 12 bytes   DAMAGED
//
// Sixty-four witnesses of thirty bytes each were beside them and not one was
// touched. An instrument that holds the only thing that separates the victims
// from the survivors CONSTANT can only ever report "nothing happened".
//
// So the witnesses now span 1 to 64 bytes, several at each length, which is the
// difference between "something was hit" and "objects of THIS SHAPE are hit".
// ⚠ It is a candidate and not the answer: length is what the five known cases
// order cleanly, and the next run either confirms it or names something else.
func WitnessLen(i int) int {
	// 1..64 rather than 0..63: a zero-length string has no first four bytes to
	// lose, so it could never carry the fault and its slot would be wasted.
	return i%64 + 1
}

// WitnessText is what witness i is supposed to say.
//
// ⛔ THE HEAD IS ITS OWN, AND IT SURVIVES THE SHORT ONES. The damage zeroes
// exactly the first four bytes, so a witness whose head matched its neighbour's
// could be damaged without anyone being able to tell WHERE the zeros began --
// which is half of what this exists to establish. The index therefore leads,
// base 36 so two characters reach 1295, and the rest is filler.
//
// ⚠ A witness of one, two or three bytes cannot carry a distinctive head, and
// that is not a flaw to paper over: those are the lengths that test whether the
// writer touches an object too small to hold four bytes at all. They are kept,
// and [DamagedWitnesses] still sees them change.
func WitnessText(i int) string {
	head := strconv.FormatInt(int64(i), 36)
	for len(head) < 2 {
		head = "0" + head
	}
	// head0 is the shortest a head can be, which is what the assertion needs.
	const head0 = "00"
	const filler = "-witness-VITURE-Thunderbird-Activity-Firefox-Wave-VITURE-Beast"
	// ⛔ NO PADDING LOOP AND NO RUNTIME GUARD. The head is 2 bytes and the filler
	// 63, so 65 always covers the 64 a witness can ask for. A loop that topped it
	// up, or a panic that checked, would be a branch nothing can reach -- and a
	// branch no test can cover honestly is a hole in the gate rather than safety.
	// The line below refuses to COMPILE if the filler ever stops being enough,
	// which is the same guarantee with none of the cost.
	const _ = uint(len(head0) + len(filler) - 64)
	s := head + filler
	n := WitnessLen(i)
	return s[:n]
}

// NewWitnesses allocates the witnesses, freshly, so they land in the heap the
// desk is using at the moment it starts -- which is where the two known victims
// were.
//
// ⛔ BUILT RATHER THAN CONSTANT. A package-level constant string lives in the
// binary's read-only data, where nothing can write to it and the whole
// instrument would measure nothing. These are built at run time, on the heap,
// like the display name and the line from the settings file.
func NewWitnesses() []string {
	out := make([]string, Witnesses)
	for i := range out {
		// Through a byte slice, so each is its own allocation rather than a
		// slice of one shared backing array: the victims were separate
		// allocations and a sweep that hit one of these must not be reported as
		// hitting all of them.
		out[i] = string([]byte(WitnessText(i)))
	}
	return out
}

// DamagedWitnesses reports which witnesses no longer say what they were built
// to say.
//
// It returns the indexes rather than a count, because WHICH ones matters: a
// sweep over a region hits a run of neighbours, and a stray pointer hits one.
// The caller prints both.
func DamagedWitnesses(ws []string) []int {
	var hit []int
	for i, s := range ws {
		if s != WitnessText(i) {
			hit = append(hit, i)
		}
	}
	return hit
}

// WitnessReport is what to say when a witness has been damaged: loudly, with
// the count and the indexes, because this is the only evidence the fault
// leaves.
func WitnessReport(hit []int, total int) string {
	if len(hit) == 0 {
		return ""
	}
	s := "⛔ the heap corruption was CAUGHT: " + strconv.Itoa(len(hit)) + " of " +
		strconv.Itoa(total) + " witness strings are damaged, at "
	for i, at := range hit {
		if i > 0 {
			s += ", "
		}
		if i == 8 {
			s += "and " + strconv.Itoa(len(hit)-8) + " more"
			break
		}
		s += strconv.Itoa(at)
	}
	return s + " (" + lengthsHit(hit) + "). One is a stray write; a run of " +
		"neighbours is a sweep over a region. This run is worth keeping."
}

// lengthsHit says which LENGTHS were damaged and how completely.
//
// ⛔⛔ THIS IS THE LINE THE INSTRUMENT EXISTS FOR NOW. The five known cases order
// cleanly by length -- 4 and 7 bytes intact, 8, 11 and 12 damaged -- and nothing
// else about them does. Indexes alone said "something was hit"; the lengths say
// whether objects of a particular SHAPE are hit, and four witnesses per length
// say whether a length was caught entirely or by one stray write.
//
// "4/4 at 8, 12" reads as: every witness of eight bytes and every one of twelve.
// A length where one of four is damaged reads "1/4", and that is a very different
// claim -- it is what a single stray pointer looks like.
func lengthsHit(hit []int) string {
	perLen := map[int]int{}
	for _, at := range hit {
		perLen[WitnessLen(at)]++
	}
	lens := make([]int, 0, len(perLen))
	for n := range perLen {
		lens = append(lens, n)
	}
	sort.Ints(lens)

	// Every length has the same number of witnesses, so one count describes the
	// lot: Witnesses is a whole multiple of the 64 lengths by construction.
	const each = Witnesses / 64

	// ⛔ CONTIGUOUS LENGTHS WITH THE SAME COUNT COLLAPSE, or the line that is
	// supposed to name a shape becomes sixty-four clauses nobody reads. Its own
	// test caught that: a sweep across every length printed "4/4 at 1, 4/4 at
	// 2, ..." to the end. "4/4 at 1-64" is the same fact and is a SENTENCE.
	var b strings.Builder
	b.WriteString("by length: ")
	runs := 0
	for i := 0; i < len(lens); {
		j, n := i, perLen[lens[i]]
		for j+1 < len(lens) && lens[j+1] == lens[j]+1 && perLen[lens[j+1]] == n {
			j++
		}
		if runs > 0 {
			b.WriteString(", ")
		}
		if runs == 8 {
			fmt.Fprintf(&b, "and %d more lengths", len(lens)-i)
			break
		}
		if j == i {
			fmt.Fprintf(&b, "%d/%d at %d", n, each, lens[i])
		} else {
			fmt.Fprintf(&b, "%d/%d at %d-%d", n, each, lens[i], lens[j])
		}
		runs++
		i = j + 1
	}
	return b.String()
}

// SelfCheck arms the instrument by damaging a witness of its own, on purpose,
// and says whether the damage was seen. An empty string means it was.
//
// ⛔⛔ A SILENT LOG SAYS TWO DIFFERENT THINGS AND THIS IS WHAT SEPARATES THEM.
// Without it, a run whose journal never mentions a witness could mean "nothing
// was damaged" or "the check never ran" -- a mis-wired loop, a slice that was
// never allocated, a build where this file was left out. Those are opposite
// conclusions and the reader had no way to tell them apart. Nine occurrences
// of this fault have already been lost to instruments nobody watched fire.
//
// ⭐ The damage is the SHAPE of the real one -- the first four bytes set to
// zero, the rest untouched -- so what is proved is that the comparison catches
// THAT, not merely that it catches a different string.
//
// It works on witnesses of its own, never the live ones: a control that
// consumes its subject proves nothing about the next check.
func SelfCheck() string { return selfCheck(DamagedWitnesses) }

// selfCheck takes the detector as an argument so the suite can hand it a
// broken one and watch this REFUSE. ⛔ A control that cannot fail is not a
// control: with the real detector wired in permanently, a test could only ever
// observe the happy answer and would pass just as well if the whole function
// returned the empty string.
func selfCheck(damaged func([]string) []int) string {
	ws := NewWitnesses()
	// ⛔ A WITNESS LONG ENOUGH TO LOSE FOUR BYTES. Witness 0 is one byte now that
	// the lengths vary, and zeroing four bytes of a one-byte string is a thing
	// this control cannot do -- it would have panicked, on the startup path, in
	// every run. 11 is twelve bytes, the length of "VITURE Beast".
	const at = 11
	ws[at] = "\x00\x00\x00\x00" + WitnessText(at)[4:]

	hit := damaged(ws)
	switch {
	case len(hit) == 0:
		return "⛔ the witness self-check FAILED: a head set to zero was not seen. " +
			"Nothing this run says about witnesses can be trusted."
	case len(hit) != 1 || hit[0] != at:
		return "⛔ the witness self-check FAILED: damaging witness " +
			strconv.Itoa(at) + " was reported as " + strconv.Itoa(len(hit)) +
			" damaged. The count this instrument exists to measure is wrong."
	}
	return ""
}
