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

func TestTheReportSaysWhatIsMissingAndWhereToSayYes(t *testing.T) {
	lines := permissionLines([]Grant{
		{What: "screen recording", Pane: "Screen & System Audio Recording", Held: false, Needed: true},
		{What: "Accessibility", Pane: "Accessibility", Held: true, Needed: false},
	}, "/Users/Shared/xrdesk/XR desk.app")
	all := strings.Join(lines, "\n")
	if !strings.Contains(all, "1 of 2 missing") {
		t.Errorf("the report does not count what is missing:\n%s", all)
	}
	if !strings.Contains(all, "⛔ screen recording") {
		t.Errorf("the missing one is not marked:\n%s", all)
	}
	if !strings.Contains(all, "✓ Accessibility") {
		t.Errorf("the held one is not marked:\n%s", all)
	}
	if !strings.Contains(all, "Screen & System Audio Recording") {
		t.Errorf("the report does not say which pane:\n%s", all)
	}
	if !strings.Contains(all, "/Users/Shared/xrdesk/XR desk.app") {
		t.Errorf("the report does not say what to grant it to:\n%s", all)
	}
	if strings.Count(all, "XR desk.app") != 1 {
		t.Errorf("the path is repeated rather than said once:\n%s", all)
	}
}

// ⛔ THE REPORT IS MADE WHEN NOTHING IS MISSING TOO. A grant can stop applying
// without anything being done to it, so "held at 13:42" is the fact worth
// having in the log when the same program stops working at 14:10.
func TestTheReportIsMadeWhenEverythingIsHeld(t *testing.T) {
	lines := permissionLines([]Grant{
		{What: "screen recording", Pane: "Screen & System Audio Recording", Held: true, Needed: true},
	}, "/Users/Shared/xrdesk/XR desk.app")
	if len(lines) == 0 {
		t.Fatal("nothing was said when everything was held")
	}
	all := strings.Join(lines, "\n")
	if !strings.Contains(all, "all held") {
		t.Errorf("the report does not say everything is there:\n%s", all)
	}
	if strings.Contains(all, "⛔") {
		t.Errorf("something is marked missing:\n%s", all)
	}
	// Nothing to go and do, so no path and no pane: an instruction with nothing
	// to act on reads as an instruction.
	if strings.Contains(all, "System Settings") {
		t.Errorf("the report sends a person to System Settings for nothing:\n%s", all)
	}
}

// A grant the desk starts without says so, or it reads as a reason the desk
// will not start -- and somebody goes looking for a permission problem that is
// not the one stopping them.
func TestAGrantTheDeskStartsWithoutSaysSo(t *testing.T) {
	lines := permissionLines([]Grant{
		{What: "Accessibility", Pane: "Accessibility", Held: false, Needed: false},
	}, "")
	all := strings.Join(lines, "\n")
	if !strings.Contains(all, "starts without it") {
		t.Errorf("an optional grant is not marked optional:\n%s", all)
	}
}

func TestNothingToReportSaysNothing(t *testing.T) {
	if lines := permissionLines(nil, "/somewhere.app"); lines != nil {
		t.Errorf("a report was made about no permissions: %v", lines)
	}
}
