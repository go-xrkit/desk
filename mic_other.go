// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build !darwin

package desk

// platformMicrophone reports that there is none.
//
// CoreAudio is a macOS framework and nothing here stands in for it. Saying so
// is what makes the key report "no microphone this can silence" rather than
// appear to work.
func platformMicrophone() (Microphone, error) { return nil, ErrNoMicrophone }
