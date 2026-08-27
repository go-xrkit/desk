// xrdesk shows several screens on a 360° ribbon inside AR glasses.
//
// It is the application; cmd/deskcheck is the probe that says what this machine
// can do without taking a display over.
package main

import (
	"context"
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
	quiet := flag.Bool("quiet", false, "say less")
	noGlobal := flag.Bool("no-global", false,
		"do not claim the system-wide shortcuts (\u2325\u2318\u2190/\u2192 and \u2325\u2318Space)")
	settingsWin := flag.Bool("settings", false,
		"open the settings window instead of the desk")
	snap := flag.Bool("snapshot", false, "write the first frame shown, so it can be looked at afterwards")
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
	if tray, err := desk.OpenTray(logf, actions); err != nil {
		logf("%v", err)
	} else {
		defer func() { _ = tray.Close() }()
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
	session := func(n int, model string, dist, splay float64, settings desk.Config) (again bool, code int) {
		ss, err := window.Screens()
		if err != nil {
			fmt.Printf("cannot list displays: %v\n", err)
			return false, 1
		}
		ds := make([]glasses.Display, len(ss))
		for i, s := range ss {
			ds[i] = glasses.Display{Name: s.Name, Width: s.Width, Height: s.Height, Primary: s.Primary}
		}
		chosen, err := glasses.ChooseDisplay(ds, model)
		if err != nil {
			fmt.Printf("%v\n", err)
			return false, 1
		}
		fmt.Printf("on %s\n", chosen)
		if advice := glasses.ScalingAdvice(chosen); advice != "" {
			logf("%s", advice)
		}

		plan, err := desk.NewPlan(chosen, desk.Options{
			Screens: n, FOVDeg: *fov,
			USB: desk.EvidenceFor(chosen, model != "", desk.Peripherals()),
		})
		if err != nil {
			fmt.Printf("%v\n", err)
			return false, 1
		}
		logf("%s", plan)

		// Ctrl-C must reach the same exit as the quit key, or a session left running
		// keeps virtual displays the person never asked for.
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()

		screens, err := desk.Provide(ctx, plan, logf)
		if err != nil {
			fmt.Printf("%v\n", err)
			return false, 1
		}
		defer func() {
			if err := screens.Close(); err != nil {
				fmt.Printf("WARNING: could not remove every virtual display: %v\n", err)
			}
		}()

		// The applications, before the pixels. A screen with nothing on it is what
		// the first version of this showed a person wearing the glasses, and it
		// read as broken rather than as empty.
		if places := settings.Placements(); len(places) > 0 {
			done, err := desk.Send(desk.TheBench(), screens.IDs, places)
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

		feeds, err := desk.Capture(ctx, plan, screens, logf)
		if err != nil {
			fmt.Printf("%v\n", err)
			return false, 1
		}
		d, err := desk.New(plan, feeds)
		if err != nil {
			fmt.Printf("%v\n", err)
			return false, 1
		}
		defer d.Close()

		// What a ribbon position shows is chosen while it runs. The inventory is the
		// list; Cycle is that list reduced to one key.
		if offers, err := desk.Sources(ctx, screens); err != nil {
			logf("cannot list what could be shown: %v", err)
		} else if inv, err := desk.NewInventory(plan.Count(), offers); err != nil {
			logf("inventory: %v", err)
		} else {
			// Fill the ribbon the way one key would, so a session starts with
			// something on every position rather than with a ring of holes.
			for i := 0; i < plan.Count(); i++ {
				if o, ok := inv.Cycle(i); ok {
					logf("screen %d: %s", i+1, o.Name)
				}
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

			d.OnCycle = func(pos int) {
				o, ok := inv.Cycle(pos)
				if !ok {
					old, _ := d.SetFeed(pos, nil)
					closeFeed(old)
					fmt.Printf("screen %d: nothing\n", pos+1)
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
				fmt.Printf("screen %d: %s\n", pos+1, o.Name)
			}
		}

		fmt.Printf("arrow keys or h/l turn the ribbon, space promotes, tab or c changes what a screen shows, q quits\n")
		start := time.Now()
		opts := desk.RunOptions{
			Title: "xrdesk", Screen: chosen, For: *forDur, Logf: logf,
			NoGlobal:  *noGlobal,
			Badge:     settings.BadgeSeconds(),
			Windowed:  !settings.Immersive(),
			Shortcuts: settings.ShortcutsOr(desk.DefaultShortcuts()),
			Hotkeys:   settings.HotkeyOptions(),
			// The menu bar is an input like any other: same actions, same loop.
			Actions: actions,
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
			return false, 1
		}
		fmt.Printf("ran for %s\n", time.Since(start).Round(time.Millisecond))
		return d.WantsSettings(), 0
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
		again, code := session(n, model, dist, sp, settings)
		if !again {
			return code
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
