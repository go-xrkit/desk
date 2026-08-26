// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"os"
	"path/filepath"
	"testing"
)

// sysfs builds a USB tree the way Linux publishes one. A nil value means the
// file is absent, which is how sysfs says an attribute does not apply.
func sysfs(t *testing.T, devices map[string]map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, attrs := range devices {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for file, content := range attrs {
			if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	return root
}

// TestTheLinuxBusNamesTheHeadset uses the real numbers off a pair of XREAL One
// S, in the exact shape sysfs writes them: lower-case hex, no 0x, one trailing
// newline.
func TestTheLinuxBusNamesTheHeadset(t *testing.T) {
	root := sysfs(t, map[string]map[string]string{
		// An interface directory: no idVendor at all.
		"1-1:1.0": {"bInterfaceClass": "03\n"},
		// A hub, which is not a headset.
		"1-1": {"idVendor": "2109\n", "idProduct": "2822\n", "product": "USB2.0 Hub\n"},
		// The glasses.
		"1-2": {"idVendor": "3318\n", "idProduct": "043e\n", "product": "XREAL 1S\n"},
	})
	us := peripheralsFromSysfs(root)
	if len(us) != 1 {
		t.Fatalf("found %d headsets, want the one", len(us))
	}
	u := us[0]
	if u.Vendor != 0x3318 || u.Product != 0x043e || u.Name != "XREAL 1S" {
		t.Errorf("found %04x:%04x %q", u.Vendor, u.Product, u.Name)
	}
}

// TestALinuxBusWithNoHeadset covers every way a directory is not the glasses.
func TestALinuxBusWithNoHeadset(t *testing.T) {
	for name, devices := range map[string]map[string]map[string]string{
		"nothing at all": {},
		"an interface, with no ids": {
			"1-1:1.0": {"bInterfaceClass": "03\n"},
		},
		"an id that is not hexadecimal": {
			"1-1": {"idVendor": "zzzz\n", "idProduct": "043e\n"},
		},
		"a product id that is not hexadecimal": {
			"1-1": {"idVendor": "3318\n", "idProduct": "zzzz\n"},
		},
		"a vendor and no product id": {
			"1-1": {"idVendor": "3318\n"},
		},
		// The brand is XREAL and the model is one the catalogue does not know.
		// A brand cannot give a field of view, so this is not a headset either.
		"the right brand, an unknown model": {
			"1-1": {"idVendor": "3318\n", "idProduct": "ffff\n", "product": "XREAL Something\n"},
		},
	} {
		if us := peripheralsFromSysfs(sysfs(t, devices)); len(us) != 0 {
			t.Errorf("%s: found %v", name, us)
		}
	}
}

// TestNoUSBTreeAtAll: a kernel without sysfs mounted, or a container that does
// not carry it, is a reason to know nothing and not a reason to fail.
func TestNoUSBTreeAtAll(t *testing.T) {
	if us := peripheralsFromSysfs(filepath.Join(t.TempDir(), "nothing here")); len(us) != 0 {
		t.Error("a tree that does not exist produced a headset")
	}
}

// TestAHeadsetThatDoesNotSayItsName: the product string is optional, and the
// product id alone names the model.
func TestAHeadsetThatDoesNotSayItsName(t *testing.T) {
	root := sysfs(t, map[string]map[string]string{
		"1-1": {"idVendor": "3318\n", "idProduct": "043e\n"},
	})
	us := peripheralsFromSysfs(root)
	if len(us) != 1 {
		t.Fatalf("found %d headsets without a product string, want the one", len(us))
	}
	if us[0].Name != "" {
		t.Errorf("name is %q, want empty", us[0].Name)
	}
}
