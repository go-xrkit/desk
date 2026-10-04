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

// cameraGrant is the darwin wiring: it reads this Mac's decision and how this
// process was launched, and leaves the shaping to [cameraRow], which is
// portable and tested.
func cameraGrant(a avfoundation.CameraAccess) Grant {
	return cameraRow(a.String(), a.Granted(),
		a == avfoundation.CameraNotDetermined, WhyTheHeadCannotBeFollowed())
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
