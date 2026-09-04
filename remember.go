// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

// RememberScreens writes the screen count into a settings file, leaving
// everything else in it exactly as it was.
//
// ⛔ It does NOT go through [Config.Save]. That renders the whole file from the
// struct with an empty hclwrite document, so everything the struct does not
// carry — every comment, every blank line somebody put between two blocks, the
// order they chose — is gone. That is defensible behind a Save button, where a
// person asked for their settings to be written down. It is not defensible on
// the side of an action that was about adding a screen: nobody clicking a tile
// in the gallery has agreed to have their file rewritten.
//
// So the file is parsed and EDITED. hclwrite exists for exactly this: it keeps
// the tokens it did not touch.
//
// A file that is not there yet is created holding only this, which is honest —
// it says the one thing that was decided and claims nothing else.
func RememberScreens(path string, n int) error {
	if path == "" {
		return fmt.Errorf("%w: no settings file to remember %d screens in", ErrConfig, n)
	}
	// Checked here rather than at the end, because the point of writing this
	// down is that the next start-up will read it: a number that start-up would
	// REFUSE must not reach the file. The refusal names the ceiling, for the
	// same reason [Config.check] does.
	if n < 1 {
		return fmt.Errorf("%w: %d screens is not a desk", ErrConfig, n)
	}
	if n > MaxScreens {
		return fmt.Errorf("%w: ribbon screens = %d, and %d is the most a desk carries",
			ErrConfig, n, MaxScreens)
	}

	src, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("%w: %w", ErrConfig, err)
	}

	f, diags := hclwrite.ParseConfig(src, filepath.Base(path), hcl.InitialPos)
	if diags.HasErrors() {
		// A file that does not parse is somebody's file with a mistake in it.
		// Rewriting it from scratch would replace their mistake with our
		// opinion and lose the rest, so this refuses and says where to look.
		return fmt.Errorf("%w: %s: %s", ErrConfig, path, diags.Error())
	}

	body := f.Body()
	ribbon := body.FirstMatchingBlock("ribbon", nil)
	if ribbon == nil {
		// A blank line first, so an appended block does not grow out of
		// whatever the last line happened to be.
		if len(src) > 0 {
			body.AppendNewline()
		}
		ribbon = body.AppendNewBlock("ribbon", nil)
	}
	ribbon.Body().SetAttributeValue("screens", cty.NumberIntVal(int64(n)))

	return writeAtomically(path, f.Bytes())
}
