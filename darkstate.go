// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// A NOTE WRITTEN BEFORE THE LIGHTS GO OUT.
//
// The desk turns a panel off while the band is showing a copy of it, and puts
// it back from a deferred call. A process that is KILLED runs no deferred call:
// SIGKILL, a force quit, a panic in another goroutine before that was handled,
// a power cut. Then somebody is left looking at a black screen, with the menu
// bar item that would have quit the program drawn on it.
//
// Measured by the bench, on its first afternoon:
//
//	── killed outright, the way a crash does
//	   ✗ display 1 was left at 0.00, from 0.79
//
// Nothing this program can do runs after SIGKILL. So it writes down what it is
// about to darken BEFORE darkening it, and the next start reads that note and
// puts the panel back. The screen is dark until then -- there is no way around
// that -- but it is one start away from being right rather than a setting a
// person has to find in the dark.

// darkNote is one panel and what it was lit at.
type darkNote struct {
	Display uint64  `json:"display"`
	Was     float64 `json:"was"`
}

// DarkNotePath reports the file the notes are kept in.
func DarkNotePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("%w: cannot locate the settings: %w", ErrConfig, err)
	}
	return filepath.Join(dir, "go-xrkit", "darkened.json"), nil
}

// noteDark writes down that these panels are about to be darkened.
//
// The seams are here rather than in the caller because this is written on a
// path a test must be able to take without a machine.
var (
	writeNote = writeDarkFile
	readNote  = readDarkFile
	dropNote  = removeDarkFile
)

func writeDarkFile(notes []darkNote) error {
	path, err := DarkNotePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// The error is dropped because there is none to have: a slice of two
	// exported scalars is always marshalable, and the alternative is a branch
	// nothing can take and therefore nothing can test.
	// TestEveryNoteCanBeWrittenDown pins that rather than assuming it.
	b, _ := json.Marshal(notes)
	return os.WriteFile(path, b, 0o644)
}

func readDarkFile() ([]darkNote, error) {
	path, err := DarkNotePath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var notes []darkNote
	if err := json.Unmarshal(b, &notes); err != nil {
		// A note nobody can read is worse than none: it would stop this trying
		// again for ever. Say so and start clean.
		return nil, fmt.Errorf("desk: the note of what was darkened is unreadable: %w", err)
	}
	return notes, nil
}

func removeDarkFile() error {
	path, err := DarkNotePath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// note records what is dark, or removes the note when nothing is.
// Note writes down which panels are dark and what they were lit at, or removes
// the note when none are.
//
// Called after every change, because the point of it is to be there when this
// process is not.
func (m *Dimmer) Note() error {
	m.mu.Lock()
	notes := make([]darkNote, 0, len(m.on))
	for id := range m.on {
		notes = append(notes, darkNote{Display: id, Was: m.was[id]})
	}
	m.mu.Unlock()
	if len(notes) == 0 {
		return dropNote()
	}
	// Sorted, so the file does not churn between runs that darkened the same
	// screens: a file that changes for no reason is a file nobody trusts.
	sort.Slice(notes, func(i, j int) bool { return notes[i].Display < notes[j].Display })
	return writeNote(notes)
}

// PutBackWhatWasLeftDark lights any panel a previous run darkened and never
// restored, and reports what it did.
//
// It is called at start-up, before anything else touches a backlight. A panel
// that is ALREADY brighter than the note says is left alone: somebody has
// turned it up since, and taking that away would be this program overruling a
// person to tidy up after itself.
func PutBackWhatWasLeftDark() ([]string, error) {
	notes, err := readNote()
	if err != nil {
		// Unreadable: drop it rather than carry it for ever.
		_ = dropNote()
		return nil, err
	}
	if len(notes) == 0 {
		return nil, nil
	}
	var said []string
	var errs []error
	// A NOTE THAT COULD NOT BE HONOURED IS KEPT.
	//
	// The note used to be dropped whatever happened, and that loses the only
	// record there is. Measured today: a run darkened this Mac's own panel,
	// the lid was shut, and the next start could not read that display's
	// brightness at all -- so it failed to put it back AND forgot that it
	// owed it. Open the lid tomorrow and the screen is black with nothing
	// left to say why.
	//
	// A display that is merely absent comes back, and the note is honoured
	// then. Carrying it costs a few bytes; dropping it costs somebody their
	// screen.
	var left []darkNote
	for _, n := range notes {
		now, err := dimRead(n.Display)
		if err != nil {
			errs = append(errs, err)
			left = append(left, n)
			continue
		}
		if now >= n.Was {
			continue // somebody, or something, already put it back
		}
		if err := dimSet(n.Display, n.Was); err != nil {
			errs = append(errs, err)
			left = append(left, n)
			continue
		}
		said = append(said, fmt.Sprintf(
			"display %d was left dark by a run that did not finish; back to %.2f",
			n.Display, n.Was))
	}
	if len(left) == 0 {
		if err := dropNote(); err != nil {
			errs = append(errs, err)
		}
	} else if err := writeNote(left); err != nil {
		errs = append(errs, err)
	}
	return said, errors.Join(errs...)
}
