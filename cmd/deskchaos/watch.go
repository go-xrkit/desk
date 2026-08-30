// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-macos/pointer"
	"github.com/go-widgets/window"
)

// watcher looks at the machine WHILE a session runs.
//
// Everything else this bench does asks what a session left behind, and a
// session that misbehaves for ten seconds and tidies up on the way out leaves
// nothing behind at all. The mouse that could not be found was exactly that:
// nothing wrong with the end of the session, everything wrong with the middle.
type watcher struct {
	mu    sync.Mutex
	found map[string]bool
	stop  chan struct{}
	done  chan struct{}
}

// watch starts looking. headset is the display the desk is drawing ON: the one
// place the pointer must never be, because a pointer there is a pointer on a
// picture of this program.
// alive is asked before anything is recorded: once the session is gone, nothing
// about the machine is its doing. Measured, on a round that killed one: the
// desk's screens go with it, macOS moves the pointer to a display that is left,
// and the display that is left is the stand-in headset. That is the window
// server tidying up, not a desk misbehaving.
func watch(headset string, alive func() bool) *watcher {
	w := &watcher{found: map[string]bool{}, stop: make(chan struct{}), done: make(chan struct{})}
	go func() {
		defer close(w.done)
		for {
			select {
			case <-w.stop:
				return
			case <-time.After(250 * time.Millisecond):
			}
			if !alive() {
				return
			}
			w.look(headset)
		}
	}()
	return w
}

// look is one glance at the machine.
func (w *watcher) look(headset string) {
	ss, err := window.Screens()
	if err != nil {
		return
	}
	var on *window.Screen
	for i := range ss {
		if ss[i].Name == headset {
			on = &ss[i]
		}
	}
	if on == nil {
		// Unplugged, or not there yet. Nothing to say.
		return
	}
	at, err := pointer.Position()
	if err != nil {
		return
	}
	x0, y0 := float64(on.X), float64(on.Y)
	if at.X >= x0 && at.X < x0+float64(on.Width) && at.Y >= y0 && at.Y < y0+float64(on.Height) {
		w.say(fmt.Sprintf("the pointer was on %q, the screen the desk is drawing on", headset))
	}
}

func (w *watcher) say(s string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.found[s] = true
}

// close stops looking and reports what was seen, once each.
func (w *watcher) close() []string {
	close(w.stop)
	<-w.done
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]string, 0, len(w.found))
	for s := range w.found {
		out = append(out, s)
	}
	return out
}
