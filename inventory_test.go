// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"strings"
	"testing"
)

func threeOffers() []Offer {
	return []Offer{
		{ID: "d1", Name: "Built-in", Kind: KindDisplay, W: 1920, H: 1080},
		{ID: "d2", Name: "Odyssey", Kind: KindDisplay, W: 3840, H: 1080},
		{ID: "p1", Name: "a web page", Kind: KindPanel},
	}
}

func TestNewInventoryRefusesWhatCannotBeChosenBetween(t *testing.T) {
	if _, err := NewInventory(0, nil); !errors.Is(err, ErrPosition) {
		t.Errorf("no positions: err = %v, want ErrPosition", err)
	}
	if _, err := NewInventory(-1, nil); !errors.Is(err, ErrPosition) {
		t.Errorf("negative positions: err = %v, want ErrPosition", err)
	}
	// An identifier that names two things cannot be used to choose between them.
	dup := []Offer{{ID: "a", Name: "one"}, {ID: "a", Name: "two"}}
	if _, err := NewInventory(2, dup); !errors.Is(err, ErrNoSuchOffer) {
		t.Errorf("duplicate id: err = %v, want ErrNoSuchOffer", err)
	}
	for _, bad := range []string{"", "   ", "\t"} {
		if _, err := NewInventory(2, []Offer{{ID: bad, Name: "nameless"}}); !errors.Is(err, ErrNoSuchOffer) {
			t.Errorf("id %q: err = %v, want ErrNoSuchOffer", bad, err)
		}
	}
	// An inventory with nothing to offer is legitimate: a machine may have
	// nothing capturable yet.
	inv, err := NewInventory(3, nil)
	if err != nil {
		t.Fatalf("empty inventory: %v", err)
	}
	if inv.Positions() != 3 || len(inv.Offers()) != 0 {
		t.Errorf("empty inventory has %d positions and %d offers", inv.Positions(), len(inv.Offers()))
	}
}

func TestAssignAndClear(t *testing.T) {
	inv, err := NewInventory(3, threeOffers())
	if err != nil {
		t.Fatalf("NewInventory = %v", err)
	}
	if _, ok := inv.At(0); ok {
		t.Error("a new position already holds something")
	}
	if err := inv.Assign(0, "d1"); err != nil {
		t.Fatalf("Assign = %v", err)
	}
	o, ok := inv.At(0)
	if !ok || o.ID != "d1" {
		t.Errorf("At(0) = %v/%v, want d1", o, ok)
	}
	if got := inv.Where("d1"); got != 0 {
		t.Errorf("Where(d1) = %d, want 0", got)
	}
	if got := inv.Where("d2"); got != -1 {
		t.Errorf("Where(d2) = %d, want -1 for something not shown", got)
	}
	if got := inv.Where(""); got != -1 {
		t.Errorf("Where(\"\") = %d; an empty position is not a source", got)
	}

	// Clearing is idempotent: a viewer pressing the same key twice meant it
	// both times.
	for i := 0; i < 2; i++ {
		if err := inv.Clear(0); err != nil {
			t.Fatalf("Clear %d = %v", i, err)
		}
	}
	if _, ok := inv.At(0); ok {
		t.Error("the position still holds something after being cleared")
	}
}

// TestASourceIsOnAtMostOnePosition. Two positions showing the same display would
// each cost a capture of the same pixels, and a viewer turning the ribbon would
// pass the same screen twice and wonder which is real.
func TestASourceIsOnAtMostOnePosition(t *testing.T) {
	inv, _ := NewInventory(3, threeOffers())
	if err := inv.Assign(0, "d1"); err != nil {
		t.Fatalf("Assign = %v", err)
	}
	err := inv.Assign(1, "d1")
	if !errors.Is(err, ErrAlreadyShown) {
		t.Fatalf("Assign elsewhere = %v, want ErrAlreadyShown", err)
	}
	if !strings.Contains(err.Error(), "position 0") {
		t.Errorf("the refusal does not say where it already is: %v", err)
	}
	// Assigning it to the position it is already on is not a move, and must be
	// allowed — it is what a picker does when the viewer re-picks the same one.
	if err := inv.Assign(0, "d1"); err != nil {
		t.Errorf("re-assigning to the same position = %v", err)
	}
}

func TestAssignRefusesNonsense(t *testing.T) {
	inv, _ := NewInventory(2, threeOffers())
	for _, pos := range []int{-1, 2, 99} {
		if err := inv.Assign(pos, "d1"); !errors.Is(err, ErrPosition) {
			t.Errorf("Assign(%d) = %v, want ErrPosition", pos, err)
		}
		if err := inv.Clear(pos); !errors.Is(err, ErrPosition) {
			t.Errorf("Clear(%d) = %v, want ErrPosition", pos, err)
		}
		if _, ok := inv.At(pos); ok {
			t.Errorf("At(%d) returned something", pos)
		}
	}
	if err := inv.Assign(0, "nope"); !errors.Is(err, ErrNoSuchOffer) {
		t.Errorf("Assign of an unknown source = %v, want ErrNoSuchOffer", err)
	}
}

// TestCycleWalksEverythingAndEnds is the sequence that makes "choose what this
// screen shows" reachable with one key.
func TestCycleWalksEverythingAndEnds(t *testing.T) {
	inv, _ := NewInventory(3, threeOffers())

	want := []string{"d1", "d2", "p1"}
	for i, id := range want {
		o, ok := inv.Cycle(0)
		if !ok {
			t.Fatalf("step %d: Cycle emptied the position early", i)
		}
		if o.ID != id {
			t.Fatalf("step %d: Cycle gave %q, want %q — it must walk the offers in their own order", i, o.ID, id)
		}
	}
	// After the last one it empties, rather than wrapping straight round: a
	// viewer holding the key needs a way to arrive at "nothing".
	if _, ok := inv.Cycle(0); ok {
		t.Error("Cycle did not pass through empty after the last source")
	}
	if _, ok := inv.At(0); ok {
		t.Error("the position is not empty after cycling past the end")
	}
	// And then round again.
	if o, ok := inv.Cycle(0); !ok || o.ID != "d1" {
		t.Errorf("Cycle after empty = %v/%v, want d1 again", o, ok)
	}
}

// TestCycleSkipsWhatIsAlreadyShownElsewhere: the exclusivity rule has to hold
// through the one-key path too, or cycling would produce the duplicate that
// Assign refuses.
func TestCycleSkipsWhatIsAlreadyShownElsewhere(t *testing.T) {
	inv, _ := NewInventory(3, threeOffers())
	if err := inv.Assign(1, "d2"); err != nil {
		t.Fatalf("Assign = %v", err)
	}
	o, ok := inv.Cycle(0)
	if !ok || o.ID != "d1" {
		t.Fatalf("first Cycle = %v/%v, want d1", o, ok)
	}
	o, ok = inv.Cycle(0)
	if !ok || o.ID != "p1" {
		t.Errorf("second Cycle = %v/%v, want p1 — d2 is on position 1 and must be skipped", o, ok)
	}
	if got := inv.Where("d2"); got != 1 {
		t.Errorf("cycling moved d2: Where(d2) = %d", got)
	}
}

func TestCycleRefusesAPositionThatIsNotThere(t *testing.T) {
	inv, _ := NewInventory(2, threeOffers())
	for _, pos := range []int{-1, 2} {
		if o, ok := inv.Cycle(pos); ok {
			t.Errorf("Cycle(%d) = %v", pos, o)
		}
	}
}

// TestCycleOnAnEmptyInventoryStaysEmpty covers the machine that has nothing to
// show yet — a start-up state, not an error.
func TestCycleOnAnEmptyInventoryStaysEmpty(t *testing.T) {
	inv, _ := NewInventory(2, nil)
	if o, ok := inv.Cycle(0); ok {
		t.Errorf("Cycle on an empty inventory gave %v", o)
	}
}

func TestUnused(t *testing.T) {
	inv, _ := NewInventory(3, threeOffers())
	if got := len(inv.Unused()); got != 3 {
		t.Errorf("nothing assigned: Unused() has %d, want 3", got)
	}
	_ = inv.Assign(0, "d2")
	unused := inv.Unused()
	if len(unused) != 2 || unused[0].ID != "d1" || unused[1].ID != "p1" {
		t.Errorf("Unused() = %v, want d1 and p1 in the order offered", unused)
	}
}

func TestDescribe(t *testing.T) {
	inv, _ := NewInventory(2, threeOffers())
	_ = inv.Assign(0, "d1")
	s := inv.Describe()
	for _, want := range []string{"1: display", "Built-in", "1920x1080", "2: empty", "not shown"} {
		if !strings.Contains(s, want) {
			t.Errorf("Describe() is missing %q:\n%s", want, s)
		}
	}
	// With everything shown there is nothing to list as not shown.
	inv, _ = NewInventory(3, threeOffers())
	_ = inv.Assign(0, "d1")
	_ = inv.Assign(1, "d2")
	_ = inv.Assign(2, "p1")
	if s := inv.Describe(); strings.Contains(s, "not shown") {
		t.Errorf("Describe() lists nothing-not-shown:\n%s", s)
	}
}

func TestOfferAndKindStrings(t *testing.T) {
	if got := (Offer{Name: "x", Kind: KindDisplay, W: 800, H: 600}).String(); !strings.Contains(got, "800x600") {
		t.Errorf("Offer.String() = %q, want the size", got)
	}
	// A panel has no size until it is opened, and must not claim 0x0.
	if got := (Offer{Name: "x", Kind: KindPanel}).String(); strings.Contains(got, "0x0") {
		t.Errorf("Offer.String() = %q, want no size at all", got)
	}
	for k, want := range map[Kind]string{KindDisplay: "display", KindPanel: "panel", Kind(9): "unknown"} {
		if got := k.String(); got != want {
			t.Errorf("Kind(%d).String() = %q, want %q", k, got, want)
		}
	}
}

func TestWithoutKeepsTheDesksOwnScreenOutOfItsOwnBand(t *testing.T) {
	offers := []Offer{
		{ID: "display-101", Name: "XR screen 1", Kind: KindDisplay},
		{ID: "display-3", Name: "display 3", Kind: KindDisplay},
		{ID: "display-1", Name: "display 1 (main)", Kind: KindDisplay, Main: true},
		{ID: "panel-7", Name: "a panel", Kind: KindPanel},
	}
	got := Without(offers, 3)
	if len(got) != 3 {
		t.Fatalf("Without left %d offers, want 3", len(got))
	}
	for _, o := range got {
		if o.ID == "display-3" {
			t.Error("the display the desk is on is still on offer")
		}
	}
	// A panel has no display behind it and is not touched by a display id.
	if got[2].ID != "panel-7" {
		t.Errorf("the last offer is %q, want the panel left alone", got[2].ID)
	}
	// Nothing to leave out, and nothing that CAN be left out, both give the
	// list back rather than a copy missing something.
	for _, c := range []struct {
		name string
		ids  []uint64
	}{
		{"no displays named", nil},
		{"only a zero, which is no display", []uint64{0}},
	} {
		if got := Without(offers, c.ids...); len(got) != len(offers) {
			t.Errorf("%s: Without left %d offers, want all %d", c.name, len(got), len(offers))
		}
	}
	// And an offer this cannot read a display out of stays: leaving out what
	// you do not understand is how a band ends up with holes in it.
	odd := []Offer{{ID: "not-a-display", Kind: KindDisplay}}
	if got := Without(odd, 3); len(got) != 1 {
		t.Errorf("Without dropped an offer it could not read: %v", got)
	}
}
