// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

// Command macapp assembles xrdesk into a macOS application bundle.
//
// ⛔ THE BUNDLE IS NOT PACKAGING, IT IS THE ONLY WAY TO REACH A CAMERA. macOS
// does not DENY a program with no NSCameraUsageDescription, it ENDS it: TCC
// terminates the process before anything can be caught. A bare binary run from
// a terminal has no Info.plist at all, so xrdesk built with `go build` can list
// cameras and can never open one.
//
//	go run ./internal/macapp            # writes ./dist/XR desk.app
//	go run ./internal/macapp -dir /tmp
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/go-macos/appbundle"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	dir := flag.String("dir", "dist", "where to write the bundle")
	version := flag.String("version", "0.1.0", "what the application reports as its version")
	flag.Parse()

	exe := filepath.Join(os.TempDir(), "xrdesk-bundle-exe")
	build := exec.Command("go", "build", "-o", exe, "./cmd/xrdesk")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	build.Stdout, build.Stderr = os.Stdout, os.Stderr
	if err := build.Run(); err != nil {
		return fmt.Errorf("build the executable: %w", err)
	}
	defer os.Remove(exe)

	b, err := appbundle.Build(appbundle.Spec{
		Dir: *dir, Name: "XR desk", Identifier: "io.github.go-xrkit.xrdesk",
		Version: *version, Executable: exe,
		// A desk lives in the menu bar: no dock tile, nothing to switch to.
		Accessory:     true,
		MinimumSystem: "13.0",
		UsageDescriptions: map[string]string{
			// A SENTENCE ABOUT WHAT IT IS FOR, because this is what the person
			// reads in the prompt. "Access the camera" is what the API is
			// called, not a reason to say yes.
			"NSCameraUsageDescription": "XR desk shows what the glasses' cameras see, " +
				"and takes a photograph when you ask for one.",
		},
	})
	if err != nil {
		return err
	}
	fmt.Println(b.Path)
	return nil
}
