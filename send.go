// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-macos/accessibility"
)

// ErrNoSuchApp says nothing on this machine answers to that name.
var ErrNoSuchApp = errors.New("desk: no such application")

// A Bench is what putting an application on a screen needs of the platform.
//
// It is an interface because the whole of the policy below — which windows an
// application has, which screen a position is, what to do when the application
// is not running or the screen is not there — is worth testing on a machine
// with no windows on it at all, which is every machine a test runs on.
type Bench interface {
	// Displays lists what the window server is driving, virtual ones included.
	Displays() ([]accessibility.Display, error)
	// Windows lists the windows of every application whose name contains app,
	// case-insensitively. A person types "safari", not "Safari".
	Windows(app string) ([]accessibility.Window, []string, error)
	// Trusted reports whether this process may move another one's windows.
	Trusted() bool
}

// Placement is one line of a desk: an application, and the position on the
// ribbon its windows belong on.
type Placement struct {
	App string
	Pos int
}

// Send puts an application's windows on a ribbon position.
//
// The position is an index into ids, which is the desk's own screens in the
// order the plan made them — so position 1 in a settings file is screen 1 on
// the band, and a person never has to know what a CGDirectDisplayID is.
//
// It returns what it moved and what it could not, rather than the first error:
// a desk of six applications where one is not running should place the other
// five and say which one it did not.
func Send(b Bench, ids []uint64, places []Placement) ([]string, error) {
	if len(places) == 0 {
		return nil, nil
	}
	if !b.Trusted() {
		return nil, fmt.Errorf("desk: this application may not move another one's "+
			"windows: grant it Accessibility in System Settings > Privacy & Security "+
			"> Accessibility, then run it again (%w)", accessibility.ErrNotTrusted)
	}
	displays, err := b.Displays()
	if err != nil {
		return nil, fmt.Errorf("desk: listing displays: %w", err)
	}
	var done []string
	var problems []error
	for _, p := range places {
		if p.Pos < 1 || p.Pos > len(ids) {
			problems = append(problems, fmt.Errorf("%q: there is no screen %d; this desk has %d",
				p.App, p.Pos, len(ids)))
			continue
		}
		to, ok := accessibility.DisplayByID(displays, uint32(ids[p.Pos-1]))
		if !ok {
			problems = append(problems, fmt.Errorf("%q: screen %d is not one the window "+
				"server is driving", p.App, p.Pos))
			continue
		}
		ws, names, err := b.Windows(p.App)
		if err != nil {
			problems = append(problems, fmt.Errorf("%q: %w", p.App, err))
			continue
		}
		if len(ws) == 0 {
			problems = append(problems, fmt.Errorf("%q: %w, or it has no window open",
				p.App, ErrNoSuchApp))
			continue
		}
		for i, w := range ws {
			// Fill, not Relative: a ribbon screen is a whole desktop and an
			// application sent to one is meant to have it. Relative would scale
			// the window by the ratio of the two displays, which from a 7680
			// wide monitor to a 1920 wide panel is a window a quarter the size
			// on a screen that is entirely free.
			res, err := accessibility.MoveToDisplay(w, to, displays,
				&accessibility.Options{Placement: accessibility.Fill, Tolerance: furniture})
			if err != nil {
				problems = append(problems, fmt.Errorf("%q (%s): %w", p.App, names[i], err))
				continue
			}
			done = append(done, fmt.Sprintf("%s -> screen %d, %s", names[i], p.Pos, res.Got))
		}
	}
	return done, errors.Join(problems...)
}

// matches reports whether an application's name is the one a person typed.
//
// A prefix or a fragment, case-insensitively: somebody writing a settings file
// types "safari", not "Safari", and "Visual Studio Code" is reachable as
// "code" without anyone having to know it is not called that.
func matches(name, want string) bool {
	return strings.Contains(strings.ToLower(name), strings.ToLower(strings.TrimSpace(want)))
}

// furniture is how far a window may land from where it was told before the
// move counts as refused.
//
// Not a fudge factor, and not "near enough". macOS reserves the top of a
// display for the menu bar and clamps a window out of it, so a window told to
// fill a 1920x1200 panel lands at 1920x1169, thirty-one points down — measured
// on a VITURE Beast, where it looked exactly like a failure and was not one.
//
// Sixty-four points is a menu bar at any scale this has been seen at, and it is
// SMALLER THAN HALF A SCREEN. That is what makes it safe rather than lax: a
// window whose rectangle is within sixty-four points of a screen's is centred
// on that screen and cannot be centred on a neighbour, so the allowance can
// forgive furniture without ever forgiving a window that went to the wrong
// place.
const furniture = 64
