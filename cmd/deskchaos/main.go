// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/go-macos/pointer"
	"github.com/go-macos/virtualdisplay"
	"github.com/go-xrkit/xrkit/glasses"
)

func main() { os.Exit(run()) }

func run() int {
	rounds := flag.Int("rounds", 5, "how many sessions to run")
	take := flag.Bool("take-the-machine", false,
		"yes, nobody is using this Mac: move its pointer, dim its screen and kill things on it")
	deskBin := flag.String("desk", "xrdesk", "the desk to run")
	seed := flag.Int64("seed", 0, "the seed to replay; 0 picks one and prints it")
	only := flag.String("fault", "", "run only the fault whose name contains this")
	flag.Parse()

	if !*take {
		fmt.Fprintln(os.Stderr, "deskchaos moves the pointer, turns a backlight off, "+
			"creates displays and kills processes.")
		fmt.Fprintln(os.Stderr, "Run it on a Mac nobody is working on, with -take-the-machine.")
		return 2
	}
	if *seed == 0 {
		*seed = time.Now().UnixNano()
	}
	fmt.Printf("seed %d — pass -seed %d to run these very sessions again\n", *seed, *seed)
	rng := rand.New(rand.NewSource(*seed))

	names := glasses.Names()
	if len(names) == 0 {
		fmt.Fprintln(os.Stderr, "the catalogue names no headset to stand in for")
		return 1
	}

	// Its own directory for the settings it writes: a person's own are never
	// read and never written.
	dir, err := os.MkdirTemp("", "deskchaos-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.RemoveAll(dir)

	bad, skipped := 0, 0
	for i := 1; i <= *rounds; i++ {
		f := pick(rng, *only)
		if f == nil {
			fmt.Fprintf(os.Stderr, "no fault matches %q; there are:\n", *only)
			for _, c := range faults {
				fmt.Fprintf(os.Stderr, "  %s\n", c.name)
			}
			return 2
		}
		name := names[rng.Intn(len(names))]
		setting, how, err := settingsFor(dir, rng)
		if err != nil {
			fmt.Fprintf(os.Stderr, "writing the settings for this round: %v\n", err)
			return 1
		}
		fmt.Printf("\n── round %d/%d — %s\n   a %q, %s\n", i, *rounds, f.name, name, how)
		waitForAQuietMachine()
		found, err := round(*deskBin, name, setting, *f, rng)
		if err != nil {
			// The bench could not set the round up. Not a defect, and not a
			// pass either: said, counted separately, and the run goes on.
			fmt.Printf("   — skipped: %v\n", err)
			skipped++
			continue
		}
		for _, s := range found {
			fmt.Printf("   ✗ %s\n", s)
		}
		if len(found) == 0 {
			fmt.Printf("   ✓ nothing left behind\n")
			continue
		}
		bad += len(found)
	}
	fmt.Printf("\n%d thing(s) left behind over %d rounds", bad, *rounds)
	if skipped > 0 {
		fmt.Printf(", %d of which the machine was too tired to run", skipped)
	}
	fmt.Println()
	if bad > 0 {
		return 1
	}
	return 0
}

// fault is something done to a session on purpose.
type fault struct {
	name string
	// do runs while the session is up. It returns when the session should be
	// expected to be finishing.
	do func(s *session, rng *rand.Rand)
}

var faults = []fault{
	{"a session left to finish on its own", func(s *session, rng *rand.Rand) {
		s.settle(rng)
	}},
	{"the glasses unplugged mid-session", func(s *session, rng *rand.Rand) {
		s.settle(rng)
		s.unplugGlasses()
		s.expectStopWithin = 8 * time.Second
	}},
	{"killed outright, the way a crash does", func(s *session, rng *rand.Rand) {
		s.settle(rng)
		s.kill()
	}},
	{"a second headset plugged in", func(s *session, rng *rand.Rand) {
		s.settle(rng)
		s.plugAnother()
	}},
}

// session is one run of the desk, with a made headset in front of it.
type session struct {
	cmd     *exec.Cmd
	log     *os.File
	logPath string

	bin     string
	stand   *virtualdisplay.Display
	eyes    *watcher
	setting string
	// expectStopWithin is how long the session may take to notice a fault and
	// stop. Zero means it is left to its own deadline.
	expectStopWithin time.Duration
	unpluggedAt      time.Time
	stopped          time.Time
	extra            *virtualdisplay.Display
	name             string
	killed           bool
	wasLit           map[uint32]float64
	pointer          pointer.Point
}

// pick chooses a fault, or the one a person asked for by name.
//
// By NAME rather than by number: a run that says which fault it used, and a
// person asking for that one again, should be saying the same thing.
func pick(rng *rand.Rand, want string) *fault {
	if strings.TrimSpace(want) == "" {
		return &faults[rng.Intn(len(faults))]
	}
	for i := range faults {
		if strings.Contains(faults[i].name, want) {
			return &faults[i]
		}
	}
	return nil
}
