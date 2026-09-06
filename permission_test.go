// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"strings"
	"testing"
)

// The message a person reads when the capture is refused has one job: to end
// with them clicking the right thing. Naming the bundle is what does that --
// the list in System Settings is a list of names, several of which can be the
// same, and the + button next to it takes a path.
func TestTheMessageNamesTheApplicationToGrantItTo(t *testing.T) {
	got := permissionMessage("/Users/Shared/xrdesk/XR desk.app")
	if !strings.Contains(got, "/Users/Shared/xrdesk/XR desk.app") {
		t.Errorf("the message does not say which application:\n%s", got)
	}
	if !strings.Contains(got, "+") {
		t.Errorf("the message does not say how to add one that is not listed:\n%s", got)
	}
	if strings.Contains(got, "terminal") {
		t.Errorf("the message still talks about a shell it was not started from:\n%s", got)
	}
}

// Outside a bundle there is nothing truthful to name: the grant belongs to
// whatever launched this, and this cannot see what that was. Saying so beats
// naming the wrong thing.
func TestOutsideABundleItSaysWhatItCannotKnow(t *testing.T) {
	got := permissionMessage("")
	if !strings.Contains(got, "the application that launched this program") {
		t.Errorf("the message should say the grant belongs elsewhere:\n%s", got)
	}
	if strings.Contains(got, ".app") {
		t.Errorf("the message names a bundle it does not have:\n%s", got)
	}
}

func TestEitherWayItSaysWhereToGoAndToRestart(t *testing.T) {
	for _, app := range []string{"", "/Users/Shared/xrdesk/XR desk.app"} {
		got := permissionMessage(app)
		if !strings.Contains(got, "Screen & System Audio Recording") {
			t.Errorf("%q: no pane named:\n%s", app, got)
		}
		if !strings.Contains(got, "restart it") {
			t.Errorf("%q: a grant given to a running program does nothing until it "+
				"is restarted, and the message does not say so:\n%s", app, got)
		}
	}
}
