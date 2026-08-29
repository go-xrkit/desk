// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"sort"
	"testing"

	"github.com/go-xrkit/xrkit/glasses"
)

// panels is a machine whose backlights can be watched.
type panels struct {
	dark  map[uint64]bool
	order []string
	fail  map[uint64]error // displays that refuse to be darkened
	stuck map[uint64]error // displays that refuse to come back
}

func newPanels() *panels {
	return &panels{dark: map[uint64]bool{}, fail: map[uint64]error{}, stuck: map[uint64]error{}}
}

// install puts this machine behind the seam for one test.
func (p *panels) install(t *testing.T) {
	t.Helper()
	was := dimDisplay
	dimDisplay = func(id uint64) (func() error, error) {
		if err := p.fail[id]; err != nil {
			return nil, err
		}
		p.dark[id] = true
		p.order = append(p.order, "dim")
		return func() error {
			if err := p.stuck[id]; err != nil {
				return err
			}
			delete(p.dark, id)
			p.order = append(p.order, "restore")
			return nil
		}, nil
	}
	t.Cleanup(func() { dimDisplay = was })
}

func (p *panels) darkList() []uint64 {
	out := make([]uint64, 0, len(p.dark))
	for id := range p.dark {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func sameIDs(a, b []uint64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestDimmerDarkensWhatArrivedAndPutsBackWhatLeft(t *testing.T) {
	p := newPanels()
	p.install(t)
	var m Dimmer

	if err := m.Showing([]uint64{1, 2}); err != nil {
		t.Fatalf("Showing: %v", err)
	}
	if got := p.darkList(); !sameIDs(got, []uint64{1, 2}) {
		t.Errorf("dark = %v, want both panels off", got)
	}
	if m.Dark() != 2 {
		t.Errorf("Dark = %d, want 2", m.Dark())
	}

	// 2 leaves the ribbon and 3 arrives: 1 is NOT touched again, which is the
	// point of handing the whole picture rather than one change at a time.
	before := len(p.order)
	if err := m.Showing([]uint64{1, 3}); err != nil {
		t.Fatalf("Showing: %v", err)
	}
	if got := p.darkList(); !sameIDs(got, []uint64{1, 3}) {
		t.Errorf("dark = %v, want 1 still off and 3 off", got)
	}
	if n := len(p.order) - before; n != 2 {
		t.Errorf("%d backlight changes for one swap, want 2", n)
	}

	if err := m.Restore(); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if got := p.darkList(); len(got) != 0 {
		t.Errorf("dark = %v after Restore, want nothing left dark", got)
	}
	// Deferred beside a Showing that ran many times: the second one must be
	// quiet rather than an error or a second round of restores.
	before = len(p.order)
	if err := m.Restore(); err != nil {
		t.Errorf("second Restore: %v", err)
	}
	if len(p.order) != before {
		t.Error("the second Restore touched a backlight")
	}
	if m.Dark() != 0 {
		t.Errorf("Dark = %d after Restore, want 0", m.Dark())
	}
}

func TestDimmerReportsAPanelThatWillNotGoDarkAndKeepsTheRest(t *testing.T) {
	p := newPanels()
	noDDC := errors.New("this display does not report a brightness")
	p.fail[2] = noDDC
	p.install(t)
	var m Dimmer

	err := m.Showing([]uint64{1, 2})
	if !errors.Is(err, noDDC) {
		t.Errorf("Showing = %v, want the refusal from display 2", err)
	}
	// An external panel with no DDC is the ordinary case, and it must not cost
	// the built-in one its darkening.
	if got := p.darkList(); !sameIDs(got, []uint64{1}) {
		t.Errorf("dark = %v, want display 1 off anyway", got)
	}
	if m.Dark() != 1 {
		t.Errorf("Dark = %d, want the one that worked", m.Dark())
	}
	// And the one that refused is not remembered as dark, so it is retried
	// rather than silently skipped for the rest of the session.
	before := len(p.order)
	_ = m.Showing([]uint64{1, 2})
	if len(p.order) != before {
		t.Error("display 1 was darkened twice")
	}
}

func TestDimmerReportsAPanelThatWillNotComeBack(t *testing.T) {
	p := newPanels()
	stuck := errors.New("the panel refused")
	p.stuck[1] = stuck
	p.install(t)
	var m Dimmer

	if err := m.Showing([]uint64{1}); err != nil {
		t.Fatalf("Showing: %v", err)
	}
	if err := m.Restore(); !errors.Is(err, stuck) {
		t.Errorf("Restore = %v, want the refusal from the panel", err)
	}
	// It is dropped even so: a way home that failed once is not worth keeping,
	// and holding it would make every later Restore fail the same way.
	if m.Dark() != 0 {
		t.Errorf("Dark = %d, want the failed one forgotten", m.Dark())
	}
}

func TestDimmerWithNothingOnTheRibbonDoesNothing(t *testing.T) {
	p := newPanels()
	p.install(t)
	var m Dimmer

	if err := m.Showing(nil); err != nil {
		t.Errorf("Showing(nil) = %v, want nothing to happen", err)
	}
	if len(p.order) != 0 {
		t.Error("a ribbon with no display on it changed a backlight")
	}
}

func TestPlatformDimAnswersEverywhere(t *testing.T) {
	// The real seam, not the fake: away from macOS it must refuse rather than
	// be absent, and on macOS a display id no machine has must not be dimmed.
	restore, err := platformDim(^uint64(0))
	if err == nil {
		t.Error("a display that does not exist was darkened")
	}
	if restore != nil {
		t.Error("platformDim handed back a way home for something it never changed")
	}
}

func TestDisplayOfReadsTheDisplayBackOutOfAnOfferID(t *testing.T) {
	if id, ok := DisplayOf(Offer{ID: "display-7", Kind: KindDisplay}); !ok || id != 7 {
		t.Errorf("DisplayOf = %d,%v, want 7,true", id, ok)
	}
	for _, o := range []Offer{
		{ID: "display-3", Kind: KindPanel},    // a rendered panel has no backlight
		{ID: "window-3", Kind: KindDisplay},   // not the shape Sources builds
		{ID: "display-3x", Kind: KindDisplay}, // not a number
		{ID: "display--1", Kind: KindDisplay}, // nor that
	} {
		if id, ok := DisplayOf(o); ok {
			t.Errorf("DisplayOf(%v) = %d,true, want false", o, id)
		}
	}
}

// sizes installs a machine whose displays have known geometry.
func sizes(t *testing.T, m map[uint64][2]int) {
	t.Helper()
	was := displaySize
	displaySize = func(id uint64) (int, int, bool) {
		wh, ok := m[id]
		return wh[0], wh[1], ok
	}
	t.Cleanup(func() { displaySize = was })
}

func offers(ids ...string) []Offer {
	out := make([]Offer, len(ids))
	for i, id := range ids {
		out[i] = Offer{ID: id, Kind: KindDisplay}
	}
	return out
}

func TestMirrorsLeavesOutTheScreensThatMustNotGoDark(t *testing.T) {
	sizes(t, map[uint64][2]int{
		1: {1512, 982},  // the Mac's own panel
		2: {2560, 1440}, // an external monitor
		3: {1920, 1080}, // the glasses
		4: {2056, 1156}, // one this program made
		// 5 is missing on purpose: a display nothing can measure
		6: {3840, 2160},
	})
	mine := glasses.Display{Width: 3840, Height: 2160, Scale: 2} // 1920x1080 in points

	got := Mirrors(offers("display-1", "display-2", "display-3", "display-4", "display-5", "display-6", "window-9"),
		[]uint64{4}, mine)
	want := []uint64{1, 2}
	if !sameIDs(got, want) {
		t.Errorf("Mirrors = %v, want %v", got, want)
	}
}

func TestMirrorsNamesAPanelOnceHoweverManyPositionsShowIt(t *testing.T) {
	sizes(t, map[uint64][2]int{1: {1512, 982}})
	got := Mirrors(offers("display-1", "display-1"), nil, glasses.Display{})
	if !sameIDs(got, []uint64{1}) {
		t.Errorf("Mirrors = %v, want the panel named once", got)
	}
}

func TestMirrorsMatchesTheChosenScreenAtItsDeclaredSizeToo(t *testing.T) {
	// Scale 1: the screen is described in the same units its rectangle is
	// measured in, and the division above must not be what saves it.
	sizes(t, map[uint64][2]int{1: {1920, 1080}})
	if got := Mirrors(offers("display-1"), nil, glasses.Display{Width: 1920, Height: 1080, Scale: 1}); len(got) != 0 {
		t.Errorf("Mirrors = %v, want the desk's own screen left alone", got)
	}
	// And the negative control: the same screen, a different desk.
	if got := Mirrors(offers("display-1"), nil, glasses.Display{Width: 3840, Height: 1080, Scale: 1}); !sameIDs(got, []uint64{1}) {
		t.Errorf("Mirrors = %v, want the panel darkened when it is not the desk's own", got)
	}
}

func TestPlatformDisplaySizeAnswersEverywhere(t *testing.T) {
	if _, _, ok := platformDisplaySize(^uint64(0)); ok {
		t.Error("a display that does not exist reported a size")
	}
}
