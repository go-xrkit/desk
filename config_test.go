// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-macos/hotkey"
)

// write puts a settings file somewhere and points EnvConfig at it.
func write(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "desk.hcl")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvConfig, path)
	return path
}

// TestASettingsFileSaysWhatAPersonMeant, in the form they would actually write
// it: with comments, which is why this is HCL and not JSON.
func TestASettingsFileSaysWhatAPersonMeant(t *testing.T) {
	write(t, `
# The Finder holds option+command+space, so the gallery falls back. Control
# first: it is freer, and easier to hold than three modifiers under one hand.
shortcut "gallery"  { keys = "option+command+space" }
shortcut "previous" { keys = "control+option+command+left" }

fallback = ["control", "shift"]

ribbon  { screens = 6 }
glasses { model   = "VITURE Luma Ultra" }
`)
	c, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig = %v", err)
	}
	if got, want := c.Screens(), 6; got != want {
		t.Errorf("Screens() = %d, want %d", got, want)
	}
	if got, want := c.Model(), "VITURE Luma Ultra"; got != want {
		t.Errorf("Model() = %q, want %q", got, want)
	}
	if got, want := c.Ladder(), []hotkey.Modifier{hotkey.Control, hotkey.Shift}; len(got) != 2 ||
		got[0] != want[0] || got[1] != want[1] {
		t.Errorf("Ladder() = %v, want %v", got, want)
	}

	// A file that names two shortcuts must not drop the third: moving the
	// gallery key is not asking to lose the arrows.
	got := c.ShortcutsOr(DefaultShortcuts())
	if len(got) != len(DefaultShortcuts()) {
		t.Fatalf("got %d shortcuts, want the %d there are", len(got), len(DefaultShortcuts()))
	}
	for _, s := range got {
		switch s.Does {
		case ActionPrev:
			if want := (hotkey.Combo{Key: hotkey.KeyLeftArrow,
				Mods: hotkey.Control | hotkey.Option | hotkey.Command}); s.Want != want {
				t.Errorf("previous is %v, want %v", s.Want, want)
			}
		case ActionNext: // untouched by the file
			if want := (hotkey.Combo{Key: hotkey.KeyRightArrow,
				Mods: hotkey.Option | hotkey.Command}); s.Want != want {
				t.Errorf("next is %v, want the default %v", s.Want, want)
			}
		}
	}
}

// TestABindingForAnActionThatIsNotInTheDefaults is a person binding cycle,
// which no default shortcut carries.
func TestABindingForAnActionThatIsNotInTheDefaults(t *testing.T) {
	write(t, `shortcut "cycle" { keys = "control+option+command+c" }`)
	c, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig = %v", err)
	}
	got := c.ShortcutsOr(DefaultShortcuts())
	if len(got) != len(DefaultShortcuts())+1 {
		t.Fatalf("got %d shortcuts, want one more than the %d defaults",
			len(got), len(DefaultShortcuts()))
	}
	last := got[len(got)-1]
	if last.Does != ActionCycle || last.Want.Key != hotkey.KeyC {
		t.Errorf("the added shortcut is %v for %v", last.Want, last.Does)
	}
}

// TestNoFileIsNotAnError: every default stands, and nothing has to exist.
func TestNoFileIsNotAnError(t *testing.T) {
	t.Setenv(EnvConfig, filepath.Join(t.TempDir(), "not there.hcl"))
	c, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig = %v", err)
	}
	if c.Screens() != 0 || c.Model() != "" {
		t.Errorf("an absent file produced settings: %+v", c)
	}
	if len(c.Ladder()) != len(DefaultLadder) || c.Ladder()[0] != hotkey.Control {
		t.Errorf("Ladder() = %v, want the default starting at Control", c.Ladder())
	}
	if got := c.ShortcutsOr(DefaultShortcuts()); len(got) != len(DefaultShortcuts()) {
		t.Errorf("got %d shortcuts, want the %d defaults", len(got), len(DefaultShortcuts()))
	}
	if c.HotkeyOptions() == nil {
		t.Error("HotkeyOptions() is nil")
	}
}

// TestAnEmptyFallbackIsARealThingToWant: it says "give me what I asked for or
// tell me you could not", which is different from saying nothing.
func TestAnEmptyFallbackIsARealThingToWant(t *testing.T) {
	write(t, `fallback = []`)
	c, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig = %v", err)
	}
	if got := c.Ladder(); len(got) != 0 {
		t.Errorf("Ladder() = %v, want none at all", got)
	}
}

// TestASettingsFileThatCannotBeUsedSaysWhy. A typo must be a diagnostic, not a
// field silently left at its zero value — which is the whole reason this is HCL.
func TestASettingsFileThatCannotBeUsedSaysWhy(t *testing.T) {
	for name, body := range map[string]string{
		"not HCL at all":        `shortcut "gallery" {`,
		"an action there is no": `shortcut "teleport" { keys = "option+command+t" }`,
		"the same action twice": "shortcut \"next\" { keys = \"option+command+n\" }\n" +
			"shortcut \"next\" { keys = \"option+command+m\" }",
		"a key nobody names":                `shortcut "next" { keys = "option+command+banana" }`,
		"no modifier at all":                `shortcut "next" { keys = "space" }`,
		"a fallback that is not a modifier": `fallback = ["space"]`,
		"a negative screen count":           `ribbon { screens = -1 }`,
		"an attribute there is no":          `nonsense = 3`,
	} {
		write(t, body)
		_, err := LoadConfig()
		if err == nil {
			t.Errorf("%s: LoadConfig accepted it", name)
			continue
		}
		if !errors.Is(err, ErrConfig) {
			t.Errorf("%s: LoadConfig = %v, want an ErrConfig", name, err)
		}
	}
	// And the refusal for an unknown action must name the ones that exist.
	write(t, `shortcut "teleport" { keys = "option+command+t" }`)
	_, err := LoadConfig()
	if !strings.Contains(err.Error(), "gallery") {
		t.Errorf("the error does not list the actions that would work: %v", err)
	}
}

// TestConfigPathIsBesideTheGlassesCatalogue.
func TestConfigPathIsBesideTheGlassesCatalogue(t *testing.T) {
	t.Setenv(EnvConfig, "/somewhere/else.hcl")
	if got, err := ConfigPath(); err != nil || got != "/somewhere/else.hcl" {
		t.Fatalf("ConfigPath() = %q, %v", got, err)
	}
	t.Setenv(EnvConfig, "")
	got, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath() = %v", err)
	}
	if filepath.Base(got) != "desk.hcl" || filepath.Base(filepath.Dir(got)) != "go-xrkit" {
		t.Errorf("ConfigPath() = %q, want go-xrkit/desk.hcl under the config directory", got)
	}
}

// TestLoadConfigFileReportsAnUnreadableFileAsItself, rather than as a
// diagnostic about its contents.
func TestLoadConfigFileReportsAnUnreadableFileAsItself(t *testing.T) {
	_, err := LoadConfigFile(filepath.Join(t.TempDir(), "not there.hcl"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("LoadConfigFile = %v, want an os.ErrNotExist", err)
	}
}

// TestWithNowhereToPutSettings: on a Unix, os.UserConfigDir needs HOME. A
// process without one cannot be told where its settings live, and must say so
// rather than reading some path it invented.
func TestWithNowhereToPutSettings(t *testing.T) {
	t.Setenv(EnvConfig, "")
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("AppData", "")
	if got, err := ConfigPath(); !errors.Is(err, ErrConfig) {
		t.Errorf("ConfigPath() = %q, %v; want an ErrConfig", got, err)
	}
	if _, err := LoadConfig(); !errors.Is(err, ErrConfig) {
		t.Errorf("LoadConfig() = %v, want an ErrConfig", err)
	}
}

// TestApplicationsToPutOnTheBand.
func TestApplicationsToPutOnTheBand(t *testing.T) {
	write(t, `
place "Safari"   { screen = 1 }
place "Terminal" { screen = 3 }
`)
	c, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig = %v", err)
	}
	got := c.Placements()
	if len(got) != 2 {
		t.Fatalf("Placements() = %v, want two", got)
	}
	// In the order written: a person reading their own file down the page
	// should see the desk built in that order.
	if got[0] != (Placement{App: "Safari", Pos: 1}) || got[1] != (Placement{App: "Terminal", Pos: 3}) {
		t.Errorf("Placements() = %v", got)
	}
}

func TestAPlaceThatCannotBeUsed(t *testing.T) {
	for name, body := range map[string]string{
		"a screen counted from zero": `place "Safari" { screen = 0 }`,
		"a screen before the first":  `place "Safari" { screen = -1 }`,
		"no application at all":      `place "  " { screen = 1 }`,
	} {
		write(t, body)
		if _, err := LoadConfig(); !errors.Is(err, ErrConfig) {
			t.Errorf("%s: LoadConfig accepted it (%v)", name, err)
		}
	}
	// And a file with no place block asks for nothing.
	write(t, `ribbon { screens = 3 }`)
	c, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig = %v", err)
	}
	if got := c.Placements(); got != nil {
		t.Errorf("Placements() = %v, want none", got)
	}
}
