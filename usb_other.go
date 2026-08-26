// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build !darwin

package desk

import "github.com/go-xrkit/xrkit/glasses"

// Peripheral finds the headset on the USB bus, or returns nil.
//
// Only macOS is wired up so far. Linux can read the same evidence out of
// /sys/bus/usb and Windows out of SetupAPI; until they are, the display name is
// what these platforms have, which is what they had before.
func Peripheral() *glasses.USB { return nil }
