// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-xrkit/xrkit/glasses"
)

// SysfsUSB is where Linux publishes what it enumerated.
const SysfsUSB = "/sys/bus/usb/devices"

// peripheralsFromSysfs lists every headset in a Linux USB tree.
//
// It reads three files per device and opens no device node, so it needs no
// permission and cannot disturb whatever holds the glasses — the same property
// the macOS side has, by a different road. Directories that are interfaces
// rather than devices simply have no idVendor and are skipped by the read
// failing, which is why nothing here tries to recognise a name shape.
func peripheralsFromSysfs(root string) []glasses.USB {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var out []glasses.USB
	for _, e := range entries {
		dir := filepath.Join(root, e.Name())
		vendor, ok := hexFile(filepath.Join(dir, "idVendor"))
		if !ok {
			continue
		}
		product, ok := hexFile(filepath.Join(dir, "idProduct"))
		if !ok {
			continue
		}
		name, _ := textFile(filepath.Join(dir, "product"))
		u := glasses.USB{Vendor: vendor, Product: product, Name: name, Hops: hopsFromSysfsName(e.Name())}
		// A product match only, for the reason [Peripheral] gives: a vendor
		// match names a brand, and a brand cannot give a field of view.
		if _, how := glasses.IdentifyDevice("", &u); how == glasses.ByUSBProduct {
			out = append(out, u)
		}
	}
	return out
}

// hexFile reads one of sysfs's four-digit hexadecimal ids.
func hexFile(path string) (uint16, bool) {
	s, ok := textFile(path)
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseUint(s, 16, 16)
	if err != nil {
		return 0, false
	}
	return uint16(n), true
}

// textFile reads a sysfs attribute, without its trailing newline.
func textFile(path string) (string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(b)), true
}

// billboardsFromSysfs lists the USB Billboard devices in a Linux USB tree.
//
// bDeviceClass is what says so: 0x11 is the class a USB-C device presents when
// an alternate mode it supports could not be entered. sysfs writes it as two
// hexadecimal digits, which hexFile reads like any other id.
func billboardsFromSysfs(root string) []glasses.USB {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var out []glasses.USB
	for _, e := range entries {
		dir := filepath.Join(root, e.Name())
		class, ok := hexFile(filepath.Join(dir, "bDeviceClass"))
		if !ok || class != usbClassBillboard {
			continue
		}
		vendor, ok := hexFile(filepath.Join(dir, "idVendor"))
		if !ok {
			continue
		}
		product, ok := hexFile(filepath.Join(dir, "idProduct"))
		if !ok {
			continue
		}
		name, _ := textFile(filepath.Join(dir, "product"))
		out = append(out, glasses.USB{Vendor: vendor, Product: product, Name: name})
	}
	return out
}

// usbClassBillboard is the USB device class a USB-C device presents when an
// ALTERNATE MODE it supports could not be entered.
//
// It is in the specification for exactly this: the device has no other way to
// tell anybody that the DisplayPort lanes it can drive are not being driven, so
// it enumerates as a Billboard whose only job is to carry that message. Every
// platform reads the same number, which is why it lives in the portable file.
const usbClassBillboard = 0x11

// hopsFromSysfsName counts how far a device is from the machine's own port,
// from the name Linux gives its directory.
//
// sysfs spells the topology: "1-2" is bus 1, port 2, plugged straight in, and
// "1-2.3" is behind one hub. So the hop count is one more than the number of
// dots. A name with no dash at all is a root hub, which is the machine's own
// port rather than anything in it: zero, meaning nothing to say.
//
// Anything from a colon on is cut first: "1-1:1.0" is an INTERFACE of the
// device at 1-1, and the dot in it numbers the interface rather than a hub.
func hopsFromSysfsName(name string) int {
	if colon := strings.IndexByte(name, ':'); colon >= 0 {
		name = name[:colon]
	}
	dash := strings.IndexByte(name, '-')
	if dash < 0 {
		return 0
	}
	return 1 + strings.Count(name[dash+1:], ".")
}
