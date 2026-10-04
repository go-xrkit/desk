// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"testing"

	"github.com/go-xrkit/desk"
	"github.com/go-xrkit/xrkit/glasses"
)

// TestRibbonIDsPutsTheMacFirst.
//
// The off-by-one this exists to stop: with screen 1 showing this Mac's own
// display, "the screens this program made" and "the screens on the band" are no
// longer the same list, and everything that turns a POSITION into a display --
// placing an application, taking a screen away, holding the pointer -- reads
// the wrong one for the whole session if it asks the wrong question.
func TestRibbonIDsPutsTheMacFirst(t *testing.T) {
	made := []uint64{101, 102, 103}

	got := ribbonIDs(true, 1, made)
	want := []uint64{1, 101, 102, 103}
	if len(got) != len(want) {
		t.Fatalf("ribbonIDs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ribbonIDs = %v, want %v", got, want)
		}
	}
	// And it does not disturb the slice it was given: appending to a slice with
	// spare capacity would write into the caller's.
	if made[0] != 101 {
		t.Errorf("the screens this program made were rewritten: %v", made)
	}

	// Not mirroring, and mirroring with nothing to mirror, are both the band as
	// this program made it.
	for _, c := range []struct {
		name   string
		mirror bool
		mac    uint64
	}{
		{"mirroring turned off", false, 1},
		{"no main display to show", true, 0},
	} {
		got := ribbonIDs(c.mirror, c.mac, made)
		if len(got) != len(made) || got[0] != made[0] {
			t.Errorf("%s: ribbonIDs = %v, want %v", c.name, got, made)
		}
	}
}

func TestMainDisplayIsTheOneThatSaysSo(t *testing.T) {
	offers := []desk.Offer{
		{ID: "display-101", Name: "XR screen 1", Kind: desk.KindDisplay},
		{ID: "display-2", Name: "display 2", Kind: desk.KindDisplay},
		{ID: "display-1", Name: "display 1 (main)", Kind: desk.KindDisplay, Main: true},
	}
	o, ok := mainDisplay(offers)
	if !ok || o.ID != "display-1" {
		t.Errorf("mainDisplay = %v,%v, want the one marked Main", o, ok)
	}
	// By the PROPERTY, not the label: a name that says "(main)" without the
	// field is a sentence, and reading a sentence back is how this breaks
	// silently the day the sentence changes.
	liar := []desk.Offer{{ID: "display-9", Name: "display 9 (main)", Kind: desk.KindDisplay}}
	if o, ok := mainDisplay(liar); ok {
		t.Errorf("mainDisplay = %v, want nothing: no offer says it is the main one", o)
	}
	if _, ok := mainDisplay(nil); ok {
		t.Error("mainDisplay found a screen among no offers")
	}
	// A panel this program renders onto is not a screen of the machine.
	if _, ok := mainDisplay([]desk.Offer{{ID: "panel-1", Kind: desk.KindPanel, Main: true}}); ok {
		t.Error("mainDisplay took a rendered panel for this Mac's screen")
	}
}

// TestThePlanCarriesTheDistanceAndTheSplay.
//
// ⛔⛔ FOR THREE WEEKS NEITHER OF THEM REACHED THE BAND. -distance and -splay
// were parsed, fetched from the settings when the flag was zero, threaded
// through four call sites and the session's own parameter list -- and left out
// of the Options the plan was built from. Go says nothing about a parameter
// nobody reads: `dist` and `splay` appeared EXACTLY ONCE each in a 677-line
// function, in its signature, and the build, the vet and the whole suite were
// green.
//
// It was found by using the flag as an instrument rather than trusting it: a
// capture taken with "-distance 2" came back with the note "curved 20.0° at
// 1.00x" beside it.
//
// So this asserts the one thing nobody was asserting -- that a number given on
// the command line is a number the band has.
func TestThePlanCarriesTheDistanceAndTheSplay(t *testing.T) {
	beast := glasses.Display{Name: "VITURE Beast", Width: 3840, Height: 1080}

	p, err := planFor(beast, 6, 2.5, 40, 0, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Distance(); got != 2.5 {
		t.Errorf("the band sits at %g, want the 2.5 that was asked for", got)
	}
	if got := p.SplayDeg(); got != 40 {
		t.Errorf("the screens are splayed %g°, want the 40 that was asked for", got)
	}
	if got := p.Count(); got != 6 {
		t.Errorf("the band has %d screens, want 6", got)
	}

	// ⭐ AND ZERO STILL MEANS "NOBODY SAID", which is the convention the flags
	// rely on: it is what lets an unset flag fall through to the settings and
	// an unset setting fall through to the plan's own default. A test that only
	// checked the numbers above would pass on a planFor that ignored zero and
	// forced one.
	p, err = planFor(beast, 6, 0, 0, 0, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Distance(); got != 1 {
		t.Errorf("a band nobody placed sits at %g, want 1", got)
	}
	if got := p.SplayDeg(); got != desk.DefaultSplayDeg {
		t.Errorf("a band nobody splayed is %g°, want %g", got, desk.DefaultSplayDeg)
	}

	// And a negative splay is the flat band, which is the other half of that
	// convention and the only way to ask for no curvature at all.
	p, err = planFor(beast, 6, 0, -1, 0, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.SplayDeg(); got != 0 {
		t.Errorf("a band asked to be flat is splayed %g°", got)
	}
}

// ⛔⛔ THE MIRROR OCCUPIES POSITION ZERO, and forgetting it cost a session.
// With -wide asking for one screen and the mirror on, the desk planned one
// position, made one virtual display for it, and then had TWO feeds -- the
// Mac's own screen and the wide one -- for a single place to put them. It
// stopped with "no screens: 2 feeds for 1 screens".
//
// Every unit test passed. It took running the thing to find out, which is why
// the arithmetic now lives in a function a test can reach.
func TestAWideDeskCountsTheMirrorIn(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name   string
		n      int
		wide   int
		mirror bool
		want   int
	}{
		// ⭐ AS MANY AS WERE ASKED FOR, WIDE OR NOT, and the mirror does not add
		// one. This answered 1 for any wide desk, on a report that was about
		// something else: "en mode ecran large on ne veux que un ecran, la j'en
		// vois plusieurs" named the Mac's MIRROR beside the wide screen, not a
		// second wide screen. Asked for since: "moi je verrais bien 3 ecrans
		// large qu'on peut faire defiler".
		{"wide, whatever the mirror setting says", 6, 6400, true, 6},
		{"wide, mirror off", 6, 6400, false, 6},
		{"three wide screens, which is what was asked for", 3, 6400, true, 3},
		// ⚠ And no width changes nothing at all: -wide is opt-in, and a desk
		// that quietly became two screens because the mirror is on would be a
		// setting nobody asked for.
		{"no width, mirror on", 6, 0, true, 6},
		{"no width, mirror off", 6, 0, false, 6},
		{"no width, and the plan chooses", 0, 0, true, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			if got := screensForWide(c.n, c.wide, c.mirror); got != c.want {
				t.Errorf("screensForWide(%d, %d, %t) = %d, want %d",
					c.n, c.wide, c.mirror, got, c.want)
			}
		})
	}
}

// ⛔⛔ FOUR CONDITIONS, AND THE FOURTH IS THE ONE THAT WAS MISSING. Dimming this
// Mac's panel is only worth anything while the band shows a copy of it; wide
// mode drops the mirror, so there is no copy, and turning the panel off then
// takes the menu bar away from a person who has nowhere else to find it.
//
// ⚠ AND IT IS A PREDICATE RATHER THAN A LINE IN A CLOSURE because that is where
// the last one of these hid: a condition nothing can be handed is a condition
// nothing measures. See desk.looksClipped.
func TestTheMacsPanelIsOnlyDimmedWhileItsCopyIsShowing(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name                             string
		setting, flag, immersive, mirror bool
		want                             bool
	}{
		{"everything asks for it", true, true, true, true, true},
		// ⭐ THE DEFECT. Wide mode: the mirror is off, so the desktop is NOT in
		// front of the person twice and the panel carries the only menu bar.
		{"no mirror, which is wide mode", true, true, true, false, false},
		{"the person turned it off in the settings", false, true, true, true, false},
		{"the flag forced it off for one run", true, false, true, true, false},
		// Windowed, the desk is a window ON one of these screens, so darkening
		// them would black out the thing being used.
		{"windowed rather than immersive", true, true, false, true, false},
		{"nothing asks for it", false, false, false, false, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			if got := dimTheMacsPanel(c.setting, c.flag, c.immersive, c.mirror); got != c.want {
				t.Errorf("dimTheMacsPanel(setting=%v, flag=%v, immersive=%v, mirror=%v) "+
					"= %v, want %v", c.setting, c.flag, c.immersive, c.mirror, got, c.want)
			}
		})
	}
}

// ⛔⛔ A WIDE SCREEN NOBODY HAS SPOKEN FOR IS CURVED, which is a default this
// command changed on the strength of one sentence from inside the glasses:
// "l'ecran plat de 6400 sans courbure n'est franchement pas utilisable aux
// bors, c'est trop loin".
//
// ⭐ THE GEOMETRY AGREES EXACTLY. A flat plane keeps its edges further from the
// eye than its middle, and a rectilinear projection draws them smaller for it:
//
//	screen            edge distance   seen at   scale at the edge
//	1920, 1 view              1.11x       26°                 81%
//	3840, 2 views             1.39x       44°                 52%
//	6400, 3.33 views          1.90x       58°                 28%
//	8640, 4.5 views           2.39x       65°                 17%
//
// Which is why "the desk's screens are flat" was right for its own case and
// wrong for this one: at one view the penalty is 81% and nobody notices, and
// that judgement -- measured, worn and kept on 2026-08-26 -- stands. At 3.33
// views the edge is nearly twice as far and drawn at 28%.
//
// ⚠ AND A CHOICE STAYS A CHOICE. Somebody who typed `-bend 0`, or `bend = 0` in
// the settings, asked for a flat wide screen and gets one; see
// desk.Config.BendChosen for why the two could not be told apart before.
func TestAWideScreenNobodySpokeForIsCurved(t *testing.T) {
	t.Parallel()

	flat, gentle := desk.FlatBend, 4.0
	for _, c := range []struct {
		name string
		flag float64
		cfg  desk.Config
		want float64
	}{
		// -1 is the flag's "not given"; see the note on bendOr's callers.
		{"nobody said anything at all", -1, desk.Config{}, desk.DefaultBend},
		{"a ribbon block that says nothing about it", -1,
			desk.Config{Ribbon: &desk.ConfigRibbon{}}, desk.DefaultBend},
		// ⛔ THE TWO WAYS OF ASKING FOR FLAT, which must both survive the default.
		{"the flag asked for flat", 0, desk.Config{}, desk.FlatBend},
		{"the settings asked for flat", -1,
			desk.Config{Ribbon: &desk.ConfigRibbon{Bend: &flat}}, desk.FlatBend},
		// And a radius either way is that radius.
		{"the flag named a radius", 2, desk.Config{}, 2},
		{"the settings named a radius", -1,
			desk.Config{Ribbon: &desk.ConfigRibbon{Bend: &gentle}}, 4},
		// ⚠ THE FLAG WINS, like every other number here: it is for one run.
		{"the flag overrules the settings", 0,
			desk.Config{Ribbon: &desk.ConfigRibbon{Bend: &gentle}}, desk.FlatBend},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			if got := bendOr(c.flag, c.cfg); got != c.want {
				t.Errorf("bendOr(%g, %+v) = %g, want %g", c.flag, c.cfg.Ribbon, got, c.want)
			}
		})
	}
}

// ⛔⛔ A WIDE DESK DEFAULTS TO THE FLAT BAND, because a turned band is sharp
// only while the head is still -- and a screen of 6400 is reached by turning.
// The measurement is in splayForWide's own doc; the package test
// TestAWideScreenIsSharpAtEveryAngleOnlyWhenFlat keeps it honest.
//
// ⚠ AND A CHOICE IS LEFT ALONE. Zero is the only value that means "nobody
// said": the flag's own default, and what desk.Config.SplayDeg answers for a
// settings file that is silent.
func TestAWideDeskTakesTheFlatBandUnlessSomebodyChose(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name  string
		splay float64
		wide  int
		want  float64
	}{
		{"wide, nobody said: the flat band", 0, 6400, -1},
		{"wide, a splay was asked for: left alone", 20, 6400, 20},
		{"wide, flat was asked for: already flat", -1, 6400, -1},
		// ⛔ AND IT REACHES ONLY A WIDE DESK. An ordinary band derives its own
		// splay from its optics, and flattening that would undo the ribbon.
		{"not wide, nobody said: the plan derives it", 0, 0, 0},
		{"not wide, a splay was asked for", 20, 0, 20},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := splayForWide(c.splay, c.wide); got != c.want {
				t.Errorf("splayForWide(%g, %d) = %g, want %g",
					c.splay, c.wide, got, c.want)
			}
		})
	}
}

// ⛔⛔ AND THE PLAN REALLY COMES OUT FLAT, which the table above does not show.
// splayForWide answers -1 and desk.NewPlan reads -1 as "one flat plane", but
// those are two conventions meeting on one number -- zero means "nobody said"
// on one side and "the flat band" on the other, and the translation between
// them has been wrong here before. A plan whose SplayDeg stayed positive would
// build a fan, and the whole measurement that chose this default is about not
// building one.
//
// ⚠ IT GOES THROUGH planFor, the way the session does, rather than calling
// NewPlan itself: the predicate is already tested above, and what is left to
// check is the wiring.
func TestAWideDesksPlanBuildsNoFan(t *testing.T) {
	t.Parallel()

	beast := glasses.Display{Name: "VITURE Beast", Width: 3840, Height: 1080}
	flat, err := planFor(beast, screensForWide(3, 6400, false), 1,
		splayForWide(0, 6400), 0, nil, desk.AnchorOnGaze)
	if err != nil {
		t.Fatal(err)
	}
	if flat.SplayDeg() > 0 {
		t.Errorf("a wide desk's splay came out %g°, which is the angle that "+
			"builds a fan -- see desk.build", flat.SplayDeg())
	}
	if got := flat.Count(); got != 3 {
		t.Errorf("the wide desk has %d screens, want the 3 asked for", got)
	}
	// ⚠ AND AN ORDINARY DESK STILL DERIVES ITS OWN, which is the ribbon. A
	// change that flattened every band would pass the check above.
	band, err := planFor(beast, screensForWide(3, 0, false), 1,
		splayForWide(0, 0), 0, nil, desk.AnchorOnGaze)
	if err != nil {
		t.Fatal(err)
	}
	if band.SplayDeg() <= 0 {
		t.Errorf("an ordinary band's splay came out %g°; it derives a positive "+
			"one from its own optics, and flattening it would undo the ribbon",
			band.SplayDeg())
	}
}
