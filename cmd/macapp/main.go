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
// ⛔ AND IT IS SIGNED, OR EVERY PERMISSION IS LOST ON THE NEXT BUILD. macOS
// grants screen recording, the camera and the microphone to a CODE IDENTITY,
// and an ad-hoc signature -- what the Go linker leaves behind -- IS the hash of
// the binary. Change one line and it is a different application that has been
// granted nothing. That is not theoretical: the desk recorded the screen in 72
// consecutive runs, took a one-line dependency bump, and could not record it in
// the 73rd. So this signs by default with whatever certificate is on the
// machine, and says so.
//
//	go run ./cmd/macapp            # writes ./dist/XR desk.app
//	go run ./cmd/macapp -dir /tmp
//	go run ./cmd/macapp -sign -    # ad hoc, and the grants will not survive
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
	sign := flag.String("sign", "", "signing certificate; empty takes the machine's only one")
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
	if err := signIt(b, *sign); err != nil {
		return err
	}
	fmt.Println(b.Path)
	return nil
}

// signIt gives the bundle a code identity that outlives it being rebuilt.
//
// An empty identity means "the one on this machine" rather than "none",
// because none is the answer that quietly costs the person their permissions,
// and because the certificate here is a local, per-machine thing that no
// build script can sensibly name. If there is exactly one it is used and said
// aloud; if there are several this refuses rather than picking; if there are
// none it says what to do and carries on with what the linker left, since a
// bundle that cannot be signed is still a bundle that can be run.
func signIt(b appbundle.Bundle, identity string) error {
	if identity == "" {
		found, err := signingIdentities()
		if err != nil {
			return err
		}
		switch len(found) {
		case 0:
			fmt.Fprintln(os.Stderr, "not signed: there is no signing certificate on this machine, so this\n"+
				"build is a new application that has been granted nothing. Make a self-signed\n"+
				"code-signing certificate in Keychain Access, or pass -sign - to say ad hoc is meant.")
			return nil
		case 1:
			identity = found[0]
		default:
			return fmt.Errorf("there are %d signing certificates here (%s): say which with -sign",
				len(found), strings.Join(found, ", "))
		}
	}
	if err := b.Sign(appbundle.Signer{Identity: identity}); err != nil {
		return err
	}
	req, err := b.DesignatedRequirement()
	if err != nil {
		return err
	}
	// Print the requirement rather than the certificate's name, because the
	// requirement is the thing that either survives the next build or does not,
	// and it is legible: one shape names this program, the other names bytes.
	fmt.Fprintf(os.Stderr, "signed by %q\n  %s\n", identity, req)
	if strings.HasPrefix(req, "cdhash") {
		fmt.Fprintln(os.Stderr, "  ⛔ that is the hash of the binary: the next build will be granted nothing")
	}
	return nil
}

// signingIdentities is every code-signing certificate on this machine.
//
// A self-signed certificate is reported untrusted -- it has no chain to a root
// anyone vouches for -- and signs perfectly well, which is why this asks
// without -v and keeps what that would have thrown away.
func signingIdentities() ([]string, error) {
	out, err := exec.Command("security", "find-identity", "-p", "codesigning").Output()
	if err != nil {
		return nil, fmt.Errorf("ask for the signing certificates: %w", err)
	}
	var found []string
	for line := range strings.Lines(string(out)) {
		_, after, ok := strings.Cut(line, `"`)
		if !ok {
			continue
		}
		if name, _, ok := strings.Cut(after, `"`); ok && name != "" {
			found = append(found, name)
		}
	}
	return found, nil
}
