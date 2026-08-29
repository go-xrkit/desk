// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build darwin

package desk

import (
	"github.com/go-macos/brightness"
	"github.com/go-macos/pointer"
)

// platformDim turns a panel off through DisplayServices, which is a PRIVATE
// framework: it is looked up at run time and a system that has moved on says so
// rather than crashing. See go-macos/brightness.
func platformDim(id uint64) (func() error, error) { return brightness.Dim(uint32(id)) }

// platformDisplaySize answers from the window server's own geometry, in points.
func platformDisplaySize(id uint64) (w, h int, ok bool) {
	r, err := pointer.Bounds(uint32(id))
	if err != nil {
		return 0, 0, false
	}
	return int(r.W), int(r.H), true
}
