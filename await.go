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
)

// pump is the seam: the platform gives AppKit the main thread for a moment, and
// a test replaces it.
var pump = platformPump

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

	// said is the set of displays last reported, so plugging in a monitor that
	// is not the glasses says something and a second of nothing happening says
	// nothing.
	said := "\x00"
	for waited := false; ; waited = true {
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
		// But taking the other one silently is not right either -- "quand j'ai
		// branché les lunettes j'aurais dû avoir un panneau de choix des
		// lunettes". Nobody has said anything about THIS pair, and a setting
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
		if now := describeDisplays(ds); now != said {
			said = now
			logf("%v", why)
			if !waited {
				// Said once, on the way in. A person who reads one line reads
				// this one.
				logf("waiting for it: plug the glasses in and the desk starts by itself")
				logf("  nothing has been created and nothing on this Mac has been changed")
				logf("  the %s in the menu bar still works: Settings, or Quit", TrayTitle)
			}
		}

		// The wait is spent INSIDE AppKit, not asleep beside it.
		//
		// A menu is not drawn, it is TRACKED: the press has to be delivered
		// and then AppKit runs its own loop until the person lets go. Lending
		// it twenty milliseconds a second was enough to draw the item in the
		// menu bar and not enough to open it -- "l'icon est visible dans le
		// tray mais je n'ai aucun menu" -- because the press landed in one of
		// the gaps.
		//
		// So the poll interval is spent pumping, in slices, and the channels
		// are looked at between slices rather than waited on.
		done := time.Now().Add(every)
		for {
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
			default:
			}
			if !time.Now().Before(done) {
				break
			}
			pump()
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
