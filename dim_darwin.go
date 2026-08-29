// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build darwin

package desk

import "github.com/go-macos/brightness"

// platformDim turns a panel off through DisplayServices, which is a PRIVATE
// framework: it is looked up at run time and a system that has moved on says so
// rather than crashing. See go-macos/brightness.
func platformDim(id uint64) (func() error, error) { return brightness.Dim(uint32(id)) }
