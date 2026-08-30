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
	// SAY WHETHER IT IS THERE, rather than that it was asked for.
	//
	// "je devrais voir le tray icon meme sans lunette" -- and the log said the
	// menu bar carried it while a person was looking at a menu bar that did
	// not. An item built in a process whose main thread never runs a loop is,
	// in go-macos's own words, indistinguishable from Go from one that works.
	// Since v0.2.0 it is not: OnScreen asks AppKit.
	// The symbol, with the emoji left as what it falls back to.
	if err := it.SetSymbol(TraySymbol, TrayLabel); err != nil {
		logf("keeping %s in the menu bar: %v", TrayTitle, err)
	}
	on, err := it.OnScreen()
	if err != nil {
		logf("the menu bar item cannot say whether it is there: %v", err)
	} else if !on {
		logf("the %s is NOT in the menu bar, though it was made: nothing has drawn it yet", TrayTitle)
	} else {
		logf("the menu bar carries the %s symbol with %d rows", TraySymbol, len(items))
	}
	return it, nil
}
