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
		u := glasses.USB{Vendor: vendor, Product: product, Name: name}
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
