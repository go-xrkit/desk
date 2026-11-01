// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build !android

package desk

import (
	"context"

	"github.com/go-widgets/window"
	"github.com/go-xrkit/xrkit/glasses"
)

// ListDisplays answers with every display attached, as the chooser reads them.
//
// Everywhere but Android that is the window system's answer, which is also the
// one the full-screen window will be placed against — so the list and the
// placement cannot disagree about what is attached.
//
// ⚠ ctx is accepted and not used. The back-ends read a cache the window server
// keeps and never block; the parameter is here because the Android sibling has
// a socket to talk to, and a signature that differs per platform would push the
// difference back into every caller.
func ListDisplays(_ context.Context) ([]glasses.Display, error) {
	ss, err := window.Screens()
	if err != nil {
		return nil, err
	}
	return displaysFromScreens(ss.All()), nil
}

// displaysFromScreens is the mapping, apart from the call that fetches the
// list, so a test can hand it displays that are not attached to this machine —
// which is the only way to pin the field that was being dropped.
func displaysFromScreens(all []window.Screen) []glasses.Display {
	ds := make([]glasses.Display, len(all))
	for i, s := range all {
		ds[i] = glasses.Display{
			Name:    s.Name,
			Width:   s.Width,
			Height:  s.Height,
			Primary: s.Primary,
			// ⛔⛔ SCALE USED TO BE DROPPED HERE, AND IT KILLED A WARNING THAT
			// MATTERS. The two call sites in cmd/xrdesk built this value field
			// by field and left Scale out, so it was always 0 — and
			// glasses.ScalingAdvice opens with `if d.Scale <= 1 { return "" }`.
			//
			// So `if advice := glasses.ScalingAdvice(chosen); advice != ""` could
			// not fire, ever, on any platform. The advice it was meant to carry
			// is not cosmetic: a VITURE Beast was observed reporting "5120x1600
			// rendered, looks like 2560x800" against a 3840x1080 panel — three
			// quarters of the rendered pixels downsampled away, on glasses where
			// a scaled mode buys nothing at all.
			//
			// Go says nothing about a struct field left unset, which is why this
			// built, vetted, shipped and stayed silent. The same shape as
			// -distance and -splay being threaded through four call sites and
			// never read (see run_display.go).
			Scale: s.Scale,
		}
	}
	return ds
}
