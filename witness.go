// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import "strconv"

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
// Sixty-four: enough that a sweep of any width lands on several, few enough to
// cost nothing -- 64 short strings is under two kilobytes, allocated once.
const Witnesses = 64

// WitnessText is what witness i is supposed to say.
//
// Short, and different in its first four bytes from every other: the damage
// zeroes exactly those, so a witness whose head matched its neighbour's could
// be damaged without anyone being able to tell.
func WitnessText(i int) string {
	// The index FIRST and zero-padded, so every witness's first four bytes are
	// its own. Not decoration: the damage zeroes exactly those four, so a
	// distinctive head is what lets the wreckage confirm the SHAPE of the fault
	// -- four bytes at offset zero -- instead of only that something changed.
	// With a shared head there is no telling where the zeros began.
	pad := strconv.Itoa(i)
	if i < 10 {
		pad = "0" + pad
	}
	return "w" + pad + "-witness-VITURE-Thunderbird"
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
	return s + ". One is a stray write; a run of neighbours is a sweep over a " +
		"region. This run is worth keeping."
}
