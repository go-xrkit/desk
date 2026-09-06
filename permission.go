// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import "fmt"

// permissionMessage says how to grant screen recording, naming the application
// to grant it to when that is known.
//
// The grant belongs to whatever the system holds RESPONSIBLE for the capture,
// and that is not always this program: started from a shell, the responsible
// application is the terminal. Inside a bundle it is the bundle, and then the
// only useful thing to say is its path -- a person looking at a list of
// applications cannot tell which "XR desk" is meant, and there is a + button
// that wants a path.
//
// app is the .app directory this is running inside, or empty.
func permissionMessage(app string) string {
	const where = "desk: screen recording is not permitted. " +
		"Grant it in System Settings > Privacy & Security > Screen & System Audio Recording "
	if app == "" {
		return where + "to the application that launched this program — for a program " +
			"started from a shell that is the terminal or editor, not the program itself — " +
			"then restart it"
	}
	return where + "to " + app + " — add it with the + button if it is not " +
		"listed — then restart it"
}

// Grant is one thing macOS lets a person say yes or no to, and whether they
// have.
type Grant struct {
	// What it is called here, in a sentence.
	What string
	// Pane is where it is said yes to, as System Settings names it.
	Pane string
	// Held is whether it has been granted to this program as it stands.
	Held bool
	// Needed is whether the desk cannot work without it, as against working
	// with one thing missing.
	Needed bool
	// How is what to do about it, when System Settings is the wrong answer.
	//
	// ⛔ A PERMISSION NOBODY HAS ASKED FOR IS NOT IN SYSTEM SETTINGS AT ALL.
	// macOS lists an application under Camera once it has asked once, and not
	// before -- so "grant it in System Settings > Camera" sends a person to
	// look for a row that does not exist, and the thing they actually have to
	// do is press the key and answer the prompt. Empty means the pane is the
	// right answer.
	How string
}

// permissionLines is the startup report on what has been granted.
//
// ⛔ IT IS SAID AT THE START, ALL AT ONCE, AND WHETHER OR NOT ANYTHING IS
// MISSING. Discovering a missing grant at the moment it is first needed means
// a person grants one thing, restarts, and is told about the next -- as many
// round trips as there are permissions. And saying nothing when everything is
// held is worse than it sounds: a grant can be lost without anything being
// done to it, so "it was there at 13:42" is the fact worth having in the log
// when the same program stops working at 14:10.
func permissionLines(gs []Grant, app string) []string {
	if len(gs) == 0 {
		return nil
	}
	var lines []string
	missing := 0
	for _, g := range gs {
		mark, note := "✓", ""
		if !g.Held {
			mark, note = "⛔", " — grant it in System Settings > Privacy & Security > "+g.Pane
			if g.How != "" {
				note = " — " + g.How
			}
			if !g.Needed {
				note += " (the desk starts without it)"
			}
			missing++
		}
		lines = append(lines, fmt.Sprintf("%s %s%s", mark, g.What, note))
	}
	if missing == 0 {
		return append([]string{"permissions, all held:"}, lines...)
	}
	head := fmt.Sprintf("permissions, %d of %d missing:", missing, len(gs))
	lines = append([]string{head}, lines...)
	// The path once, at the end, rather than on every line that wants it: it is
	// the same answer for all of them, and it is what the + button takes.
	if app != "" {
		lines = append(lines, "grant them to "+app+" — the + button takes that path")
	}
	return lines
}
