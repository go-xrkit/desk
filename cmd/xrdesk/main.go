// xrdesk shows several screens on a 360° ribbon inside AR glasses.
//
// It is the application; cmd/deskcheck is the probe that says what this machine
// can do without taking a display over.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/go-macos/appicon"
	"github.com/go-widgets/window"
	"github.com/go-xrkit/desk"
	"github.com/go-xrkit/xrkit/glasses"
)

// init pins the main goroutine to the process main OS thread, before anything
// else in this program runs.
//
// AppKit refuses to create an NSWindow anywhere else, and window.Open's own
// LockOSThread is TOO LATE: it locks the goroutine to whatever thread it is on
// by then, and a foreign call made earlier — creating the virtual displays, in
// this program — can leave the main goroutine resumed on a different thread.
//
// The failure is intermittent, which is worse than reliable: it depends on
// scheduling. This one aborted on the SECOND live run with "NSWindow should
// only be instantiated on the main thread!", having got further than the first.
//
// init runs on the main goroutine before main, which is the earliest this can
// be said.
func init() {
	runtime.LockOSThread()
}

// retryAfter is how long xrdesk waits before trying again when the screens
// could not be made.
//
// Long enough that a window server busy reconfiguring displays has finished,
// short enough that a person who just plugged their glasses in does not think
// nothing happened.
const retryAfter = 3 * time.Second

// maxProvideTries is how many times the desk waits for a window server that
// will not make a display before settling for the screens this Mac already
// has. See the fallback in session.
const maxProvideTries = 5

func main() { os.Exit(run()) }

func run() int {
	screen := flag.String("screen", "", "which display to take over, matched by name")
	fov := flag.Float64("fov", 0, "horizontal field of view in degrees, when the catalogue does not know")
	count := flag.Int("screens", 0, fmt.Sprintf("how many screens on the ribbon, 1 to %d (0 = the setting, or six)", desk.MaxScreens))
	distance := flag.Float64("distance", 0, fmt.Sprintf("how far the band sits, 1 to %g screens across the view (0 = the setting, or one)", desk.MaxDistance))
	splay := flag.Float64("splay", 0,
		fmt.Sprintf("the angle between one screen and the next, 0 to %g degrees "+
			"(0 = the setting, or %g; a negative asks for one flat plane)",
			desk.MaxSplayDeg, desk.DefaultSplayDeg))
	forDur := flag.Duration("for", 0, "stop after this long; 0 runs until you quit")
	photoCamera := flag.String("photo-camera", "",
		"which camera a photograph comes from, by its unique id (empty = the first listed)")
	quiet := flag.Bool("quiet", false, "say less")
	noGlobal := flag.Bool("no-global", false,
		"do not claim the system-wide shortcuts (\u2325\u2318\u2190/\u2192 and \u2325\u2318Space)")
	interactive := flag.Bool("interactive", false,
		"let the desk window take the keyboard and the mouse (it does not by "+
			"default: they belong to the applications on the screens)")
	settingsWin := flag.Bool("settings", false,
		"open the settings window instead of the desk")
	snap := flag.Bool("snapshot", false, "write the first frame shown, so it can be looked at afterwards")
	stereo3D := flag.Bool("3d", false,
		"start with the 3D conversion on; it can be turned on and off from the menu at any time")
	depthModel := flag.String("depth-model", "",
		"a Core ML depth model (.mlpackage or .mlmodelc) for -3d; without one, depth is guessed from the picture and is visibly worse")
	dim := flag.Bool("dim", true,
		"turn a Mac panel off while the ribbon is showing a copy of it; -dim=false leaves every screen lit")
	flag.Parse()

	logf := func(f string, a ...any) { fmt.Printf("  "+f+"\n", a...) }
	if *quiet {
		logf = func(string, ...any) {}
	}

	// The settings file comes first: a flag given on the command line is a
	// person overriding what they wrote down, so the file has to be read before
	// the flags are consulted.
	settings, err := desk.LoadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	// A panel a run before this one left dark, before anything here touches a
	// backlight. That run was killed -- SIGKILL, a force quit, a crash -- and
	// could not put it back itself; nothing runs after SIGKILL. Measured by the
	// bench: "display 1 was left at 0.00, from 0.79".
	if said, err := desk.PutBackWhatWasLeftDark(); err != nil {
		logf("putting back a screen left dark: %v", err)
	} else {
		for _, line := range said {
			fmt.Printf("%s\n", line)
		}
	}
	if *settingsWin {
		if err := desk.RunSettings(desk.SettingsOptions{
			Logf: logf, DisplayH: tallestDisplay(),
		}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}
	// Several headsets attached and nothing said about which: ask, once,
	// rather than pick one. Naming a display with -screen is the answer for a
	// script, and giving one in the settings is the answer for a person who has
	// already decided; this is only for the case where nobody has.
	if desk.ShouldChoose(settings, *screen, desk.Peripherals()) {
		logf("several headsets are attached and none is chosen")
		if err := desk.RunSettings(desk.SettingsOptions{
			Logf: logf, DisplayH: tallestDisplay(),
		}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		// Read it back: the choice has to take effect in THIS run, or the
		// person is asked and then ignored.
		if again, err := desk.LoadConfig(); err == nil {
			settings = again
		}
	}

	// The menu-bar item, once for the whole process.
	//
	// It outlives every session, which is the point: a desk that has stopped to
	// show its settings, or has not started yet, is exactly when a person needs
	// somewhere to click. Choosing a row sends an action into the queue the run
	// loop reads.
	actions := make(chan desk.Action, desk.TrayQueue)
	menuBar, err := desk.OpenTray(logf, actions)
	if err != nil {
		logf("%v", err)
	} else {
		defer func() { _ = menuBar.Close() }()
	}

	// What the command line asked for, kept apart from what the settings say: a
	// session after the settings window has to re-apply the same precedence, and
	// overwriting the flags on the first pass would lose the question.
	flagCount, flagScreen, flagDistance, flagSplay := *count, *screen, *distance, *splay

	// One session: the plan, the displays, the captures, the ribbon. It returns
	// true when it stopped to show the settings, and everything it made is
	// released before the settings window opens -- a desk holding six virtual
	// displays while a person changes how many there should be is a desk that
	// then has to be told twice.
	// askedAboutGlasses remembers that a person has already been shown the
	// choice once. Asking twice with the same answer is a loop.
	askedAboutGlasses := false
	// provideTries counts how often the window server has refused to make the
	// screens, so a Mac that will not make any is waited for and then accepted
	// rather than waited for for ever.
	provideTries := 0

	session := func(n int, model string, dist, splay float64, settings desk.Config) (again, wantSettings bool, code int) {
		// Ctrl-C must reach the same exit as the quit key, or a session left
		// running keeps virtual displays the person never asked for. It is set
		// up BEFORE the wait below, so a person who changes their mind about
		// plugging the glasses in can stop the program.
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()

		// -for bounds the WHOLE run, waiting included.
		//
		// It used to bound only the session, so `-for 3s` against a display that
		// was not plugged in waited for ever: the three seconds never started,
		// because nothing had. "Stop after this long" is what the flag says, and
		// a run that never begins is exactly the case somebody automating this
		// needs it to cover.
		if *forDur > 0 {
			var until context.CancelFunc
			ctx, until = context.WithTimeout(ctx, *forDur)
			defer until()
		}

		// The glasses are a cable. Wait for them rather than making the order
		// of two actions matter: this returns at once when the display is
		// already there, and says nothing in that case.
		// THE MENU BAR HOLDS THE LOOP WHILE THIS WAITS.
		//
		// A menu-bar item is an object with a window, and neither is drawn --
		// nor is a menu ever opened -- without a platform run loop. There is
		// none here yet: the window that will own one needs a display, and the
		// display is what this is waiting for. So the tray runs the loop and
		// the waiting happens on a goroutine, which is the way round
		// go-widgets/tray is built for: Run when a program has no loop of its
		// own, Attach when it does.
		await := func(opt desk.AwaitOptions) (glasses.Display, error) {
			if menuBar == nil {
				return desk.Await(ctx, opt)
			}
			var got glasses.Display
			var err error
			done := make(chan struct{})
			go func() {
				defer close(done)
				got, err = desk.Await(ctx, opt)
				// Whatever it found, the loop has to stop so the window can
				// have the main thread.
				menuBar.Release()
			}()
			if herr := menuBar.Hold(); herr != nil {
				// No loop to hold -- a build without the native tray, or a
				// platform without one. The wait still works and there is
				// simply no item, which is worth saying twice: a menu bar
				// nobody can use, in silence, cost an afternoon.
				logf("no menu bar item: %v", herr)
			}
			<-done
			return got, err
		}
		chosen, err := await(desk.AwaitOptions{
			Want: model, Actions: actions, Logf: logf, Asked: askedAboutGlasses,
			List: func() ([]glasses.Display, error) {
				ss, err := window.Screens()
				if err != nil {
					return nil, err
				}
				ds := make([]glasses.Display, len(ss))
				for i, s := range ss {
					ds[i] = glasses.Display{Name: s.Name, Width: s.Width, Height: s.Height, Primary: s.Primary}
				}
				return ds, nil
			},
		})
		switch {
		case errors.Is(err, desk.ErrAwaitQuit), errors.Is(err, context.Canceled):
			return false, false, 0
		case errors.Is(err, context.DeadlineExceeded):
			// -for ran out while still waiting. Not a failure: the run did
			// exactly what it was told, and saying so is better than a silent
			// zero after nothing happened.
			fmt.Printf("gave up waiting after %s; nothing was created\n", *forDur)
			return false, false, 0
		case errors.Is(err, desk.ErrAwaitSettings):
			askedAboutGlasses = true
			return true, true, 0
		case err != nil:
			fmt.Printf("%v\n", err)
			return false, false, 1
		}
		fmt.Printf("on %s\n", chosen)
		// THE MENU BAR SAYS SO. A green dot on the icon while a desk is up,
		// and the plain glyph when there is none: a person glancing at their
		// menu bar learns whether the glasses are live without opening
		// anything, which is the one thing a menu bar is for.
		if menuBar != nil {
			menuBar.State().Set(desk.TrayRunning)
			defer menuBar.State().Set(desk.TrayWaiting)
		}

		if advice := glasses.ScalingAdvice(chosen); advice != "" {
			logf("%s", advice)
		}

		plan, err := desk.NewPlan(chosen, desk.Options{
			Screens: n, FOVDeg: *fov,
			USB: desk.EvidenceFor(chosen, model != "", desk.Peripherals()),
		})
		if err != nil {
			fmt.Printf("%v\n", err)
			return false, false, 1
		}
		logf("%s", plan)

		// SCREEN 1 IS THIS MAC'S OWN SCREEN.
		//
		// Somebody wearing the glasses still has a Mac in front of them, with a
		// menu bar, a Dock and whatever was already open on it. Reaching it
		// should not mean taking the glasses off, and it should not mean
		// knowing a key: it is where the band starts.
		//
		// So one virtual display FEWER is made, and the position it would have
		// taken goes to the machine's own screen. Everything that maps a ribbon
		// position to a display goes through ribbonIDs from here on, because
		// screens.IDs is now the virtual ones only and position i is IDs[i-1].
		mirror := settings.Mirror()
		// macID is the display screen 1 shows, so that every place that maps a
		// ribbon POSITION to a display can do it: with the mirror in front,
		// position i is the screen this program made at i-1, and position 0 is
		// this one. Zero until the sources have been listed.
		var macID uint64
		made := plan
		if mirror {
			made = plan.WithScreens(plan.Count() - 1)
		}
		screens, err := desk.Provide(ctx, made, logf)
		if err != nil {
			// Back to waiting rather than out of the program.
			//
			// Measured: the glasses were plugged in, the desk woke up, the fifth
			// of six virtual displays never became active, the fallback found
			// the display list mid-reconfiguration and empty, and xrdesk exited
			// with "no displays at all" while the person was looking at their
			// screens. Whatever the window server was busy with, the answer is
			// to ask again in a moment, not to give up on the session.
			fmt.Printf("%v\n", err)
			fmt.Printf("waiting, and trying again in %v\n", retryAfter)
			select {
			case <-ctx.Done():
				return false, false, 0
			case <-time.After(retryAfter):
			}
			return true, false, 0
		}

		// MADE NOTHING, AND SHOWING THE MACHINE'S OWN SCREENS INSTEAD.
		//
		// Provide falls back to the displays this Mac already has when the
		// window server refuses to make one, and that fallback is a bad desk: it
		// puts the glasses' own screen on the band, it will not place an
		// application, and it darkens a panel somebody is looking at.
		//
		// MEASURED, and it is not rare: after a session that made five or six
		// displays, this Mac refuses the next one for the best part of a minute
		// -- a single display asked for on a quiet machine succeeds at once. A
		// person who quits the desk and starts it again lands exactly there.
		//
		// So it waits, a few times, before settling for that.
		if !screens.Virtual && provideTries < maxProvideTries {
			provideTries++
			fmt.Printf("%s\n", screens.Why)
			fmt.Printf("waiting %v for the window server, and asking again (%d of %d)\n",
				retryAfter, provideTries, maxProvideTries)
			if err := screens.Close(); err != nil {
				logf("%v", err)
			}
			select {
			case <-ctx.Done():
				return false, false, 0
			case <-time.After(retryAfter):
			}
			return true, false, 0
		}

		defer func() {
			// Waiting for the removal is right when the desk is coming back —
			// the settings window opens next and a person may well look at
			// System Settings — and wrong on the way out, where it was measured
			// costing eight seconds and printing a warning nobody can act on.
			// See desk.Screens.Close.
			var err error
			if again {
				err = screens.Close()
			} else {
				err = screens.Release()
			}
			if err != nil {
				fmt.Printf("WARNING: could not remove every virtual display: %v\n", err)
			}
		}()

		feeds, err := desk.Capture(ctx, made, screens, logf)
		if err != nil {
			fmt.Printf("%v\n", err)
			return false, false, 1
		}
		// Position 0 is left empty for now: what goes there is opened once the
		// inventory can name it, through the same path as every other source.
		if mirror {
			feeds = append([]desk.Feed{nil}, feeds...)
		}
		d, err := desk.New(plan, feeds)
		if err != nil {
			fmt.Printf("%v\n", err)
			return false, false, 1
		}
		defer d.Close()

		// What each ribbon position is showing, right now. Until there is an
		// inventory it is the screens this program made, in order.
		showing := func() []uint64 { return screens.IDs }

		// What a ribbon position shows is chosen while it runs. The inventory is the
		// list; Cycle is that list reduced to one key.
		// Everything this Mac can show, except the screen the desk is ON. The
		// glasses are a display like any other to the window server, and a
		// position showing them shows this program's own window -- a picture
		// inside itself, which is what "l'affichage a bugue" looked like.
		own, _ := desk.OwnDisplay(chosen.Name)
		if offers, err := desk.Sources(ctx, screens); err != nil {
			logf("cannot list what could be shown: %v", err)
		} else if inv, err := desk.NewInventory(plan.Count(), desk.Without(offers, own)); err != nil {
			logf("inventory: %v", err)
		} else {
			// A position mirroring this Mac's own panel is a screen of the band
			// like any other: the band has to follow the pointer onto it, or
			// moving the mouse there loses it in exactly the way following was
			// written to prevent.
			showing = func() []uint64 {
				out := make([]uint64, inv.Positions())
				for i := range out {
					if o, ok := inv.At(i); ok {
						out[i], _ = desk.DisplayOf(o)
					}
				}
				return out
			}

			// Screen 1 first, and it is this Mac's own screen when the settings
			// ask for it. Assigned by NAME rather than taken by cycling: the
			// cycle walks the desk's own screens first, so the machine's would
			// never come up on position 1 by itself.
			if mirror {
				if o, ok := mainDisplay(inv.Offers()); ok {
					macID, _ = desk.DisplayOf(o)
					if err := inv.Assign(0, o.ID); err != nil {
						logf("screen 1: %v", err)
					} else if f, err := desk.OpenOffer(ctx, plan, o); err != nil {
						logf("screen 1: %v", err)
					} else if old, err := d.SetFeed(0, f); err != nil {
						logf("screen 1: %v", err)
						closeFeed(f)
					} else {
						closeFeed(old)
						logf("screen 1: %s — this Mac's own screen", o.Name)
					}
				} else {
					logf("screen 1: this Mac offers no screen to show here")
				}
			}
			// Then the rest, filled the way one key would, so a session starts
			// with something on every position rather than with a ring of holes.
			for i := 0; i < plan.Count(); i++ {
				if mirror && i == 0 {
					continue
				}
				if o, ok := inv.Cycle(i); ok {
					logf("screen %d: %s", i+1, o.Name)
				}
			}
			// The applications, once the band knows what each position shows. It used
			// to happen earlier, before the sources were listed -- and then a
			// place "Firefox" { screen = 1 } went to the first screen this program
			// MADE, which is no longer position 1. A screen with nothing on it still
			// reads as broken rather than as empty, so this is still before the
			// first frame; it is just after the arrangement is known.
			//
			// ONLY onto screens this desk made. When the window server refuses a
			// virtual display, Provide falls back to the displays the machine
			// already has -- and placing an application then means picking up
			// somebody's windows and rearranging their actual desktop, which no
			// setting in a file about a headset asked for. Measured: a run where
			// creation failed moved Firefox onto the main screen.
			if places := settings.Placements(); len(places) > 0 && !screens.Virtual {
				logf("not placing %d application(s): these are the displays this Mac "+
					"already has (%s), and moving your windows about on them is not "+
					"what a desk setting asks for", len(places), screens.Why)
			} else if len(places) > 0 {
				done, err := desk.Send(desk.TheBench(), ribbonIDs(mirror, macID, screens.IDs), places)
				for _, line := range done {
					logf("%s", line)
				}
				if err != nil {
					// Not fatal. A desk of six applications where one is not running
					// should show the other five.
					for _, line := range strings.Split(err.Error(), "\n") {
						logf("%s", line)
					}
				}
			}
			// And with this Mac's own panels dark while the ribbon shows a copy
			// of them. A person wearing the glasses is looking at the copy; the
			// panel itself is then private work at reading distance, facing
			// whoever walks past, lit for nobody.
			//
			// The way back is deferred FIRST, before anything is turned off, so
			// that every path out of here — a refusal below, a quit, the -for
			// timer — goes through it.
			// The screen the desk itself is on is never darkened, and it is
			// identified by its RECTANGLE: two identical monitors are the same
			// size and are not in the same place.
			own, _ := desk.OwnDisplay(chosen.Name)
			var dark desk.Dimmer
			defer func() {
				if err := dark.Restore(); err != nil {
					fmt.Printf("putting a screen's brightness back: %v\n", err)
				}
			}()
			remirror := func() {
				// Only in the glasses. Windowed, the desk is a window ON one of
				// these screens and darkening them would black out the very
				// thing being used.
				if !*dim || !settings.Immersive() {
					return
				}
				on := make([]desk.Offer, 0, inv.Positions())
				for i := 0; i < inv.Positions(); i++ {
					if o, ok := inv.At(i); ok {
						on = append(on, o)
					}
				}
				// The displays this program made are not this Mac's panels;
				// when it could not make any, the ribbon is showing the real
				// ones and every one of them is a candidate.
				var ours []uint64
				if screens.Virtual {
					ours = screens.IDs
				}
				was := dark.Dark()
				if err := dark.Showing(desk.Mirrors(on, ours, own)); err != nil {
					logf("darkening: %v", err)
				}
				// Written down before this run can be killed. Nothing runs
				// after SIGKILL, so the note is what puts a panel back.
				if err := dark.Note(); err != nil {
					logf("noting what is dark: %v", err)
				}
				if n := dark.Dark(); n != was {
					fmt.Printf("%d of this Mac's screens are off while the ribbon shows them\n", n)
				}
			}
			remirror()

			// And taking one away: the desk has already shrunk by the time this is
			// called, so what is left is the platform work.
			d.OnRemove = func(pos int, f desk.Feed) {
				closeFeed(f)
				if mirror && pos == 0 {
					fmt.Printf("screen 1 is this Mac's own screen; it cannot be taken away\n")
					return
				}
				if mirror {
					// Position i is the screen this program made at i-1.
					pos--
				}
				if err := screens.Remove(pos); err != nil {
					fmt.Printf("%v\n", err)
					return
				}
				// The inventory keeps one row per POSITION, and there is one
				// fewer now: clearing the last row is what stops it offering a
				// screen that is not there.
				if n := inv.Positions(); n > 0 {
					_ = inv.Clear(n - 1)
				}
				remirror()
				fmt.Printf("screen %d is gone; %d left\n", pos+1, len(screens.IDs))
			}

			// The gallery's "add a screen" cell. Making a display is the platform's
			// business, so the desk asks rather than doing it.
			d.OnAdd = func() (desk.Feed, error) {
				id, err := screens.Add(plan.ScreenW, plan.ScreenH)
				if err != nil {
					return nil, err
				}
				f, err := desk.OpenOffer(ctx, plan, desk.Offer{
					ID:   desk.DisplayOfferID(id),
					Name: fmt.Sprintf("XR screen %d", len(screens.IDs)),
					Kind: desk.KindDisplay,
					W:    plan.ScreenW, H: plan.ScreenH,
				})
				if err != nil {
					return nil, err
				}
				fmt.Printf("added screen %d\n", len(screens.IDs))
				return f, nil
			}

			// A desk grown or shrunk by hand comes back the same size.
			//
			// This is the whole of the persistence: the package does not know
			// where settings live, and RememberScreens EDITS the file rather
			// than rendering it from a struct — so the comments of somebody who
			// wrote their own desk.hcl survive an action that was about adding a
			// screen, not about saving settings.
			d.OnScreens = func(n int) {
				path, err := desk.ConfigPath()
				if err != nil {
					fmt.Printf("cannot remember %d screens: %v\n", n, err)
					return
				}
				if err := desk.RememberScreens(path, n); err != nil {
					fmt.Printf("%v\n", err)
					return
				}
				fmt.Printf("%d screens, remembered in %s\n", n, path)
			}

			// One key, and the pointer is on the screen being looked at. Without it the
			// applications on these displays cannot be reached: the picture is a
			// capture, so dragging the mouse towards it is dragging it blind.
			d.OnPoint = func(pos int) {
				if err := desk.BringPointer(screens.IDs, pos); err != nil {
					fmt.Printf("%v\n", err)
					return
				}
				fmt.Printf("the pointer is on screen %d\n", pos+1)
			}

			// A photograph, through one of the glasses' cameras.
			//
			// ⛔ THE CAMERA IS OPENED FOR THE PHOTOGRAPH AND CLOSED AFTER IT.
			// A camera held open is a camera left ON, light and all, and a
			// desk that kept one for a session would be a headset watching the
			// room all afternoon so that one key press could be quick.
			d.OnPhoto = func() (string, error) {
				return desk.TakePhoto(*photoCamera, logf)
			}

			// What is running, asked every time the gallery opens: a list read
			// once would offer a screen to something that quit an hour ago.
			// 256 because that is a size an .icns really holds, and because a
			// tile on a 1200-row panel is about that: bigger would be scaled
			// down for nothing, smaller would be scaled up and look it.
			const iconPx = 256
			icons := map[int32]*desk.Icon{}
			d.OnApps = func() ([]desk.App, error) {
				b := desk.TheBench()
				if !b.Trusted() {
					return nil, fmt.Errorf("this application may not see another " +
						"one's windows: grant it Accessibility in System Settings " +
						"> Privacy & Security > Accessibility")
				}
				list, err := b.Listing()
				if err != nil {
					return nil, err
				}
				apps := desk.AppsFrom(list, screens.IDs)
				// Their own icons, which is how a person recognises a
				// program. Cached by pid: the gallery is re-read every time
				// it opens, and rasterising an .icns is the expensive part.
				// It needs no permission, and an application that will not
				// give one just keeps the drawn glyph.
				for i := range apps {
					if ic, ok := icons[apps[i].PID]; ok {
						apps[i].Icon = ic
						continue
					}
					var ic *desk.Icon
					if px, err := appicon.ForPID(apps[i].PID, iconPx); err == nil {
						ic = &desk.Icon{Pix: px.Pix, W: px.W, H: px.H}
					}
					icons[apps[i].PID] = ic
					apps[i].Icon = ic
				}
				return apps, nil
			}

			// And the same placement path as the settings file: desk.Send, with
			// its menu-bar allowance and its reporting. A second path here would
			// drift from the one that runs at start-up.
			d.OnPlace = func(places []desk.Placement) {
				if !screens.Virtual {
					fmt.Printf("not moving anything: these are the displays this "+
						"Mac already has (%s)\n", screens.Why)
					return
				}
				done, err := desk.Send(desk.TheBench(), ribbonIDs(mirror, macID, screens.IDs), places)
				for _, line := range done {
					fmt.Printf("%s\n", line)
				}
				if err != nil {
					for _, line := range strings.Split(err.Error(), "\n") {
						fmt.Printf("%s\n", line)
					}
				}
			}

			d.OnCycle = func(pos int) {
				o, ok := inv.Cycle(pos)
				if !ok {
					old, _ := d.SetFeed(pos, nil)
					closeFeed(old)
					fmt.Printf("screen %d: nothing\n", pos+1)
					remirror()
					return
				}
				f, err := desk.OpenOffer(ctx, plan, o)
				if err != nil {
					fmt.Printf("screen %d: %v\n", pos+1, err)
					return
				}
				old, err := d.SetFeed(pos, f)
				if err != nil {
					fmt.Printf("screen %d: %v\n", pos+1, err)
					closeFeed(f)
					return
				}
				closeFeed(old)
				// What this position shows has changed, so what may be dark
				// has changed with it -- in both directions.
				remirror()
				fmt.Printf("screen %d: %s\n", pos+1, o.Name)
			}
		}

		fmt.Printf("arrow keys or h/l turn the ribbon, space promotes, tab or c changes what a screen shows, q quits\n")
		start := time.Now()
		opts := desk.RunOptions{
			Title: "xrdesk", Screen: chosen, For: *forDur, Logf: logf,
			NoGlobal:    *noGlobal,
			Interactive: *interactive,
			Stereo3D:    *stereo3D,
			DepthModel:  *depthModel,
			Badge:       settings.BadgeSeconds(),
			Windowed:    !settings.Immersive(),
			Shortcuts:   settings.ShortcutsOr(desk.DefaultShortcuts()),
			Hotkeys:     settings.HotkeyOptions(),
			// The menu bar is an input like any other: same actions, same loop.
			Actions: actions,
			// And it says which key does the same thing, once the machine has
			// said which keys we may have. The item is made before this session
			// and outlives it, so it is told rather than asked.
			OnGranted: menuBar.ShowShortcuts,
			// And the band follows the pointer onto any of these, whether it is a
			// screen this program made or this Mac's own panel mirrored onto one.
			Screens: ribbonIDs(mirror, macID, screens.IDs),
			Showing: showing,
		}
		if *snap {
			opts.Snapshot = func(pix []byte, w, h int) {
				path, err := writeSnapshot(pix, w, h)
				if err != nil {
					fmt.Printf("snapshot: %v\n", err)
					return
				}
				fmt.Printf("first frame written to %s\n", path)
			}
		}
		if err := desk.Run(ctx, plan, d, opts); err != nil {
			fmt.Printf("%v\n", err)
			return false, false, 1
		}
		fmt.Printf("ran for %s\n", time.Since(start).Round(time.Millisecond))
		// The desk stopped. It asked for the settings, or it is simply done.
		return d.WantsSettings(), d.WantsSettings(), 0
	}

	for {
		n, model, dist, sp := flagCount, flagScreen, flagDistance, flagSplay
		if n == 0 {
			n = settings.Screens()
		}
		if model == "" {
			model = settings.Model()
		}
		if dist == 0 {
			dist = settings.Distance()
		}
		if sp == 0 {
			// The settings' own answer, which distinguishes "not said" from "flat"
			// -- and a flat band asked for in the file has to come through as one.
			if sp = settings.SplayDeg(); sp == 0 {
				sp = -1
			}
		}
		again, wantSettings, code := session(n, model, dist, sp, settings)
		if !again {
			return code
		}
		if !wantSettings {
			// The session asked to be run again without changing anything --
			// the screens could not be made and it is waiting for the machine to
			// be ready. Straight back in.
			continue
		}
		// Asked for from the glasses or from the menu bar. The desk is down, so
		// the settings window has the screen to itself and what it changes is
		// read on the way back in -- which is the only order in which a new
		// screen count or a different headset can mean anything.
		if err := desk.RunSettings(desk.SettingsOptions{
			Logf: logf, DisplayH: tallestDisplay(),
		}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if fresh, err := desk.LoadConfig(); err == nil {
			settings = fresh
		} else {
			logf("%v", err)
		}
	}
}

// writeSnapshot saves the picture the glasses were shown, OUTSIDE any
// repository.
//
// A frame of this program is a picture of whoever ran it, at work — every screen
// on the ribbon is one of their displays. It goes where durable per-user data
// goes, never where a `git add` could reach it, and the path is printed so it
// can be found.
func writeSnapshot(pix []byte, w, h int) (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("no user configuration directory: %w", err)
	}
	dir := filepath.Join(base, "go-xrkit-desk", "snapshots")
	if root := repoRootOf(dir); root != "" {
		return "", fmt.Errorf("%s is inside the git work tree at %s; "+
			"a picture of somebody's screens must never be written where it can be committed",
			dir, root)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	img := &image.NRGBA{Pix: pix, Stride: w * 4, Rect: image.Rect(0, 0, w, h)}
	path := filepath.Join(dir, fmt.Sprintf("xrdesk-%dx%d.png", w, h))
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	return path, png.Encode(f, img)
}

// repoRootOf returns the work tree dir is inside, or "" if it is in none. A
// .git that is a FILE is a worktree, and commits just as well as a directory.
func repoRootOf(dir string) string {
	for d := dir; ; {
		if fi, err := os.Stat(filepath.Join(d, ".git")); err == nil &&
			(fi.IsDir() || fi.Mode().IsRegular()) {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			return ""
		}
		d = parent
	}
}

// closeFeed releases a feed the ribbon no longer holds, and says so if it
// refuses. SetFeed hands the old one back rather than closing it, because
// swapping and discarding are not the same gesture — this is the discarding.
func closeFeed(f desk.Feed) {
	if f == nil {
		return
	}
	if err := f.Close(); err != nil {
		fmt.Printf("closing a replaced screen: %v\n", err)
	}
}

// tallestDisplay is the height of the biggest display attached, which is where
// a window macOS places for itself is most likely to land — and the one whose
// pixels are smallest to look at.
//
// The settings window is scaled for it rather than for the glasses: it is shown
// BEFORE the glasses are chosen, so it appears on the desktop, and a person
// reading it is not wearing anything yet.
func tallestDisplay() int {
	ss, err := window.Screens()
	if err != nil {
		return 0 // no scaling rather than a guess
	}
	tallest := 0
	for _, s := range ss {
		if s.Height > tallest {
			tallest = s.Height
		}
	}
	return tallest
}

// mainDisplay is the offer for this machine's own main screen.
//
// By the property, not by the label: Offer.Main says which screen carries the
// menu bar, and a caller that looked for "(main)" in the NAME would be reading
// a sentence back out of something written for a person to read.
func mainDisplay(offers []desk.Offer) (desk.Offer, bool) {
	for _, o := range offers {
		if o.Kind == desk.KindDisplay && o.Main {
			return o, true
		}
	}
	return desk.Offer{}, false
}

// ribbonIDs is the display each ribbon position shows, in order.
//
// It exists because "the screens this program made" and "the screens on the
// band" stopped being the same list the day screen 1 became this Mac's own.
// Everything that maps a POSITION to a display -- placing an application,
// taking a screen away, following the pointer -- has to ask this and not
// Screens.IDs, or it is off by one for the whole session.
func ribbonIDs(mirror bool, mac uint64, made []uint64) []uint64 {
	if !mirror || mac == 0 {
		return made
	}
	return append([]uint64{mac}, made...)
}
