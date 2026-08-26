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
	count := flag.Int("screens", 0, "how many screens on the ribbon (0 = as many as fit)")
	forDur := flag.Duration("for", 0, "stop after this long; 0 runs until you quit")
	quiet := flag.Bool("quiet", false, "say less")
	noGlobal := flag.Bool("no-global", false,
		"do not claim the system-wide shortcuts (\u2325\u2318\u2190/\u2192 and \u2325\u2318Space)")
	snap := flag.Bool("snapshot", false, "write the first frame shown, so it can be looked at afterwards")
	flag.Parse()

	logf := func(f string, a ...any) { fmt.Printf("  "+f+"\n", a...) }
	if *quiet {
		logf = func(string, ...any) {}
	}

	ss, err := window.Screens()
	if err != nil {
		fmt.Printf("cannot list displays: %v\n", err)
		return 1
	}
	ds := make([]glasses.Display, len(ss))
	for i, s := range ss {
		ds[i] = glasses.Display{Name: s.Name, Width: s.Width, Height: s.Height, Primary: s.Primary}
	}
	chosen, err := glasses.ChooseDisplay(ds, *screen)
	if err != nil {
		fmt.Printf("%v\n", err)
		return 1
	}
	fmt.Printf("on %s\n", chosen)
	if advice := glasses.ScalingAdvice(chosen); advice != "" {
		logf("%s", advice)
	}

	plan, err := desk.NewPlan(chosen, desk.Options{
		Screens: *count, FOVDeg: *fov, USB: desk.Peripheral(),
	})
	if err != nil {
		fmt.Printf("%v\n", err)
		return 1
	}
	logf("%s", plan)

	// Ctrl-C must reach the same exit as the quit key, or a session left running
	// keeps virtual displays the person never asked for.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	screens, err := desk.Provide(ctx, plan, logf)
	if err != nil {
		fmt.Printf("%v\n", err)
		return 1
	}
	defer func() {
		if err := screens.Close(); err != nil {
			fmt.Printf("WARNING: could not remove every virtual display: %v\n", err)
		}
	}()

	feeds, err := desk.Capture(ctx, plan, screens, logf)
	if err != nil {
		fmt.Printf("%v\n", err)
		return 1
	}
	d, err := desk.New(plan, feeds)
	if err != nil {
		fmt.Printf("%v\n", err)
		return 1
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
		NoGlobal: *noGlobal,
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
		return 1
	}
	fmt.Printf("ran for %s\n", time.Since(start).Round(time.Millisecond))
	return 0
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
