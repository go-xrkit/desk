// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"sort"
	"testing"
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

func offers(ids ...string) []Offer {
	out := make([]Offer, len(ids))
	for i, id := range ids {
		out[i] = Offer{ID: id, Kind: KindDisplay}
	}
	return out
}

func TestMirrorsLeavesOutTheScreensThatMustNotGoDark(t *testing.T) {
	dt := &desktop{
		rects: map[uint64]rect{
			1:  {X: 0, Y: 0, W: 2056, H: 1329},      // this Mac's own panel
			2:  {X: 2056, Y: 0, W: 2560, H: 1440},   // an external monitor
			3:  {X: -13440, Y: 0, W: 1920, H: 1080}, // the glasses
			59: {X: -11520, Y: 0, W: 1920, H: 1080}, // one this program made
		},
	}
	dt.install(t)

	got := Mirrors(offers("display-1", "display-2", "display-3", "display-59", "window-9"),
		[]uint64{59}, 3)
	want := []uint64{1, 2}
	if !sameIDs(got, want) {
		t.Errorf("Mirrors = %v, want %v", got, want)
	}
	// The negative control: with no screen of its own to protect, the glasses'
	// display is an ordinary panel and is named like the rest.
	if got := Mirrors(offers("display-3"), nil, 0); !sameIDs(got, []uint64{3}) {
		t.Errorf("Mirrors = %v, want the panel named when it is nobody's own", got)
	}
}

func TestMirrorsNamesAPanelOnceHoweverManyPositionsShowIt(t *testing.T) {
	dt := &desktop{rects: map[uint64]rect{1: {X: 0, Y: 0, W: 2056, H: 1329}}}
	dt.install(t)
	got := Mirrors(offers("display-1", "display-1"), nil, 0)
	if !sameIDs(got, []uint64{1}) {
		t.Errorf("Mirrors = %v, want the panel named once", got)
	}
}

func TestDisplayAtIdentifiesAScreenByItsRectangleAndNotItsSize(t *testing.T) {
	// Two identical monitors: the same size, in different places. A size
	// cannot tell them apart and a rectangle can.
	dt := &desktop{
		rects: map[uint64]rect{
			7: {X: 0, Y: 0, W: 2560, H: 1440},
			8: {X: 2560, Y: 0, W: 2560, H: 1440},
		},
		all: []uint64{7, 8},
	}
	dt.install(t)

	if id, ok := DisplayAt(2560, 0, 2560, 1440); !ok || id != 8 {
		t.Errorf("DisplayAt = %d,%v, want 8,true", id, ok)
	}
	if id, ok := DisplayAt(99, 0, 2560, 1440); ok {
		t.Errorf("DisplayAt on a rectangle no display has = %d,true, want false", id)
	}
}
