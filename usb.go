// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"strings"

	"github.com/go-xrkit/xrkit/glasses"
)

// EvidenceFor returns the bus evidence that belongs to this display, and nil
// when nothing ties any of it to this display.
//
// The bus says which headsets are ATTACHED. It does not say which display is
// which headset, and those are not the same claim. On the desk this was written
// at, a pair of XREAL One S enumerated over USB with no video link at all while
// a 7680x2160 monitor was the only display present: taking the model from the
// bus and the pixels from that monitor produced a plan for "XREAL 1S: 7 screens
// of 3840x2160", which is a headset's optics wrapped round somebody's desktop
// screen. A wrong field of view renders everything, in the wrong place, with no
// symptom — which is the whole reason the catalogue refuses to guess.
//
// So evidence is applied only where the DISPLAY ties it to itself: it names a
// headset, and one on the bus is that model — or names a brand, and one on the
// bus is of that brand. Anything else gets nothing, including a display the
// person named by hand: with two headsets attached and nothing to tell them
// apart, the answer is no evidence rather than a coin toss, and with one it is
// still a guess at a name that would be printed as fact.
func EvidenceFor(d glasses.Display, _ bool, us []glasses.USB) *glasses.USB {
	if len(us) == 0 {
		return nil
	}
	if p, ok := glasses.Identify(d.Name); ok {
		for i := range us {
			q, how := glasses.IdentifyDevice("", &us[i])
			if how != glasses.ByUSBProduct {
				continue
			}
			if q.Model == p.Model {
				return &us[i]
			}
			// The display named a BRAND and the bus names a model of it. This is
			// the case the bus exists for: a pair of VITURE Luma Ultra puts the
			// bare word "VITURE" in its EDID, which the catalogue holds as a
			// brand with no optics at all — it cannot say what to render —
			// while the bus says "VITURE Luma Ultra", which it can. Requiring
			// the two strings to be equal would refuse the one refinement that
			// matters.
			//
			// The brand must still be POSITIVE evidence: the display's own name
			// has to begin the model the bus names. "VITURE" begins "VITURE
			// Luma Ultra"; "Odyssey G95NC" begins nothing, which is what keeps a
			// monitor from being handed a headset's optics.
			if !p.Known() && strings.HasPrefix(
				strings.ToLower(q.Model), strings.ToLower(strings.TrimSpace(d.Name))) {
				return &us[i]
			}
		}
		return nil
	}
	// A display whose own name says nothing gets nothing, even when the person
	// named it and there is exactly one headset on the bus.
	//
	// That shortcut used to earn its keep: the field of view decided the
	// geometry, so pairing an unnamed display with the only headset attached was
	// the difference between running and refusing. It no longer decides
	// anything, and what is left of it is a GUESS at a name — which then gets
	// printed as a model and a figure, as fact. A person who wants a particular
	// headset named can say so in the settings.
	return nil
}
