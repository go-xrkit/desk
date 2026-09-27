// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

// Command narrowwindow runs the 77 milliseconds the corruption happens in, over
// and over, and says what got damaged.
//
// ⛔⛔ THE JOURNAL NAMED THE WINDOW AND IT IS SMALL. On 2026-09-27 an ordinary
// launch read three strings correctly from a clean settings file and found them
// with their first four bytes zeroed 77 ms later:
//
//	15:22:00.650  journal opened
//	15:22:00.669  permissions
//	15:22:00.713  the menu bar item is built, with 15 rows
//	15:22:00.724  the menu draws 17 key equivalents from 40 granted shortcuts
//	15:22:00.727  no display matches "\x00\x00\x00\x00RE Beast"
//
// Between the clean read and the damaged use there is nothing but the permission
// check and the menu bar. No virtual display had been made -- which retires the
// framing the hunt ran on for three weeks, that the fault depends on how many
// virtual displays exist.
//
// ⭐⭐ AND THIS IS THE FIRST BENCH THAT IS SAFE TO LOOP ON A WORKING MACHINE.
// Every earlier one made virtual displays, which froze the author's Mac twice.
// This one makes a menu bar item and reads a file. Nothing else. So it can run
// two hundred times in a minute on the machine where the fault actually appears,
// which is the one thing this hunt has never been able to do.
//
// It watches BOTH: the witnesses, which say what shape of object is hit, and the
// strings from the settings file, which are the objects actually known to have
// been hit.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-macos/hotkey"
	"github.com/go-xrkit/desk"
)

func main() { os.Exit(run()) }

const maxRounds = 2000

func run() int {
	rounds := flag.Int("rounds", 200, "how many times to walk the window")
	keys := flag.Bool("shortcuts", false, "claim the 40 system-wide shortcuts each round and draw their key equivalents; this is the half the journal points at, and it briefly takes global keys while somebody may be typing")
	byPosition := flag.Bool("by-position", false, "claim keys by POSITION instead of by legend, which skips the keyboard-layout lookup; the author types on AZERTY and the runner that never reproduced this is QWERTY")
	noReserved := flag.Bool("no-reserved", false, "skip the system-shortcut check, which reads the machine's reserved set through CoreFoundation")
	draw := flag.Bool("draw", true, "also draw the granted keys into the menu; -draw=false claims them and stops there, which is the bisection")
	verbose := flag.Bool("v", false, "say what each round did, so a bench that walks an EMPTY corridor can be told from one that walks the window")
	tray := flag.Bool("tray", true, "build the menu bar item too; -tray=false leaves only the file read")
	settle := flag.Duration("settle", 0, "wait this long inside each round before looking")
	flag.Parse()

	if *rounds < 1 || *rounds > maxRounds {
		fmt.Fprintf(os.Stderr, "rounds must be 1 to %d\n", maxRounds)
		return 2
	}
	// ⛔ NO -i-can-freeze-this-machine HERE, and that is the point rather than an
	// omission: this bench creates no display, claims no key and holds nothing.
	// The guard belongs on the bench that can freeze a machine, and putting it
	// on one that cannot would teach the next reader that the flag means
	// nothing.
	if bad := desk.SelfCheck(); bad != "" {
		fmt.Println(bad)
		fmt.Println("RESULT instrument-broken")
		return 3
	}

	fmt.Printf("%d witnesses, %d rounds, tray=%t\n", desk.Witnesses, *rounds, *tray)

	hits, confHits, done := 0, 0, 0
	for r := 1; r <= *rounds; r++ {
		ws := desk.NewWitnesses()

		conf, err := desk.LoadConfig()
		if err != nil {
			fmt.Printf("round %d: reading the settings: %v\n", r, err)
			break
		}
		// What the file said, read back the moment it was parsed. The victims
		// were exactly these.
		want := stringsOf(conf)

		if *tray {
			actions := make(chan desk.Action, desk.TrayQueue)
			say := func(string, ...any) {}
			if *verbose {
				say = func(f string, a ...any) { fmt.Printf("  "+f+"\n", a...) }
			}
			t, err := desk.OpenTray(say, actions)
			if err != nil {
				fmt.Printf("round %d: the menu bar: %v\n", r, err)
				break
			}
			if *keys {
				// ⛔ THE HALF THE BENCH WAS MISSING. Without this the menu is
				// built and NO key equivalents are drawn -- the run that
				// carried the fault logged "the menu draws 17 key equivalents
				// from 40 granted shortcuts" three milliseconds before the
				// damaged string, and a bench without it walks an empty
				// corridor and reports the window clean.
				opts := conf.HotkeyOptions()
				if *noReserved {
					opts.Reserved = hotkey.NoReserved{}
				}
				if *byPosition {
					opts.OnThisKeyboard = false
				}
				hk := desk.ClaimGlobal(desk.DefaultShortcuts(), opts)
				if *draw {
					t.ShowShortcuts(hk.Granted())
				}
				if *verbose && r == 1 {
					// ⭐ WHAT THIS MACHINE ACTUALLY GRANTED, once. Whether the
					// halves of a gesture share a prefix is a question about the
					// keys something else holds HERE, and no test can answer it:
					// the fake register in the suite grants what it is told to.
					fmt.Print(hk.Describe())
				}
				_ = hk.Close()
			}
			if *settle > 0 {
				time.Sleep(*settle)
			}
			_ = t.Close()
		} else if *settle > 0 {
			time.Sleep(*settle)
		}
		done++

		// ⛔ THE SETTINGS STRINGS ARE RE-READ FROM THE SAME Config, not parsed
		// again: parsing again would make a fresh copy and measure nothing.
		if got := stringsOf(conf); got != want {
			confHits++
			fmt.Printf("⛔ round %d: a string from desk.hcl CHANGED under the program:\n  was %q\n  now %q\n",
				r, want, got)
		}
		if hit := desk.DamagedWitnesses(ws); len(hit) > 0 {
			hits++
			fmt.Printf("round %d: %s\n", r, desk.WitnessReport(hit, len(ws)))
		}
	}

	fmt.Printf("RESULT %d of %d rounds damaged a witness, %d damaged a settings string\n",
		hits, done, confHits)
	if done == 0 {
		return 3
	}
	return 0
}

// stringsOf is every string the settings file put on the heap, joined so one
// comparison covers the lot.
//
// ⚠ JOINED RATHER THAN COMPARED FIELD BY FIELD because the join is what gets
// printed: a person reading the log needs to see WHICH string lost its head,
// and "was X now Y" beside each other is the shortest way to show it.
func stringsOf(c desk.Config) string {
	var b strings.Builder
	// ⛔ DEREFERENCED, not printed as a pointer: %q on a *string gives an
	// address, which never changes when the text under it does -- a watch that
	// could not see the one victim it was written for.
	if c.Glasses != nil && c.Glasses.Model != nil {
		fmt.Fprintf(&b, "glasses=%q ", *c.Glasses.Model)
	}
	for _, p := range c.Places {
		fmt.Fprintf(&b, "place=%q ", p.App)
	}
	for _, s := range c.Shortcuts {
		fmt.Fprintf(&b, "%s=%q ", s.Action, s.Keys)
	}
	return b.String()
}
