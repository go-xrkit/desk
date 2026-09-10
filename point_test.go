// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"strings"
	"testing"
)

// TestWhatTheDeskSaysWhenThePointerComesHome.
//
// ⛔⛔ THE WAY OUT USED TO BE UNPLUGGING THE HEADSET. The desk's screens are real
// displays to the window server and they sit BESIDE the physical ones in its
// coordinate space, so the pointer can leave past the right-hand edge of the
// last real screen and land on one that only the glasses show. Wearing them,
// that is the whole point and the band follows it there. With them on the
// table: "je n'avais plus la sourie sur l'ecran physique odyssey, j'ai du
// debrancher les beast pour la recuperer".
//
// ⛔ AND IT IS SAID EVERY TIME, in all three outcomes. A key pressed blind --
// which this one always is, because it is pressed when the picture cannot be
// seen -- that says nothing is indistinguishable from a key that did nothing.
func TestWhatTheDeskSaysWhenThePointerComesHome(t *testing.T) {
	newDesk := func(home func() error) *Desk {
		d := &Desk{OnPointHome: home}
		d.Badge(1, nil, nil)
		return d
	}

	// Nothing wired to it: a platform with no way to move a pointer says so
	// rather than swallowing the key.
	d := newDesk(nil)
	d.Do(ActionPointHome)
	if !strings.Contains(d.notice.toast.Text, "move the pointer") {
		t.Errorf("a desk with no way to move the pointer said %q", d.notice.toast.Text)
	}

	// It refused: what it said is what is shown, because the two refusals mean
	// different things -- no display to move to, or no pointer to move.
	d = newDesk(func() error { return ErrNoHostDisplay })
	d.Do(ActionPointHome)
	if !strings.Contains(d.notice.toast.Text, "did not make") {
		t.Errorf("a refusal came out as %q", d.notice.toast.Text)
	}

	// It worked.
	moved := 0
	d = newDesk(func() error { moved++; return nil })
	d.Do(ActionPointHome)
	if moved != 1 {
		t.Errorf("the pointer was moved %d times", moved)
	}
	if !strings.Contains(d.notice.toast.Text, "back on this Mac") {
		t.Errorf("a move came out as %q", d.notice.toast.Text)
	}

	// ⛔ AND IT WORKS WITH A GALLERY OPEN, which is the state somebody lost
	// enough to press it may well be in. It is answered before every gallery
	// for exactly that reason; a way out that only works in some states is not
	// one.
	p := testPlan(t)
	real, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	defer real.Close()
	real.OnPointHome = func() error { moved++; return nil }
	real.Do(ActionGalleryOpen)
	moved = 0
	real.Do(ActionPointHome)
	if moved != 1 {
		t.Errorf("with the gallery open the pointer was moved %d times", moved)
	}
}
