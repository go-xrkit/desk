// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"

	"github.com/go-xrkit/desk"
)

// settingsFor writes a settings file nobody chose, and reports where.
//
// A bench that always runs the same desk finds the defects of that desk. Six
// screens with the mirror on is one arrangement out of dozens, and the ones
// that broke this week were all edges: one screen, nine screens, the mirror
// off so position 1 is not what everything else assumes.
//
// It goes to the bench's own directory through XRDESK_CONFIG, so a person's
// own settings are never read and never written.
func settingsFor(dir string, rng *rand.Rand) (string, string, error) {
	// Up to six: a round of seven or eight leaves the window server refusing
	// the next round's displays for a while, so a bench that asks for them
	// spends its time reporting the machine it exhausted.
	screens := 1 + rng.Intn(6)
	mirror := rng.Intn(4) > 0 // mostly on, which is the default
	splay := 0.0
	if rng.Intn(3) == 0 {
		splay = float64(4 + rng.Intn(8))
	}
	line := ""
	if splay > 0 {
		line = fmt.Sprintf("\n  splay     = %.0f", splay)
	}
	body := fmt.Sprintf(`ribbon {
  screens   = %d
  immersive = true
  mirror    = %v%s
}
`, screens, mirror, line)
	path := filepath.Join(dir, "desk.hcl")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return "", "", err
	}
	how := fmt.Sprintf("%d screens, mirror %v", screens, mirror)
	if splay > 0 {
		how += fmt.Sprintf(", splay %.0f°", splay)
	}
	return path, how, nil
}

// configEnv is the environment a session runs with, pointing the desk at the
// settings this bench wrote rather than at a person's own.
func configEnv(path string) []string {
	return append(os.Environ(), desk.EnvConfig+"="+path)
}
