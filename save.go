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
	if c.Ribbon != nil && (c.Ribbon.Screens != nil || c.Ribbon.Immersive != nil ||
		c.Ribbon.Distance != nil) {
		b := body.AppendNewBlock("ribbon", nil).Body()
		if c.Ribbon.Screens != nil {
			b.SetAttributeValue("screens", cty.NumberIntVal(int64(*c.Ribbon.Screens)))
		}
		if c.Ribbon.Distance != nil {
			b.SetAttributeValue("distance", cty.NumberFloatVal(*c.Ribbon.Distance))
		}
		if c.Ribbon.Immersive != nil {
			b.SetAttributeValue("immersive", cty.BoolVal(*c.Ribbon.Immersive))
		}
		body.AppendNewline()
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
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("%w: %w", ErrConfig, err)
	}
	// One named temporary file rather than CreateTemp + Write + Close: three
	// error branches nothing portable can reach are three branches nothing
	// portable can test, and a settings file is not worth pretending about.
	tmp := filepath.Join(dir, ".desk.hcl.new")
	if err := os.WriteFile(tmp, c.Bytes(), 0o644); err != nil {
		return fmt.Errorf("%w: %w", ErrConfig, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("%w: %w", ErrConfig, err)
	}
	return nil
}
