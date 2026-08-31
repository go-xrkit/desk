// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

// Command standin holds a virtual display open under a headset's name, so the
// desk can be exercised with no glasses on the desk.
//
// The desk chooses what to take over by matching a DISPLAY NAME against the
// catalogue in go-xrkit/xrkit/glasses. A virtual display called "XREAL 1S" at
// that model's size is therefore taken for the real thing, and everything after
// the choice runs unchanged: the plan, the virtual screens, the application
// placement, the ribbon, the render, and the clean stop.
//
// That is the whole point. Before this existed, every change to any of it
// needed somebody to plug a headset in and say what they saw -- which meant a
// person, a room and a pair of glasses for what a machine can check by itself.
//
// Proved end to end on 2026-08-31: five virtual screens made, three real
// applications placed on them, twenty-one seconds of frames, everything
// released, nothing left behind.
//
// Two things to know before running it.
//
// It holds the display until it is stopped or its time runs out, so give it a
// bound: an unattended stand-in keeps a machine believing it has a monitor
// nobody can see.
//
// And with no real headset the desk's first screen MIRRORS this Mac's own
// screen, which means it darkens that panel for as long as it runs. On a
// display that does not report a brightness nothing happens and the desk says
// so; on a laptop panel it really does go dark, and it is put back when the
// desk stops -- or by the next start, which is what the note in
// PutBackWhatWasLeftDark is for.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/go-macos/virtualdisplay"
	"github.com/go-xrkit/xrkit/glasses"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("standin", flag.ContinueOnError)
	fs.SetOutput(stderr)
	model := fs.String("model", "XREAL 1S", "the headset to stand in for; one of the catalogue's models")
	dur := fs.Duration("for", 2*time.Minute, "how long to hold it open")
	list := fs.Bool("models", false, "list the models the catalogue knows and stop")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *list {
		for _, m := range glasses.Models() {
			fmt.Fprintln(stdout, m)
		}
		return 0
	}

	// The size comes from the CATALOGUE, not from a number typed here: a
	// stand-in at the wrong size would exercise a plan the real glasses would
	// never produce, which is the one thing it must not do.
	p, ok := glasses.Identify(*model)
	if !ok || p.EyeWidth <= 0 || p.EyeHeight <= 0 {
		fmt.Fprintf(stderr, "standin: the catalogue has no panel size for %q\n", *model)
		fmt.Fprintf(stderr, "  the models it does know: %s\n", strings.Join(glasses.Models(), ", "))
		return 1
	}

	// The main thread, because that is where a program that owns a display
	// keeps it.
	runtime.LockOSThread()
	d, err := virtualdisplay.Open(virtualdisplay.Spec{
		Name:   p.Model,
		Width:  uint32(p.EyeWidth),
		Height: uint32(p.EyeHeight),
	})
	if err != nil {
		fmt.Fprintf(stderr, "standin: could not stand in for the glasses: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "standing in for %s as display %d, %dx%d, for %s\n",
		p.Model, d.ID(), p.EyeWidth, p.EyeHeight, *dur)
	fmt.Fprintf(stdout, "  the desk will take it for the real thing and start on it\n")
	fmt.Fprintf(stdout, "  stop it with ctrl-C, or wait\n")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	select {
	case <-stop:
	case <-time.After(*dur):
	}
	if err := d.Close(); err != nil {
		fmt.Fprintf(stderr, "standin: taking it away: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "the stand-in is gone")
	return 0
}
