// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"fmt"
	"strings"
)

// DescribeWidths says what was actually made, in the words a person reading a
// journal needs.
//
// ⚠ NOT describeDisplays, which lives in await.go and answers a different
// question -- which displays are ATTACHED. Two names a majuscule apart, for two
// meanings, is a trap for whoever reads one and believes they have read the
// other.
//
// ⛔ THE WIDTHS ACTUALLY USED, NOT THE ONE PLANNED. Screens no longer all share
// a width -- a desk can be one wide screen for a spreadsheet, or a ribbon of
// panel-sized ones -- and a sentence naming a single width would be one the
// display list contradicts. It used to say "5 virtual displays of 1920x1080"
// with one number, which was true only while every screen was the same.
//
// ⚠ IDENTICAL WIDTHS STILL READ AS ONE PHRASE. The common case is a ribbon of
// equals, and "1920x1080, 1920x1080, 1920x1080, 1920x1080, 1920x1080" is a
// worse sentence than the one it replaced.
func DescribeWidths(widths []int, h int) string {
	if len(widths) == 0 {
		return "no virtual displays"
	}
	noun := "virtual displays"
	if len(widths) == 1 {
		noun = "virtual display"
	}

	// Runs of equal widths, in the order they were made: a desk with one wide
	// screen and four ordinary ones reads as exactly that.
	var b strings.Builder
	fmt.Fprintf(&b, "%d %s of ", len(widths), noun)
	for i := 0; i < len(widths); {
		j := i
		for j+1 < len(widths) && widths[j+1] == widths[i] {
			j++
		}
		if i > 0 {
			if j == len(widths)-1 {
				b.WriteString(" and ")
			} else {
				b.WriteString(", ")
			}
		}
		if n := j - i + 1; n > 1 && len(widths) > n {
			fmt.Fprintf(&b, "%d of %dx%d", n, widths[i], h)
		} else {
			fmt.Fprintf(&b, "%dx%d", widths[i], h)
		}
		i = j + 1
	}
	return b.String()
}

// widestBeyondTheView is the widest screen on the band that does not fit the
// view, and whether there is one.
//
// ⛔⛔ IT EXISTS BECAUSE "WIDER THAN THE VIEW" IS THE CONDITION UNDER WHICH
// TURNING THE HEAD STOPS BEING A COMFORT. The band does not scale a source down
// -- one source pixel per panel pixel is the whole point -- so a screen wider
// than [Plan.ScreenW] keeps the remainder of itself out of the view until the
// head moves. A 6400-pixel screen in a 1920 view is 70% unreachable.
//
// ⚠ AND IT IS A FUNCTION RATHER THAN A LINE IN run_display BECAUSE THAT FILE IS
// EXEMPT FROM THE COVERAGE GATE: it needs a window server. A condition that
// decides whether a person is told how to reach most of their screen should not
// live where nothing measures it.
func widestBeyondTheView(p Plan) (width int, wider bool) {
	for i := range p.Count() {
		if w := p.ScreenWidth(i); w > p.ScreenW && w > width {
			width, wider = w, true
		}
	}
	return width, wider
}

// ComfortableYawDeg is how far a seated person turns their head to either side
// without it becoming a movement they notice making.
//
// ⚠ IT IS A HUMAN RANGE, NOT A MEASUREMENT OF THIS SYSTEM, and saying so is the
// point: every other number in this package is derived from the optics, and this
// one is not. Forty degrees is the middle of what the ergonomics literature
// calls comfortable sustained head rotation; sixty is reachable and not
// sustainable. Somebody who disagrees should change it here, where it is named,
// rather than discover it as a feeling.
const ComfortableYawDeg = 40.0

// HeadYawFor is how far the head must turn, to either side, to bring the far end
// of a screen that many views across into the middle of the view -- and whether
// that is beyond [ComfortableYawDeg].
//
// ⛔⛔ A WIDE SCREEN HAS A CEILING THE OPTICS DO NOT SET. At one source pixel per
// panel pixel a screen that is `views` wide subtends views*fovDeg, so reaching
// its end means turning half that, less the half-view already in front of you.
// Reported from inside the glasses, of a screen 6400 across: "l'ecran de 6400
// est trop grand pour le voir d'un bout a l'autre en tournant la tete".
//
//	screen            views   arc    head needed
//	1920                1.00    52°            0°
//	3840                2.00   103°           26°
//	5120                2.67   138°           43°
//	6400                3.33   172°           60°
//	8640                4.50   232°           90°
//
// ⭐ AND AMPLIFYING THE HEAD IS NOT THE ANSWER, which is worth writing down
// because it is the first idea anybody has. Desk.headToBand exists to make the
// picture hold still while the head moves -- "a tracked head dragged the desk
// with it instead of leaving it where it was" -- and that was a defect somebody
// reported and somebody fixed. Making the band move faster than the head would
// reintroduce it deliberately. So the ceiling is told to the person, not
// engineered around.
func HeadYawFor(views, fovDeg float64) (deg float64, beyondComfort bool) {
	if views <= 0 || fovDeg <= 0 {
		return 0, false
	}
	deg = (views*fovDeg - fovDeg) / 2
	if deg < 0 {
		deg = 0
	}
	return deg, deg > ComfortableYawDeg
}

// WidthWithinReach is the widest screen whose far end a comfortable head turn
// brings into the middle of the view, in pixels, on a band of this shape.
//
// It is HeadYawFor solved for the width, so the two cannot drift apart: a screen
// this wide needs exactly [ComfortableYawDeg], and a wider one needs more.
func WidthWithinReach(panelW int, fovDeg float64) int {
	if panelW <= 0 || fovDeg <= 0 {
		return 0
	}
	return int(float64(panelW) * (2*ComfortableYawDeg + fovDeg) / fovDeg)
}
