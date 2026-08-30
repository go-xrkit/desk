// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build darwin

package desk

import "github.com/go-macos/brightness"

func platformDimRead(id uint64) (float64, error)    { return brightness.Of(uint32(id)) }
func platformDimSet(id uint64, level float64) error { return brightness.Set(uint32(id), level) }
