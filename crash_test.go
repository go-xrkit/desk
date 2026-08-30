// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"strings"
	"sync"
	"testing"
)

func TestSafelyTurnsACrashIntoAStop(t *testing.T) {
	var said []string
	err := Safely(func(f string, a ...any) { said = append(said, f) }, func() {
		panic("slice bounds out of range [:6684] with capacity 6680")
	})
	if !errors.Is(err, ErrCrashed) {
		t.Fatalf("Safely = %v, want an ErrCrashed", err)
	}
	if !strings.Contains(err.Error(), "6684") {
		t.Errorf("the error is %q, want what it crashed with in it", err)
	}
	if len(said) != 1 || !strings.Contains(said[0], "crashed") {
		t.Errorf("it said %q, want one line saying the desk is stopping", said)
	}
}

func TestSafelyKeepsTheStack(t *testing.T) {
	var said string
	_ = Safely(func(f string, a ...any) {
		said = f
		if len(a) > 1 {
			if s, ok := a[1].(string); ok {
				said += s
			}
		}
	}, func() { panic("boom") })
	// A crash that is swallowed silently is worse than one that is loud: the
	// stack is what says WHERE, and this whole file exists because of one.
	if !strings.Contains(said, "%s") && !strings.Contains(said, "desk.") {
		t.Errorf("nothing in %q carries a stack", said)
	}
}

func TestSafelyLeavesAQuietRunAlone(t *testing.T) {
	ran := false
	if err := Safely(nil, func() { ran = true }); err != nil {
		t.Errorf("Safely = %v, want nil", err)
	}
	if !ran {
		t.Error("Safely did not run what it was given")
	}
	// And a crash with nowhere to report it is still an error rather than a
	// dead process: the logf is optional, the recovery is not.
	if err := Safely(nil, func() { panic("quietly") }); !errors.Is(err, ErrCrashed) {
		t.Errorf("Safely with no log = %v, want an ErrCrashed", err)
	}
}

// TestACrashInTheFrameLoopDoesNotTakeTheBacklightWithIt is the reason the whole
// thing exists, at the level a test can reach: the frame loop is another
// goroutine, and a panic there kills the PROCESS -- every deferred restore in
// the caller included. Wrapped, the goroutine ends, the waiter is released, and
// the caller's deferred restore runs.
func TestACrashInTheFrameLoopDoesNotTakeTheBacklightWithIt(t *testing.T) {
	var lit sync.WaitGroup
	lit.Add(1)
	restored := false

	done := make(chan error, 1)
	go func() {
		// Exactly the shape of the loop in Run: a deferred restore in the
		// caller, and the work wrapped.
		defer func() { restored = true; lit.Done() }()
		done <- Safely(nil, func() { panic("the fan read past the end of a row") })
	}()

	if err := <-done; !errors.Is(err, ErrCrashed) {
		t.Fatalf("the loop reported %v, want an ErrCrashed", err)
	}
	lit.Wait()
	if !restored {
		t.Error("the deferred restore did not run")
	}
}
