// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

// Command witnessbench makes virtual displays with witness strings beside them
// and says how many of the witnesses were damaged.
//
// ⛔⛔ IT EXISTS SO THE HUNT DOES NOT NEED THE AUTHOR'S MAC. A Go string in
// xrdesk loses its first four bytes now and then, and the one thing that makes
// the fault tractable is that it depends on the NUMBER of virtual displays --
// measured 0 of 3 runs with one display and 2 of 3 with five, the damage
// arriving within about 1.4 seconds of their creation. Reproducing it therefore
// means creating five virtual displays, over and over, which is exactly the
// thing that froze a working machine on 2026-09-11: an unattended loop of desk
// sessions, each making five or six, while somebody was using the Mac.
//
// So this does the smallest thing that can carry the fault -- no desk, no
// glasses, no capture, no ribbon, no applications -- and it is meant for a
// machine nobody minds losing: a GitHub macOS runner or a throwaway VM.
//
// ⛔ THE GUARDS ARE IN THE TOOL RATHER THAN IN GOOD INTENTIONS, because the
// freeze happened to somebody who had written the warning down and read it the
// same day.
//
//   - it refuses to run without -i-can-freeze-this-machine;
//   - rounds and displays are capped;
//   - displays are closed and waited for before the next round;
//   - and it STOPS BY ITSELF when creation slows down. A bench getting slower
//     is not a bench being patient: the window server saying "after five or six
//     displays this Mac refuses the next one for the best part of a minute" is
//     the measurement, and it means stop.
//
// The answer it gives is a rate, which is what the question needs: one run
// proves nothing about a fault that appears two times in three.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/go-macos/virtualdisplay"
	"github.com/go-xrkit/desk"
)

func main() { os.Exit(run()) }

// Caps, so a typo cannot become the loop that froze a machine.
const (
	maxRounds   = 50
	maxDisplays = 8
	// slowdownFactor is how much longer than the FIRST round's creation a later
	// one may take before this gives up. The first round is the baseline
	// because it is the only one measured on a window server this bench has not
	// touched yet.
	slowdownFactor = 4
	// removalBudget is how long macOS is given to stop listing a display that
	// has been closed. Spelled here rather than taken from desk.RemovalBudget,
	// which is darwin-only: this command has to COMPILE on every platform the
	// fleet cross-builds, even though it can only run on one.
	removalBudget = 8 * time.Second
)

func run() int {
	rounds := flag.Int("rounds", 10, "how many times to make and destroy the displays")
	displays := flag.Int("displays", 5, "how many virtual displays per round; the fault depends on this number")
	width := flag.Int("width", 1920, "how wide each virtual display is asked to be; particular widths are REFUSED and the refusal is what this exists to test")
	height := flag.Int("height", 1080, "how tall")
	settle := flag.Duration("settle", 2*time.Second, "how long to leave them up before looking at the witnesses")
	allowed := flag.Bool("i-can-freeze-this-machine", false,
		"yes, this is a runner or a throwaway VM and nobody is using it")
	flag.Parse()

	if !*allowed {
		fmt.Fprintln(os.Stderr, "witnessbench makes and destroys virtual displays in a loop, "+
			"which has frozen a Mac somebody was working on. Pass -i-can-freeze-this-machine "+
			"if this is a runner or a throwaway VM.")
		return 2
	}
	if *rounds < 1 || *rounds > maxRounds {
		fmt.Fprintf(os.Stderr, "rounds must be 1 to %d\n", maxRounds)
		return 2
	}
	if *displays < 1 || *displays > maxDisplays {
		fmt.Fprintf(os.Stderr, "displays must be 1 to %d\n", maxDisplays)
		return 2
	}

	// ⛔ SAID BEFORE ANYTHING IS CREATED. A machine without the private API
	// reports every round clean, which is indistinguishable from a machine
	// where the fault did not happen -- and that is the reading that would
	// waste a week.
	if err := virtualdisplay.Available(); err != nil {
		fmt.Printf("this machine cannot make virtual displays: %v\n", err)
		fmt.Println("RESULT unavailable")
		return 3
	}
	if bad := desk.SelfCheck(); bad != "" {
		fmt.Println(bad)
		fmt.Println("RESULT instrument-broken")
		return 3
	}
	fmt.Printf("%d witnesses per round, %d displays, %v to settle, %d rounds\n",
		desk.Witnesses, *displays, *settle, *rounds)

	hits, done, refusals := 0, 0, 0
	var baseline time.Duration
	for r := 1; r <= *rounds; r++ {
		hit, refused, took, err := oneRound(*displays, *width, *height, *settle)
		if err != nil {
			fmt.Printf("round %d: %v\n", r, err)
			break
		}
		done++
		if hit > 0 {
			hits++
		}
		refusals += refused
		fmt.Printf("round %d: %d of %d witnesses damaged, %d of %d displays REFUSED at %dx%d, %v\n",
			r, hit, desk.Witnesses, refused, *displays, *width, *height, took.Round(time.Millisecond))

		if baseline == 0 {
			baseline = took
		} else if took > baseline*slowdownFactor {
			fmt.Printf("stopping: making them took %v against %v in the first round. "+
				"A bench getting slower is the window server saying it has had enough.\n",
				took.Round(time.Millisecond), baseline.Round(time.Millisecond))
			break
		}
	}

	fmt.Printf("RESULT %d of %d rounds damaged a witness, at %d displays of %dx%d, %d refusals in all\n",
		hits, done, *displays, *width, *height, refusals)
	if done == 0 {
		return 3
	}
	return 0
}

// oneRound allocates witnesses, makes the displays, waits, and looks.
//
// The witnesses are allocated HERE rather than once for the whole bench, so
// each round asks the same question the desk asks: are the strings this process
// just put on the heap still what they were a moment later.
func oneRound(n, w, h int, settle time.Duration) (hit, refused int, took time.Duration, err error) {
	ws := desk.NewWitnesses()

	started := time.Now()
	open := make([]*virtualdisplay.Display, 0, n)
	// ⛔ CLOSED WHATEVER HAPPENS. A round that fails half way through must not
	// leave displays behind: the next round would then measure a machine this
	// bench has already damaged, and a person who walks up to it finds a Mac
	// that believes it has monitors nobody can see.
	defer func() {
		ids := make([]uint32, 0, len(open))
		for _, d := range open {
			ids = append(ids, d.ID())
			_ = d.Close()
		}
		_ = virtualdisplay.WaitGone(removalBudget, ids...)
	}()

	for i := range n {
		d, e := virtualdisplay.Open(virtualdisplay.Spec{
			Name:   fmt.Sprintf("witnessbench %d", i+1),
			Width:  uint32(w),
			Height: uint32(h),
		})
		if e != nil {
			// ⛔⛔ A REFUSAL IS NOT A BROKEN ROUND, IT IS THE CONDITION UNDER
			// TEST. Particular widths are poison on the author's machine --
			// 3840, 4096 and 7680 refused at every height, 3839, 3841 and 4095
			// opening in half a second -- so the desk ASKS, IS REFUSED, and
			// asks again one pixel over. The runner opened 1920x1200 first try
			// and therefore never walked that path at all, which makes "what
			// does the private API do with the pointers it was handed when it
			// then fails" a question nothing has asked yet.
			//
			// So the round carries on and still looks at the witnesses. Giving
			// up here would have thrown away the very measurement.
			refused++
			continue
		}
		open = append(open, d)
	}
	took = time.Since(started)

	time.Sleep(settle)

	damaged := desk.DamagedWitnesses(ws)
	if len(damaged) > 0 {
		fmt.Println(desk.WitnessReport(damaged, len(ws)))
	}
	return len(damaged), refused, took, nil
}
