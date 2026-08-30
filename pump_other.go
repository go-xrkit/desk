// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build !darwin

package desk

// platformPump has nothing to give the main thread here: the menu-bar item is
// macOS's.
func platformPump() {}
