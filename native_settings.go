// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import "github.com/go-widgets/toolkit"

// nativeSwitch is the platform's own switch, with the drawn one underneath it.
//
// ⛔ THE FALLBACK IS NOT DECORATION. A Native draws NOTHING and receives NO
// events while no host has claimed its region -- which is every platform
// without a native backend, every build without one, and the moment before the
// window is claimed. Without the drawn widget under it, the settings window
// would have two holes where its switches are, and nothing would say so.
//
// ⭐ AND THE TWO ARE BOUND BOTH WAYS. mvvm treats setting an equal value as a
// no-op, which is what keeps a two-way binding from looping: whichever of the
// two a person reaches, the other follows, and [settingsRoot] goes on reading
// one place.
func nativeSwitch(on bool) *toolkit.Native {
	n := toolkit.NewNativeSwitch(on)
	drawn := toolkit.NewSwitch(on)
	drawn.On().Subscribe(func(v bool) { n.On().Set(v) })
	n.On().Subscribe(func(v bool) { drawn.On().Set(v) })
	n.Fallback = drawn
	return n
}

// nativeButton is the platform's own button, with the drawn one underneath.
//
// One handler, given to both: a button has no state to keep in step, so the
// only thing that could drift is which of the two actually runs, and neither
// can run while the other is showing.
func nativeButton(title string, onClick func()) *toolkit.Native {
	n := toolkit.NewNativeButton(title, onClick)
	n.Fallback = toolkit.NewButton(title, onClick)
	return n
}
