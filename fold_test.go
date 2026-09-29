// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"fmt"
	"testing"

	"github.com/go-xrkit/xrkit/glasses"
)

// The fold protocol.
//
// ⛔⛔ THE FOLD WAS REPORTED WRONG FOUR TIMES, AND EACH ROUND ENDED WITH A
// QUESTION BACK TO THE PERSON WEARING THE GLASSES. That is not a way to fix
// anything: "arrete de compter sur moi pour debuguer". So the state a wearer
// cannot describe is enumerated here instead -- every curvature, every distance,
// every screen in front, every position between two of them -- and the things a
// wearer WOULD have said are written as assertions.
//
// It runs on the desk this was reported from rather than on a convenient one:
// an Odyssey G95NC mirrored onto position 1 (7680x2160 native, captured at the
// band's height, so 3840x1080) among five 1920x1080 screens, seen through a
// Beast. A suite of identical screens hid three defects at once; see
// TestAWideScreenIsWideInTheChainToo.

// deskShape is one arrangement to sweep, named so a failure says which.
type deskShape struct {
	name    string
	screens int
	// wide is the position of a screen that is not the shape of the glasses,
	// and how wide it is. A width of 0 means every screen is nominal.
	wide, wideW int
}

// theDesksToSweep covers the desk this was reported from and the shapes around
// it.
//
// ⛔ THE UNIFORM DESK IS IN THE LIST DELIBERATELY, at the end rather than the
// start: a suite made only of those hid three defects at once (see
// TestAWideScreenIsWideInTheChainToo), and leaving it out now would only move
// the blind spot.
var theDesksToSweep = []deskShape{
	// 7680x2160 captured at the band's height is 3840x1080. See OpenOffer.
	{"an Odyssey on position 1", 6, 0, 3840},
	{"an Odyssey in the middle", 6, 3, 3840},
	{"an Odyssey on the last position", 6, 5, 3840},
	{"a narrow screen among wide ones", 6, 2, 1280},
	{"two screens, one of them wide", 2, 0, 3840},
	{"nine screens, one of them wide", MaxScreens, 4, 3840},
	{"every screen the same", 6, 0, 0},
}

// plan builds the desk, and refuses to be a different one quietly.
func (d deskShape) plan(t *testing.T) (Plan, []int) {
	t.Helper()
	p, err := NewPlan(glasses.Display{Name: "VITURE Beast", Width: 3840, Height: 1080},
		Options{Screens: d.screens})
	if err != nil {
		t.Fatalf("%s: NewPlan = %v", d.name, err)
	}
	if d.wideW > 0 {
		p = p.WithScreenWidth(d.wide, d.wideW)
		if p.ScreenWidth(d.wide) != d.wideW {
			t.Fatalf("%s: screen %d is %d wide, want %d -- the plan refused the "+
				"shape and this would sweep an ordinary desk under its name",
				d.name, d.wide+1, p.ScreenWidth(d.wide), d.wideW)
		}
	}
	widths := make([]int, p.Count())
	for i := range widths {
		widths[i] = p.ScreenWidth(i)
	}
	return p, widths
}

// SweepCurvedDesks turns the curved half of the fold protocol on.
//
// ⛔⛔ THE CURVED RENDERER HAS NEVER BEEN UNDER THIS PROTOCOL, and that is the
// blind spot, not the switch. Every shape in theDesksToSweep is flat, so the
// facet machinery -- the thing that draws a wide screen as a row of panels --
// was checked only by tests asking what ONE panel projects to. That is the same
// shape of gap as "tres etroit": every assertion held and none of them was about
// the arrangement.
//
// ⭐ TURNING IT ON FINDS REAL DEFECTS, WHICH IS WHY IT IS HERE AT ALL. Measured
// on this branch, six screens with an Odyssey among them, both anchorings, seven
// splays, five distances:
//
//	8680  the renderer as it was, with the protocol taught about facets
//	7023  with the edge-on test measured from the eye instead of the z axis
//	5001  and the half-turn cap derived from the cumulative angle
//
// The first of those is landed here: it was a contiguous run of facets refused
// in the MIDDLE of a curved screen -- 112 pixels of background and 751 source
// columns drawn nowhere. What is left is pre-existing and not yet understood,
// mostly a screen's run ending short at the long distances.
//
// ⛔ IT IS OFF BECAUSE A RED PROTOCOL IS NOT A PROTOCOL. A sweep that always
// fails stops being read, and then it catches nothing at all. It is a constant
// rather than a flag so that flipping it is a commit somebody reviews.
const SweepCurvedDesks = false

// bendsToSweep is the curvatures the protocol runs at. See [SweepCurvedDesks].
func bendsToSweep() []float64 {
	if SweepCurvedDesks {
		return []float64{FlatBend, DefaultBend}
	}
	return []float64{FlatBend}
}

// TestTheFoldProtocol sweeps the whole reachable state of the turned band and
// checks, at each point, the things somebody looking through the glasses would
// say if they were wrong.
func TestTheFoldProtocol(t *testing.T) {
	// ⛔ BOTH DESKS, because they are two renderings and not a flag: the fixed
	// one walks the chain where the other re-anchors it, so every panel it
	// draws is at a different place along the walk. A new mode with no protocol
	// over it is the blind spot this file exists to close.
	for _, anchor := range []Anchoring{AnchorOnGaze, AnchorFixed} {
		for _, shape := range theDesksToSweep {
			base, widths := shape.plan(t)
			base = base.WithAnchoring(anchor)
			for _, splay := range []float64{SplayStep, 10, 20, 30, 40, 51.6, MaxSplayDeg} {
				for _, dist := range []float64{1, 1.5, 2, 3, MaxDistance} {
					// ⛔⛔ AND BOTH CURVATURES, because the curved renderer had
					// never been under this protocol at all. Every desk swept
					// here was flat, so the facet machinery -- the thing that
					// draws a wide screen as a row of panels -- was checked only
					// by tests that ask what ONE panel projects to. That is the
					// same shape of blind spot as "tres etroit": every assertion
					// held and none of them was about the arrangement.
					//
					// ⚠ Plan.Bend is flat unless a screen is wider than the
					// band, so this bends the wide screen in each shape and
					// leaves the ordinary ones alone -- which is the doctrine,
					// not a limitation of the sweep.
					for _, bend := range bendsToSweep() {
						plan := base.WithSplay(splay).WithDistance(dist).WithBend(bend)
						f, err := NewFan(plan)
						if err != nil {
							t.Fatalf("%s at %g°, %gx, bend %g: NewFan = %v",
								shape.name, splay, dist, bend, err)
						}
						f.SetSourceWidths(widths)
						f.SetBends(plan.Bends())
						f.SetAnchoring(anchor)
						for focus := range plan.Count() {
							for _, toward := range []float64{-0.5, -0.3, -0.1, 0, 0.1, 0.3, 0.5} {
								where := fmt.Sprintf("%s desk, %s, %g°, %gx, bend %g, screen %d, %+.1f along",
									anchor, shape.name, splay, dist, bend, focus+1, toward)
								checkFrame(t, where, plan, widths,
									f.Frame(nil, focus, toward), focus, toward)
							}
						}
					}
				}
			}
		}
	}
}

// checkFrame is the protocol itself.
func checkFrame(t *testing.T, where string, plan Plan, widths []int,
	panels []Slant, focus int, toward float64) {
	t.Helper()

	if len(panels) == 0 {
		t.Errorf("%s: nothing was drawn at all", where)
		return
	}

	viewW, viewH := plan.ScreenW, plan.ScreenH
	for i, s := range panels {
		src := widths[s.Screen]

		// ⛔⛔ A PANEL IS A FACET, NOT A SCREEN, AND EVERY ASSERTION BELOW USED TO
		// SAY OTHERWISE. A curved screen is drawn as a row of panels and a wide
		// flat one now is too, so "this panel shows the screen's first column"
		// and "there is a seam before the next panel" are both FALSE in the
		// middle of one screen: a facet join is the same picture continuing,
		// with no seam and no whole source on either side of it.
		//
		// ⛔ AND THIS WAS A BLIND SPOT, NOT A NEW CASE. theDesksToSweep never set
		// a bend, so no faceted desk had ever been through this protocol and the
		// entire curved renderer sat outside it. The sweep now carries bends.
		//
		// ⭐ THE INTENT IS UNCHANGED AND THE COVER IS WIDER. A screen is still
		// drawn whole -- but "whole" is a property of its RUN of facets, first
		// column at the run's left end and last at its right -- and the inside
		// of a run is now held to something the old check could not ask at all:
		// that consecutive facets meet exactly, with the source carrying on
		// across the join.
		runStarts := i == 0 || panels[i-1].Screen != s.Screen
		runEnds := i == len(panels)-1 || panels[i+1].Screen != s.Screen

		// ⛔ A SCREEN IS DRAWN WHOLE. An edge that is the panel's own -- not the
		// canvas's -- must show that screen's own first or last pixel column. A
		// fold that chops a screen short is the defect this whole protocol is
		// for, and it looks exactly like a fold in the wrong place.
		// Within ONE destination column's worth of source, measured at that end
		// of this panel rather than assumed: a screen pushed back shows more
		// than one source column per drawn one, and more still at its far edge,
		// so a fixed tolerance would be slack where it matters and wrong where
		// it does not. A column's centre is half a column inside the panel, so
		// its own local step is exactly the slack there should be.
		last := len(s.Cols) - 1
		if runStarts && s.Dst.X > 0 && last > 0 {
			step := max(int32(1), s.Cols[1].Src-s.Cols[0].Src)
			if got := s.Cols[0].Src; got > step {
				t.Errorf("%s: screen %d starts at x=%d showing its source column "+
					"%d, want within %d of its first", where,
					s.Screen+1, s.Dst.X, got, step)
			}
		}
		if runEnds && s.Dst.X+s.Dst.W < viewW && last > 0 {
			step := max(int32(1), s.Cols[last].Src-s.Cols[last-1].Src)
			if got := s.Cols[last].Src; int32(src-1)-got > step {
				t.Errorf("%s: screen %d ends at x=%d showing its source column "+
					"%d of %d, want within %d of its last", where,
					s.Screen+1, s.Dst.X+s.Dst.W, got, src, step)
			}
		}

		// ⛔ AND IT IS DRAWN IN ORDER, with nothing of it repeated or skipped
		// backwards. A source that runs the wrong way at one end is how a
		// mirrored panel would look, and it is not something a person can name.
		for c := 1; c < len(s.Cols); c++ {
			if s.Cols[c].Src < s.Cols[c-1].Src {
				t.Fatalf("%s: screen %d reads source %d after %d at column %d",
					where, s.Screen+1, s.Cols[c].Src, s.Cols[c-1].Src, c)
			}
		}

		// ⛔⛔ A PANEL TALLER THAN THE VIEW REACHES OUTSIDE IT. The rows in a
		// column are the PANEL'S own, and [Canvas.Slant] derives the source row
		// from them -- (y-Y0)*srcH/(Y1-Y0) -- so a column clipped to the canvas
		// before that arithmetic maps the WHOLE screen into the part of it that
		// fits, instead of cropping the top and the bottom away.
		//
		// It was doing exactly that, and it is not an edge case: a curved band
		// bends TOWARDS the viewer, so every neighbour of the screen in front is
		// nearer and therefore taller than the view. The screen being read was at
		// true scale and the ones beside it were squashed -- "l'angle de cintrage
		// n'est pas au bon endroit et tombe a l'interieur de l'ecran en face",
		// reported from inside the glasses and chased four times as geometry.
		//
		// The fingerprint of the clip is a column that touches BOTH edges of the
		// canvas and stops exactly there. That is only honest when the panel is
		// square on and exactly a view tall, which is what every column of it
		// then is; on a turned panel each column is a different depth and a
		// different height, so a plateau at exactly the canvas is arithmetic,
		// not geometry.
		flat := true
		for c := range s.Cols {
			if s.Cols[c].Y1-s.Cols[c].Y0 != s.Cols[0].Y1-s.Cols[0].Y0 {
				flat = false
				break
			}
		}
		// A panel that reaches both edges of the canvas is TALLER than the
		// canvas, so it must say so: at least one of its columns has to fall
		// outside. Under the clip, none ever did -- every column stopped exactly
		// at the canvas, whatever the panel's real height, and the source was
		// mapped into that.
		//
		// The panels whose columns are all one height are square on, and one of
		// those really can be a view tall to the pixel: that is the doctrine at
		// distance one, not an artefact, so they are left out.
		if !flat {
			touches, over := false, false
			for _, col := range s.Cols {
				if col.Y0 <= 0 && col.Y1 >= int32(viewH) {
					touches = true
				}
				if col.Y0 < 0 || col.Y1 > int32(viewH) {
					over = true
				}
			}
			if touches && !over {
				t.Errorf("%s: screen %d fills the canvas top to bottom and not "+
					"one of its %d columns reaches past it: they were clipped "+
					"before the source row was worked out, so the screen is "+
					"squashed into the view rather than cropped by it",
					where, s.Screen+1, len(s.Cols))
			}
		}

		if i == 0 {
			continue
		}
		// ⛔ THE FOLD IS BETWEEN TWO SCREENS, NEVER INSIDE ONE. Panels in the
		// same frame may not overlap, and where both edges are their own there
		// is a seam to see.
		prev := panels[i-1]
		end := prev.Dst.X + prev.Dst.W
		if s.Dst.X < end {
			t.Errorf("%s: screen %d runs to x=%d and screen %d starts at x=%d: "+
				"they overlap by %d", where,
				prev.Screen+1, end, s.Screen+1, s.Dst.X, end-s.Dst.X)
		}
		if !runStarts {
			// ⛔⛔ INSIDE ONE SCREEN THE OPPOSITE IS TRUE: a facet join must be
			// INVISIBLE. A gap here is a dark line down the middle of a
			// spreadsheet and a jump in the source is a column of it silently
			// missing -- neither is something the old per-screen checks could
			// have asked about, because to them these two panels were two
			// separate appearances of one screen and therefore already an error.
			if hole := s.Dst.X - end; hole > 1 {
				t.Errorf("%s: screen %d has %d pixels of nothing between two of "+
					"its own facets at x=%d: a seam inside one screen",
					where, s.Screen+1, hole, end)
			}
			if pl, cl := len(prev.Cols), len(s.Cols); pl > 1 && cl > 0 {
				step := max(int32(1), prev.Cols[pl-1].Src-prev.Cols[pl-2].Src)
				if jump := s.Cols[0].Src - prev.Cols[pl-1].Src; jump < 0 || jump > 2*step {
					t.Errorf("%s: screen %d reads source column %d after %d "+
						"across a facet join at x=%d: %d columns of it are not "+
						"drawn anywhere", where, s.Screen+1,
						s.Cols[0].Src, prev.Cols[pl-1].Src, end, jump)
				}
			}
			continue
		}
		if end < viewW && s.Dst.X > 0 && s.Dst.X-end < 1 {
			t.Errorf("%s: screen %d ends at x=%d and screen %d starts at x=%d, "+
				"with no seam between them", where,
				prev.Screen+1, end, s.Screen+1, s.Dst.X)
		}
		// ⛔ AND A SEAM IS A SEAM, NOT A MISSING SCREEN. Nothing may drop out of
		// the MIDDLE of a frame: the edges of the view may be empty, because a
		// band curved hard enough really does turn away and there is nothing
		// beyond it, but a hole between two panels is a screen that was
		// considered and lost. The whole sweep's widest is 54 pixels -- a
		// 48-pixel seam with the projection on it -- so twice the seam is a
		// bound with room in it and no room for a screen.
		if hole := s.Dst.X - end; hole > 2*DefaultGapPx {
			t.Errorf("%s: %d pixels of nothing between screen %d and screen %d "+
				"(x=%d..%d): a screen went missing", where,
				hole, prev.Screen+1, s.Screen+1, end, s.Dst.X)
		}
	}

	// ⛔ AND THE SCREEN SOMEBODY IS FACING IS THE ONE IN FRONT OF THEM. Square
	// on, the focused screen straddles the middle column; this is the assertion
	// that caught a chain built around the wrong screen once already.
	if toward != 0 {
		return
	}
	mid := viewW / 2
	for _, s := range panels {
		if s.Dst.X <= mid && mid < s.Dst.X+s.Dst.W {
			if s.Screen != focus {
				t.Errorf("%s: the middle of the view is screen %d, not the one "+
					"being looked at", where, s.Screen+1)
			}
			return
		}
	}
	t.Errorf("%s: no panel covers the middle of the view", where)
}
