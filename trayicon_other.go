// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build !darwin

package desk

import (
	"errors"

	"github.com/go-widgets/toolkit"
)

// platformTrayIcon has no system symbols to draw here, so the toolkit's own
// glasses are the icon.
func platformTrayIcon(int) ([]byte, error) {
	return nil, errors.New("desk: system symbols are macOS's")
}

// platformGlassesIcon has no system symbol to offer, so the caller keeps the
// toolkit's own drawn glasses.
func platformGlassesIcon(int) toolkit.IconFunc { return nil }
