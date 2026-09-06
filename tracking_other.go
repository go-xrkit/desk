// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build !darwin

package desk

import "errors"

// SetTracking reports that there is nothing to ask away from macOS.
func SetTracking(int, bool) error {
	return errors.New("desk: the headset's own tracking is reached through IOKit, which is macOS")
}

// ReadTracking says why the rows are not available, rather than showing three
// rows none of which is ticked.
func ReadTracking() Tracking {
	return Tracking{Why: "the headset's tracking is reached through IOKit, which is macOS"}
}
