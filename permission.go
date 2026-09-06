// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

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
