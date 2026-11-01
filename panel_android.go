// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/go-widgets/toolkit"
	"github.com/go-xrkit/android"
)

// hostPanel shows the ribbon on the display the glasses provide, through the
// Java host's Presentation.
//
// ⛔ THE LOOP IS OURS HERE, AND THAT IS THE WHOLE DIFFERENCE. A window back-end
// owns an event loop and PULLS a frame when it repaints; the host lends slots
// of a shared buffer and waits for us to PUSH one. So Show is a loop, Repaint
// wakes it, and the back-pressure is the host's: a slot comes back only once
// the frame in it reached the view, which is what stops the next composite
// tearing the one being blitted.
type hostPanel struct {
	s *android.Screen
	// wake carries "there is a newer frame", with depth one: the ribbon redraws
	// every 16 ms and the host may well be slower, and queueing those ticks
	// would present stale frames long after they stopped being true. One
	// pending wake-up means "draw the newest thing", which is the only frame
	// anybody wants.
	wake chan struct{}
	// closed is shut once, so Close is idempotent and Show returns from it.
	closed chan struct{}
	once   sync.Once
}

// openPanel opens a Presentation on the display the run chose.
func openPanel(ctx context.Context, opt RunOptions, _ string,
	logf func(string, ...any), _ *damagedNames) (panel, error) {

	if !android.Available() {
		return nil, errors.New("desk: this process was not started by the Android host, " +
			"so there is no display to present on")
	}
	d, err := displayNamed(ctx, opt.Screen.Name)
	if err != nil {
		return nil, err
	}
	// ⛔ ASK THE DISPLAY, DO NOT ASSUME. Whether a display will take a
	// Presentation at all is the platform's own answer, carried in its FLAG_
	// bits — and it is the only route an ordinary APK has: starting an activity
	// on a display it does not own is refused with a SecurityException, and the
	// flag that would lift that needs a signature permission. Measured on this
	// phone: `id 12 "VITURE Beast" 1920x1080 @110dpi 60.0Hz flags 0x8088
	// presentation true`.
	if !d.Presentation() {
		return nil, fmt.Errorf("desk: %s will not take a presentation (flags %#x), so there "+
			"is no way to draw on it from an ordinary application", d, d.Flags)
	}
	s, err := android.ShowOn(ctx, d, android.ScreenOptions{})
	if err != nil {
		return nil, fmt.Errorf("desk: cannot present on %s: %w", d, err)
	}
	w, h := s.Size()
	logf("presenting on %s at %dx%d, %d slot(s)", d, w, h, s.Options().QueueDepth)
	return &hostPanel{
		s:      s,
		wake:   make(chan struct{}, 1),
		closed: make(chan struct{}),
	}, nil
}

func (p *hostPanel) Size() (int, int) { return p.s.Size() }

// Show presents the newest frame each time the ribbon asks, until Close.
//
// ⚠ in is never called. There is no keyboard and no pointer on a headset: the
// ribbon is driven by the head and by the shortcuts the phone has none of, and
// pretending otherwise would be an input path nothing can produce an event on.
func (p *hostPanel) Show(frame func() ([]byte, int, int), _ func(toolkit.Event)) error {
	for {
		select {
		case <-p.closed:
			return nil
		case <-p.wake:
		}
		// Closing between the wake-up and the draw is ordinary: the ribbon
		// quits on its own goroutine. Checking again costs nothing and avoids
		// asking the host for a slot nobody will present.
		select {
		case <-p.closed:
			return nil
		default:
		}
		c, err := p.s.Next(context.Background())
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return fmt.Errorf("desk: waiting for a frame slot: %w", err)
		}
		pix, w, h := frame()
		blit(c, pix, w, h)
		if err := p.s.Present(context.Background(), c); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return fmt.Errorf("desk: presenting a frame: %w", err)
		}
	}
}

// blit copies one composed frame into a borrowed slot, row by row.
//
// ⛔ ROW BY ROW, BECAUSE THE STRIDES DIFFER. The host announces the alignment
// its surface wants and that is "at least Width*4 and usually more", while the
// ribbon composes tightly packed rows. One flat copy would shear the picture
// diagonally — the classic symptom, and it would look like a warp bug rather
// than an arithmetic one.
//
// ⚠ Both sides are RGBA with eight bits a channel: the compositor already
// swapped the capture's red and blue on the way in (see swapRB in render.go),
// so no conversion belongs here. The short-row and short-frame guards are not
// defensive padding: a frame arrives before the first capture has settled the
// geometry, and the slot is the display's size while the composite is the
// ribbon's.
func blit(c android.Canvas, pix []byte, w, h int) {
	if len(pix) == 0 || w <= 0 || h <= 0 {
		return
	}
	src := w * 4
	rows := h
	if rows > c.Height {
		rows = c.Height
	}
	n := src
	if n > c.Stride {
		n = c.Stride
	}
	for y := 0; y < rows; y++ {
		from := y * src
		if from+n > len(pix) {
			return
		}
		copy(c.Row(y), pix[from:from+n])
	}
}

func (p *hostPanel) Repaint() {
	select {
	case p.wake <- struct{}{}:
	default: // one pending wake-up already means "the newest frame"
	}
}

func (p *hostPanel) Close() error {
	p.once.Do(func() { close(p.closed) })
	return p.s.Close()
}

// displayNamed finds the display the run chose among the host's answer.
func displayNamed(ctx context.Context, name string) (android.Display, error) {
	ds, err := android.Screens(ctx)
	if err != nil {
		return android.Display{}, fmt.Errorf("desk: asking the host for the displays: %w", err)
	}
	if len(ds) == 0 {
		return android.Display{}, errDisplaysUnreadable
	}
	for _, d := range ds {
		if d.Name == name {
			return d, nil
		}
	}
	var had []string
	for _, d := range ds {
		had = append(had, fmt.Sprintf("%q", d.Name))
	}
	return android.Display{}, fmt.Errorf("desk: %q is not attached any more; there is %s",
		name, strings.Join(had, ", "))
}

// displayStillThere reports whether the run's display is still attached.
//
// ⚠ It answers in the SAME vocabulary as the window-system road, because the
// watch in Run quits on an error it does not recognise: an empty list is
// errDisplaysUnreadable ("I could not tell"), and only a list that genuinely
// lacks the display is a plain error.
func displayStillThere(ctx context.Context, name string) error {
	_, err := displayNamed(ctx, name)
	return err
}
