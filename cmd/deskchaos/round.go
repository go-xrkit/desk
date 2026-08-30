// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/go-macos/brightness"
	"github.com/go-macos/pointer"
	"github.com/go-macos/virtualdisplay"
	"github.com/go-widgets/window"
)

// round runs one session under one fault and reports what was left behind.
func round(bin, headset string, f fault, rng *rand.Rand) []string {
	s, err := start(bin, headset, rng)
	if err != nil {
		return []string{fmt.Sprintf("the session would not start: %v", err)}
	}
	defer s.cleanup()

	f.do(s, rng)
	s.wait()
	return s.leftBehind()
}

// start makes a headset, remembers what the machine looked like, and runs a
// desk at it.
func start(bin, headset string, rng *rand.Rand) (*session, error) {
	s := &session{name: headset, wasLit: map[uint32]float64{}}

	// What every panel was lit at, BEFORE anything is asked to darken one.
	ids, err := pointer.Displays()
	if err != nil {
		return nil, fmt.Errorf("listing displays: %w", err)
	}
	for _, id := range ids {
		if b, err := brightness.Of(id); err == nil {
			s.wasLit[id] = b
		}
	}
	s.pointer, _ = pointer.Position()

	d, err := virtualdisplay.Open(virtualdisplay.Spec{
		Name: headset, Width: 1920, Height: 1080,
	})
	if err != nil {
		return nil, fmt.Errorf("standing in a %q: %w", headset, err)
	}
	s.stand = d
	settleArrangement()

	s.logPath = fmt.Sprintf("%s/deskchaos-%d.log", os.TempDir(), time.Now().UnixNano())
	s.log, err = os.Create(s.logPath)
	if err != nil {
		return nil, err
	}
	// A bounded session: whatever the fault does, this one ends.
	secs := 20 + rng.Intn(15)
	s.cmd = exec.Command(bin, "-screen", headset, "-for", fmt.Sprintf("%ds", secs))
	s.cmd.Stdout, s.cmd.Stderr = s.log, s.log
	if err := s.cmd.Start(); err != nil {
		return nil, err
	}
	// Up and drawing before anything is done to it.
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(s.read(), "framebuffer ") {
			return s, nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return s, nil
}

// settle waits a little, at random, so a fault does not always land at the same
// moment of a session.
func (s *session) settle(rng *rand.Rand) {
	time.Sleep(time.Duration(2000+rng.Intn(6000)) * time.Millisecond)
}

// unplugGlasses takes the headset away while the desk is showing on it.
func (s *session) unplugGlasses() {
	if s.stand != nil {
		_ = s.stand.Close()
		s.stand = nil
	}
}

// plugAnother puts a second headset on the machine mid-session.
func (s *session) plugAnother() {
	d, err := virtualdisplay.Open(virtualdisplay.Spec{
		Name: s.name, Width: 1920, Height: 1080,
	})
	if err == nil {
		s.extra = d
	}
}

// kill stops the process the way a crash does: no deferred anything.
func (s *session) kill() {
	if s.cmd.Process != nil {
		_ = s.cmd.Process.Signal(syscall.SIGKILL)
		s.killed = true
	}
}

func (s *session) wait() { _ = s.cmd.Wait() }

func (s *session) read() string {
	b, err := os.ReadFile(s.logPath)
	if err != nil {
		return ""
	}
	return string(b)
}

// cleanup puts the machine back whatever happened, including after a fault that
// was supposed to leave something behind.
func (s *session) cleanup() {
	if s.stand != nil {
		_ = s.stand.Close()
	}
	if s.extra != nil {
		_ = s.extra.Close()
	}
	if s.log != nil {
		_ = s.log.Close()
	}
	for id, was := range s.wasLit {
		if b, err := brightness.Of(id); err == nil && b < was-0.01 {
			_ = brightness.Set(id, was)
		}
	}
	settleArrangement()
}

// settleArrangement gives the window server time to finish rearranging.
func settleArrangement() { time.Sleep(900 * time.Millisecond) }

// leftBehind is the whole point: what is still true of this machine that should
// not be.
func (s *session) leftBehind() []string {
	settleArrangement()
	var found []string
	log := s.read()

	// A crash is a defect however it ends.
	if strings.Contains(log, "panic:") {
		found = append(found, "the session panicked:\n"+firstLines(log[strings.Index(log, "panic:"):], 3))
	}

	// Every panel back where it was. A killed session is expected to fail this
	// -- a process that is gone runs nothing -- and it is reported anyway,
	// because it is the reason a person is left in the dark.
	//
	// The tolerance is wide on purpose. A Mac changes its own backlight while
	// nobody asks: measured, 0.70 before a session and 0.68 after one that
	// never touched it, and 0.78 a minute later. What a person notices is a
	// screen left DARK, not two hundredths.
	for id, was := range s.wasLit {
		b, err := brightness.Of(id)
		if err != nil {
			continue
		}
		if b < was-driftAllowed {
			found = append(found, fmt.Sprintf(
				"display %d was left at %.2f, from %.2f", id, b, was))
		}
	}

	// No display outlives the process that made it -- but the window server
	// takes them away in its own time, and looking too early reports a defect
	// that is not there. Measured: three of them still listed the moment a
	// session ended, and all gone a second later.
	if left := waitForDisplaysToGo(); len(left) > 0 {
		for _, name := range left {
			found = append(found, fmt.Sprintf("%q is still attached", name))
		}
	}

	// The desk's own screen is never a screen of its own band.
	if line := showsItsOwnScreen(log, s.name); line != "" {
		found = append(found, "the band was showing the desk's own screen: "+line)
	}
	return found
}

// showsItsOwnScreen finds a "screen n: ..." line naming the display the desk is
// running on.
func showsItsOwnScreen(log, headset string) string {
	for _, l := range strings.Split(log, "\n") {
		t := strings.TrimSpace(l)
		// A "screen n: ..." line is the band saying what it draws. The lines
		// that merely NAME the headset -- what is attached, what was chosen --
		// start with something else.
		if strings.HasPrefix(t, "screen ") && strings.Contains(t, headset) {
			return t
		}
	}
	return ""
}

func firstLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return "      " + strings.Join(lines, "\n      ")
}

// driftAllowed is how much a backlight may differ from what it was before a
// session before it counts as left dark. See leftBehind.
const driftAllowed = 0.10

// waitForDisplaysToGo gives the window server time to take away the displays a
// session made, and reports the ones that are still there when it is out.
func waitForDisplaysToGo() []string {
	deadline := time.Now().Add(8 * time.Second)
	for {
		ss, err := window.Screens()
		if err != nil {
			return nil
		}
		var left []string
		for _, sc := range ss {
			if strings.HasPrefix(sc.Name, madePrefix) {
				left = append(left, sc.Name)
			}
		}
		if len(left) == 0 || time.Now().After(deadline) {
			return left
		}
		time.Sleep(400 * time.Millisecond)
	}
}

// madePrefix is what the desk calls the displays it makes. It is written down
// here because this bench is OUTSIDE the desk: it reads a machine, not a
// package, and a machine only knows the names.
const madePrefix = "XR desk "
