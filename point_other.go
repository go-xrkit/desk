// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build !darwin

package desk

import "errors"

// ErrNoPointer means this platform has no way for a program to move the mouse.
//
// Not a gap to be filled in silence: an X11 pointer is XWarpPointer, a Wayland
// one CANNOT be moved by a client at all, and a Windows one is SetCursorPos --
// three different answers with three different rules about who may ask. On the
// platforms where the desk shows the screens a machine already has, rather than
// ones it made, the pointer is on them anyway.
var ErrNoPointer = errors.New("desk: no way to move the pointer on this platform")

// BringPointer reports [ErrNoPointer].
func BringPointer([]uint64, int) error { return ErrNoPointer }

// PointerHome has nothing to remember here.
func PointerHome() func() { return func() {} }
