// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build linux && !android

package desk

import "github.com/go-xrkit/xrkit/glasses"

// Peripherals lists every headset on the USB bus.
//
// Linux publishes what it enumerated under [SysfsUSB], so this reads three
// files and opens no device node: no permission, and no chance of disturbing
// whatever holds the glasses.
func Peripherals() []glasses.USB { return peripheralsFromSysfs(SysfsUSB) }

// Billboards lists the USB Billboard devices on the bus -- the class a USB-C
// device presents when an alternate mode it supports could not be entered.
func Billboards() []glasses.USB { return billboardsFromSysfs(SysfsUSB) }
