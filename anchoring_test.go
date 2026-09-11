// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"strings"
	"testing"
)

// TestWhatTheDeskSaysAboutStayingPut.
//
// ⛔⛔ "C'EST DEROUTANT QUAND ON TOURNE LA TETE D'AVOIR LES ECRANS QUI SE REPLACE
// FACE A SOIT D'UN COUP." The two rows are not on and off, they are two
// different desks -- see [Anchoring] -- so each says which one it is, including
// when it is already in force: somebody who presses the row that is already
// chosen is asking which one that is, and macOS draws nothing at all for a
// checkbox that is off.
func TestWhatTheDeskSaysAboutStayingPut(t *testing.T) {
	p := testPlan(t)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	d.Badge(1, nil, nil)

	if got := d.Plan().Anchoring(); got != AnchorOnGaze {
		t.Fatalf("a plan nobody told starts %v, want the way it has always been", got)
	}

	d.Do(ActionDeskStaysPut)
	if got := d.Plan().Anchoring(); got != AnchorFixed {
		t.Errorf("after asking for a fixed desk the plan is %v", got)
	}
	if !strings.Contains(d.notice.toast.Text, "stays where it is") {
		t.Errorf("it said %q", d.notice.toast.Text)
	}

	// ⭐ ASKING AGAIN FOR THE DESK IT IS ALREADY IN SAYS SO AND REBUILDS NOTHING.
	d.notice.toast.Text = ""
	d.Do(ActionDeskStaysPut)
	if got := d.Plan().Anchoring(); got != AnchorFixed {
		t.Errorf("asking twice left it %v", got)
	}
	if !strings.Contains(d.notice.toast.Text, "stays where it is") {
		t.Errorf("asking twice said %q, which tells nobody which desk this is",
			d.notice.toast.Text)
	}

	d.Do(ActionDeskFacesMe)
	if got := d.Plan().Anchoring(); got != AnchorOnGaze {
		t.Errorf("after asking for screens that face me the plan is %v", got)
	}
	if !strings.Contains(d.notice.toast.Text, "turns towards you") {
		t.Errorf("it said %q", d.notice.toast.Text)
	}
}

// TestTheAnchoringIsNamedEverywhereItIsShown: in a log line, in a notice, and
// in the note written beside every capture.
func TestTheAnchoringIsNamedEverywhereItIsShown(t *testing.T) {
	if got := AnchorFixed.String(); got != "fixed" {
		t.Errorf("AnchorFixed is %q", got)
	}
	if got := AnchorOnGaze.String(); !strings.Contains(got, "gaze") {
		t.Errorf("AnchorOnGaze is %q", got)
	}
	if got := Anchoring(42).String(); !strings.Contains(got, "gaze") {
		t.Errorf("an Anchoring nobody defined is %q, want the default's name", got)
	}

	p := testPlan(t)
	// ⭐ ONLY WHEN IT IS NOT THE DEFAULT. The line goes into the note beside
	// every capture, and a state nobody chose is not news.
	if s := p.String(); strings.Contains(s, "stays put") {
		t.Errorf("a plan nobody told claims a fixed desk: %q", s)
	}
	if s := p.WithAnchoring(AnchorFixed).String(); !strings.Contains(s, "stays put") {
		t.Errorf("a fixed desk does not say so: %q", s)
	}
	// Anything that is not the fixed desk is the one it has always been, so a
	// number nobody defined cannot make a third kind of desk.
	if got := p.WithAnchoring(Anchoring(42)).Anchoring(); got != AnchorOnGaze {
		t.Errorf("WithAnchoring(42) gave %v", got)
	}
}

// TestTheSettingsFileCanChooseTheDesk, and says nothing when nobody has.
func TestTheSettingsFileCanChooseTheDesk(t *testing.T) {
	yes, no := true, false
	for _, c := range []struct {
		what string
		cfg  Config
		want Anchoring
	}{
		{"an empty file", Config{}, AnchorOnGaze},
		{"a ribbon block that says nothing", Config{Ribbon: &ConfigRibbon{}}, AnchorOnGaze},
		{"faces_me = true", Config{Ribbon: &ConfigRibbon{FacesMe: &yes}}, AnchorOnGaze},
		{"faces_me = false", Config{Ribbon: &ConfigRibbon{FacesMe: &no}}, AnchorFixed},
	} {
		if got := c.cfg.Anchoring(); got != c.want {
			t.Errorf("%s: %v, want %v", c.what, got, c.want)
		}
	}

	// And it survives a round trip through the file, or a choice made once is a
	// choice made every morning.
	b := Config{Ribbon: &ConfigRibbon{FacesMe: &no}}.Bytes()
	if !strings.Contains(string(b), "faces_me") {
		t.Errorf("the written file does not mention it:\n%s", b)
	}
}

// TestTheChoiceReachesTheRendererThatUsesIt.
//
// ⛔ THE FLAT BAND HAS NO CHAIN TO ANCHOR, so the plan records the choice and
// nothing else happens. A curved one has a fan, and the fan is what reads it
// every frame -- a choice that stopped at the plan would be a menu row that
// does nothing.
func TestTheChoiceReachesTheRendererThatUsesIt(t *testing.T) {
	p := fanPlan(t, 6, DefaultSplayDeg, 1)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	d.Badge(1, nil, nil)
	if d.fan == nil {
		t.Fatal("a curved plan built no fan, so this measures the flat band")
	}

	d.Do(ActionDeskStaysPut)
	if got := d.fan.anchor; got != AnchorFixed {
		t.Errorf("the fan is %v after the row was pressed", got)
	}
	d.Do(ActionDeskFacesMe)
	if got := d.fan.anchor; got != AnchorOnGaze {
		t.Errorf("the fan is %v after the other row was pressed", got)
	}
}
