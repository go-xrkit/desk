// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"fmt"
	"sort"
	"strings"

	"github.com/go-macos/accessibility"
)

// An App is a running application with at least one window, and where its
// windows are on this desk.
//
// It is a description, like [Offer]: it holds no handle on anything, so a
// gallery of applications can be built, shown and chosen from without opening a
// window of any kind — and tested on a machine with none.
// An Icon is an application's own icon, as straight RGBA, W*H*4 bytes.
//
// It is pixels rather than a path or a handle because this package has no
// operating system in it: whoever fills it in knows how to ask -- on macOS that
// is go-macos/appicon, which needs no permission -- and everything here just
// draws it.
type Icon struct {
	Pix  []byte
	W, H int
}

type App struct {
	// Name is the application's own name, as the window server reports it.
	Name string
	// PID is the process it runs in, from the first window seen of it. Two
	// processes with the same application name are one row in a gallery and one
	// of these -- which is what a person means by "Firefox" whether or not it
	// is two copies.
	PID int32
	// Icon is the application's own icon, or nil when nobody looked it up or
	// the system would not give it. A gallery falls back to a drawn glyph.
	Icon *Icon
	// Windows is how many windows it has.
	Windows int
	// On is the ribbon positions its windows are on, ascending and without
	// repeats. Empty means every window of it is somewhere else — on a real
	// display, most likely the one the person is looking at.
	On []int
	// Minimized is how many of its windows are in the Dock. A minimized window
	// can still be moved, so it is counted rather than hidden, but a gallery
	// should say so: an application that is entirely minimized looks missing.
	Minimized int
}

// String names an application the way a gallery cell would.
func (a App) String() string {
	s := a.Name
	if a.Windows > 1 {
		s += fmt.Sprintf(" (%d windows)", a.Windows)
	}
	switch {
	case len(a.On) == 1:
		s += fmt.Sprintf(" on screen %d", a.On[0]+1)
	case len(a.On) > 1:
		var pos []string
		for _, p := range a.On {
			pos = append(pos, fmt.Sprint(p+1))
		}
		s += " on screens " + strings.Join(pos, ", ")
	}
	return s
}

// Here reports whether any window of this application is on the desk.
func (a App) Here() bool { return len(a.On) > 0 }

// AppsFrom groups a listing of windows into applications, and works out which
// ribbon position each window is on.
//
// It is pure, which is the point: the listing comes from the platform once, and
// everything a person then chooses over is decided here, where it can be tested.
//
// ids is the desk's screens in ribbon order — the same slice [Send] takes — so
// position 1 in a settings file, screen 1 on the band and "on screen 1" in a
// gallery are the same screen. A window on a display that is not the desk's
// counts towards the application's window total and towards no position.
//
// The result is sorted by name, so a gallery does not reshuffle itself between
// two looks at it. Windows with no application name are ignored: there is
// nothing to show and nothing to move.
func AppsFrom(list []accessibility.WindowInfo, ids []uint64) []App {
	pos := map[uint32]int{}
	for i, id := range ids {
		pos[uint32(id)] = i
	}
	byName := map[string]*App{}
	for _, w := range list {
		if strings.TrimSpace(w.App) == "" {
			continue
		}
		a := byName[w.App]
		if a == nil {
			a = &App{Name: w.App, PID: int32(w.PID)}
			byName[w.App] = a
		}
		a.Windows++
		if w.Minimized {
			a.Minimized++
		}
		if p, ok := pos[w.Display]; ok && !containsInt(a.On, p) {
			a.On = append(a.On, p)
		}
	}
	out := make([]App, 0, len(byName))
	for _, a := range byName {
		sort.Ints(a.On)
		out = append(out, *a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// containsInt is a helper the standard library has under a name this package
// cannot use without a newer Go than the fleet builds with.
func containsInt(xs []int, x int) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// Spread hands out one screen per application, in order, up to screens.
//
// One press, and a desk that was showing six empty desktops is showing six
// applications. What it will NOT do is wrap: an application past the last screen
// is left exactly where it is, because moving two applications onto one screen
// hides one of them, and a person who pressed one key cannot be expected to
// guess which.
//
// Applications already on a screen are placed too, and deliberately: the point
// of the key is that the desk ends up in a state a person can predict — the
// first application on screen 1, the second on screen 2 — not that it ends up
// in whatever state the previous arrangement plus this press produces.
//
// The result is [Placement]s, so the live path is the one the settings file
// already uses. It never returns more placements than there are screens.
func Spread(apps []App, screens int) []Placement {
	if screens <= 0 {
		return nil
	}
	var out []Placement
	for i, a := range apps {
		if i >= screens {
			break
		}
		out = append(out, Placement{App: a.Name, Pos: i + 1})
	}
	return out
}
