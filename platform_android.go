// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/go-xrkit/android"
)

// Screens is what the ribbon will show, and how it was obtained.
//
// On Android it is the phone's own screen, and that is not a shortcoming of this
// code — it is settled. An ordinary application may create a virtual display,
// but the display comes back WITHOUT the trusted flag and launching anything
// onto it, even the application's own activity, is refused. Asking for the
// trusted flag wants a permission whose protection level is `signature`, and the
// flag is not in the public SDK at all. So the ribbon carries a mirror of the
// phone plus whatever this application draws itself, which is also what the
// manufacturer's own software does here.
type Screens struct {
	// IDs identify the displays, in ribbon order.
	IDs []uint64
	// Virtual reports whether these were created by us. Never true here.
	Virtual bool
	// granted carries the answer to a consent dialog nobody has answered yet,
	// so the desk does not wait on a person. Nil when there was nothing to ask.
	// See Provide.
	granted chan bool

	// Settled says that waiting will not change this answer.
	//
	// ⛔⛔ WITHOUT IT, A macOS POLICY RAN ON A PHONE FOR FIFTEEN SECONDS. The
	// caller reads `!Virtual` as "the window server refused, and it may
	// relent" — true on macOS, where a Mac that has just made six displays
	// refuses the next for the best part of a minute — so it retried five
	// times, three seconds apart, printing the refusal each round. Measured on
	// a Pixel 11 Pro Fold on 2026-11-01, 11:55:51 to 11:56:06:
	//
	//	Android does not let an ordinary application put other apps on a
	//	display it creates, so the ribbon shows this phone's own screen
	//	waiting 3s for the window server, and asking again (1 of 5)
	//	… (2 of 5) … (3 of 5) … (4 of 5) … (5 of 5)
	//
	// Here `!Virtual` is not a refusal to wait out: it is the platform's
	// settled answer, asked directly and refused four different ways (see the
	// type comment above). Waiting for it is waiting for Android to change.
	//
	// ⚠ The field is on every platform's Screens, and its zero value means
	// "keep waiting" DELIBERATELY: a platform that forgets to set it behaves as
	// it does today, which is slow rather than wrong. Losing macOS's retry
	// would cost somebody a bad desk, and that is the worse way to be wrong.
	Settled bool
	// Why explains what happened, and is worth showing to a person.
	Why string
}

// Close releases anything this created. There is nothing to release.
func (s *Screens) Close() error { return nil }

// Release is Close here: nothing was created, so nothing has to go.
func (s *Screens) Release() error { return nil }

// Provide gets the ribbon its screens.
//
// Consent is asked for HERE rather than at the first capture. MediaProjection
// puts a system dialog in front of the person, once per session and never
// remembered, and a dialog that appears in the middle of setting up a desk reads
// as a fault. Asking while nothing is on screen yet reads as a question.
func Provide(ctx context.Context, plan Plan, logf func(string, ...any)) (*Screens, error) {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	if !android.Available() {
		return nil, errors.New("desk: this process was not started by the Android host")
	}
	if !android.Authorized() {
		// ⛔⛔ THE DESK NO LONGER WAITS ON A PERSON, AND THAT IS THE WHOLE
		// POINT OF A HEADSET. RequestAuthorization blocks until the dialog is
		// answered, and the dialog is on the PHONE's screen, which is in a
		// pocket or on a table while somebody is wearing the glasses. Measured
		// on 2026-11-01: the band was ready, the glasses were on a head, and
		// the session sat at "asking for permission to capture the screen" for
		// twelve minutes and then for half an hour, because nothing downstream
		// of this line could start.
		//
		// ⭐ ONLY ONE RIBBON POSITION NEEDS THIS PERMISSION. The mirror of the
		// phone is one screen of six; the band, the head-following, the gallery
		// and everything this application draws need no consent at all — see
		// the Wall in go-xrkit/android, which creates displays and shows a
		// Presentation on them with no permission, no dialog and no
		// MediaProjection. So the answer is worth having and worth waiting for,
		// but not worth stopping for.
		//
		// ⚠ AND A LATE YES STILL COUNTS. The ask carries on in the background;
		// when it lands, the mirror joins position 0 (see lazyFeed in Capture).
		// Giving up on the answer would make a dialog answered five seconds too
		// slowly indistinguishable from one refused.
		//
		// ⚠ It is also the same mistake the Java host made the same day, in the
		// other direction: there a refusal killed the whole process thirty
		// seconds later (go-xrkit/android#52). A refusal is an ANSWER, and an
		// answer is something to carry on from.
		logf("asking for permission to capture this phone's screen; the band starts " +
			"without it and the phone joins position 1 if you allow it")
		granted := make(chan bool, 1)
		go func() {
			ok, err := android.RequestAuthorization(ctx)
			if err != nil {
				logf("asking to capture the screen: %v", err)
			}
			granted <- ok && err == nil
			close(granted)
		}()
		s := &Screens{Settled: true, granted: granted}
		s.Why = "the band carries what this application draws; this phone's screen joins " +
			"it when the dialog on the phone is answered"
		logf("%s", s.Why)
		return s, nil
	}

	d, err := android.DefaultDisplay(ctx)
	if err != nil {
		return nil, fmt.Errorf("desk: cannot find the screen to capture: %w", err)
	}
	// Settled: Android was asked directly and refused four different ways. No
	// amount of waiting turns this into virtual displays.
	s := &Screens{IDs: []uint64{uint64(d.ID)}, Settled: true}
	s.Why = fmt.Sprintf("Android does not let an ordinary application put other apps on a "+
		"display it creates, so the ribbon shows this phone's own screen: %q %dx%d",
		d.Name, d.Width, d.Height)
	logf("%s", s.Why)
	return s, nil
}

// lazyFeed is a ribbon position whose pixels may arrive later, or never.
//
// ⛔⛔ IT EXISTS BECAUSE CONSENT IS A PERSON, AND A PERSON IS NOT A DEADLINE.
// The MediaProjection dialog is on the phone's screen while somebody is wearing
// the glasses; the desk has no business waiting for it, and no business giving
// up on it either. So this hands over nothing until the answer lands, and the
// phone appears on the band the moment it does.
//
// ⚠ "Nothing" is (Source{}, false) and NOT a black frame. The compositor reads
// the second value as "there is no picture here", which is what makes the
// position show background rather than a convincing black screen — and a black
// screen is exactly what a viewer would read as a broken capture.
type lazyFeed struct {
	mu sync.Mutex
	s  *android.Stream
}

// waitAndOpen waits for the answer and, if it is yes, starts the capture.
func (f *lazyFeed) waitAndOpen(ctx context.Context, plan Plan, granted <-chan bool,
	logf func(string, ...any)) {

	var ok bool
	select {
	case <-ctx.Done():
		return
	case ok = <-granted:
	}
	if !ok {
		// Said once, and not as a fault: a person who declined is told what it
		// costs them and nothing else happens.
		logf("capturing this phone's screen was declined; the band carries what this " +
			"application draws, and the head still turns it")
		return
	}
	d, err := android.DefaultDisplay(ctx)
	if err != nil {
		logf("cannot find the screen to capture: %v", err)
		return
	}
	st, err := android.CaptureDisplay(ctx, d, android.Options{
		Width: plan.ScreenW, Height: plan.ScreenH, FPS: 60,
	})
	if err != nil {
		logf("cannot capture this phone's screen: %v", err)
		return
	}
	f.mu.Lock()
	f.s = st
	f.mu.Unlock()
	logf("this phone's screen joined the band: %q %dx%d", d.Name, d.Width, d.Height)
}

func (f *lazyFeed) Frame() (Source, bool) {
	f.mu.Lock()
	st := f.s
	f.mu.Unlock()
	if st == nil {
		return Source{}, false
	}
	fr, fresh := st.Frame()
	if fr.Width <= 0 || fr.Height <= 0 || len(fr.Pix) == 0 {
		return Source{}, false
	}
	return Source{Pix: fr.Pix, W: fr.Width, H: fr.Height, Stride: fr.Stride}, fresh
}

func (f *lazyFeed) Close() error {
	f.mu.Lock()
	st := f.s
	f.s = nil
	f.mu.Unlock()
	if st == nil {
		return nil
	}
	return st.Close()
}

// captureFeed adapts one capture stream to the portable Feed the desk consumes.
type captureFeed struct{ s *android.Stream }

func (f *captureFeed) Frame() (Source, bool) {
	fr, fresh := f.s.Frame()
	if fr.Width <= 0 || fr.Height <= 0 || len(fr.Pix) == 0 {
		return Source{}, false
	}
	return Source{Pix: fr.Pix, W: fr.Width, H: fr.Height, Stride: fr.Stride}, fresh
}

func (f *captureFeed) Close() error { return f.s.Close() }

// Capture opens a stream on the screen.
//
// There is exactly one to open. Every ribbon position beyond the first gets a
// NIL feed and shows background — the arrangement is still a ribbon, it simply
// has one screen on it, and pretending otherwise by repeating the same picture
// would tell the viewer something untrue about their phone.
func Capture(ctx context.Context, plan Plan, s *Screens, logf func(string, ...any)) ([]Feed, error) {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	// ⛔ NO MIRROR IS A RIBBON WITH NOTHING CAPTURED ON IT, NOT A FAILURE. It
	// means the consent dialog has not been answered yet, or was refused (see
	// Provide), and the band, the head and everything this application draws
	// are unaffected. Returning an error here would end a session over the one
	// position that needed a permission.
	//
	// ⚠ The feeds are still plan.Count() long and every one of them is nil or
	// empty. A shorter slice would be a different defect: the compositor
	// indexes it by ribbon position.
	if len(s.IDs) == 0 {
		feeds := make([]Feed, plan.Count())
		if s.granted != nil {
			f := &lazyFeed{}
			feeds[0] = f
			go f.waitAndOpen(ctx, plan, s.granted, logf)
			logf("the band is running; this phone's screen will join position 1 if the " +
				"dialog is allowed")
		} else {
			logf("no screen is being captured, so every ribbon position shows what this " +
				"application draws")
		}
		return feeds, nil
	}
	d, err := android.DefaultDisplay(ctx)
	if err != nil {
		return nil, fmt.Errorf("desk: cannot find the screen to capture: %w", err)
	}
	st, err := android.CaptureDisplay(ctx, d, android.Options{
		Width:  plan.ScreenW,
		Height: plan.ScreenH,
		// A ceiling, not a rate: MediaProjection is change-driven, and a still
		// screen delivers nothing at all until it moves.
		FPS: 60,
	})
	if err != nil {
		return nil, fmt.Errorf("desk: cannot capture the screen: %w", err)
	}

	feeds := make([]Feed, plan.Count())
	feeds[0] = &captureFeed{s: st}
	if plan.Count() > 1 {
		logf("%d ribbon positions have nothing to show: this platform offers one screen",
			plan.Count()-1)
	}
	return feeds, nil
}

// Sources lists everything a ribbon position could show.
//
// The phone's own screen is one, and it is the only CAPTURED one an ordinary
// application gets. Panels this program renders onto are the other kind, and
// they are opened rather than enumerated: what a panel shows is chosen when it
// is created, not discovered.
func Sources(ctx context.Context, _ *Screens) ([]Offer, error) {
	if !android.Available() {
		return nil, errors.New("desk: this process was not started by the Android host")
	}
	// ⛔ AND NOTHING IS OFFERED THAT CANNOT BE OPENED. Without consent
	// CaptureDisplay is refused, so listing the phone's screen here would put a
	// row in front of somebody that answers an error when they choose it. An
	// empty list is the honest answer, and Provide's Why says why.
	if !android.Authorized() {
		return nil, nil
	}
	d, err := android.DefaultDisplay(ctx)
	if err != nil {
		return nil, fmt.Errorf("desk: cannot find the screen to capture: %w", err)
	}
	name := d.Name
	if name == "" {
		name = "this phone"
	}
	return []Offer{{
		ID: fmt.Sprintf("display-%d", d.ID), Name: name,
		Kind: KindDisplay, W: d.Width, H: d.Height,
	}}, nil
}

// OpenOffer starts capturing one source, ready to be put on a position.
func OpenOffer(ctx context.Context, plan Plan, o Offer) (Feed, error) {
	d, err := android.DefaultDisplay(ctx)
	if err != nil {
		return nil, fmt.Errorf("desk: cannot find the screen to capture: %w", err)
	}
	if o.ID != fmt.Sprintf("display-%d", d.ID) {
		return nil, fmt.Errorf("%w: %q is not this phone's screen", ErrNoSuchOffer, o.ID)
	}
	st, err := android.CaptureDisplay(ctx, d, android.Options{
		Width: plan.ScreenW, Height: plan.ScreenH, FPS: 60,
	})
	if err != nil {
		return nil, fmt.Errorf("desk: cannot capture %s: %w", o, err)
	}
	return &captureFeed{s: st}, nil
}

// Add creates one more display and appends it.
//
// Nothing here can: creating a display is the one part of this that has no
// portable answer, and only macOS is wired up. A person is told rather than
// left pressing a key that does nothing.
func (s *Screens) Add(w, h int) (uint64, error) {
	return 0, fmt.Errorf("desk: adding a screen is not supported on this platform")
}

// DisplayOfferID is the [Offer] id for a display, for a caller that has just
// created one and wants it opened.
func DisplayOfferID(id uint64) string { return fmt.Sprintf("display-%d", id) }

// Remove gives back a display. Nothing here can: only macOS is wired up, and a
// person is told rather than left pressing a key that does nothing.
func (s *Screens) Remove(int) error {
	return fmt.Errorf("desk: removing a screen is not supported on this platform")
}
