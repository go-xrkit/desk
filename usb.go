// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import "github.com/go-xrkit/xrkit/glasses"

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
// So evidence is applied only where something actually ties it to this display:
// the display names a headset and one on the bus is that same model, or the
// person named the display and the bus holds exactly one headset. With two
// headsets attached and nothing to tell them apart, the answer is no evidence
// rather than a coin toss.
func EvidenceFor(d glasses.Display, named bool, us []glasses.USB) *glasses.USB {
	if len(us) == 0 {
		return nil
	}
	if p, ok := glasses.Identify(d.Name); ok {
		for i := range us {
			if q, how := glasses.IdentifyDevice("", &us[i]); how == glasses.ByUSBProduct && q.Model == p.Model {
				return &us[i]
			}
		}
		return nil
	}
	if named && len(us) == 1 {
		return &us[0]
	}
	return nil
}
