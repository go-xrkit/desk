// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoPictureIsInThisRepository.
//
// This package RENDERS things and writes them out to be looked at, which is how
// several defects in the gallery and the settings window were found. Every one
// of those pictures must land outside every work tree, and renderDir enforces
// that by walking up for a .git and refusing.
//
// It was not enough. On 2026-08-27 three renders — band.png, gallery.png,
// gallery-plus.go — were committed to main, from a throwaway test that wrote to
// filepath.Join(os.Getenv("XRDESK_RENDER_DIR"), name) with the variable unset:
// the join then yields a bare filename, which is the repository root. The
// pictures were synthetic, of fake feeds with flat grey tiles, so nothing of
// anybody's screen was published. That was luck, not design.
//
// So the rule gets a barrier instead of a habit. An image ANYWHERE in this tree
// fails here, tracked or not — an untracked one is one `git add -A` from being
// committed, which is exactly how those three got in.
func TestNoPictureIsInThisRepository(t *testing.T) {
	suffixes := []string{".png", ".jpg", ".jpeg", ".gif", ".bmp", ".tiff", ".webp"}
	var found []string
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		}
		lower := strings.ToLower(d.Name())
		for _, s := range suffixes {
			if strings.HasSuffix(lower, s) {
				found = append(found, path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the tree: %v", err)
	}
	if len(found) > 0 {
		t.Errorf("there are pictures in this work tree, which is one `git add -A` "+
			"from publishing them: %v\n\nA render belongs outside every repository — "+
			"see renderDir, which will put it under %s.", found, renderDirName())
	}
}

// renderDirName is where a render should have gone, for the message above.
func renderDirName() string {
	if dir := os.Getenv(EnvRenderDir); dir != "" {
		return dir
	}
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "the platform's configuration directory"
	}
	return filepath.Join(cfg, "go-xrkit", "renders")
}
