// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import "github.com/go-xrkit/xrkit/ribbon"

// HeadSource is where a head's yaw comes from, and the seam that keeps a camera
// out of this file.
//
// ⛔ ok IS NOT "THE HEAD DID NOT MOVE". It means this could not see whether it
// did -- a dark room, a blank wall, a scene that changed entirely. The two are
// the same number and opposite facts, and treating a blind moment as stillness
// is what makes a desk drift silently instead of saying it is lost.
type HeadSource interface {
	// Yaw is radians since the source was last recentred, positive to the
	// right, and whether the latest frame could be used at all.
	Yaw() (float64, bool)
	// Blind is how many frames in a row could not be used. One or two is a
	// blur; a steady count is a room with nothing to see in it.
	Blind() int
	// Recenter makes the current view the source's origin.
	Recenter()
}

// blindEnoughToSaySo is how many refused frames in a row it takes before the
// desk says the tracking is lost.
//
// ⭐ IT IS A RUN, NOT A TOTAL. At about 25 frames a second this is a fifth of a
// second of not being able to see, which is longer than a blink past a blank
// wall and far shorter than somebody would sit wondering why nothing moves.
const blindEnoughToSaySo = 5

// headTracking is the desk's side of a head tracker: where the yaw was when it
// was switched on, and what it has been told since.
type headTracking struct {
	src HeadSource
	// base is the nav yaw that the source's zero corresponds to. The source
	// measures from wherever it started; the ribbon has its own longitudes.
	base float64
	// on is whether the viewer asked for this at all.
	on bool
	// lost is whether the desk is currently telling them it cannot see.
	lost bool
	// close puts away whatever is behind src, when there is anything.
	close func() error
}

// FollowHead turns head tracking on or off.
//
// ⭐ TURNING IT ON ANCHORS HERE. The source counts from its own origin, so the
// yaw the viewer is already at becomes the offset everything is measured from --
// switching on does not move the desk, which is the only behaviour that would
// not startle somebody wearing it.
func (d *Desk) FollowHead(on bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.head.on = on
	d.head.lost = false
	if on && d.head.src != nil {
		d.head.src.Recenter()
		d.head.base = d.nav.Yaw()
	}
}

// FollowingHead reports whether head tracking is on.
func (d *Desk) FollowingHead() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.head.on
}

// SetHeadSource gives the desk somewhere to read a head's yaw from. A nil
// source turns the feature off rather than leaving it on with nothing behind it.
func (d *Desk) SetHeadSource(src HeadSource) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.head.src = src
	if src == nil {
		d.head.on, d.head.lost = false, false
	}
}

// followHead moves the view to wherever the head is now. The caller holds the
// lock.
//
// ⛔ IT SETS THE YAW AND LEAVES THE FOCUS ALONE. Where somebody is looking and
// which screen they last chose are different facts: turning the head to glance
// at a neighbour must not silently make it the screen that new windows open on.
// [ribbon.Ribbon.Nearest] reconciles them where that is wanted.
func (d *Desk) followHead() {
	if !d.head.on || d.head.src == nil || d.nav.Mode() == ribbon.ModeGallery {
		return
	}
	yaw, ok := d.head.src.Yaw()
	if !ok {
		// ⛔ THE VIEW STAYS WHERE IT WAS. Not moving is the only honest thing to
		// do with no information, and it is also what looks right: a picture
		// that held still while somebody moved reads as a lost tracker, while
		// one that snapped home reads as a broken desk.
		if !d.head.lost && d.head.src.Blind() >= blindEnoughToSaySo {
			d.head.lost = true
			// ⛔ notice.say, NOT d.say: that one takes the lock this already
			// holds, and the deadlock would be a desk that stops the moment
			// somebody turns the light off.
			d.notice.say("Too dark to follow your head")
		}
		return
	}
	if d.head.lost {
		d.head.lost = false
		// ⭐ AND THE ORIGIN IS RETAKEN. Nobody knows how far the head turned
		// while this could not see, so carrying on from the old base would
		// apply an unknown error as though it were motion. Here becomes zero.
		d.head.src.Recenter()
		d.head.base = d.nav.Yaw()
		return
	}
	d.nav.SetYaw(d.head.base + yaw)
}

// toggleFollowHead turns head tracking on, opening the camera the first time,
// and off again.
//
// ⛔ OUTSIDE THE LOCK, like every handler that talks to hardware. Opening a
// camera takes long enough to stall the frame loop, and a light coming on while
// nothing can be drawn is the worst moment for it.
//
// ⭐ THE CAMERA IS KEPT ONCE OPENED, and closed when the feature goes off. A
// tracker reopened on every toggle would relearn the room each time, and the
// light would flicker with the menu.
func (d *Desk) toggleFollowHead() {
	if d.FollowingHead() {
		d.FollowHead(false)
		d.closeHead()
		d.say("no longer following your head")
		return
	}
	if err := d.openHead(); err != nil {
		// ⛔ SAY WHY, rather than leaving a menu item that does nothing. The
		// commonest reason is a headset plugged in for its picture only, which
		// presents no camera at all.
		d.say(err.Error())
		return
	}
	d.FollowHead(true)
	d.say("following your head, with the camera light on while it does")
}

// openCameraHead is the seam: the real one opens a camera, which a test cannot
// arrange and must not need to.
//
// ⛔ THE COVERAGE GATE IS WHAT ASKED FOR IT, and it was right to. Three
// functions here were reachable only with a headset plugged in, which means the
// menu row nobody could test was also the menu row nobody could be sure of.
var openCameraHead = func(display string) (HeadSource, func() error, error) {
	h, err := OpenCameraHead(display)
	if err != nil {
		return nil, nil, err
	}
	return h, h.Close, nil
}

// openHead opens the headset camera if it is not already open.
func (d *Desk) openHead() error {
	d.mu.Lock()
	already := d.head.src != nil
	display := d.plan.Model
	d.mu.Unlock()
	if already {
		return nil
	}
	h, closer, err := openCameraHead(display)
	if err != nil {
		return err
	}
	d.mu.Lock()
	d.head.src, d.head.close = h, closer
	d.mu.Unlock()
	return nil
}

// closeHead puts the camera away and turns its light off.
func (d *Desk) closeHead() {
	d.mu.Lock()
	closer := d.head.close
	d.head.src, d.head.close = nil, nil
	d.mu.Unlock()
	if closer != nil {
		_ = closer()
	}
}
