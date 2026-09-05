// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

// Bytes renders the settings as the file they came from.
//
// Written with hclwrite rather than by printing strings, so that what comes out
// is HCL by construction: a quotation mark in an application's name, or a name
// with a newline in it, cannot produce a file that then fails to parse. The
// round trip — write, read back, compare — is the test.
//
// What is NOT set is left out entirely rather than written as a zero. A file
// with no `screens` line means "as many as the default says", and a file with
// `screens = 0` means the same thing while looking like a decision; only one of
// those is honest about what the person chose.
func (c Config) Bytes() []byte {
	f := hclwrite.NewEmptyFile()
	body := f.Body()

	for _, s := range c.Shortcuts {
		b := body.AppendNewBlock("shortcut", []string{s.Action})
		b.Body().SetAttributeValue("keys", cty.StringVal(s.Keys))
	}
	if len(c.Shortcuts) > 0 {
		body.AppendNewline()
	}
	if c.Fallback != nil {
		vals := make([]cty.Value, len(*c.Fallback))
		for i, m := range *c.Fallback {
			vals[i] = cty.StringVal(m)
		}
		// An EMPTY list is a decision — "give me what I asked for or tell me you
		// could not" — and cty has no empty list of a known type to say it with,
		// so it is written as an empty tuple, which parses back the same.
		if len(vals) == 0 {
			body.SetAttributeValue("fallback", cty.EmptyTupleVal)
		} else {
			body.SetAttributeValue("fallback", cty.TupleVal(vals))
		}
		body.AppendNewline()
	}
	// ⛔ EVERY FIELD, AND THE GUARD DERIVED FROM THEM. This was a hand-kept
	// list -- "screens or immersive or distance or splay" -- and it had already
	// fallen behind the struct twice: `mirror` and `badge_seconds` were not in
	// it, so a person who set either by hand LOST IT the first time they
	// pressed Save. Nothing said so; the file simply came back without the
	// line. Building the block first and asking whether anything went into it
	// is the same question with no list to forget.
	if c.Ribbon != nil {
		block := hclwrite.NewBlock("ribbon", nil)
		b := block.Body()
		wrote := false
		set := func(name string, v cty.Value, ok bool) {
			if ok {
				b.SetAttributeValue(name, v)
				wrote = true
			}
		}
		if c.Ribbon.Screens != nil {
			set("screens", cty.NumberIntVal(int64(*c.Ribbon.Screens)), true)
		}
		if c.Ribbon.Distance != nil {
			set("distance", cty.NumberFloatVal(*c.Ribbon.Distance), true)
		}
		if c.Ribbon.Splay != nil {
			set("splay", cty.NumberFloatVal(*c.Ribbon.Splay), true)
		}
		if c.Ribbon.BadgeSeconds != nil {
			set("badge_seconds", cty.NumberFloatVal(*c.Ribbon.BadgeSeconds), true)
		}
		if c.Ribbon.Mirror != nil {
			set("mirror", cty.BoolVal(*c.Ribbon.Mirror), true)
		}
		if c.Ribbon.Immersive != nil {
			set("immersive", cty.BoolVal(*c.Ribbon.Immersive), true)
		}
		if c.Ribbon.Dim != nil {
			set("dim", cty.BoolVal(*c.Ribbon.Dim), true)
		}
		if wrote {
			body.AppendBlock(block)
			body.AppendNewline()
		}
	}
	if c.Glasses != nil && c.Glasses.Model != nil {
		b := body.AppendNewBlock("glasses", nil).Body()
		b.SetAttributeValue("model", cty.StringVal(*c.Glasses.Model))
		body.AppendNewline()
	}
	for _, p := range c.Places {
		b := body.AppendNewBlock("place", []string{p.App})
		b.Body().SetAttributeValue("screen", cty.NumberIntVal(int64(p.Screen)))
	}
	return hclwrite.Format(f.Bytes())
}

// Save writes the settings to [ConfigPath], creating the directory if it is not
// there.
//
// It writes a new file and renames it over the old one. A settings file half
// written is a settings file that will not parse, and the next start-up would
// then refuse to run over a power cut in the middle of a save.
func (c Config) Save() (string, error) {
	path, err := ConfigPath()
	if err != nil {
		return "", err
	}
	return path, c.SaveTo(path)
}

// SaveTo writes the settings to one named file.
func (c Config) SaveTo(path string) error {
	if err := c.check(); err != nil {
		return err
	}
	return writeAtomically(path, c.Bytes())
}

// writeAtomically writes bytes where a settings file goes: into a named
// temporary beside it, then renamed over it.
//
// One named temporary file rather than CreateTemp + Write + Close: three error
// branches nothing portable can reach are three branches nothing portable can
// test, and a settings file is not worth pretending about.
//
// It is shared with [RememberScreens], which produces its bytes by EDITING the
// file rather than rendering a whole Config. Two copies of a rename dance is
// one copy too many: the one that gets fixed and the one that does not.
func writeAtomically(path string, b []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("%w: %w", ErrConfig, err)
	}
	tmp := filepath.Join(dir, ".desk.hcl.new")
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return fmt.Errorf("%w: %w", ErrConfig, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("%w: %w", ErrConfig, err)
	}
	return nil
}
