// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-xrkit/android"
	"github.com/go-xrkit/xrkit/glasses"
)

// ListDisplays answers with every display attached, as the chooser reads them.
//
// On Android that is the HOST's answer and not a window server's: see the long
// note in listdisplays.go for the run where this was asked of X11 on a phone.
func ListDisplays(ctx context.Context) ([]glasses.Display, error) {
	if !android.Available() {
		return nil, errors.New("desk: this process was not started by the Android host, " +
			"so there is nobody to ask which displays are attached")
	}
	ds, err := android.Screens(ctx)
	if err != nil {
		return nil, fmt.Errorf("desk: asking the host for the displays: %w", err)
	}
	return displaysFromHost(ds), nil
}
