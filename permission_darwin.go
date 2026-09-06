// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build darwin

package desk

import (
	"github.com/go-macos/accessibility"
	"github.com/go-macos/appbundle"
	"github.com/go-macos/avfoundation"
	"github.com/go-macos/screencapture"
)

// Permissions is what this program has been granted, asked of the system now.
//
// Asked rather than remembered: a grant is attached to a code identity, and it
// can stop applying without anything here changing -- a rebuild under an ad-hoc
// signature was enough, until the bundle was signed with a certificate.
//
// ⛔ THE CAMERA IS IN THE LIST, AND LEAVING IT OUT COST AN AFTERNOON. It was
// left out on the belief that asking about it meant OPENING it -- which would
// put a prompt on somebody's screen, and a report is not a reason to do that.
// The belief was wrong: -[AVCaptureDevice authorizationStatusForMediaType:]
// reports a decision already made without prompting and without lighting
// anything, and go-macos/avfoundation had been calling it internally all along.
//
// What it cost: passthrough did nothing, said nothing, and looked like a defect
// in choosing the headset's camera. That code was correct the whole time; the
// grant had simply been lost when the bundle was signed, exactly as screen
// recording was.
func Permissions() []Grant {
	return []Grant{
		{
			What:   "screen recording — the desk is made of captured screens",
			Pane:   "Screen & System Audio Recording",
			Held:   screencapture.Authorized(),
			Needed: true,
		},
		{
			What:   "Accessibility — placing an application's windows on a screen",
			Pane:   "Accessibility",
			Held:   accessibility.Trusted(),
			Needed: false,
		},
		cameraGrant(avfoundation.CameraAuthorization()),
	}
}

// cameraGrant says where the camera stands and what to do about it.
//
// ⛔ THE TWO WAYS OF NOT HAVING IT NEED DIFFERENT ADVICE, AND ONE OF THEM MAKES
// SYSTEM SETTINGS THE WRONG ANSWER. macOS lists an application under Camera
// once it has ASKED once, and not before -- so telling somebody with a
// never-asked camera to go and switch it on there sends them to look for a row
// that does not exist. What they have to do is press the key and answer the
// prompt. A refusal is the opposite: the prompt will not come back, and System
// Settings is the only place left.
func cameraGrant(a avfoundation.CameraAccess) Grant {
	g := Grant{
		What: "the camera — showing the room, and photographs (" + a.String() + ")",
		Pane: "Camera",
		Held: a.Granted(),
		// The desk starts without it; only passthrough and photographs stop.
		Needed: false,
	}
	if a == avfoundation.CameraNotDetermined {
		g.How = "nothing to do in System Settings, which will not list this " +
			"application until it has asked once: press the key that shows the " +
			"room and macOS will ask"
	}
	return g
}

// LogPermissions says what has been granted, once, at the start.
func LogPermissions(logf func(string, ...any)) {
	if logf == nil {
		return
	}
	var app string
	if b, ok := appbundle.Running(); ok {
		app = b.Path
	}
	for _, line := range permissionLines(Permissions(), app) {
		logf("%s", line)
	}
}
