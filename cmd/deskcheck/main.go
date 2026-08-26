// deskcheck says what this machine can actually do, and proves each answer by
// doing it rather than by reporting a capability bit.
//
// It exists because every part of this stack has a way of looking available and
// then declining: a headset whose optics are not catalogued, virtual displays
// behind a private API, a capture behind a permission that shows no dialog. Each
// of those has a different remedy, and a person needs to be told which one they
// are in.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-widgets/window"
	"github.com/go-xrkit/desk"
	"github.com/go-xrkit/xrkit/glasses"
)

func main() { os.Exit(run()) }

func run() int {
	screen := flag.String("screen", "", "which display to use, matched by name")
	fov := flag.Float64("fov", 0, "horizontal field of view in degrees, when the catalogue does not know")
	count := flag.Int("screens", 0, "how many screens on the ribbon (0 = as many as fit)")
	hold := flag.Duration("hold", 0, "keep the virtual displays this long, so they can be looked at")
	// Simulation is spelled out rather than inferred, and every line it produces
	// says so. A run that pretends to have hardware and does not say which parts
	// were pretend is worse than no run at all.
	sim := flag.String("simulate", "", "pretend a headset is attached, as NAME:WIDTHxHEIGHT")
	flag.Parse()

	logf := func(f string, a ...any) { fmt.Printf("  "+f+"\n", a...) }

	fmt.Println("displays")
	ss, err := window.Screens()
	if err != nil {
		fmt.Printf("  cannot list displays: %v\n", err)
		return 1
	}
	if *sim != "" {
		name, w, h, err := parseSim(*sim)
		if err != nil {
			fmt.Printf("  %v\n", err)
			return 1
		}
		ss = append(ss, window.Screen{Name: name, Width: w, Height: h})
		fmt.Printf("  SIMULATED: %q %dx%d — no such display is attached\n", name, w, h)
	}
	ds := make([]glasses.Display, len(ss))
	for i, s := range ss {
		ds[i] = glasses.Display{Name: s.Name, Width: s.Width, Height: s.Height, Primary: s.Primary}
		mark := "  "
		if p, ok := glasses.Identify(s.Name); ok {
			mark = fmt.Sprintf("  -> %s (%s)", p.Model, p.Confidence)
		}
		logf("%s%s", ds[i], mark)
	}

	chosen, err := glasses.ChooseDisplay(ds, *screen)
	if err != nil {
		fmt.Printf("\n%v\n", err)
		return 1
	}
	fmt.Printf("\nchosen: %s\n", chosen)
	if advice := glasses.ScalingAdvice(chosen); advice != "" {
		logf("%s", advice)
	}

	plan, err := desk.NewPlan(chosen, desk.Options{Screens: *count, FOVDeg: *fov})
	if err != nil {
		fmt.Printf("\nplan: %v\n", err)
		return 1
	}
	fmt.Printf("\nplan\n")
	logf("%s", plan)
	logf("one screen spans %.2f° across and %.2f° down, which is the whole view",
		plan.HFOVDeg, plan.VFOVDeg)

	ctx := context.Background()
	fmt.Printf("\nscreens\n")
	screens, err := desk.Provide(ctx, plan, logf)
	if err != nil {
		fmt.Printf("  %v\n", err)
		return 1
	}
	defer func() {
		if err := screens.Close(); err != nil {
			fmt.Printf("\nWARNING: could not remove every virtual display: %v\n", err)
		}
	}()
	logf("virtual=%v ids=%v", screens.Virtual, screens.IDs)

	fmt.Printf("\nsources\n")
	offers, err := desk.Sources(ctx, screens)
	if err != nil {
		logf("%v", err)
	} else {
		for _, o := range offers {
			logf("%s  id=%s", o, o.ID)
		}
		inv, err := desk.NewInventory(plan.Count(), offers)
		if err != nil {
			logf("inventory: %v", err)
		} else {
			// Fill the ribbon the way one key would: cycle each position
			// once, which takes the next unused source.
			for i := 0; i < plan.Count(); i++ {
				inv.Cycle(i)
			}
			fmt.Printf("\narrangement après une passe de Cycle\n")
			for _, line := range strings.Split(strings.TrimRight(inv.Describe(), "\n"), "\n") {
				logf("%s", line)
			}
		}
	}

	// What the machine will actually let us have. This is the only way to find
	// out: two of the three are taken on a stock macOS, and an application's own
	// menu key cannot be detected at all.
	fmt.Printf("\nglobal shortcuts\n")
	hk := desk.ClaimGlobal(desk.DefaultShortcuts(), nil)
	for _, line := range strings.Split(strings.TrimRight(hk.Describe(), "\n"), "\n") {
		logf("%s", line)
	}
	if err := hk.Close(); err != nil {
		logf("releasing them: %v", err)
	}

	fmt.Printf("\ncapture\n")
	feeds, err := desk.Capture(ctx, plan, screens, logf)
	if err != nil {
		logf("%v", err)
		logf("everything above works; only the pixels are missing")
	} else {
		d, err := desk.New(plan, feeds)
		if err != nil {
			fmt.Printf("  %v\n", err)
			return 1
		}
		defer d.Close()
		// Give the captures a moment to produce something, then draw a frame and
		// say how much of the panorama actually carries a screen.
		time.Sleep(500 * time.Millisecond)
		c := d.Render()
		painted := 0
		for i := 0; i+4 <= len(c.Pix); i += 4 {
			if c.Pix[i] != d.Background[0] || c.Pix[i+1] != d.Background[1] {
				painted++
			}
		}
		logf("panorama %dx%d, %.1f%% of it carries a screen",
			c.W, c.H, 100*float64(painted)/float64(c.W*c.H))
	}

	if *hold > 0 {
		fmt.Printf("\nholding for %s\n", *hold)
		time.Sleep(*hold)
	}
	fmt.Printf("\ncleaning up\n")
	return 0
}

// parseSim reads NAME:WIDTHxHEIGHT.
func parseSim(s string) (name string, w, h int, err error) {
	i := strings.LastIndex(s, ":")
	if i < 0 {
		return "", 0, 0, fmt.Errorf("simulate: want NAME:WIDTHxHEIGHT, got %q", s)
	}
	name = s[:i]
	if _, err := fmt.Sscanf(s[i+1:], "%dx%d", &w, &h); err != nil || w <= 0 || h <= 0 {
		return "", 0, 0, fmt.Errorf("simulate: want NAME:WIDTHxHEIGHT, got %q", s)
	}
	return name, w, h, nil
}
