// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ⛔⛔ AN EMPTY DISPLAY LIST IS NOT AN UNPLUGGED DISPLAY.
//
// A machine running a window server has at least one screen: somebody is
// looking at something. So a list with nothing in it is not the news that every
// display went away, it is the news that THE LIST COULD NOT BE READ -- and the
// two arrive as the same value, because go-widgets/window's darwin backend
// answers "no displays" with (nil, nil).
//
// Measured, twice on 2026-09-08, on a desk somebody was wearing:
//
//	desk: "VITURE Beast" is not attached any more; there is  -- stopping,
//	and putting back everything this changed
//
// Note what follows "there is": nothing. The message names every attached
// display, and it named none, which is the whole story in one blank. The desk
// then put the arrangement back and quit, and a person lost their screens --
// the second time that day, and the unexplained closure reported earlier ("l'app
// s'est fermée ou a crashé") has exactly this shape: an orderly exit, code 0, no
// crash report, nothing in the log but a sentence about an unplugged display.
//
// ⭐ SO AN UNREADABLE ANSWER IS ITS OWN OUTCOME, distinct from both "here it is"
// and "it is gone". This is the same mistake as reading a damaged name as an
// unplugged headset (damagedname.go) and the same shape as a blind frame read as
// stillness (headtracking.go): three states, not two, and collapsing them always
// loses the one nobody wanted.
var errDisplaysUnreadable = errors.New(
	"desk: the display list came back EMPTY, which is not an answer -- a machine " +
		"with a window server has at least one screen, so this is a failed read " +
		"and not an unplugged display. Carrying on with the display we had")

// lookUpScreen finds name among the attached displays, and says what its
// absence means.
//
// It returns the index of the screen to use and, when that index came from
// anything but an exact match, why. An error with no usable index comes back
// as -1, so a caller cannot take a screen it was not given.
func lookUpScreen(name string, attached []string) (int, error) {
	for i, at := range attached {
		if at == name {
			return i, nil
		}
	}
	// ⛔ THE EMPTY CASE IS CHECKED BEFORE THE ABSENT CASE, because every name is
	// absent from an empty list -- so testing "is it there?" first would report
	// an unplugged display every time the read failed, which is exactly the bug.
	if len(attached) == 0 {
		return -1, errDisplaysUnreadable
	}
	// ⛔ A NAME WITH A NUL IN IT IS A BUG, NOT AN UNPLUGGED DISPLAY. See
	// damagedname.go: a Go string was seen losing its first four bytes four
	// times in one day, and looking a display up by that name ended the session
	// while the display was sitting in this very list.
	if i, ok := undamage(name, attached); ok {
		return i, errDamagedName{want: name, got: attached[i]}
	}
	quoted := make([]string, len(attached))
	for i, at := range attached {
		quoted[i] = strconv.Quote(at)
	}
	return -1, fmt.Errorf("desk: %q is not attached any more; there is %s",
		name, strings.Join(quoted, ", "))
}
