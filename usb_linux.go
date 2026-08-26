// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build linux && !android

package desk

import "github.com/go-xrkit/xrkit/glasses"

// Peripheral finds the headset on the USB bus, or returns nil.
//
// Linux publishes what it enumerated under [SysfsUSB], so this reads three
// files and opens no device node: no permission, and no chance of disturbing
// whatever holds the glasses.
func Peripheral() *glasses.USB { return peripheralFromSysfs(SysfsUSB) }
