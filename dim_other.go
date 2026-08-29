// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build !darwin

package desk

// platformDim answers that it cannot, rather than being absent: the ribbon runs
// on this platform, it simply cannot turn a backlight off there.
func platformDim(uint64) (func() error, error) { return nil, ErrNoDimming }
