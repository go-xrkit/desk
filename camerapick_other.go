// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build !darwin

package desk

// HeadsetCamera has no bus to ask here.
//
// Empty means "the first the machine lists", which is what the photograph key
// did everywhere before this: no worse than it was, and honest about why.
func HeadsetCamera(string) string { return "" }
