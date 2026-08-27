// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

// The menu-bar item, and what it says.
//
// A desk in glasses is used while another application has the keyboard, so
// everything it can do is a system-wide shortcut. That is right for the things
// done constantly -- turn the band, open the gallery -- and wrong for the two
// things done once: change a setting, and stop. A shortcut for those is a
// combination to remember for something a menu can simply offer, and there is
// nowhere else to offer it from: the window covers a display entirely and
// belongs to the glasses, not to the desktop the person is looking at.
//
// So the desktop gets a menu-bar item. The rows are here, portable and
// testable; putting one in the menu bar is the platform's business
// (tray_darwin.go).

// TrayTitle is what the item says in the menu bar.
//
// Glasses, in one glyph. Text would be four characters of somebody's menu bar
// for something they use twice a session, and the menu bar is not ours to
// spend.
const TrayTitle = "\U0001F453"

// TrayQueue is how many menu choices are held for the run loop.
//
// It is small on purpose: the loop reads continuously while the desk is up, so
// the only time anything queues is while the settings window has the desk
// stopped -- and a person clicking the menu five times then means the fifth
// click, not five actions replayed when the desk comes back.
const TrayQueue = 4

// TrayRow is one row of the menu: what it says, its key equivalent, and the
// action it asks for. An ActionNone row is a separator.
type TrayRow struct {
	Title  string
	Key    string
	Action Action
}

// TrayRows is the menu, in order.
//
// Deliberately short. It is not a second copy of the keyboard: turning the band
// and moving in the gallery are done while looking at the band, by feel, and a
// row in a menu bar for either would be a row nobody uses. What is here is what
// a person cannot do from inside the glasses.
func TrayRows() []TrayRow {
	return []TrayRow{
		{Title: "Settings...", Key: ",", Action: ActionSettings},
		{},
		{Title: "Show the gallery", Action: ActionGalleryOpen},
		{Title: "Leave the gallery", Action: ActionGalleryClose},
		{},
		{Title: "Quit the desk", Key: "q", Action: ActionQuit},
	}
}

// Closer is what OpenTray returns: the item, to be taken out of the menu bar
// when the session ends.
//
// It is io.Closer by shape and not by import, so the portable half of this file
// carries no dependency on the platform half.
type Closer interface{ Close() error }
