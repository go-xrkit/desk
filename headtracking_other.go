// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build !darwin

package desk

import "errors"

// ErrNoCameraHead is returned where no camera can be opened at all.
//
// ⛔ IT REFUSES RATHER THAN RETURNING SOMETHING THAT NEVER MOVES. A head tracker
// that silently reports stillness is indistinguishable from one whose owner is
// sitting still, and the difference is the whole of what a caller needs.
var ErrNoCameraHead = errors.New("desk: following the head needs a camera, and this platform has no way to open one")

// CameraHead is the shape the darwin build fills in.
type CameraHead struct{}

// OpenCameraHead refuses on platforms with no camera support.
func OpenCameraHead(string) (*CameraHead, error) { return nil, ErrNoCameraHead }

// Yaw never moves, and says so.
func (h *CameraHead) Yaw() (float64, bool) { return 0, false }

// Blind is large enough that a caller treats it as lost rather than as a blink.
func (h *CameraHead) Blind() int { return blindEnoughToSaySo }

// Recenter has nothing to recentre.
func (h *CameraHead) Recenter() {}

// Close has nothing to close.
func (h *CameraHead) Close() error { return nil }
