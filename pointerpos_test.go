// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import "testing"

// onDisplay makes the pointer be on a display, for the duration of one test.
func onDisplay(t *testing.T, id uint32, ok bool) {
	t.Helper()
	was := underPointer
	underPointer = func() (uint32, bool) { return id, ok }
	t.Cleanup(func() { underPointer = was })
}

func TestPositionOfNamesTheRibbonPositionAndNotTheDisplay(t *testing.T) {
	ids := []uint64{101, 102, 103}

	onDisplay(t, 102, true)
	if pos, ok := PositionOf(ids); !ok || pos != 1 {
		t.Errorf("PositionOf = %d,%v, want 1,true", pos, ok)
	}
	onDisplay(t, 101, true)
	if pos, ok := PositionOf(ids); !ok || pos != 0 {
		t.Errorf("PositionOf = %d,%v, want 0,true", pos, ok)
	}
}

// TestAPointerSomewhereElseIsNotOnTheBand, which is an ordinary state and not a
// failure: the machine's own display is where the pointer spends most of its
// life.
func TestAPointerSomewhereElseIsNotOnTheBand(t *testing.T) {
	ids := []uint64{101, 102}

	onDisplay(t, 7, true) // a display this desk is not showing
	if _, ok := PositionOf(ids); ok {
		t.Error("a pointer on somebody else's display was called a position")
	}
	onDisplay(t, 0, false) // nothing could be read at all
	if _, ok := PositionOf(ids); ok {
		t.Error("an unreadable pointer was called a position")
	}
	// And a desk with no screens of its own: nothing to be on.
	onDisplay(t, 101, true)
	if _, ok := PositionOf(nil); ok {
		t.Error("a desk with no screens found the pointer on one")
	}
}
