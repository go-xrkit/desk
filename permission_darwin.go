// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build darwin

package desk

import (
	"github.com/go-macos/accessibility"
	"github.com/go-macos/appbundle"
	"github.com/go-macos/screencapture"
)

// Permissions is what this program has been granted, asked of the system now.
//
// Asked rather than remembered: a grant is attached to a code identity, and it
// can stop applying without anything here changing -- a rebuild under an ad-hoc
// signature was enough, until the bundle was signed with a certificate.
//
// The camera is not in the list. There is no way to ask about it that does not
// open it, and opening it puts a prompt on the person's screen; a report is not
// a reason to do that. It asks for itself the first time passthrough is used,
// which is the moment it makes sense to.
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
	}
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
