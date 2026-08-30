// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"bytes"
	"errors"
	"image/png"
	"strings"
	"testing"
	"time"

	"github.com/go-widgets/tray"
)

// headless puts the item nowhere for the duration of one test. A menu bar is
// one per machine, and a test that put an item in somebody's would leave it
// there.
func headless(t *testing.T) *tray.Headless {
	t.Helper()
	h := tray.NewHeadless()
	was := trayBackend
	trayBackend = func() tray.Backend { return h }
	t.Cleanup(func() { trayBackend = was })
	return h
}

func TestTheMenuIsTheRowsTheDeskOffers(t *testing.T) {
	h := headless(t)
	actions := make(chan Action, TrayQueue)
	item, err := OpenTray(nil, actions)
	if err != nil {
		t.Fatalf("OpenTray = %v", err)
	}
	defer func() { _ = item.Close() }()

	// The headless backend answers once it is running.
	go func() { _ = item.Hold() }()
	waitFor(t, func() bool {
		_, _, menu := h.Snapshot()
		return menu != nil && len(menu.Items) > 0
	}, "the menu to arrive")

	icon, tip, menu := h.Snapshot()
	if tip != TrayTooltip {
		t.Errorf("tooltip %q, want %q", tip, TrayTooltip)
	}
	if len(icon) == 0 {
		t.Error("the item has no icon")
	}
	rows := TrayRows()
	if len(menu.Items) != len(rows) {
		t.Fatalf("the menu has %d rows, want the %d the desk offers", len(menu.Items), len(rows))
	}
	for i, r := range rows {
		got := menu.Items[i]
		if r.Action == ActionNone {
			if !got.Separator {
				t.Errorf("row %d is %q, want a separator", i, got.Label)
			}
			continue
		}
		if got.Label != r.Title {
			t.Errorf("row %d is %q, want %q", i, got.Label, r.Title)
		}
	}
}

func TestChoosingARowSendsItsAction(t *testing.T) {
	h := headless(t)
	actions := make(chan Action, TrayQueue)
	item, err := OpenTray(nil, actions)
	if err != nil {
		t.Fatalf("OpenTray = %v", err)
	}
	defer func() { _ = item.Close() }()
	go func() { _ = item.Hold() }()
	waitFor(t, func() bool {
		_, _, menu := h.Snapshot()
		return menu != nil && len(menu.Items) > 0
	}, "the menu to arrive")

	_, _, menu := h.Snapshot()
	var chose *tray.MenuItem
	var want Action
	for i, r := range TrayRows() {
		if r.Action != ActionNone {
			chose, want = menu.Items[i], r.Action
			break
		}
	}
	if chose == nil {
		t.Fatal("the desk offers no row to choose")
	}
	chose.OnClick()
	select {
	case got := <-actions:
		if got != want {
			t.Errorf("choosing %q sent %v, want %v", chose.Label, got, want)
		}
	default:
		t.Errorf("choosing %q sent nothing", chose.Label)
	}
}

func TestAChoiceNobodyIsListeningForIsDroppedAndSaidSo(t *testing.T) {
	h := headless(t)
	// A queue with no room and nobody reading: the desk is stopped, the
	// settings window is up. A blocking send would park the handler until
	// somebody came back and then replay every click at once.
	actions := make(chan Action)
	var said []string
	item, err := OpenTray(func(f string, a ...any) { said = append(said, f) }, actions)
	if err != nil {
		t.Fatalf("OpenTray = %v", err)
	}
	defer func() { _ = item.Close() }()
	go func() { _ = item.Hold() }()
	waitFor(t, func() bool {
		_, _, menu := h.Snapshot()
		return menu != nil && len(menu.Items) > 0
	}, "the menu to arrive")

	_, _, menu := h.Snapshot()
	for i, r := range TrayRows() {
		if r.Action != ActionNone {
			menu.Items[i].OnClick() // must not block
			break
		}
	}
	if !strings.Contains(strings.Join(said, "\n"), "dropped") {
		t.Errorf("it said %v, want a line saying the choice was dropped", said)
	}
}

func TestTheIconIsAPictureOfGlasses(t *testing.T) {
	b, err := TrayIcon(TrayIconPx)
	if err != nil {
		t.Fatalf("TrayIcon = %v", err)
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("what came back is not a PNG: %v", err)
	}
	if w := img.Bounds().Dx(); w != TrayIconPx {
		t.Errorf("the icon is %d pixels wide, want %d", w, TrayIconPx)
	}
	// Something was drawn: an icon of nothing is an item nobody can find.
	inked := 0
	for y := 0; y < TrayIconPx; y++ {
		for x := 0; x < TrayIconPx; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a > 0 {
				inked++
			}
		}
	}
	if inked < TrayIconPx {
		t.Errorf("%d pixels of the icon are drawn; it is empty", inked)
	}
	if _, err := TrayIcon(0); err == nil {
		t.Error("an icon of no pixels was rendered")
	}
}

// waitFor spins until cond, or fails saying what it was waiting for.
func waitFor(t *testing.T, cond func() bool, what string) {
	t.Helper()
	for i := 0; i < 200; i++ {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("waited for %s and it never happened", what)
}

func TestReleaseAndAttachReachTheBackend(t *testing.T) {
	h := headless(t)
	actions := make(chan Action, TrayQueue)
	item, err := OpenTray(nil, actions)
	if err != nil {
		t.Fatalf("OpenTray = %v", err)
	}
	held := make(chan error, 1)
	go func() { held <- item.Hold() }()
	waitFor(t, func() bool {
		_, _, menu := h.Snapshot()
		return menu != nil && len(menu.Items) > 0
	}, "the menu to arrive")

	// Release stops the loop Hold is running, which is what lets the caller go
	// on to open a window and drive its own.
	item.Release()
	select {
	case <-held:
	case <-time.After(2 * time.Second):
		t.Fatal("Release did not stop the loop")
	}

	// Attach is for a loop somebody else is running. The headless backend does
	// not attach, and saying so is the contract: a caller that thinks it
	// attached and did not has an item nobody can see.
	if err := item.Attach(); err == nil {
		t.Error("Attach said yes on a backend that cannot attach")
	}
	if err := item.Close(); err != nil {
		t.Errorf("Close = %v", err)
	}
}

func TestAnItemWithNoPictureIsNotMade(t *testing.T) {
	headless(t)
	was := trayIcon
	trayIcon = func() ([]byte, error) { return nil, errors.New("no ink") }
	t.Cleanup(func() { trayIcon = was })

	if item, err := OpenTray(nil, make(chan Action, 1)); err == nil {
		_ = item.Close()
		t.Error("an item was made with no picture; a blank space in a menu bar " +
			"is a thing nobody can find")
	}
}

func TestEverySizeOfIconEncodes(t *testing.T) {
	// TrayIcon drops the encoding error because an NRGBA image with a positive
	// size always encodes. This is that assumption, pinned: the size it uses,
	// the smallest that means anything, and one far larger.
	for _, px := range []int{1, 16, TrayIconPx, 256} {
		b, err := TrayIcon(px)
		if err != nil {
			t.Fatalf("TrayIcon(%d) = %v", px, err)
		}
		img, err := png.Decode(bytes.NewReader(b))
		if err != nil {
			t.Errorf("TrayIcon(%d) is not a PNG: %v", px, err)
			continue
		}
		if got := img.Bounds().Dx(); got != px {
			t.Errorf("TrayIcon(%d) is %d wide", px, got)
		}
	}
	for _, px := range []int{0, -1} {
		if _, err := TrayIcon(px); err == nil {
			t.Errorf("TrayIcon(%d) made an icon", px)
		}
	}
}
