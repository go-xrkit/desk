// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build !darwin

package desk

import "errors"

// ErrNoTray means this platform has no menu bar of the kind the desk puts an
// item in.
//
// It is not a gap to be filled in silence: a Linux desktop's notification area
// is a StatusNotifierItem over D-Bus and a Windows one is a shell notification
// icon, and both are real work with different semantics -- neither is "the same
// thing on another platform". The rows ([TrayRows]) are portable and waiting.
var ErrNoTray = errors.New("desk: no menu-bar item on this platform")

// OpenTray reports [ErrNoTray]. See tray_darwin.go for the one that works.
func OpenTray(logf func(string, ...any), _ chan<- Action) (Closer, error) {
	if logf != nil {
		logf("%v", ErrNoTray)
	}
	return nil, ErrNoTray
}
