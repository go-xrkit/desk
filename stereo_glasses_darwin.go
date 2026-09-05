// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build darwin

package desk

import (
	"context"
	"fmt"
	"time"

	"github.com/go-macos/iokit/hid"
	"github.com/go-macos/iokit/viture"
)

// platformSet3D asks the glasses to put a different picture in front of each
// eye, or to stop.
//
// ⛔ THIS IS WHAT MAKES THE 3D ROW A SWITCH RATHER THAN A SIGN. The conversion
// needs a display with room for two eyes, and the headset only offers one when
// it has been told to: measured, its EDID changes with its mode, model 0x120
// with eleven modes up to 1920x1080 and model 0x220 with eleven up to
// 3840x1080. CGDisplaySetDisplayMode cannot reach across that, because in 2D
// the wide modes are not on offer at all.
//
// The command was decoded from the headset itself and is in
// go-macos/iokit/viture. What matters here is that the glasses ANSWER it, so
// this reports what they said rather than guessing from the display -- a screen
// that does not change cannot tell a refusal from a command that never arrived.
func platformSet3D(on bool) error {
	mode := viture.Mode1920x1080At60
	if on {
		mode = viture.Mode3840x1080At60
	}

	devs, err := hid.Devices(hid.Filter{VendorID: vitureVendor, UsagePage: vendorPage})
	if err != nil {
		return fmt.Errorf("desk: looking for the glasses: %w", err)
	}
	if len(devs) == 0 {
		return fmt.Errorf("%w: no VITURE control interface on the bus", ErrNoGlasses3D)
	}
	d := devs[0]
	if err := d.Open(); err != nil {
		return fmt.Errorf("desk: opening the glasses: %w", err)
	}

	// Listen BEFORE asking, or the answer arrives while nobody is looking.
	ctx, cancel := context.WithTimeout(context.Background(), glasses3DWait)
	replies := make(chan uint16, 8)
	streamed := make(chan struct{})
	go func() {
		defer close(streamed)
		_ = hid.Stream(ctx, func(_ *hid.Device, b []byte) {
			if e, ok := viture.ParseEvent(b); ok &&
				e.ID == viture.MsgDisplayMode && e.Kind == viture.DirWrite+viture.ReplyBit {
				select {
				case replies <- e.Value:
				default:
				}
			}
		}, d)
	}()
	// The same order as [withGlasses], for the same reason: closing a device
	// the run loop still holds ends the PROCESS, in silence.
	defer func() {
		cancel()
		<-streamed
		_ = d.Close()
	}()
	time.Sleep(100 * time.Millisecond)

	if err := d.SetReport(hid.Output, 0, viture.SetDisplayMode(mode)); err != nil {
		return fmt.Errorf("desk: telling the glasses: %w", err)
	}
	select {
	case status := <-replies:
		return statusError(status, on)
	case <-ctx.Done():
		// ⚠ NOT AN ERROR BY ITSELF. The glasses answer within milliseconds when
		// they answer at all, but the display tearing down and coming back is
		// what a person will see, and it happens either way. Saying "no answer"
		// is honest; claiming failure would not be.
		return fmt.Errorf("%w: they did not answer in %v", ErrNoGlasses3D, glasses3DWait)
	}
}

// platformGlassesGet reads one setting from the headset.
//
// ⛔ A READ CHANGES NOTHING, which is why every path here starts with one: it is
// safe to repeat, and its success is visible where a command's is not.
func platformGlassesGet(id byte) (uint16, error) {
	var out uint16
	err := withGlasses(func(d *hid.Device, in chan viture.Event) error {
		if err := d.SetReport(hid.Output, 0, readOf(id)); err != nil {
			return fmt.Errorf("desk: asking the glasses: %w", err)
		}
		for {
			select {
			case e := <-in:
				if e.ID == id && e.Kind == viture.DirRead+viture.ReplyBit {
					if e.Value == 2 {
						return fmt.Errorf("%w: they have no setting %#02x", ErrNoGlasses3D, id)
					}
					out = e.Value
					return nil
				}
			case <-time.After(glasses3DWait):
				return fmt.Errorf("%w: no answer about %#02x", ErrNoGlasses3D, id)
			}
		}
	})
	return out, err
}

// platformGlassesSet writes one, and reports the headset's own answer.
func platformGlassesSet(id byte, value uint16) error {
	return withGlasses(func(d *hid.Device, in chan viture.Event) error {
		if err := d.SetReport(hid.Output, 0, writeOf(id, value)); err != nil {
			return fmt.Errorf("desk: telling the glasses: %w", err)
		}
		for {
			select {
			case e := <-in:
				if e.ID == id && e.Kind == viture.DirWrite+viture.ReplyBit {
					return statusError(e.Value, true)
				}
			case <-time.After(glasses3DWait):
				return fmt.Errorf("%w: they did not answer", ErrNoGlasses3D)
			}
		}
	})
}

// readOf and writeOf build the two reports.
//
// ⛔ THE VALUE GOES IN TWICE, LITTLE-ENDIAN. That one detail is what nineteen
// attempts had wrong: the headset's own REPLIES carry a value little-endian and
// then big-endian, and copying that shape into a command has it refused.
func readOf(id byte) []byte {
	b := make([]byte, viture.ReportSize)
	copy(b, []byte{0x10, 0x00, id, viture.DirRead, 0x03, 0x00, 0, 0, 0, 0})
	return b
}

func writeOf(id byte, v uint16) []byte {
	b := make([]byte, viture.ReportSize)
	copy(b, []byte{0x10, 0x00, id, viture.DirWrite, 0x02, 0x00,
		byte(v), byte(v >> 8), byte(v), byte(v >> 8)})
	return b
}

// withGlasses opens the control interface, listens, and runs fn.
//
// Opened and closed around each exchange: the interface is shared with whatever
// else is talking to the headset, and holding it for a session would be holding
// it against them.
func withGlasses(fn func(*hid.Device, chan viture.Event) error) error {
	devs, err := hid.Devices(hid.Filter{VendorID: vitureVendor, UsagePage: vendorPage})
	if err != nil {
		return fmt.Errorf("desk: looking for the glasses: %w", err)
	}
	if len(devs) == 0 {
		return fmt.Errorf("%w: no control interface on the bus", ErrNoGlasses3D)
	}
	d := devs[0]
	if err := d.Open(); err != nil {
		return fmt.Errorf("desk: opening the glasses: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*glasses3DWait)
	in := make(chan viture.Event, 16)
	streamed := make(chan struct{})
	go func() {
		defer close(streamed)
		_ = hid.Stream(ctx, func(_ *hid.Device, b []byte) {
			if e, ok := viture.ParseEvent(b); ok {
				select {
				case in <- e:
				default:
				}
			}
		}, d)
	}()
	// ⛔ THE DEVICE IS CLOSED ONLY AFTER THE STREAM HAS LET GO OF IT.
	//
	// This used to be a plain `defer d.Close()`, and it killed the program:
	// cancelling the context does not stop the run loop, it only asks -- the
	// pump is inside CFRunLoopRunInMode and checks the context when it comes
	// out. So Close ran while IOKit still had the device scheduled with a
	// registered callback, and macOS ended the process with
	// "BUG IN CLIENT OF LIBPLATFORM: os_unfair_lock is corrupt".
	//
	// ⛔ AND NOTHING IS PRINTED WHEN THAT HAPPENS. It is not a Go panic:
	// there is no stack, no message, the log simply stops after the line that
	// says which shortcut was pressed. Measured 2026-09-05 17:36 -- the only
	// witness was ~/Library/Logs/DiagnosticReports.
	defer func() {
		cancel()
		<-streamed
		_ = d.Close()
	}()
	time.Sleep(80 * time.Millisecond)
	return fn(d, in)
}
