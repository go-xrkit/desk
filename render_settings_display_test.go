// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

// Drawing the settings window and leaving the picture where a person can look
// at it.
//
// Bounds prove a layout is a column and not a row; they say nothing about
// whether it can be READ. This window was a column, and unreadable: the text
// began at the window's own left edge, the help captions were drawn in a grey
// a shade off the background, and every glyph the font lacked — ⌥ ⌘ ← ' — came
// out as a hole in the middle of a sentence. None of that is visible in a
// rectangle. It was visible the moment the thing was rendered and looked at.
package desk

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit"
	"github.com/go-xrkit/xrkit/glasses"
)

// EnvRenderDir overrides where the rendered window is written.
const EnvRenderDir = "XRDESK_RENDER_DIR"

// renderDir is a durable directory OUTSIDE every repository.
//
// Not t.TempDir: that is removed when the test ends, so the picture is gone
// before anyone can open it, and the whole point is that somebody looks. Not
// the working directory either — a picture written inside a work tree is one
// `git add -A` from being published, and this is a rule about screen captures
// that a rendered window has no business being an exception to.
//
// So the choice is checked rather than trusted: the directory is walked up to
// the filesystem root, and a `.git` anywhere above it fails the test naming the
// tree. A `.git` FILE counts — that is a worktree, and it is committable too.
func renderDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv(EnvRenderDir)
	if dir == "" {
		cfg, err := os.UserConfigDir()
		if err != nil {
			t.Skipf("nowhere durable to write a picture: %v", err)
		}
		dir = filepath.Join(cfg, "go-xrkit", "renders")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatalf("%s: %v", dir, err)
	}
	for at := abs; ; {
		if _, err := os.Stat(filepath.Join(at, ".git")); err == nil {
			t.Fatalf("%s is inside the work tree at %s; a rendered window written "+
				"there is one `git add -A` from being published", abs, at)
		}
		parent := filepath.Dir(at)
		if parent == at {
			break
		}
		at = parent
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		t.Fatalf("%s: %v", abs, err)
	}
	return abs
}

// TestTheRenderDirectoryRefusesAWorkTree is the negative control for the rule
// above: both directions, because a check that only ever passes is not a check.
func TestTheRenderDirectoryRefusesAWorkTree(t *testing.T) {
	// Inside a work tree: this repository is one.
	inside := &testing.T{}
	func() {
		defer func() { recover() }() // t.Fatalf on a bare T calls runtime.Goexit
		done := make(chan struct{})
		go func() {
			defer close(done)
			defer func() { recover() }()
			t.Setenv(EnvRenderDir, ".")
			renderDir(inside)
		}()
		<-done
	}()
	if !inside.Failed() {
		t.Error("the working directory is inside a work tree and was accepted")
	}

	// Outside one: a plain directory under the system's temporary area.
	outside := filepath.Join(os.TempDir(), "xrkit-render-check")
	t.Setenv(EnvRenderDir, outside)
	if got := renderDir(t); got == "" {
		t.Error("a directory outside every work tree was refused")
	}
	os.RemoveAll(outside)
}

// TestRenderTheSettingsWindow draws it and writes it out.
func TestRenderTheSettingsWindow(t *testing.T) {
	const w, h = settingsW, settingsH
	buf := make([]byte, w*h*4)
	p := painter.NewPixelPainter(buf, w, h)
	theme := toolkit.DefaultDark()
	root, _ := settingsRoot(&Config{}, []glasses.USB{oneS, luma}, nil, func() {})
	root.SetBounds(toolkit.Rect{X: 0, Y: 0, W: w, H: h})
	p.FillRect(painter.Rect{X: 0, Y: 0, W: w, H: h}, theme.Background)
	root.Draw(p, theme)

	// Something was drawn, and it is not one flat colour. A window that renders
	// as its own background is a window whose widgets were never given bounds.
	first := [4]byte{buf[0], buf[1], buf[2], buf[3]}
	different := 0
	for i := 0; i+3 < len(buf); i += 4 {
		if [4]byte{buf[i], buf[i+1], buf[i+2], buf[i+3]} != first {
			different++
		}
	}
	if different < w*h/100 {
		t.Errorf("only %d of %d pixels differ from the corner; nothing was drawn",
			different, w*h)
	}

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	copy(img.Pix, buf)
	out := filepath.Join(renderDir(t), "settings.png")
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	t.Logf("the settings window is at %s", out)
}

// TestNoShortcutLineRunsOffTheWindow.
//
// The line saying which combination the gallery got ran past the right edge at
// "it was taken)", so the reader was told everything except the part that
// mattered. It breaks at the parenthesis, which is where the sentence divides —
// anywhere else would split a combination in half, and half a combination is
// worse than none.
func TestNoShortcutLineRunsOffTheWindow(t *testing.T) {
	report := "previous: Option-Command-Left\n" +
		"gallery: Control-Option-Command-Space (asked for Option-Command-Space, it was taken)\n"
	got := wrapShortcuts(report)
	if len(got) != 3 {
		t.Fatalf("wrapShortcuts gave %d lines: %q", len(got), got)
	}
	for i, line := range got {
		if len(line) > shortcutCols {
			t.Errorf("line %d is %d characters, past the window's %d: %q",
				i, len(line), shortcutCols, line)
		}
	}
	// The break keeps the granted combination whole on its own line.
	if got[1] != "gallery: Control-Option-Command-Space" {
		t.Errorf("the granted combination was broken up: %q", got[1])
	}
	// A line with nothing to break at is left alone rather than cut.
	long := "no global shortcut for " + strings.Repeat("x", shortcutCols)
	if out := wrapShortcuts(long); len(out) != 1 || out[0] != long {
		t.Errorf("a line with no parenthesis was cut: %q", out)
	}
	// A blank line between two reports is dropped rather than drawn as an empty
	// row, which would open a gap nobody put there.
	if out := wrapShortcuts("next: Option-Command-Right\n\nquit: Escape\n"); len(out) != 2 {
		t.Errorf("a blank line survived: %q", out)
	}
}
