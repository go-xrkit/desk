// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"math"
	"strconv"

	"github.com/go-xrkit/xrkit/ribbon"
)

// Sight is what a head tracker could see of the frame it has just read.
//
// ⛔⛔ THREE STATES, AND THE TYPE CARRIES ALL THREE. This used to be a bool
// beside the yaw, with the third state in a SEPARATE METHOD -- Blind() int --
// that a caller had to remember to consult and compare against a constant. It
// had exactly one caller, which did it correctly; nothing made it. And the
// platform with no camera had to RETURN THE CONSTANT from Blind() to say "lost"
// at all, which is a value invented to encode a state the type would not hold.
//
// It is the same collapse that cost two sessions on 2026-09-08 in two other
// places: an empty display list read as a Mac with no screens
// (displaylist.go), and a name damaged in memory read as an unplugged headset
// (damagedname.go). Found, absent, and could-not-tell are three answers.
type Sight int

const (
	// SightSeen: the frame was usable, and the yaw beside it means something.
	SightSeen Sight = iota
	// SightBlinked: this frame could not be used, and that is not news -- a
	// blur, a hand across the lens, one dark frame in a lit room. The yaw
	// means nothing and nothing is wrong.
	//
	// ⛔ IT IS NOT STILLNESS. "The head did not move" and "I could not see
	// whether it moved" are the same number and opposite facts, and treating
	// the second as the first is what makes a desk drift silently instead of
	// saying it is lost.
	SightBlinked
	// SightLost: enough frames in a row could not be used to say so out loud.
	SightLost
)

// String names a Sight, for a log line and for a test failure that has to be
// readable without counting iota.
func (s Sight) String() string {
	switch s {
	case SightSeen:
		return "seen"
	case SightBlinked:
		return "blinked"
	case SightLost:
		return "lost"
	}
	return "Sight(" + strconv.Itoa(int(s)) + ")"
}

// HeadSource is where a head's yaw comes from, and the seam that keeps a camera
// out of this file.
type HeadSource interface {
	// Yaw is radians since the source was last recentred, positive to the
	// right, and what the source could see of the frame it read.
	Yaw() (float64, Sight)
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

// sightOf is the one place that decides how long a blink lasts, from whether
// the latest frame was usable and how many in a row were not.
//
// ⭐ ONE IMPLEMENTATION, NOT ONE PER SOURCE. Handing every tracker the run
// length to apply for itself would be handing each of them a chance to disagree
// about what "lost" means, and a desk that says it is lost at different moments
// depending on which camera is plugged in is a desk nobody can describe.
func sightOf(used bool, blind int) Sight {
	switch {
	case used:
		return SightSeen
	case blind >= blindEnoughToSaySo:
		return SightLost
	}
	return SightBlinked
}

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
	// landing is set while a deliberate turn is in flight, so that its arrival
	// can move the head's origin to meet it.
	landing bool
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

// RecenterHead brings the screen the viewer is facing to the middle of the
// glasses, and makes that the head tracker's origin.
//
// ⛔⛔ THE FIRST VERSION ONLY MOVED THE ORIGIN, AND THAT IS NOT WHAT ANYBODY
// MEANS BY RECENTRING. It set the tracker's zero to wherever the view had
// drifted to and changed NOTHING on screen -- so somebody whose screen had
// wandered half out of sight pressed "put it back in front of me", watched it
// stay exactly where it was, and reported the feature broken. They were right:
// a row that says it puts the picture back has to move the picture.
//
// ⭐ THE SCREEN IT CHOOSES IS THE NEAREST ONE, NOT THE FOCUSED ONE. Those are
// different facts -- glancing at a neighbour does not make it the screen new
// windows open on -- and the one somebody wants in front of them is the one
// they are facing, which is what [ribbon.Ribbon.Nearest] answers.
//
// ⭐ AND IT IS WHAT BOUNDS THE DRIFT. Error accumulates only while the view is
// moving; a person who recentres when they notice it never lets it run further
// than one journey.
func (d *Desk) RecenterHead() {
	d.mu.Lock()
	defer d.mu.Unlock()
	// ⛔ A GUARD MOVED IS A GUARD LOST. The previous version returned early when
	// there was no head source and so never reached the navigator; this one
	// touches the navigator FIRST, and crashed on a desk built without one. The
	// key can be pressed at any moment, including before there is anything to
	// recentre.
	if d.nav == nil {
		return
	}
	// ⛔ THE PICTURE MOVES WHETHER OR NOT A HEAD IS BEING FOLLOWED. This is the
	// menu row and the key everybody has; tying it to a camera nobody switched
	// on would make it do nothing on the desks that need it most.
	// ⛔ THE ERROR IS DROPPED BECAUSE IT CANNOT HAPPEN, and a coverage gate is
	// what proved it: GoTo refuses only an index outside the ribbon, Nearest
	// returns one inside it by construction, and New refuses a desk with no
	// screens at all. Handling it would be a branch no test could ever reach --
	// caution with nothing to be cautious about.
	at := d.nav.Ribbon().Nearest(d.nav.Yaw())
	_ = d.nav.GoTo(at)
	if d.head.src == nil {
		return
	}
	d.head.src.Recenter()
	// ⭐ THE ORIGIN GOES WHERE THE TURN WILL LAND, not where the view is now:
	// GoTo sets a target the easing has yet to reach, and anchoring to the
	// current yaw would make the head fight the arrival.
	d.head.base = d.nav.Target()
	d.head.landing = false
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
	// ⛔⛔ A DELIBERATE TURN IS NOT FOUGHT. Next, Prev and GoTo set a target and
	// let Advance ease towards it; writing the yaw every frame makes yaw and
	// target equal, so the easing never runs and the keyboard appears dead --
	// measured: the focus moved from 0 to 1 and the view did not move at all,
	// which leaves an ACTIVE SCREEN NOBODY IS LOOKING AT and new windows opening
	// on it. So the head stands aside while a turn is in flight.
	if d.nav.Moving() {
		d.head.landing = true
		return
	}
	if d.head.landing {
		// ⭐ AND THE ORIGIN MOVES WITH IT. The turn has arrived somewhere the
		// head did not take it, so carrying on from the old offset would drag
		// the view straight back. Here becomes zero, and the head continues
		// from wherever the keyboard left off.
		d.head.landing = false
		d.head.src.Recenter()
		d.head.base = d.nav.Yaw()
		return
	}
	yaw, sight := d.head.src.Yaw()
	if sight != SightSeen {
		// ⛔ THE VIEW STAYS WHERE IT WAS. Not moving is the only honest thing to
		// do with no information, and it is also what looks right: a picture
		// that held still while somebody moved reads as a lost tracker, while
		// one that snapped home reads as a broken desk.
		if sight == SightLost && !d.head.lost {
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
	d.nav.SetYaw(d.head.base + yaw*d.headToBand())
}

// headToBand converts a real head rotation into ribbon yaw.
//
// ⛔⛔ THEY ARE NOT THE SAME UNIT, AND THIS USED TO ADD RADIANS OF ONE TO THE
// OTHER. [HeadSource.Yaw] is "radians since the source was last recentred" -- a
// real angle, measured off a camera. The ribbon's yaw is a COORDINATE ON A
// BAND: a full turn is the whole desk, so with six screens one screen is sixty
// degrees of it whatever the picture looks like. The turned band puts one
// screen at about a field of view, because one screen fills the view at
// distance one. Measured, one screen, chain against ribbon:
//
//	screens  splay  chain    ribbon  the desk moves
//	4        20°    49.16°   90.0°   1.83x too slowly
//	6        20°    49.16°   60.0°   1.22x too slowly
//	9        20°    49.16°   40.0°   0.81x too fast
//
// So a tracked head dragged the desk with it instead of leaving it where it
// was, by a factor that depends on how many screens somebody has -- and the
// fold they turned to look at was not where their head said it would be. The
// old code corrected the ORIGIN of the two spaces (see [headTracking.base]) and
// never the SCALE.
//
// ⛔ THE STEP IS THE ONE IN FRONT, not an average. On a desk with a screen of
// another shape on it the step to the left and the step to the right are
// different lengths, so a single ratio for the whole band would be wrong on
// both sides of a mismatched screen. See [Strip.Toward], which walks the same
// steps for the same reason.
//
// The flat band is left at 1: [Strip] SLIDES a band of pixels in front of a
// fixed window rather than turning the viewer, so "the desk stayed where it
// was" is not a thing that arrangement can do at all, and a scale factor would
// be a correction towards nothing. See TestAFanOfNothingDrawsWhatTheStripDraws.
func (d *Desk) headToBand() float64 {
	if d.fan == nil || d.strip.n < 2 {
		return 1
	}
	focus := d.nav.Focus()
	next := (focus + 1) % d.strip.n
	// The step in ribbon radians: a full turn is the whole band.
	band := 2 * math.Pi * float64(d.strip.short(d.strip.centre[next]-d.strip.centre[focus])) /
		float64(d.strip.total)
	// And the same step as the chain actually turns the viewer. It is never
	// zero: [NewFan] refuses a splay of nothing and a screen of no size, so
	// panel 1's centre is strictly to the right of panel 0's, and there is no
	// division here to guard against.
	x0, z0 := d.fan.centre(focus, 0)
	x1, z1 := d.fan.centre(focus, 1)
	return band / (math.Atan2(x1, z1) - math.Atan2(x0, z0))
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
	// ⛔⛔ WHAT HAPPENED, NOT WHAT WAS ASKED, and it is told on EVERY path --
	// including the one that refuses. Asking to follow a head opens a camera,
	// and a headset plugged in for its picture only presents none; a tick that
	// moved because a row was clicked would then say the desk is following a
	// head it cannot see. Same rule as OnTracking, which learned it from a
	// headset that refuses whenever it is passing the picture through.
	defer func() {
		if on := d.OnFollowingHead; on != nil {
			on(d.FollowingHead())
		}
	}()
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

// curveBy turns every screen a little further towards the viewer, or a little
// flatter. The caller holds the lock.
//
// ⭐ A STEPPER AT ITS STOP DOES NOTHING, QUIETLY. Nobody expects a volume key to
// report anything at maximum; they expect it to stop. [Plan.WithSplay] clamps to
// 0..[MaxSplayDeg], so a press past either end asks for the shape the desk is
// already in -- and rebuilding the band, the gallery and the fan to arrive back
// where it started is work nobody asked for, once per press, held.
func (d *Desk) curveBy(deg float64) {
	want := d.plan.WithSplay(d.plan.SplayDeg() + deg)
	if want.SplayDeg() == d.plan.SplayDeg() {
		return
	}
	d.err = d.reshape(want)
}

// anchorAs changes what the chain does when the gaze moves, and says so. The
// caller holds the lock.
//
// ⭐ ASKING FOR THE DESK IT IS ALREADY IN DOES NOTHING, QUIETLY, like curveBy
// beside it: rebuilding the band, the gallery and the fan to arrive back where
// it started is work nobody asked for. It still SAYS which desk this is,
// because the two rows are not on and off -- somebody who presses the one that
// is already in force is asking which one that is.
// ⛔ NO REBUILD. The band, the gallery and the ribbon are the same either way --
// what changes is one number the fan reads per frame -- so reshape would tear
// the navigator down and put it back to arrive exactly where it started. That
// is the twitch curveBy refuses to make, for the same reason.
//
// A nil fan is the flat band, where there is nothing to tell: the plan still
// records the choice, and build hands it over the moment a curvature makes one.
func (d *Desk) anchorAs(a Anchoring) {
	d.plan = d.plan.WithAnchoring(a)
	if d.fan != nil {
		d.fan.SetAnchoring(d.plan.Anchoring())
	}
	d.notice.say(a.said())
}
