// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build !darwin

package desk

// Permissions is empty away from macOS: nothing here is asked permission for.
//
// Empty rather than a list of things reported as held, because "granted" would
// be a claim about a system that was never asked.
func Permissions() []Grant { return nil }

// LogPermissions says nothing where there is nothing to say.
func LogPermissions(func(string, ...any)) {}
