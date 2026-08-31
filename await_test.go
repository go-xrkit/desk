// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/go-xrkit/xrkit/glasses"
)

// theGlasses and aMonitor are the two displays these tests plug in and out.
var (
	theGlasses = glasses.Display{Name: "VITURE Luma Ultra", Width: 1920, Height: 1200}
	aMonitor   = glasses.Display{Name: "DELL U3417W", Width: 3440, Height: 1440, Primary: true}
)

// lister answers a different set of displays on each call, staying on the last,
// and counts how many times it was asked.
func lister(steps ...[]glasses.Display) (func() ([]glasses.Display, error), *int) {
	calls := 0
	return func() ([]glasses.Display, error) {
		i := calls
		calls++
		if i >= len(steps) {
			i = len(steps) - 1
		}
		return steps[i], nil
	}, &calls
}

// record collects log lines.
func record(lines *[]string) func(string, ...any) {
	return func(f string, a ...any) {
		*lines = append(*lines, strings.TrimSpace(fmt.Sprintf(f, a...)))
	}
}

func TestAwaitReturnsAtOnceWhenTheDisplayIsAlreadyThere(t *testing.T) {
	list, calls := lister([]glasses.Display{aMonitor, theGlasses})
	var lines []string

	got, err := Await(context.Background(), AwaitOptions{
		Want: "VITURE", List: list, Logf: record(&lines), Every: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Await: %v", err)
	}
	if got.Name != theGlasses.Name {
		t.Errorf("chose %q, want %q", got.Name, theGlasses.Name)
	}
	if *calls != 1 {
		t.Errorf("asked %d times, want 1", *calls)
	}
	// The normal path says nothing at all.
	if len(lines) != 0 {
		t.Errorf("logged %v; a display that is already there is not news", lines)
	}
}

func TestAwaitWaitsForTheGlassesToBePluggedIn(t *testing.T) {
	list, calls := lister(
		[]glasses.Display{aMonitor},
		[]glasses.Display{aMonitor},
		[]glasses.Display{aMonitor, theGlasses},
	)
	var lines []string

	got, err := Await(context.Background(), AwaitOptions{
		Want: "VITURE", List: list, Logf: record(&lines), Every: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Await: %v", err)
	}
	if got.Name != theGlasses.Name {
		t.Errorf("chose %q, want %q", got.Name, theGlasses.Name)
	}
	if *calls != 3 {
		t.Errorf("asked %d times, want 3", *calls)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "no display matches") {
		t.Errorf("never said what it was waiting for: %v", lines)
	}
	if !strings.Contains(joined, "nothing has been created") {
		t.Errorf("never said the machine was untouched: %v", lines)
	}
	if !strings.Contains(joined, "the display arrived") {
		t.Errorf("never said the wait was over: %v", lines)
	}
}

func TestAwaitSaysSomethingWhenTHEDisplaysChangeAndNothingWhenTheyDoNot(t *testing.T) {
	// Nothing, nothing, then a monitor that is not the glasses, twice.
	list, _ := lister(
		nil,
		nil,
		[]glasses.Display{aMonitor},
		[]glasses.Display{aMonitor},
		[]glasses.Display{aMonitor, theGlasses},
	)
	var lines []string

	if _, err := Await(context.Background(), AwaitOptions{
		Want: "VITURE", List: list, Logf: record(&lines), Every: time.Millisecond,
	}); err != nil {
		t.Fatalf("Await: %v", err)
	}
	// Two different situations were reported, not four polls' worth.
	reports := 0
	for _, l := range lines {
		if strings.HasPrefix(l, "glasses:") {
			reports++
		}
	}
	if reports != 2 {
		t.Errorf("reported %d times, want 2 (once per change): %v", reports, lines)
	}
}

func TestAwaitAnswersTheMenuBar(t *testing.T) {
	for _, c := range []struct {
		name string
		send Action
		want error
	}{
		{"quit", ActionQuit, ErrAwaitQuit},
		{"settings", ActionSettings, ErrAwaitSettings},
	} {
		t.Run(c.name, func(t *testing.T) {
			list, _ := lister([]glasses.Display{aMonitor})
			actions := make(chan Action, 1)
			actions <- c.send

			_, err := Await(context.Background(), AwaitOptions{
				Want: "VITURE", List: list, Actions: actions, Every: time.Hour,
			})
			if !errors.Is(err, c.want) {
				t.Fatalf("Await err = %v, want %v", err, c.want)
			}
		})
	}
}

func TestAwaitIgnoresAnActionThatNeedsADesk(t *testing.T) {
	list, _ := lister(
		[]glasses.Display{aMonitor},
		[]glasses.Display{aMonitor, theGlasses},
	)
	actions := make(chan Action, 1)
	actions <- ActionNext // there is no ribbon to turn yet

	got, err := Await(context.Background(), AwaitOptions{
		Want: "VITURE", List: list, Actions: actions, Every: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Await: %v", err)
	}
	if got.Name != theGlasses.Name {
		t.Errorf("chose %q, want the glasses", got.Name)
	}
}

func TestAwaitStopsWithTheContext(t *testing.T) {
	list, _ := lister([]glasses.Display{aMonitor})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := Await(ctx, AwaitOptions{Want: "VITURE", List: list, Every: time.Hour}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Await err = %v, want context.Canceled", err)
	}
}

func TestAwaitReportsAListThatCannotBeRead(t *testing.T) {
	boom := errors.New("boom")
	list := func() ([]glasses.Display, error) { return nil, boom }

	if _, err := Await(context.Background(), AwaitOptions{List: list}); !errors.Is(err, boom) {
		t.Fatalf("Await err = %v, want boom", err)
	}
}

func TestAwaitNeedsAWayToList(t *testing.T) {
	if _, err := Await(context.Background(), AwaitOptions{}); err == nil {
		t.Fatal("Await with no List returned no error")
	}
}

func TestAwaitWithNothingAskedForNeverWaits(t *testing.T) {
	// ChooseDisplay always finds something when nothing was named, so this is
	// the behaviour that does NOT change: a desk asked for no display in
	// particular still opens on whatever is here.
	list, calls := lister([]glasses.Display{aMonitor})
	got, err := Await(context.Background(), AwaitOptions{List: list, Every: time.Hour})
	if err != nil {
		t.Fatalf("Await: %v", err)
	}
	if got.Name != aMonitor.Name || *calls != 1 {
		t.Errorf("chose %q after %d asks, want %q after 1", got.Name, *calls, aMonitor.Name)
	}
}

func TestDescribeDisplaysNamesNothing(t *testing.T) {
	if got := describeDisplays(nil); got != "nothing" {
		t.Errorf("describeDisplays(nil) = %q, want %q", got, "nothing")
	}
	if got := describeDisplays([]glasses.Display{theGlasses}); !strings.Contains(got, "VITURE") {
		t.Errorf("describeDisplays = %q, want it to name the display", got)
	}
}

// otherGlasses is a second headset the catalogue also knows, so these tests
// never depend on which model is which.
var otherGlasses = glasses.Display{Name: "VITURE Beast", Width: 1920, Height: 1080}

func TestAwaitAsksWhenTheHeadsetThatIsHereIsNotTheChosenOne(t *testing.T) {
	// Two measurements, and they pull in different directions. The settings
	// named one model, the pair on the desk was another, and the desk waited
	// two and a half minutes with the glasses plugged in -- so it must not
	// wait. And taking the other one silently was reported straight back:
	// "quand j'ai branche les lunettes j'aurais du avoir un panneau de choix
	// des lunettes". So it asks.
	list, _ := lister([]glasses.Display{aMonitor, otherGlasses})
	var lines []string

	_, err := Await(context.Background(), AwaitOptions{
		Want: theGlasses.Name, List: list, Logf: record(&lines), Every: time.Millisecond,
	})
	if !errors.Is(err, ErrAwaitSettings) {
		t.Fatalf("Await = %v, want it to ask", err)
	}
	said := strings.Join(lines, "\n")
	if !strings.Contains(said, "asking which to use") {
		t.Errorf("said %q, want it to say what is here and that it is asking", said)
	}
	if !strings.Contains(said, otherGlasses.Name) {
		t.Errorf("said %q, want it to name the headset that IS here", said)
	}
}

func TestAwaitAsksWhenSeveralHeadsetsAreHereAndNoneIsTheChosenOne(t *testing.T) {
	list, _ := lister([]glasses.Display{aMonitor, otherGlasses,
		{Name: "Rokid Max", Width: 1920, Height: 1080}})
	var lines []string

	_, err := Await(context.Background(), AwaitOptions{
		Want: theGlasses.Name, List: list, Logf: record(&lines), Every: time.Millisecond,
	})
	if !errors.Is(err, ErrAwaitSettings) {
		t.Fatalf("Await = %v, want it to ask", err)
	}
	if said := strings.Join(lines, "\n"); !strings.Contains(said, "none of them is the one chosen") {
		t.Errorf("said %q, want it to say why it is asking", said)
	}
}

func TestAwaitWithNoChoiceAtAllTakesTheHeadsetItFinds(t *testing.T) {
	// Nothing chosen: ChooseDisplay already answers this, and it must keep
	// answering it -- the preference above must not become a requirement.
	list, _ := lister([]glasses.Display{aMonitor, otherGlasses})
	got, err := Await(context.Background(), AwaitOptions{
		List: list, Logf: func(string, ...any) {}, Every: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Await: %v", err)
	}
	if got.Name != otherGlasses.Name {
		t.Errorf("chose %q, want %q", got.Name, otherGlasses.Name)
	}
}

func TestAwaitAsksOnceAndThenGetsOnWithIt(t *testing.T) {
	// Ask, nobody answers, ask again: that is a loop, and the bench caught it
	// on an unattended machine -- one session's log with the settings window
	// opening over and over. A person who closes that window sees the same.
	list, _ := lister([]glasses.Display{aMonitor, otherGlasses})
	var lines []string

	got, err := Await(context.Background(), AwaitOptions{
		Want: theGlasses.Name, Asked: true,
		List: list, Logf: record(&lines), Every: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Await: %v", err)
	}
	if got.Name != otherGlasses.Name {
		t.Errorf("chose %q, want the headset that is here, %q", got.Name, otherGlasses.Name)
	}
	if said := strings.Join(lines, "\n"); !strings.Contains(said, "nobody chose") {
		t.Errorf("said %q, want it to say it is getting on with it", said)
	}
}

// TestAwaitSaysWhenTheBusHasGlassesAndTheVideoHasNot is the case a person hit
// with an XREAL 1S: the glasses enumerate on USB, macOS shows no display from
// them, and "no display matches" reads as "they are not plugged in" to somebody
// who has just plugged them in and can feel the cable.
func TestAwaitSaysWhenTheBusHasGlassesAndTheVideoHasNot(t *testing.T) {
	was := onTheBus
	t.Cleanup(func() { onTheBus = was })
	onTheBus = func() []glasses.USB {
		return []glasses.USB{{Vendor: 0x3318, Product: 0x043e, Name: "XREAL 1S"}}
	}

	list, _ := lister(
		[]glasses.Display{aMonitor},
		[]glasses.Display{aMonitor},
		[]glasses.Display{aMonitor, theGlasses},
	)
	var lines []string
	if _, err := Await(context.Background(), AwaitOptions{
		Want: "VITURE", List: list, Logf: record(&lines), Every: time.Millisecond,
	}); err != nil {
		t.Fatalf("Await: %v", err)
	}
	all := strings.Join(lines, "\n")
	if !strings.Contains(all, `"XREAL 1S" is on the USB bus`) {
		t.Errorf("nothing said the glasses are on the bus:\n%s", all)
	}
	if !strings.Contains(all, "not presenting a display") {
		t.Errorf("nothing said what is missing:\n%s", all)
	}
	// ONCE, not once per poll: the wait is a loop, and a line that repeats
	// every two seconds is a line nobody reads.
	if n := strings.Count(all, "is on the USB bus"); n != 1 {
		t.Errorf("said it %d times, want once:\n%s", n, all)
	}
}

// TestAwaitSaysNothingAboutTheBusWhenItIsEmpty is the negative control: the
// same wait, with nothing plugged in, must not invent a headset.
func TestAwaitSaysNothingAboutTheBusWhenItIsEmpty(t *testing.T) {
	was := onTheBus
	t.Cleanup(func() { onTheBus = was })
	onTheBus = func() []glasses.USB { return nil }

	list, _ := lister([]glasses.Display{aMonitor}, []glasses.Display{theGlasses})
	var lines []string
	if _, err := Await(context.Background(), AwaitOptions{
		Want: "VITURE", List: list, Logf: record(&lines), Every: time.Millisecond,
	}); err != nil {
		t.Fatalf("Await: %v", err)
	}
	if all := strings.Join(lines, "\n"); strings.Contains(all, "USB bus") {
		t.Errorf("the bus was empty and something was said about it:\n%s", all)
	}
}

// TestAwaitSaysTheBillboardIsWhyThereIsNoPicture is the answer to a question
// asked with the glasses in hand: "pourquoi les xreal n'apparaissent pas dans
// la liste des ecran?".
//
// A USB Billboard is not a guess. It is the device announcing, in the only way
// the specification gives it, that an alternate mode it supports was not
// entered -- so the video half of the plug failed and the data half did not.
// Measured on the machine that asked: 2109:0103 class 17 "USB 2.0 BILLBOARD"
// on the bus, an XREAL 1S enumerated, and no display for the glasses at all.
func TestAwaitSaysTheBillboardIsWhyThereIsNoPicture(t *testing.T) {
	wasBus, wasBill := onTheBus, billboardsOnTheBus
	t.Cleanup(func() { onTheBus, billboardsOnTheBus = wasBus, wasBill })
	onTheBus = func() []glasses.USB {
		return []glasses.USB{{Vendor: 0x3318, Product: 0x043e, Name: "XREAL 1S"}}
	}
	billboardsOnTheBus = func() []glasses.USB {
		return []glasses.USB{{Vendor: 0x2109, Product: 0x0103, Name: "USB 2.0 BILLBOARD"}}
	}

	list, _ := lister(
		[]glasses.Display{aMonitor},
		[]glasses.Display{aMonitor},
		[]glasses.Display{aMonitor, theGlasses},
	)
	var lines []string
	if _, err := Await(context.Background(), AwaitOptions{
		Want: "VITURE", List: list, Logf: record(&lines), Every: time.Millisecond,
	}); err != nil {
		t.Fatalf("Await: %v", err)
	}
	all := strings.Join(lines, "\n")
	if !strings.Contains(all, "USB Billboard") {
		t.Errorf("nothing named the Billboard:\n%s", all)
	}
	// WHICH port, by its ids: a line saying something is wrong without saying
	// which plug is a line that cannot be acted on.
	if !strings.Contains(all, "2109:0103") {
		t.Errorf("the Billboard was not identified:\n%s", all)
	}
	if !strings.Contains(all, "NOT entered") {
		t.Errorf("nothing said what a Billboard means:\n%s", all)
	}
	if n := strings.Count(all, "USB Billboard"); n != 1 {
		t.Errorf("said it %d times, want once:\n%s", n, all)
	}
}

// TestAwaitNoticesAHeadsetArrivingOnTheBusAlone is the gap that made the
// message useless in practice.
//
// Plugging a headset in that presents no picture changes NOTHING about the
// display list, and the wait only ever reported when that list changed -- so
// the one moment a person needs to be told something was the one moment it
// said nothing. Measured: the glasses were plugged in, the log said "waiting
// for it" and never mentioned the bus.
func TestAwaitNoticesAHeadsetArrivingOnTheBusAlone(t *testing.T) {
	wasBus, wasBill := onTheBus, billboardsOnTheBus
	t.Cleanup(func() { onTheBus, billboardsOnTheBus = wasBus, wasBill })
	billboardsOnTheBus = func() []glasses.USB { return nil }
	// Empty at first, then a headset -- with the displays never changing.
	var polls int
	onTheBus = func() []glasses.USB {
		polls++
		if polls <= 1 {
			return nil
		}
		return []glasses.USB{{Vendor: 0x3318, Product: 0x043e, Name: "XREAL 1S"}}
	}

	// The same monitor throughout, for long enough that the bus is read again:
	// it is read every busEvery polls, on purpose, because enumerating USB
	// opens a handle on every device on the machine.
	same := make([][]glasses.Display, 0, 3*busEvery)
	for range 3 * busEvery {
		same = append(same, []glasses.Display{aMonitor})
	}
	same = append(same, []glasses.Display{aMonitor, theGlasses})
	list, _ := lister(same...)

	var lines []string
	if _, err := Await(context.Background(), AwaitOptions{
		Want: "VITURE", List: list, Logf: record(&lines), Every: time.Millisecond,
	}); err != nil {
		t.Fatalf("Await: %v", err)
	}
	if all := strings.Join(lines, "\n"); !strings.Contains(all, "is on the USB bus") {
		t.Errorf("the headset arrived on the bus and nothing was said:\n%s", all)
	}
}
