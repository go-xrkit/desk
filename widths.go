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
