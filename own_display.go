// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import "github.com/go-widgets/window"

// OwnDisplay is the display id of the screen the desk's window is on.
//
// It is wiring: window.Screens says where a named screen IS, [DisplayAt] says
// which display that is, and both of those need a real window server in front
// of them. What is DONE with the answer -- never darkening that screen, and
// bringing a pointer that lands on it back onto the ribbon -- is decided in
// wrap.go and dim.go, where it can be tested.
func OwnDisplay(name string) (uint64, bool) {
	ss, err := window.Screens()
	if err != nil {
		return 0, false
	}
	for _, s := range ss {
		if s.Name != name {
			continue
		}
		return DisplayAt(float64(s.X), float64(s.Y), float64(s.Width), float64(s.Height))
	}
	return 0, false
}
