// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-macos/hotkey"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
)

// ErrConfig says a settings file could not be used.
var ErrConfig = errors.New("desk: unusable settings")

// EnvConfig names the environment variable that overrides which file the
// settings are read from. It exists so a test, or a person trying something
// out, can point at a scratch file without touching the real one.
const EnvConfig = "XRDESK_CONFIG"

// ConfigPath reports the file the settings are read from.
//
// That is $XRDESK_CONFIG when it is set, and otherwise desk.hcl under the
// platform's own configuration directory, beside the glasses catalogue:
//
//	~/Library/Application Support/go-xrkit/desk.hcl   (macOS)
//	~/.config/go-xrkit/desk.hcl                       (Linux, or $XDG_CONFIG_HOME)
//	%AppData%\go-xrkit\desk.hcl                       (Windows)
//
// It fails only when the platform cannot say where a person's configuration
// lives, which on a Unix means HOME is unset.
func ConfigPath() (string, error) {
	if path := os.Getenv(EnvConfig); path != "" {
		return path, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("%w: cannot locate the settings: %w", ErrConfig, err)
	}
	return filepath.Join(dir, "go-xrkit", "desk.hcl"), nil
}

// Config is everything a person may set without a Go toolchain.
//
// HCL rather than JSON, for the two reasons a person gives when asked: it takes
// comments, so a file can say WHY a shortcut was moved, and it has a schema, so
// a typo is a diagnostic pointing at a line rather than a field silently left
// at its zero value.
type Config struct {
	// Shortcuts are the system-wide combinations, by action name.
	Shortcuts []ConfigShortcut `hcl:"shortcut,block"`

	// Fallback is what to add, in order, when a combination is already taken.
	// Nil means [DefaultLadder]; an explicitly empty list means do not fall
	// back at all, and report the refusal instead.
	Fallback *[]string `hcl:"fallback"`

	Ribbon  *ConfigRibbon  `hcl:"ribbon,block"`
	Glasses *ConfigGlasses `hcl:"glasses,block"`

	// Places are the applications to put on the band at start-up.
	Places []ConfigPlace `hcl:"place,block"`
}

// ConfigPlace is one `place "Safari" { screen = 2 }` block.
type ConfigPlace struct {
	App    string `hcl:"app,label"`
	Screen int    `hcl:"screen"`
}

// ConfigShortcut is one `shortcut "next" { keys = "..." }` block.
type ConfigShortcut struct {
	Action string `hcl:"action,label"`
	Keys   string `hcl:"keys"`
}

// ConfigRibbon is the `ribbon { }` block.
type ConfigRibbon struct {
	// Screens is how many virtual screens to put on the band. Zero asks for
	// [DefaultScreens].
	Screens *int `hcl:"screens"`

	// BadgeSeconds is how long the screen's number stays up after the band
	// moves. Nil means [DefaultBadgeSeconds]; zero turns it off.
	BadgeSeconds *float64 `hcl:"badge_seconds"`

	// Immersive covers the glasses display's own menu bar and Dock. Nil means
	// true.
	//
	// macOS draws a menu bar on EVERY display when Spaces are separate, so the
	// glasses carry one of their own — and being drawn at a level above an
	// ordinary window, it sits on top of the picture. Covering it is the whole
	// point: the desktop being shown has a menu bar of its own already, and two
	// of them was the first thing anyone noticed wearing this.
	//
	// Turn it off if the glasses are the MAIN display, where that bar and the
	// Dock are the real ones rather than a copy on a screen nobody is using.
	Immersive *bool `hcl:"immersive"`
}

// ConfigGlasses is the `glasses { }` block.
type ConfigGlasses struct {
	// Model names which headset to use when several are attached, as the
	// catalogue names it.
	Model *string `hcl:"model"`
}

// DefaultLadder is what is added to a taken combination, in order.
//
// Control before Shift, deliberately. ⌥⌘Space is the Finder's search window on
// a stock macOS, so the gallery always falls back — and ⌃⌥⌘Space is both freer
// on a typical machine and easier to hold than ⌥⇧⌘Space, which puts three
// modifiers under one hand.
var DefaultLadder = []hotkey.Modifier{
	hotkey.Control,
	hotkey.Shift,
	hotkey.Control | hotkey.Shift,
}

// LoadConfig reads [ConfigPath]. A file that is not there is not an error: it
// means every default stands.
func LoadConfig() (Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return Config{}, err
	}
	c, err := LoadConfigFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, nil
	}
	return c, err
}

// LoadConfigFile reads one settings file.
func LoadConfigFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var c Config
	p := hclparse.NewParser()
	f, diags := p.ParseHCL(data, path)
	if !diags.HasErrors() {
		diags = append(diags, gohcl.DecodeBody(f.Body, nil, &c)...)
	}
	if diags.HasErrors() {
		return Config{}, fmt.Errorf("%w: %s", ErrConfig, diagnostics(p, diags))
	}
	if err := c.check(); err != nil {
		return Config{}, err
	}
	return c, nil
}

// check refuses a file that parses and still cannot be used.
func (c Config) check() error {
	seen := map[string]bool{}
	for _, s := range c.Shortcuts {
		if _, ok := actionByName(s.Action); !ok {
			return fmt.Errorf("%w: %q is not an action; the ones there are: %s",
				ErrConfig, s.Action, actionNames())
		}
		if seen[s.Action] {
			return fmt.Errorf("%w: %q is given twice", ErrConfig, s.Action)
		}
		seen[s.Action] = true
		if _, err := hotkey.ParseCombo(s.Keys); err != nil {
			return fmt.Errorf("%w: shortcut %q: %w", ErrConfig, s.Action, err)
		}
	}
	if c.Fallback != nil {
		for _, m := range *c.Fallback {
			if _, err := hotkey.ParseModifier(m); err != nil {
				return fmt.Errorf("%w: fallback: %w", ErrConfig, err)
			}
		}
	}
	if c.Ribbon != nil && c.Ribbon.BadgeSeconds != nil && *c.Ribbon.BadgeSeconds < 0 {
		return fmt.Errorf("%w: badge_seconds = %g", ErrConfig, *c.Ribbon.BadgeSeconds)
	}
	if c.Ribbon != nil && c.Ribbon.Screens != nil && *c.Ribbon.Screens < 0 {
		return fmt.Errorf("%w: ribbon screens = %d", ErrConfig, *c.Ribbon.Screens)
	}
	// Named, not clamped. A person who wrote a number in this file expects it to
	// mean something, and silently running six screens because they asked for
	// sixty is worse than saying so -- the ceiling is arbitrary, so it has to be
	// stated wherever it bites.
	if c.Ribbon != nil && c.Ribbon.Screens != nil && *c.Ribbon.Screens > MaxScreens {
		return fmt.Errorf("%w: ribbon screens = %d, and %d is the most a desk carries",
			ErrConfig, *c.Ribbon.Screens, MaxScreens)
	}
	for _, p := range c.Places {
		if strings.TrimSpace(p.App) == "" {
			return fmt.Errorf("%w: a place with no application to put there", ErrConfig)
		}
		if p.Screen < 1 {
			return fmt.Errorf("%w: place %q on screen %d; screens are counted from 1",
				ErrConfig, p.App, p.Screen)
		}
	}
	return nil
}

// ShortcutsOr returns the configured shortcuts, falling back to base for any
// action the file does not mention.
//
// A settings file that names one shortcut must not silently drop the other
// two: a person moving the gallery key is not asking to lose the arrows.
func (c Config) ShortcutsOr(base []Shortcut) []Shortcut {
	out := append([]Shortcut(nil), base...)
	for _, s := range c.Shortcuts {
		a, _ := actionByName(s.Action)
		combo, _ := hotkey.ParseCombo(s.Keys) // check() has already refused a bad one
		replaced := false
		for i := range out {
			if out[i].Does == a {
				out[i].Want, replaced = combo, true
				break
			}
		}
		if !replaced {
			out = append(out, Shortcut{Want: combo, Does: a})
		}
	}
	return out
}

// Ladder returns the fallback ladder this configuration asks for.
//
// A file with no `fallback` gets [DefaultLadder]. A file with `fallback = []`
// gets no fallback at all, which is a real thing to want: it says "give me the
// combination I asked for or tell me you could not".
func (c Config) Ladder() []hotkey.Modifier {
	if c.Fallback == nil {
		return DefaultLadder
	}
	out := make([]hotkey.Modifier, 0, len(*c.Fallback))
	for _, s := range *c.Fallback {
		m, _ := hotkey.ParseModifier(s) // check() has already refused a bad one
		out = append(out, m)
	}
	return out
}

// HotkeyOptions is what [ClaimGlobal] should be given for this configuration.
func (c Config) HotkeyOptions() *hotkey.Options {
	return &hotkey.Options{Ladder: c.Ladder()}
}

// Screens is how many screens to put on the ribbon, or 0 for as many as fit.
func (c Config) Screens() int {
	if c.Ribbon == nil || c.Ribbon.Screens == nil {
		return 0
	}
	return *c.Ribbon.Screens
}

// Model is the headset this configuration prefers when several are attached,
// or "" when it does not care.
func (c Config) Model() string {
	if c.Glasses == nil || c.Glasses.Model == nil {
		return ""
	}
	return *c.Glasses.Model
}

// actionNames lists the action names a settings file may use.
func actionNames() string {
	out := make([]string, 0, len(configActions))
	for name := range configActions {
		out = append(out, name)
	}
	sort.Strings(out)
	return strings.Join(out, " ")
}

// configActions are the actions a shortcut may be bound to. Not every Action is
// here: quit and fullscreen are keys in the window, and claiming them
// system-wide would take them from the application the viewer is using.
var configActions = map[string]Action{
	"previous":      ActionPrev,
	"next":          ActionNext,
	"gallery":       ActionGallery,
	"gallery-open":  ActionGalleryOpen,
	"gallery-close": ActionGalleryClose,
	"choose":        ActionChoose,
	"cycle":         ActionCycle,
}

func actionByName(name string) (Action, bool) {
	a, ok := configActions[name]
	return a, ok
}

// diagnostics renders HCL's own complaint, which names the file and the line.
func diagnostics(p *hclparse.Parser, diags hcl.Diagnostics) string {
	var b strings.Builder
	w := hcl.NewDiagnosticTextWriter(&b, p.Files(), 78, false)
	_ = w.WriteDiagnostics(diags)
	return strings.TrimSpace(b.String())
}

// Placements are the applications to put on the band, in the order written.
func (c Config) Placements() []Placement {
	if len(c.Places) == 0 {
		return nil
	}
	out := make([]Placement, len(c.Places))
	for i, p := range c.Places {
		out[i] = Placement{App: p.App, Pos: p.Screen}
	}
	return out
}

// Immersive reports whether to cover the glasses display's own menu bar and
// Dock. A file that does not say asks for it.
func (c Config) Immersive() bool {
	if c.Ribbon == nil || c.Ribbon.Immersive == nil {
		return true
	}
	return *c.Ribbon.Immersive
}

// BadgeSeconds is how long the screen's number stays up after the band moves.
// A file that does not say asks for [DefaultBadgeSeconds]; zero turns it off.
func (c Config) BadgeSeconds() float64 {
	if c.Ribbon == nil || c.Ribbon.BadgeSeconds == nil {
		return DefaultBadgeSeconds
	}
	return *c.Ribbon.BadgeSeconds
}
