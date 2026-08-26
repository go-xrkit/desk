// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func ptr[T any](v T) *T { return &v }

// TestWhatIsWrittenIsWhatIsReadBack is the property that makes a settings
// window safe to have: whatever it saves, the application then loads as the
// same settings. Nothing is checked by looking at the text.
func TestWhatIsWrittenIsWhatIsReadBack(t *testing.T) {
	for name, want := range map[string]Config{
		"empty": {},
		"everything": {
			Shortcuts: []ConfigShortcut{
				{Action: "gallery", Keys: "control+option+command+space"},
				{Action: "next", Keys: "option+command+right"},
			},
			Fallback: ptr([]string{"control", "shift", "control+shift"}),
			Ribbon:   &ConfigRibbon{Screens: ptr(9), Immersive: ptr(false)},
			Glasses:  &ConfigGlasses{Model: ptr("VITURE Luma Ultra")},
			Places: []ConfigPlace{
				{App: "Safari", Screen: 1},
				{App: "Terminal", Screen: 2},
			},
		},
		"a fallback of nothing at all": {Fallback: ptr([]string{})},
		"only a screen count":          {Ribbon: &ConfigRibbon{Screens: ptr(3)}},
		"only the immersive choice":    {Ribbon: &ConfigRibbon{Immersive: ptr(true)}},
		// A name with a quotation mark in it: written by printing strings, this
		// is the one that produces a file that will not parse.
		"an application with a quotation mark in its name": {
			Places: []ConfigPlace{{App: `He said "hello"`, Screen: 1}},
		},
	} {
		path := filepath.Join(t.TempDir(), "desk.hcl")
		if err := want.SaveTo(path); err != nil {
			t.Errorf("%s: SaveTo = %v", name, err)
			continue
		}
		got, err := LoadConfigFile(path)
		if err != nil {
			t.Errorf("%s: reading back what was written: %v\n%s", name, err, want.Bytes())
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: read back\n%#v\nwant\n%#v\nfrom:\n%s", name, got, want, want.Bytes())
		}
	}
}

// TestWhatIsNotSetIsNotWritten. A file with `screens = 0` looks like a decision
// and means the same as saying nothing; only one of those is honest.
func TestWhatIsNotSetIsNotWritten(t *testing.T) {
	text := string(Config{Ribbon: &ConfigRibbon{Immersive: ptr(true)}}.Bytes())
	if strings.Contains(text, "screens") {
		t.Errorf("a screen count nobody chose was written:\n%s", text)
	}
	if empty := string(Config{}.Bytes()); strings.TrimSpace(empty) != "" {
		t.Errorf("empty settings wrote something:\n%q", empty)
	}
}

// TestSaveRefusesSettingsItWouldNotLoad: writing a file the application then
// refuses to start with is worse than not saving.
func TestSaveRefusesSettingsItWouldNotLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "desk.hcl")
	bad := Config{Places: []ConfigPlace{{App: "Safari", Screen: 0}}}
	if err := bad.SaveTo(path); !errors.Is(err, ErrConfig) {
		t.Errorf("SaveTo = %v, want an ErrConfig", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Error("a file was written anyway")
	}
}

// TestSaveMakesItsOwnDirectory: nobody has ~/Library/Application Support/go-xrkit
// before the first save.
func TestSaveMakesItsOwnDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not", "there", "yet", "desk.hcl")
	if err := (Config{Ribbon: &ConfigRibbon{Screens: ptr(3)}}).SaveTo(path); err != nil {
		t.Fatalf("SaveTo = %v", err)
	}
	if _, err := LoadConfigFile(path); err != nil {
		t.Errorf("reading back: %v", err)
	}
}

// TestSaveDoesNotLeaveHalfAFileBehind.
//
// It writes a new file and renames it over the old one, so a save interrupted
// part-way cannot leave settings that will not parse — which the next start-up
// would refuse to run with.
func TestSaveDoesNotLeaveHalfAFileBehind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "desk.hcl")
	if err := (Config{Ribbon: &ConfigRibbon{Screens: ptr(3)}}).SaveTo(path); err != nil {
		t.Fatal(err)
	}
	if err := (Config{Ribbon: &ConfigRibbon{Screens: ptr(6)}}).SaveTo(path); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "desk.hcl" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("the directory holds %v, want only desk.hcl", names)
	}
	c, err := LoadConfigFile(path)
	if err != nil || c.Screens() != 6 {
		t.Errorf("after the second save: %v, %v", c.Screens(), err)
	}
}

// TestSaveWhereItCannotWrite.
func TestSaveWhereItCannotWrite(t *testing.T) {
	dir := t.TempDir()
	// The settings file's own name is already a DIRECTORY: everything up to the
	// rename works, and the rename is what cannot.
	taken := filepath.Join(dir, "desk.hcl")
	if err := os.Mkdir(taken, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := (Config{}).SaveTo(taken); !errors.Is(err, ErrConfig) {
		t.Errorf("SaveTo onto a directory = %v, want an ErrConfig", err)
	}

	// And the temporary name itself already taken by a directory: the write
	// cannot even start.
	blocked := t.TempDir()
	if err := os.Mkdir(filepath.Join(blocked, ".desk.hcl.new"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := (Config{}).SaveTo(filepath.Join(blocked, "desk.hcl")); !errors.Is(err, ErrConfig) {
		t.Errorf("SaveTo with its temporary name taken = %v, want an ErrConfig", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".desk.hcl.new")); !errors.Is(err, os.ErrNotExist) {
		t.Error("the half-written file was left behind")
	}

	// A path whose parent is a FILE: the directory cannot be made at all.
	f := filepath.Join(dir, "a file")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (Config{}).SaveTo(filepath.Join(f, "desk.hcl")); !errors.Is(err, ErrConfig) {
		t.Errorf("SaveTo under a file = %v, want an ErrConfig", err)
	}
}

// TestSaveGoesWhereConfigPathSays.
func TestSaveGoesWhereConfigPathSays(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chosen.hcl")
	t.Setenv(EnvConfig, path)
	got, err := (Config{Ribbon: &ConfigRibbon{Screens: ptr(4)}}).Save()
	if err != nil {
		t.Fatalf("Save = %v", err)
	}
	if got != path {
		t.Errorf("Save wrote %q, want %q", got, path)
	}
	c, err := LoadConfig()
	if err != nil || c.Screens() != 4 {
		t.Errorf("LoadConfig after Save = %d, %v", c.Screens(), err)
	}

	// And with nowhere to put settings, Save says so rather than choosing.
	t.Setenv(EnvConfig, "")
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("AppData", "")
	if _, err := (Config{}).Save(); !errors.Is(err, ErrConfig) {
		t.Errorf("Save with nowhere to write = %v, want an ErrConfig", err)
	}
}
