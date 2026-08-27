// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build darwin

package desk

import (
	"fmt"

	"github.com/go-macos/statusitem"
)

// OpenTray puts the item in the menu bar and sends what is chosen to actions.
//
// The send is NON-BLOCKING, and that is the whole of the design. A menu handler
// runs on its own goroutine while the desk may be stopped -- the settings window
// is up, or the ribbon is between sessions -- so nobody is reading. A blocking
// send would leave that goroutine parked for as long as the window is open and
// then replay every click at once. A dropped choice is logged and forgotten,
// which is what a person clicking a menu with nothing happening expects: the
// next click, not the last five.
func OpenTray(logf func(string, ...any), actions chan<- Action) (Closer, error) {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	rows := TrayRows()
	items := make([]statusitem.MenuItem, 0, len(rows))
	for _, r := range rows {
		if r.Action == ActionNone {
			items = append(items, statusitem.MenuItem{})
			continue
		}
		a, name := r.Action, r.Title
		items = append(items, statusitem.MenuItem{
			Title: r.Title,
			Key:   r.Key,
			Do: func() {
				select {
				case actions <- a:
				default:
					logf("%q was chosen while the desk was not listening; dropped", name)
				}
			},
		})
	}
	it, err := statusitem.New(TrayTitle, items)
	if err != nil {
		return nil, fmt.Errorf("desk: no menu-bar item: %w", err)
	}
	logf("the menu bar carries %s with %d rows", TrayTitle, len(items))
	return it, nil
}
