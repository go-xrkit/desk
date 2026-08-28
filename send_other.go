// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build !darwin

package desk

import "github.com/go-macos/accessibility"

// noBench is a platform that cannot move another application's windows.
type noBench struct{}

// TheBench is the platform, for [Send].
//
// macOS is the only one wired up. Moving another application's window is the
// Accessibility API's job there; Linux has the same idea in each window
// manager's own protocol and Windows in SetWindowPos, and until those are
// written this answers that it cannot — which is a thing a person can be told,
// rather than a placement that silently does nothing.
func TheBench() Bench { return noBench{} }

func (noBench) Trusted() bool { return false }

func (noBench) Displays() ([]accessibility.Display, error) {
	return nil, accessibility.ErrUnsupported
}

func (noBench) Windows(string) ([]accessibility.Window, []string, error) {
	return nil, nil, accessibility.ErrUnsupported
}

// Listing answers that it cannot, like the rest of this platform's bench: a
// gallery of running applications is a gallery of things that could then be
// MOVED, and nothing here can move one.
func (noBench) Listing() ([]accessibility.WindowInfo, error) {
	return nil, accessibility.ErrUnsupported
}
