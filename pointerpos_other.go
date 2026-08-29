// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build !darwin

package desk

// displayUnderPointer answers that it cannot: only macOS is wired up, so the
// band does not follow a pointer it has no way to find.
func displayUnderPointer() (uint32, bool) { return 0, false }
