// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-xrkit/xrkit/glasses"
)

// What Await stopped for, when it did not stop because a display arrived.
var (
	// ErrAwaitQuit says the person asked to quit while waiting.
	ErrAwaitQuit = errors.New("desk: asked to quit while waiting for a display")
	// ErrAwaitSettings says the person asked for the settings while waiting.
	// The desk has nothing to show yet, and the settings are exactly where
	// somebody goes when the glasses are not being found.
	ErrAwaitSettings = errors.New("desk: asked for the settings while waiting for a display")
	// ErrAwaitResume says the person picked the glasses back up.
	//
	// It can only come out of a RESTING wait -- see [AwaitOptions.Resting] --
	// where the same menu row that put them down is what takes them up again.
	ErrAwaitResume = errors.New("desk: asked to use the glasses again")
)

// AwaitPoll is how often Await asks again.
//
// A second, because that is under the time it takes to look up after pushing a
// plug in, and because listing displays is cheap. Measured: a display created
// halfway through a process with no NSApp running appeared in the list within
// 500 ms, so a second is a poll that cannot miss it and not a poll that is
// waiting on a cache.
const AwaitPoll = time.Second

// AwaitOptions is what [Await] needs.
type AwaitOptions struct {
	// Want is the display or headset asked for: a -screen name, or the model
	// from the settings. Empty means "whatever is here", which never waits,
	// because [glasses.ChooseDisplay] always finds something to use.
	Want string
	// List answers what is attached NOW. Required.
	List func() ([]glasses.Display, error)
	// Actions is the menu bar, so a person is not stuck with a program that
	// only waits. Nil is allowed and means nothing can interrupt but the
	// context.
	Actions <-chan Action
	// Every overrides [AwaitPoll].
	Every time.Duration
	// Asked says the person has ALREADY been asked which headset to use, and
	// did not choose one.
	//
	// Without it a settings file naming a headset that is not here, and a
	// headset here that nobody has chosen, is a loop: ask, no answer, ask
	// again. Measured by the bench on an unattended machine -- the settings
	// window opening over and over in one session's log -- and it would look
	// the same to a person who closed it.
	//
	// Asked once, the answer is to get on with it: use the headset that is
	// here and say so.
	Asked bool

	// Resting says the person put the glasses DOWN and the program stayed.
	//
	// ⛔ A RESTING WAIT IGNORES THE DISPLAY. That is the whole difference:
	// the ordinary wait is looking for a headset to arrive and starts the
	// moment one does, which is exactly the wrong answer for somebody who
	// has just taken theirs off -- the glasses are still plugged in, so it
	// would start again at once and there would be no way to stop. So this
	// waits for a PERSON instead, through the menu bar, and returns
	// [ErrAwaitResume].
	//
	// Nothing on the machine is touched meanwhile: no window, no screens, and
	// no shortcuts, which is what gives ⌃⌥⌘ back to everything else.
	Resting bool

	// Logf says what is happening. Nil is silence.
	Logf func(string, ...any)
}

// Await waits for a display to show the desk on, and returns the one it found.
//
// It exists because the glasses are a cable, and a cable is plugged in when a
// person gets round to it. The desk used to print "no display matches" and exit,
// which meant the order of two actions mattered for no reason: plug in, then
// start. Now it starts, says what it is waiting for, and begins by itself.
//
// It returns AT ONCE when the display is already there, and logs nothing in that
// case: the normal path stays quiet.
//
// While waiting it answers the menu bar — [ErrAwaitQuit] and
// [ErrAwaitSettings] — because the shortcuts are claimed by the desk's window,
// which does not exist yet. Nothing is created and nothing on the machine is
// changed until a display is found.
func Await(ctx context.Context, opt AwaitOptions) (glasses.Display, error) {
	logf := opt.Logf
	if logf == nil {
		logf = func(string, ...any) {}
	}
	if opt.List == nil {
		return glasses.Display{}, errors.New("desk: Await needs a way to list displays")
	}
	every := opt.Every
	if every <= 0 {
		every = AwaitPoll
	}
	if opt.Resting {
		return glasses.Display{}, rest(ctx, opt.Actions, logf)
	}

	// said is the set of displays last reported, so plugging in a monitor that
	// is not the glasses says something and a second of nothing happening says
	// nothing.
	said := "\x00"
	// What the BUS says, refreshed less often than the display list.
	//
	// Listing displays reads a cache; enumerating USB opens a handle on every
	// device on the machine, and doing that once a second for as long as a
	// person leaves the desk waiting is not a read, it is a poke. Every fifth
	// poll is still well under the time it takes to look up after pushing a
	// plug in, and it is counted in POLLS rather than seconds so a test with a
	// fast poll sees the second reading without waiting for a clock.
	var bus, bill []glasses.USB
	for tick, waited := 0, false; ; tick, waited = tick+1, true {
		if tick%busEvery == 0 {
			bus, bill = onTheBus(), billboardsOnTheBus()
		}
		ds, err := opt.List()
		if err != nil {
			return glasses.Display{}, fmt.Errorf("desk: listing displays: %w", err)
		}
		chosen, why := glasses.ChooseDisplay(ds, opt.Want)
		if why == nil {
			if waited {
				logf("the display arrived: %s", chosen)
			}
			return chosen, nil
		}

		// THE MODEL IN THE SETTINGS IS A PREFERENCE, AND A HEADSET THAT IS NOT
		// IT IS A QUESTION.
		//
		// A person who chose a headset once should not be made to wait for that
		// headset for ever: measured, the settings named a "VITURE Luma Ultra",
		// the pair on the desk was a Beast, and the desk waited two and a half
		// minutes and created nothing with the glasses plugged in.
		//
		// But taking the other one silently is not right either -- "when I
		// plugged the glasses in I should have had a panel to choose which
		// glasses". Nobody has said anything about THIS pair, and a setting
		// that appears to be ignored is worse than one that is asked about. So
		// it asks, once, and the answer is written down: the next session with
		// the same glasses starts without a word.
		//
		// Recognising a headset is the catalogue's job, not a name written down
		// here: glasses.Headsets asks it.
		if hs := glasses.Headsets(ds); len(hs) > 0 && opt.Want != "" {
			logf("%v", why)
			if opt.Asked {
				// Asked once and nobody chose. Getting on with it beats asking
				// again, which is a loop.
				logf("nobody chose, so: %s", hs[0])
				return hs[0], nil
			}
			if len(hs) == 1 {
				logf("%s is here instead; asking which to use", hs[0])
			} else {
				logf("%d headsets are attached and none of them is the one chosen", len(hs))
			}
			return glasses.Display{}, ErrAwaitSettings
		}
		// The BUS is part of what changed, not only the displays: plugging a
		// headset in that presents no picture changes nothing about the display
		// list, and that is exactly the moment a person needs to be told
		// something. It used to say nothing at all.
		if now := describeDisplays(ds) + " | " + describeBus(bus, bill); now != said {
			said = now
			logf("%v", why)
			// THE CABLE CARRIES DATA AND NO PICTURE.
			//
			// A headset can be on the USB bus -- enumerated, named, its
			// product id recognised -- and present no display at all,
			// because the video half of USB-C is a separate negotiation.
			// Then "no display matches" is true and useless: it reads as
			// "your glasses are not plugged in" to somebody who has just
			// plugged them in and can feel the cable.
			//
			// Seen with an XREAL 1S on an M4 Max: 3318:043e on the bus,
			// and the only display macOS had was the monitor.
			for _, u := range bus {
				logf("  %q is on the USB bus but is not presenting a display", u.Name)
				logf("    the cable carries data; the picture is a separate negotiation over the same plug")
				logf("    the glasses have to be awake -- worn, or woken with their own button")
				// WHICH advice, decided by how far away the thing is.
				//
				// "a hub is in the way" is the usual cause and the useless
				// answer for the person whose glasses are already plugged
				// straight in: it sends them to check the one thing that is
				// right. The bus knows which case this is, so it says so.
				switch {
				case u.Hops == 1:
					logf("    these are plugged STRAIGHT into the machine, so no hub is in the way")
					logf("    what is left is the glasses being asleep, or a port or cable with no DisplayPort in it")
				case u.Hops > 1:
					logf("    these are %d hops away, behind %d hub(s): a hub that does not pass the "+
						"DisplayPort lanes stops the picture and not the data", u.Hops, u.Hops-1)
					logf("    plug them straight into the machine to take the hubs out of it")
				default:
					logf("    a hub, or a cable that only charges, stops the picture and not the data")
				}
			}
			// AND THE BUS ITSELF SAYS SO. A Billboard is not a guess: it is the
			// device announcing, in the only way the specification gives it,
			// that an alternate mode it supports was not entered. Measured
			// here with an XREAL 1S behind a chain of hubs: 2109:0103 class 17
			// "USB 2.0 BILLBOARD", and no display for the glasses at all.
			for _, b := range bill {
				logf("  %q (%04x:%04x) is a USB Billboard: whatever put it there supports a "+
					"DisplayPort alternate mode that was NOT entered", b.Name, b.Vendor, b.Product)
				logf("    the hardware is saying so itself; it is not being guessed from the silence")
				// WHOSE Billboard, by the vendor id.
				//
				// A machine with a dock has hubs on it, and a hub chip that
				// failed an alternate mode on ITS port says nothing about the
				// port the glasses are on. Saying "plug them in directly" to
				// somebody whose glasses ARE plugged in directly is worse than
				// saying nothing: it sends them to check the one thing that is
				// already right. So the vendor decides which sentence is true.
				if sameMakerAs(b, bus) {
					logf("    and it is the HEADSET's own: these glasses negotiated data and not video")
				} else {
					logf("    it belongs to ANOTHER device on the bus -- a hub or a dock, not the glasses")
					logf("    so it names a different port, and says nothing about the one the glasses are on")
				}
			}
			if !waited {
				// Said once, on the way in. A person who reads one line reads
				// this one.
				logf("waiting for it: plug the glasses in and the desk starts by itself")
				logf("  nothing has been created and nothing on this Mac has been changed")
				logf("  the %s in the menu bar still works: Settings, or Quit", TrayTitle)
			}
		}

		// ASLEEP, and that is right again. This used to spend the wait lending
		// AppKit the main thread in slices, because the menu-bar item needs a
		// run loop and there was none until a window opened. Owning a platform
		// loop is go-widgets's job, not this one's: the caller holds it now
		// (see Tray.Hold), which means THIS runs on a goroutine and must not
		// touch AppKit at all.
		select {
		case <-ctx.Done():
			return glasses.Display{}, ctx.Err()
		case a := <-opt.Actions:
			switch a {
			case ActionQuit:
				return glasses.Display{}, ErrAwaitQuit
			case ActionSettings:
				return glasses.Display{}, ErrAwaitSettings
			}
			// Anything else needs a desk to act on. Keep waiting.
		case <-time.After(every):
		}
	}
}

// describeDisplays names what is attached, in one line, for comparing one
// moment to the next.
func describeDisplays(ds []glasses.Display) string {
	if len(ds) == 0 {
		return "nothing"
	}
	parts := make([]string, len(ds))
	for i, d := range ds {
		parts[i] = d.String()
	}
	return strings.Join(parts, ", ")
}

// onTheBus is what the USB bus reports, as a seam.
//
// Above the platform boundary so a test on any platform can put a headset on
// the bus without one being plugged in, and so the waiting logic stays in the
// portable file where it is measured.
var onTheBus = Peripherals

// busEvery is how many polls apart the USB bus is read. See Await.
const busEvery = 5

// billboardsOnTheBus is what the USB bus reports as Billboard devices, as a
// seam, for the same reason onTheBus is one.
var billboardsOnTheBus = Billboards

// describeBus names what the bus carries, in one line, for comparing one moment
// to the next. Headsets and Billboards both: a headset appearing changes
// nothing about the display list, and neither does the Billboard that says why.
func describeBus(headsets, billboards []glasses.USB) string {
	if len(headsets) == 0 && len(billboards) == 0 {
		return "nothing"
	}
	parts := make([]string, 0, len(headsets)+len(billboards))
	for _, u := range headsets {
		parts = append(parts, fmt.Sprintf("%04x:%04x %q", u.Vendor, u.Product, u.Name))
	}
	for _, b := range billboards {
		parts = append(parts, fmt.Sprintf("billboard %04x:%04x %q", b.Vendor, b.Product, b.Name))
	}
	return strings.Join(parts, ", ")
}

// sameMakerAs reports whether a Billboard was put up by the same maker as one
// of the headsets on the bus.
//
// The vendor id is the whole of it: a Billboard carries no reference to what it
// is about, so the only thing tying one to a headset is that the same maker put
// both on the bus. It is evidence and not proof -- a maker who sells a dock as
// well would defeat it -- which is why what it changes is a sentence and not a
// decision.
func sameMakerAs(billboard glasses.USB, headsets []glasses.USB) bool {
	for _, h := range headsets {
		if h.Vendor == billboard.Vendor {
			return true
		}
	}
	return false
}

// rest waits for a person to pick the glasses back up.
//
// ⛔ IT DOES NOT LOOK AT THE DISPLAY, and that is the point. The glasses that
// were just put down are still plugged in: a wait that starts when a headset
// appears would start again in the same second, and there would be no way to
// stop short of quitting -- which is the thing this exists to avoid.
//
// So the only thing that ends it is the menu bar, which is also the only
// control still alive: the shortcuts went back to the rest of the machine when
// the ribbon came down, deliberately.
func rest(ctx context.Context, actions <-chan Action, logf func(string, ...any)) error {
	logf("the glasses are down; the menu bar picks them up again")
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case a := <-actions:
			switch a {
			case ActionQuit:
				return ErrAwaitQuit
			case ActionSettings:
				return ErrAwaitSettings
			case ActionPause:
				// The same row both ways. It is a switch, so the row that
				// says "the glasses are down" is the row that takes them up.
				return ErrAwaitResume
			}
			// Everything else needs a ribbon. There is none.
		}
	}
}
