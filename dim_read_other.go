// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build !darwin

package desk

func platformDimRead(uint64) (float64, error) { return 0, ErrNoDimming }
func platformDimSet(uint64, float64) error    { return ErrNoDimming }
