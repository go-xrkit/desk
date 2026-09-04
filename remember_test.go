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
)

// TestRememberingAScreenKeepsWhatSomebodyWrote is the reason this does not go
// through Config.Save.
//
// Save renders the whole file from the struct, so every comment and every
// blank line a person put there disappears. That is fine behind a Save button.
// It is not fine on the side of adding a screen: nobody clicking a tile in the
// gallery has agreed to have their settings file rewritten.
func TestRememberingAScreenKeepsWhatSomebodyWrote(t *testing.T) {
	const original = `# My desk. Do not laugh at the shortcuts.
shortcut "next" {
  keys = "cmd+alt+right" # the one I actually use
}

ribbon {
  # three is enough on the train
  screens  = 3
  distance = 2
}

# Everything below is deliberate.
fallback = ["control", "shift"]
`
	path := filepath.Join(t.TempDir(), "desk.hcl")
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RememberScreens(path, 5); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(after)

	for _, want := range []string{
		"# My desk. Do not laugh at the shortcuts.",
		"# the one I actually use",
		"# three is enough on the train",
		"# Everything below is deliberate.",
		`fallback = ["control", "shift"]`,
		"distance = 2",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the file lost %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "screens  = 3") || strings.Contains(got, "screens = 3") {
		t.Errorf("the old count is still there:\n%s", got)
	}

	// And it reads back as five, which is the whole point.
	cfg, err := LoadConfigFile(path)
	if err != nil {
		t.Fatalf("the edited file no longer parses: %v", err)
	}
	if cfg.Screens() != 5 {
		t.Errorf("Screens() = %d, want 5", cfg.Screens())
	}
	if cfg.Distance() != 2 {
		t.Errorf("Distance() = %v, want the 2 that was already there", cfg.Distance())
	}
}

// TestASettingsFileWithNoRibbonBlockGetsOne.
func TestASettingsFileWithNoRibbonBlockGetsOne(t *testing.T) {
	path := filepath.Join(t.TempDir(), "desk.hcl")
	if err := os.WriteFile(path, []byte("# nothing but a note\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RememberScreens(path, 4); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Screens() != 4 {
		t.Errorf("Screens() = %d, want 4", cfg.Screens())
	}
	b, _ := os.ReadFile(path)
	if !strings.Contains(string(b), "# nothing but a note") {
		t.Errorf("the note is gone:\n%s", b)
	}
}

// TestAFileThatIsNotThereYetIsCreatedSayingOnlyThis. It claims nothing it was
// not told.
func TestAFileThatIsNotThereYetIsCreated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deeper", "desk.hcl")
	if err := RememberScreens(path, 2); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Screens() != 2 {
		t.Errorf("Screens() = %d, want 2", cfg.Screens())
	}
	if len(cfg.Shortcuts) != 0 || cfg.Fallback != nil {
		t.Errorf("the new file invented settings nobody asked for: %+v", cfg)
	}
}

// TestRememberingTwiceLeavesOneAttribute, rather than appending a second
// screens line that would make the file ambiguous.
func TestRememberingTwiceLeavesOneAttribute(t *testing.T) {
	path := filepath.Join(t.TempDir(), "desk.hcl")
	for _, n := range []int{2, 7, 3} {
		if err := RememberScreens(path, n); err != nil {
			t.Fatalf("remembering %d: %v", n, err)
		}
	}
	b, _ := os.ReadFile(path)
	if got := strings.Count(string(b), "screens"); got != 1 {
		t.Errorf("the word screens appears %d times:\n%s", got, b)
	}
	cfg, err := LoadConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Screens() != 3 {
		t.Errorf("Screens() = %d, want the last one written", cfg.Screens())
	}
}

// TestACountTheNextStartUpWouldRefuseIsNotWritten.
//
// The point of writing this down is that start-up reads it. A number Config
// would refuse must not reach the file, and the refusal names the ceiling for
// the same reason Config.check does.
func TestACountTheNextStartUpWouldRefuseIsNotWritten(t *testing.T) {
	dir := t.TempDir()
	for _, c := range []struct {
		name string
		n    int
		want string
	}{
		{"none at all", 0, "is not a desk"},
		{"a negative", -1, "is not a desk"},
		{"above the ceiling", MaxScreens + 1, "the most a desk carries"},
	} {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(dir, c.name+".hcl")
			err := RememberScreens(path, c.n)
			if err == nil {
				t.Fatal("accepted")
			}
			if !errors.Is(err, ErrConfig) {
				t.Errorf("error = %v, want it to read as a settings problem", err)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error = %q, which does not mention %q", err, c.want)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Error("a file was written anyway")
			}
		})
	}
	if err := RememberScreens("", 3); err == nil {
		t.Error("an empty path was accepted")
	}
}

// TestAFileThatDoesNotParseIsLeftAlone.
//
// It is somebody's file with a mistake in it. Rewriting it from scratch would
// replace their mistake with our opinion and lose everything else.
func TestAFileThatDoesNotParseIsLeftAlone(t *testing.T) {
	const broken = "ribbon {\n  screens = \n"
	path := filepath.Join(t.TempDir(), "desk.hcl")
	if err := os.WriteFile(path, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	err := RememberScreens(path, 4)
	if err == nil {
		t.Fatal("a file that does not parse was rewritten")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error = %q, which does not say which file to look at", err)
	}
	b, _ := os.ReadFile(path)
	if string(b) != broken {
		t.Errorf("the file was changed:\n%s", b)
	}
}

// TestADirectoryInTheWayIsReported, rather than being reported as a count
// problem.
func TestADirectoryInTheWayIsReported(t *testing.T) {
	dir := t.TempDir()
	inTheWay := filepath.Join(dir, "desk.hcl")
	if err := os.Mkdir(inTheWay, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := RememberScreens(inTheWay, 3); err == nil {
		t.Error("a directory was written to")
	}
}

// TestTheDeskTellsWhoeverWantsToRemember. The hook is what connects a viewer's
// action to a settings file, and it fires with the count there is NOW.
func TestTheDeskTellsWhoeverWantsToRemember(t *testing.T) {
	p := testPlan(t)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	was := p.Count()

	var seen []int
	d.OnScreens = func(n int) { seen = append(seen, n) }

	d.grow(func() (Feed, error) { return newFakeFeed(p.ScreenW, p.ScreenH, 42), nil })
	d.drop(0)

	if len(seen) != 2 || seen[0] != was+1 || seen[1] != was {
		t.Errorf("the desk reported %v, want [%d %d]", seen, was+1, was)
	}
}

// TestAGrowThatFailedIsNotRemembered. Writing a count down for a screen that
// was never added would make the next start-up ask for one that does not
// exist.
func TestAGrowThatFailedIsNotRemembered(t *testing.T) {
	p := testPlan(t)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	told := 0
	d.OnScreens = func(int) { told++ }

	d.grow(func() (Feed, error) { return nil, errors.New("no display to be had") })
	if told != 0 {
		t.Errorf("a failed grow was reported %d time(s)", told)
	}
	if d.Err() == nil {
		t.Error("the failure was not kept where a viewer can be shown it")
	}
}
