// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// paper is a note kept in memory, so these tests touch nobody's settings.
type paper struct {
	notes    []darkNote
	there    bool
	writes   int
	drops    int
	badRead  error
	badDrop  error
	badWrite error
}

func (p *paper) install(t *testing.T) {
	t.Helper()
	w, r, d := writeNote, readNote, dropNote
	writeNote = func(n []darkNote) error {
		p.notes, p.there, p.writes = n, true, p.writes+1
		return p.badWrite
	}
	readNote = func() ([]darkNote, error) {
		if p.badRead != nil {
			return nil, p.badRead
		}
		if !p.there {
			return nil, nil
		}
		return p.notes, nil
	}
	dropNote = func() error {
		p.drops++
		p.there, p.notes = false, nil
		return p.badDrop
	}
	t.Cleanup(func() { writeNote, readNote, dropNote = w, r, d })
}

// panelsAt installs a machine whose backlights can be read and set.
func panelsAt(t *testing.T, at map[uint64]float64) map[uint64]float64 {
	t.Helper()
	r, s, d := dimRead, dimSet, dimDisplay
	dimRead = func(id uint64) (float64, error) {
		v, ok := at[id]
		if !ok {
			return 0, errors.New("no such display")
		}
		return v, nil
	}
	dimSet = func(id uint64, level float64) error { at[id] = level; return nil }
	dimDisplay = func(id uint64) (func() error, error) {
		was := at[id]
		at[id] = 0
		return func() error { at[id] = was; return nil }, nil
	}
	t.Cleanup(func() { dimRead, dimSet, dimDisplay = r, s, d })
	return at
}

func TestTheNoteSaysWhatIsDarkAndGoesWhenNothingIs(t *testing.T) {
	var p paper
	p.install(t)
	panels := panelsAt(t, map[uint64]float64{1: 0.79, 2: 0.50})

	var m Dimmer
	if err := m.Showing([]uint64{1}); err != nil {
		t.Fatalf("Showing: %v", err)
	}
	if err := m.Note(); err != nil {
		t.Fatalf("Note: %v", err)
	}
	if len(p.notes) != 1 || p.notes[0].Display != 1 || p.notes[0].Was != 0.79 {
		t.Errorf("the note says %v, want display 1 at the 0.79 it was lit at", p.notes)
	}
	if panels[1] != 0 {
		t.Errorf("display 1 is at %v, want it dark", panels[1])
	}

	// Put back, and the note goes with it: a note left behind would light a
	// panel nobody darkened, at the next start.
	if err := m.Restore(); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if err := m.Note(); err != nil {
		t.Fatalf("Note: %v", err)
	}
	if p.there {
		t.Error("the note is still there after everything was put back")
	}
}

func TestARunThatWasKilledIsPutBackByTheNext(t *testing.T) {
	var p paper
	p.install(t)
	panels := panelsAt(t, map[uint64]float64{1: 0.79})

	// A run that darkened a panel and never got to restore it.
	var m Dimmer
	if err := m.Showing([]uint64{1}); err != nil {
		t.Fatalf("Showing: %v", err)
	}
	if err := m.Note(); err != nil {
		t.Fatalf("Note: %v", err)
	}
	// ... and is gone. The next one reads the note.
	said, err := PutBackWhatWasLeftDark()
	if err != nil {
		t.Fatalf("PutBackWhatWasLeftDark: %v", err)
	}
	if panels[1] != 0.79 {
		t.Errorf("display 1 is at %v, want the 0.79 it was lit at", panels[1])
	}
	if len(said) != 1 || !strings.Contains(said[0], "display 1") {
		t.Errorf("it said %v, want a line naming the display it lit", said)
	}
	if p.there {
		t.Error("the note survived being acted on, so the next start would do it again")
	}
}

func TestAPanelSomebodyHasTurnedUpIsLeftAlone(t *testing.T) {
	var p paper
	p.install(t)
	// The note says 0.79, and the panel is at 0.90: somebody turned it up after
	// the run that died. Taking that away would be this program overruling a
	// person to tidy up after itself.
	p.there = true
	p.notes = []darkNote{{Display: 1, Was: 0.79}}
	panels := panelsAt(t, map[uint64]float64{1: 0.90})

	said, err := PutBackWhatWasLeftDark()
	if err != nil {
		t.Fatalf("PutBackWhatWasLeftDark: %v", err)
	}
	if panels[1] != 0.90 {
		t.Errorf("display 1 was set to %v, want the 0.90 a person chose", panels[1])
	}
	if len(said) != 0 {
		t.Errorf("it said %v, want nothing: it did nothing", said)
	}
	if p.there {
		t.Error("the note is still there")
	}
}

func TestNothingToPutBackSaysNothing(t *testing.T) {
	var p paper
	p.install(t)
	panelsAt(t, map[uint64]float64{1: 0.79})

	said, err := PutBackWhatWasLeftDark()
	if err != nil || said != nil {
		t.Errorf("PutBackWhatWasLeftDark = %v,%v, want nothing at all", said, err)
	}
	if p.drops != 0 {
		t.Error("it removed a note that was not there")
	}
}

func TestANoteNobodyCanReadIsDroppedRatherThanCarriedForEver(t *testing.T) {
	var p paper
	p.badRead = errors.New("the file is nonsense")
	p.install(t)
	p.there = true

	if _, err := PutBackWhatWasLeftDark(); !errors.Is(err, p.badRead) {
		t.Errorf("PutBackWhatWasLeftDark = %v, want the read's own error", err)
	}
	if p.drops != 1 {
		t.Errorf("the unreadable note was dropped %d times, want once", p.drops)
	}
}

func TestWhatCannotBePutBackIsReported(t *testing.T) {
	var p paper
	p.install(t)
	p.there = true
	p.notes = []darkNote{{Display: 404, Was: 0.5}, {Display: 1, Was: 0.79}}
	panels := panelsAt(t, map[uint64]float64{1: 0})

	said, err := PutBackWhatWasLeftDark()
	if err == nil {
		t.Error("a display that cannot be read said nothing")
	}
	// And the one that COULD be put back was, which is the negative control:
	// one unreachable display must not cost the others their light.
	if panels[1] != 0.79 {
		t.Errorf("display 1 is at %v, want 0.79", panels[1])
	}
	if len(said) != 1 {
		t.Errorf("it said %v, want the one it lit", said)
	}
}

func TestTheNotePathIsBesideTheSettings(t *testing.T) {
	path, err := DarkNotePath()
	if err != nil {
		t.Fatalf("DarkNotePath: %v", err)
	}
	if !strings.HasSuffix(path, "go-xrkit/darkened.json") {
		t.Errorf("the note goes to %q, want it beside the settings", path)
	}
}

// TestTheNoteReallyGoesToAFileAndComesBack, with HOME sent somewhere else so
// this never touches anybody's settings.
func TestTheNoteReallyGoesToAFileAndComesBack(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if notes, err := readDarkFile(); err != nil || notes != nil {
		t.Fatalf("a note that was never written = %v,%v, want nothing and no error", notes, err)
	}
	want := []darkNote{{Display: 1, Was: 0.79}, {Display: 2, Was: 0.5}}
	if err := writeDarkFile(want); err != nil {
		t.Fatalf("writeDarkFile: %v", err)
	}
	got, err := readDarkFile()
	if err != nil {
		t.Fatalf("readDarkFile: %v", err)
	}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("read back %v, want %v", got, want)
	}
	if err := removeDarkFile(); err != nil {
		t.Fatalf("removeDarkFile: %v", err)
	}
	if notes, _ := readDarkFile(); notes != nil {
		t.Errorf("the note survived being removed: %v", notes)
	}
	// Removing one that is not there is not an error: a run that darkened
	// nothing must not report a failure on the way out.
	if err := removeDarkFile(); err != nil {
		t.Errorf("removing a note that is not there = %v, want nil", err)
	}

	// And nonsense in the file is reported rather than believed.
	path, err := DarkNotePath()
	if err != nil {
		t.Fatalf("DarkNotePath: %v", err)
	}
	if err := os.WriteFile(path, []byte("this is not json"), 0o644); err != nil {
		t.Fatalf("writing nonsense: %v", err)
	}
	if _, err := readDarkFile(); err == nil {
		t.Error("nonsense in the note was read as a note")
	}
}

func TestWithNoHomeThereIsNowhereToKeepTheNote(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	if _, err := DarkNotePath(); !errors.Is(err, ErrConfig) {
		t.Errorf("DarkNotePath with no home = %v, want an ErrConfig", err)
	}
	if err := writeDarkFile(nil); err == nil {
		t.Error("a note was written with nowhere to put it")
	}
	if _, err := readDarkFile(); err == nil {
		t.Error("a note was read from nowhere")
	}
	if err := removeDarkFile(); err == nil {
		t.Error("a note was removed from nowhere")
	}
}

func TestWhatCannotBeWrittenOrDroppedIsReported(t *testing.T) {
	var p paper
	p.install(t)
	panelsAt(t, map[uint64]float64{1: 0.79})

	// The note cannot be written: the run goes on -- a screen shown is worth
	// more than a note about it -- but it says so.
	stuck := errors.New("the disk is full")
	was := writeNote
	writeNote = func([]darkNote) error { return stuck }
	t.Cleanup(func() { writeNote = was })

	var m Dimmer
	if err := m.Showing([]uint64{1}); err != nil {
		t.Fatalf("Showing: %v", err)
	}
	if err := m.Note(); !errors.Is(err, stuck) {
		t.Errorf("Note = %v, want the write's own error", err)
	}

	// And a note that cannot be dropped after it was acted on.
	p.there = true
	p.notes = []darkNote{{Display: 1, Was: 0.79}}
	p.badDrop = errors.New("the file will not go")
	if _, err := PutBackWhatWasLeftDark(); !errors.Is(err, p.badDrop) {
		t.Errorf("PutBackWhatWasLeftDark = %v, want the drop's own error", err)
	}
}

func TestEveryNoteCanBeWrittenDown(t *testing.T) {
	// writeDarkFile drops the marshalling error because a slice of two exported
	// scalars cannot produce one. This is that assumption, pinned: the shapes
	// the desk really writes, including the ones a float can take.
	for _, notes := range [][]darkNote{
		nil,
		{},
		{{Display: 1, Was: 0}},
		{{Display: ^uint64(0), Was: 1}},
		{{Display: 1, Was: 0.7999999999}, {Display: 2, Was: 0.5}},
	} {
		if _, err := json.Marshal(notes); err != nil {
			t.Errorf("json.Marshal(%v) = %v, want no error", notes, err)
		}
	}
}

func TestAPlaceThatWillNotHoldANoteIsReported(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "conf"))

	path, err := DarkNotePath()
	if err != nil {
		t.Fatalf("DarkNotePath: %v", err)
	}
	// A DIRECTORY where the note goes: writing, reading and removing all fail,
	// and each has to say so rather than pretend there is no note.
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("making a directory in the note's place: %v", err)
	}
	// With something in it, or removing it would simply succeed: an empty
	// directory comes away as easily as a file.
	if err := os.WriteFile(filepath.Join(path, "in-the-way"), []byte("x"), 0o644); err != nil {
		t.Fatalf("filling the directory: %v", err)
	}
	if err := writeDarkFile([]darkNote{{Display: 1, Was: 0.5}}); err == nil {
		t.Error("a note was written over a directory")
	}
	if _, err := readDarkFile(); err == nil {
		t.Error("a directory was read as a note")
	}
	if err := removeDarkFile(); err == nil {
		t.Error("a directory was removed as if it were a note")
	}

	// And a place that cannot be made at all: the parent is a FILE.
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocked, []byte("x"), 0o644); err != nil {
		t.Fatalf("writing the blocker: %v", err)
	}
	t.Setenv("XDG_CONFIG_HOME", blocked)
	t.Setenv("HOME", blocked)
	if err := writeDarkFile([]darkNote{{Display: 1, Was: 0.5}}); err == nil {
		t.Error("a note was written under a file")
	}
}

func TestTheNoteIsSortedAndAPanelThatRefusesTheLightIsReported(t *testing.T) {
	var p paper
	p.install(t)
	panelsAt(t, map[uint64]float64{2: 0.5, 1: 0.79})

	// Two panels, darkened in whatever order a map hands them over: the note
	// comes out in display order, so a file does not churn between runs that
	// darkened the same screens.
	var m Dimmer
	if err := m.Showing([]uint64{2, 1}); err != nil {
		t.Fatalf("Showing: %v", err)
	}
	if err := m.Note(); err != nil {
		t.Fatalf("Note: %v", err)
	}
	if len(p.notes) != 2 || p.notes[0].Display != 1 || p.notes[1].Display != 2 {
		t.Errorf("the note says %v, want it in display order", p.notes)
	}

	// And a panel that will not take the light back says so.
	stuck := errors.New("the panel refused")
	was := dimSet
	dimSet = func(uint64, float64) error { return stuck }
	t.Cleanup(func() { dimSet = was })

	if _, err := PutBackWhatWasLeftDark(); !errors.Is(err, stuck) {
		t.Errorf("PutBackWhatWasLeftDark = %v, want the panel's own refusal", err)
	}
}

// TestANoteThatCouldNotBeHonouredIsKept is a defect a real machine showed.
//
// A run darkened this Mac's own panel, the lid was shut, and the next start
// could not read that display's brightness at all -- so it failed to put it
// back AND dropped the note saying it owed it. Open the lid tomorrow and the
// screen is black with nothing left to say why.
func TestANoteThatCouldNotBeHonouredIsKept(t *testing.T) {
	var p paper
	p.install(t)
	p.there = true
	p.notes = []darkNote{{Display: 404, Was: 0.5}, {Display: 1, Was: 0.79}}
	panels := panelsAt(t, map[uint64]float64{1: 0})

	if _, err := PutBackWhatWasLeftDark(); err == nil {
		t.Error("a display that cannot be read said nothing")
	}
	// The one that could not be reached is still owed, and the one that was
	// put back is not: a note that stayed whole would ask again for a display
	// nobody darkened any more.
	if len(p.notes) != 1 || p.notes[0].Display != 404 {
		t.Errorf("the note now holds %v, want only the display that could not be read", p.notes)
	}
	if p.drops != 0 {
		t.Error("the note was dropped although one display is still dark")
	}
	if panels[1] != 0.79 {
		t.Errorf("display 1 is at %v, want it put back", panels[1])
	}

	// And when everything is honoured, the note goes: a file that outlived its
	// reason would make every start claim it repaired something.
	var q paper
	q.install(t)
	q.there = true
	q.notes = []darkNote{{Display: 1, Was: 0.79}}
	panelsAt(t, map[uint64]float64{1: 0})
	if _, err := PutBackWhatWasLeftDark(); err != nil {
		t.Fatalf("PutBackWhatWasLeftDark: %v", err)
	}
	if q.drops != 1 {
		t.Errorf("the note was dropped %d time(s) after everything was put back, want once", q.drops)
	}
}

// TestANoteThatCannotBeKeptIsSaidSo: keeping what is still owed is the point,
// so failing to keep it has to be reported rather than swallowed. It is the
// one path where the caller learns that a dark screen is now nobody's.
func TestANoteThatCannotBeKeptIsSaidSo(t *testing.T) {
	var p paper
	p.install(t)
	p.there = true
	p.badWrite = errors.New("the disk is full")
	p.notes = []darkNote{{Display: 404, Was: 0.5}}
	panelsAt(t, map[uint64]float64{})

	_, err := PutBackWhatWasLeftDark()
	if err == nil || !strings.Contains(err.Error(), "the disk is full") {
		t.Errorf("err = %v, want it to carry the failure to keep the note", err)
	}
}
