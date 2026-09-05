// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build !darwin

package desk

import "fmt"

// platformSet3D has no way to reach the headset here: the control interface is
// spoken to through IOKit, which is macOS's.
func platformSet3D(bool) error {
	return fmt.Errorf("%w: the glasses are only reachable from macOS", ErrNoGlasses3D)
}

// platformGlassesGet and platformGlassesSet have no headset to reach here.
func platformGlassesGet(byte) (uint16, error) {
	return 0, fmt.Errorf("%w: the glasses are only reachable from macOS", ErrNoGlasses3D)
}

func platformGlassesSet(byte, uint16) error {
	return fmt.Errorf("%w: the glasses are only reachable from macOS", ErrNoGlasses3D)
}
