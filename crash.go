// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"fmt"
	"runtime/debug"
)

// ErrCrashed wraps whatever a frame loop panicked with.
var ErrCrashed = errors.New("desk: the frame loop crashed")

// Safely runs fn and turns a panic into an error.
//
// It exists because of what this program HOLDS while it runs, and what a panic
// does to that. The desk darkens the Mac's own panel while the ribbon shows a
// copy of it, and it puts the backlight back from a deferred call in the
// caller. A panic in the frame loop is a panic in ANOTHER GOROUTINE: it kills
// the process without running that deferred call, and somebody is left looking
// at a black screen with no menu bar to quit from.
//
// That is not a hypothetical. It happened, with the glasses on:
//
//	panic: slice bounds out of range [:6684] with capacity 6680
//
// and the report that came back was "I unplugged the glasses because I had
// lost access and there was no icon in the tray for me to stop the
// application" -- the icon was there, on a panel that had been turned off.
//
// So a crash in the frame loop stops the desk the ordinary way instead: the run
// returns an error, every deferred restore runs, and the person gets their
// screen back. The stack is kept, because a crash that is swallowed silently is
// worse than one that is loud.
func Safely(logf func(string, ...any), fn func()) (err error) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		stack := string(debug.Stack())
		if logf != nil {
			logf("the frame loop crashed and the desk is stopping so that everything it changed goes back:\n%v\n%s", r, stack)
		}
		err = fmt.Errorf("%w: %v", ErrCrashed, r)
	}()
	fn()
	return nil
}
