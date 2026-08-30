// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build !darwin

package desk

// Nothing here can move a pointer, so the ends of the ribbon are simply walls.
func platformPointerAt() (x, y float64, ok bool) { return 0, 0, false }
func platformDisplayRect(uint64) (rect, bool)    { return rect{}, false }
func platformDisplays() []uint64                 { return nil }
func platformMovePointer(float64, float64) error { return ErrNoPointer }
