// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"sync"

	"github.com/go-macos/multitouch"
)

// watchTouches is where trackpad frames come from, as a seam. Above the
// platform boundary so the mapping below can be driven by a test on any
// machine, with no trackpad and no hand.
var watchTouches = multitouch.Watch

// Gestures turns three-finger swipes on the trackpad into ribbon actions.
//
// It exists because the keyboard is the only way to turn the ribbon, and a
// person wearing the glasses has a trackpad under their hand. The mouse is
// deliberately NOT that way in: the pointer used to change screens and it kept
// getting lost, which is why it no longer does. A GESTURE is not the pointer --
// it moves nothing on screen and cannot wander onto a display nobody is
// looking at.
//
// It asks for no permission. macOS delivers a three-finger swipe as a private
// event the Dock consumes first, and only when the person has bound the gesture
// to switching spaces; reading the contacts underneath depends on neither. See
// go-macos/multitouch.
type Gestures struct {
	ch   chan Action
	w    *multitouch.Watcher
	once sync.Once
	// why is what stopped this from working, for a caller that wants to say so
	// rather than leave a feature silently absent.
	why error
}

// ClaimGestures starts watching the trackpad. It never fails: a machine with no
// trackpad, or one where the framework will not load, yields a Gestures whose
// channel simply stays empty -- the desk is driven by the keyboard, and the
// gesture is an addition to it.
func ClaimGestures() *Gestures {
	g := &Gestures{ch: make(chan Action, 4)}
	var swiper multitouch.Swiper
	var mu sync.Mutex
	w, err := watchTouches(func(f multitouch.Frame) {
		// The recogniser is not safe for two devices at once, and a Mac can
		// have two trackpads: one lock, one gesture at a time.
		mu.Lock()
		sw, ok := swiper.Feed(f)
		mu.Unlock()
		if !ok {
			return
		}
		if a, ok := actionForSwipe(sw); ok {
			// DROPPED rather than queued when the reader is behind, for the
			// same reason a held-down key is: a queue of turns keeps turning
			// the ribbon long after the hand has stopped.
			select {
			case g.ch <- a:
			default:
			}
		}
	})
	if err != nil {
		g.why = err
		return g
	}
	g.w = w
	return g
}

// actionForSwipe says which way the ribbon should turn, if at all.
//
// Fingers moving LEFT bring the next screen in from the right, which is what
// the same gesture does to spaces on this platform: the hand pushes the content
// aside rather than dragging the ribbon. Getting this backwards is not a bug a
// person reports, it is one they blame themselves for, so it follows the
// system's convention rather than an opinion.
//
// A vertical swipe is not a ribbon turn. The gallery is above and the
// applications are below on the keyboard, and guessing at those from a gesture
// would take an action nobody asked for.
func actionForSwipe(sw multitouch.Swipe) (Action, bool) {
	if !sw.Horizontal() {
		return ActionNone, false
	}
	if sw.Right() {
		return ActionPrev, true
	}
	return ActionNext, true
}

// C is the actions the trackpad produces.
func (g *Gestures) C() <-chan Action { return g.ch }

// Why says what stopped the trackpad being read, or nil when it is being read.
func (g *Gestures) Why() error { return g.why }

// Close stops watching. It is safe to call twice.
func (g *Gestures) Close() error {
	var err error
	g.once.Do(func() {
		if g.w != nil {
			err = g.w.Close()
		}
		close(g.ch)
	})
	return err
}
