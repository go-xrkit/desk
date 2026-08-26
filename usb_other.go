// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build !darwin && (!linux || android)

package desk

import "github.com/go-xrkit/xrkit/glasses"

// Peripheral finds the headset on the USB bus, or returns nil.
//
// macOS and Linux are wired up. Windows has the same evidence in SetupAPI and
// Android behind a permission its host app must ask for; until those are done,
// the display name is what those platforms have, which is what they had before.
func Peripheral() *glasses.USB { return nil }
