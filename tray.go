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

// TraySymbol is the system symbol the item shows in the menu bar, and
// TrayLabel is what a screen reader says about it.
//
// A SYMBOL RATHER THAN AN EMOJI, and it was measured rather than argued.
// In the item's own strip of the menu bar:
//
//	emoji title 👓            79 pixels of ink
//	symbol eyeglasses        100
//	symbol rectangle.3.group 161
//	symbol visionpro         182
//	symbol display           206
//
// "the glasses icon in the tray is not very legible" is what the first of
// those looks like to somebody using it: a title is TEXT, so an emoji arrives
// at the height of a lowercase letter, flat in the bar's own ink, among twenty
// other items. A symbol is vector, aligned to the bar's cap height, and takes
// the bar's appearance in every theme.
//
// visionpro rather than the darkest of them: a headset says what this program
// is, where a monitor says what everything else is. eyeglasses is the shape of
// the old icon and half the ink, which is the trade this was reported for.
const (
	TraySymbol = "visionpro"
	TrayLabel  = "XR desk"
)

// TrayTitle is the desk in one glyph, for anywhere a picture will not do -- a
// log line, a window title, a person asking what to look for in their menu bar.
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
	// Symbol is the system symbol drawn to the left of the label, by the name
	// the platform knows it under -- an SF Symbol on macOS. Empty leaves the
	// row text-only, which is what a separator is and what every row was.
	//
	// A NAME RATHER THAN A PICTURE, for the same reason the menu-bar item
	// carries one: a symbol is the platform's own, so it arrives at the weight
	// and in the ink of every other menu in the bar, follows a light or dark
	// appearance without being asked, and costs no bytes in the binary. An icon
	// pack would be four hundred pixels of somebody else's drawing beside
	// nineteen of Apple's -- which is exactly the report the menu-bar icon was
	// already fixed for once.
	//
	// A platform with no such symbol draws the row as it always did.
	Symbol string

	// Toggle makes this row a checkbox: it carries a tick when what it controls
	// is on, and its action TOGGLES rather than setting.
	//
	// The state is not here, because a row is a description and the state
	// changes while the menu exists. [Tray.Show3D] is how the item is told.
	Toggle bool

	// SymbolOn is the symbol a toggling row shows when what it controls is ON.
	// Empty leaves [TrayRow.Symbol] in both states.
	//
	// ⛔ A TICK SAYS "ON" AND NOTHING SAYS "OFF". macOS draws a checkmark for a
	// menu item that is on and NOTHING AT ALL for one that is off, so an
	// unticked checkbox is indistinguishable from an ordinary row -- which is
	// how somebody came to ask "how do I know whether 3D is on? nothing in the
	// menu says". A symbol that CHANGES answers in both directions.
	SymbolOn string
}

// Stereo3D is what the menu should say about the 3D conversion.
//
// ⛔ THREE STATES AND NOT TWO. A display that shows one eye has nothing to
// convert to, and a depth model that will not load cannot convert -- so the
// conversion is not merely off, it is UNAVAILABLE, and a row that looks
// pressable is a row somebody presses again and again. On the headset this was
// reported from, the log said "3D asked for, but this display shows one eye"
// and the menu said nothing whatever.
type Stereo3D struct {
	// On is whether the conversion is running.
	On bool
	// Why, when not empty, is why it cannot be turned on here -- a sentence to
	// put in front of a person, not an error to parse. Empty means it can.
	//
	// A bare string and no Available() method beside it: there is one place
	// that asks, and it reads no better for a name.
	Why string
}

// TrayRows is the menu, in order.
//
// Deliberately short. It is not a second copy of the keyboard: turning the band
// and moving in the gallery are done while looking at the band, by feel, and a
// row in a menu bar for either would be a row nobody uses. What is here is what
// a person cannot do from inside the glasses.
func TrayRows() []TrayRow {
	return []TrayRow{
		{Title: "Settings...", Key: ",", Action: ActionSettings, Symbol: "gearshape"},
		{},
		{Title: "Bring the pointer to this screen", Key: "m", Action: ActionPoint,
			Symbol: "cursorarrow.rays"},
		{Title: "Bring the pointer back to this Mac", Key: "h", Action: ActionPointHome,
			Symbol: "arrow.uturn.backward"},
		{},
		{Title: "The applications...", Key: "a", Action: ActionAppsOpen,
			Symbol: "square.grid.2x2"},
		{Title: "One application per screen", Key: "x", Action: ActionSpread,
			Symbol: "rectangle.3.group"},
		{},
		// ⛔ ONE ROW, WITH A TICK. It was two -- "3D on" and "3D off" -- and
		// that is right for a KEY, which is pressed blind: a shortcut meaning
		// "on" from outside and "off" from inside does the wrong thing every
		// time somebody has lost track. A MENU is the opposite case. The state
		// is in front of the person as they choose, so two rows are two rows
		// where one of them always does nothing, and the tick says which.
		{Title: "3D", Action: ActionStereo3D, Toggle: true,
			// The pair the system itself uses, so the row says which of the two
			// the picture is in rather than only that it could be either.
			Symbol: "view.2d", SymbolOn: "view.3d"},
		{},
		// ⛔ THREE ROWS WITH A TICK, NOT ONE THAT CYCLES -- the same argument the
		// 3D row above makes, landing the other way round. A key is pressed
		// blind, so cycling is right for a key. A MENU is read while somebody
		// chooses, so three rows say where they are AND where they can go, and
		// the tick says which is in force. One cycling row would say neither.
		//
		// ⭐ THE GLASSES DO THE TRACKING THEMSELVES. Nothing here computes a
		// pose and no camera is opened: a VITURE Beast holds its own
		// orientation and composites the host's video where that says. It is a
		// command, which is why it is three rows rather than a project.
		{Title: "Anchor the picture in the room", Action: ActionTrackAnchored,
			Toggle: true, Symbol: "pin", SymbolOn: "pin.fill"},
		{Title: "Let it follow smoothly", Action: ActionTrackSmooth,
			Toggle: true, Symbol: "arrow.trianglehead.2.clockwise.rotate.90"},
		{Title: "Fix it to the glasses", Action: ActionTrackOff,
			Toggle: true, Symbol: "person.and.background.dotted"},
		// Not a toggle: recentring is something that HAPPENS, not a state, and a
		// tick on it would be a tick that never turns off.
		{Title: "Put it back in front of me", Action: ActionRecenter, Symbol: "scope"},
		// ⭐ THE CURVE, ASKED FOR AT THE GLASSES. On a flat band the screens off to
		// the side recede -- a plane seen obliquely is further away -- and the
		// further the head turns the worse it gets. Turning each screen towards
		// the viewer faces it at you instead. The actions existed and were
		// reachable from nowhere.
		//
		// ⛔⛔ IT DOES NOT PUT THEM ALL AT ONE DISTANCE, WHICH IS WHAT THIS
		// COMMENT USED TO CLAIM -- and believing it cost three days of hunting a
		// defect that was not there. Measured at the derived splay, six screens,
		// 51.57° per eye: the panel edges sit between 0.667 and 1.110, a 66%
		// spread. The panels tile edge to edge, so the chain is a polygon whose
		// size is fixed by how wide a screen is, and the viewer is not at its
		// centre.
		//
		// ⭐ AND THAT IS A CHOICE RATHER THAN A DEFECT: with tiling panels you may
		// have any two of {a free curvature dial, tiling, equal distance} and
		// never all three. The dial and the tiling are the two kept, decided
		// after the spread was measured. See [Plan.FacingSplayDeg].
		//
		// ⛔⛔ AND THEY ARE STEPS, WHICH THE FIRST NAMES DENIED. These were
		// "Curve the screens towards me" and "Flatten the screens" -- both
		// imperatives, both promising a STATE. They move by [SplayStep], five
		// degrees of the sixty available: twelve presses from flat to fully
		// turned. Somebody chose "Flatten the screens", got five degrees, and
		// reported that one press was not enough -- which was true of the name,
		// not of the feature. The code always called them "rounder" and
		// "flatter", comparatives, and saying so was the whole of the fix.
		//
		// ⛔ AND FLATTER COMES FIRST, BECAUSE ITS KEY IS THE LEFT ONE. The pair
		// sits on the two keys after P, and a menu that lists the RIGHT key's
		// row above the LEFT key's reads backwards to anybody looking at their
		// hands: "the minus has to be to the left of the plus". Listed the other
		// way round it was reported as inverted, and it was.
		{Title: "Flatter", Action: ActionFlatter, Symbol: "rectangle"},
		{Title: "More curved", Action: ActionRounder, Symbol: "arrow.left.and.right"},
		// ⚠ NAMED FOR THE COST AS WELL AS THE BENEFIT. Following a head means a
		// lit camera light for as long as it lasts, and a row that only promised
		// the benefit would be a row that surprised somebody.
		{Title: "Follow my head (camera on)", Action: ActionFollowHead,
			Toggle: true, Symbol: "eye", SymbolOn: "eye.fill"},
		{},
		{Title: "Show the gallery", Action: ActionGalleryOpen, Symbol: "square.grid.3x3"},
		{Title: "Leave the gallery", Action: ActionGalleryClose,
			Symbol: "arrow.down.right.and.arrow.up.left"},
		{},
		// The camera. One row, because there is one thing to do with it that a
		// person asks for deliberately -- and a camera on a headset points at
		// whatever they are looking at, so nothing here is ever automatic.
		{Title: "Take a photograph", Action: ActionPhoto, Symbol: "camera"},
		// ⭐ THE PICTURE THE GLASSES ARE SHOWING, not the camera. It is what makes
		// a geometry somebody cannot describe from inside a headset describable at
		// all -- and, the same picture, what lets them show a room what they see.
		{Title: "Photograph what I see", Action: ActionCapture, Symbol: "rectangle.dashed"},
		// The microphone. One row, and it says WHICH microphone once pressed:
		// the headset's own cannot be silenced at all, so what is turned off is
		// whatever else the machine listed, and a person has to be told which.
		{Title: "Mute the microphone", Action: ActionMic, Symbol: "mic.slash"},
		// The room, on the screen being looked at. One row: it is a switch, and
		// the tick says which way pressing it will go.
		{Title: "Show the room", Action: ActionPassthrough, Symbol: "video"},
		{},
		// ⛔ PUTTING THE GLASSES DOWN IS NOT QUITTING, and until this row
		// existed there was no way to say so: the only thing that ended a
		// ribbon ended the program with it, so a person who wanted their
		// keyboard back for ten minutes had to quit and set the desk up
		// again afterwards. Asked for in those words -- "garder l'app
		// xrdesk ouverte mais arreter d'utiliser des lunettes".
		//
		// A TICK rather than two rows, for the reason the 3D row is one row:
		// the state is in front of the person as they choose. And it is the
		// one row that has to work in BOTH states, because while the glasses
		// are down this menu is the only control left alive -- the shortcuts
		// went back to the rest of the machine when the ribbon came down.
		{Title: "Use the glasses", Action: ActionPause, Toggle: true,
			Symbol: "eyeglasses", SymbolOn: "eyeglasses"},
		{Title: "Quit the desk", Key: "q", Action: ActionQuit, Symbol: "power"},
	}
}

// Closer is what OpenTray returns: the item, to be taken out of the menu bar
// when the session ends.
//
// It is io.Closer by shape and not by import, so the portable half of this file
// carries no dependency on the platform half.
type Closer interface{ Close() error }
