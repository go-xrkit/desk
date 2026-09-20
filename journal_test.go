// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ⛔⛔ THE LINE HAS TO SURVIVE A RUN WITH NO TERMINAL, which is every run
// started from the Finder. What is checked is the FILE, because that is the
// half that did not exist.
func TestAJournalKeepsWhatATerminalWouldHaveLost(t *testing.T) {
	t.Setenv(JournalDirEnv, t.TempDir())

	j, err := OpenJournal(time.Date(2026, 9, 20, 19, 7, 45, 0, time.UTC))
	if err != nil {
		t.Fatalf("opening the journal: %v", err)
	}
	// No writer at all: the Finder's idea of standard output.
	j.Logf(nil, "%d witness strings are watching", 64)
	if err := j.Close(); err != nil {
		t.Fatalf("closing the journal: %v", err)
	}

	if want := "2026-09-20-190745.log"; filepath.Base(j.Path) != want {
		t.Errorf("the file is named %q, want %q", filepath.Base(j.Path), want)
	}
	b, err := os.ReadFile(j.Path)
	if err != nil {
		t.Fatalf("reading it back: %v", err)
	}
	if !strings.Contains(string(b), "64 witness strings are watching") {
		t.Errorf("the journal does not hold the line: %q", b)
	}
	// AND IT SAYS WHEN. A file read days later has nothing else to say at what
	// moment a line happened, and "within the first second" is the whole
	// question the witnesses were built to answer.
	if !strings.Contains(string(b), ":") || len(b) < len("00:00:00.000") {
		t.Errorf("the journal line carries no time: %q", b)
	}
}

// AND A TERMINAL STILL SEES IT, so a person who started the run in a shell is
// not asked to go and read a file.
func TestAJournalStillSpeaksToTheTerminal(t *testing.T) {
	t.Setenv(JournalDirEnv, t.TempDir())

	j, err := OpenJournal(time.Now())
	if err != nil {
		t.Fatalf("opening the journal: %v", err)
	}
	t.Cleanup(func() { _ = j.Close() })

	var out bytes.Buffer
	j.Logf(&out, "a screen is %d wide", 1920)
	if got, want := out.String(), "  a screen is 1920 wide\n"; got != want {
		t.Errorf("the terminal got %q, want %q", got, want)
	}
}

// ⛔ AND A RUN WITH NO JOURNAL STILL RUNS. Losing the record of what the desk
// said is worth a line on standard error, never a desk that never appears: the
// zero value writes nowhere and takes lines without complaint.
func TestARunWithoutAJournalStillSpeaks(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	var none *Journal
	none.Logf(&out, "putting back a screen left dark")
	(&Journal{}).Logf(&out, "and again")

	if got := strings.Count(out.String(), "\n"); got != 2 {
		t.Errorf("%d lines reached the terminal, want 2: %q", got, out.String())
	}
	if err := none.Close(); err != nil {
		t.Errorf("closing nothing: %v", err)
	}
}

// ⛔ AND CLOSING TWICE IS NOT A CRASH. The desk closes its journal on the way
// out and lines are still logged during shutdown -- a screen being put back, a
// pointer being brought home.
func TestAClosedJournalTakesMoreLines(t *testing.T) {
	t.Setenv(JournalDirEnv, t.TempDir())

	j, err := OpenJournal(time.Now())
	if err != nil {
		t.Fatalf("opening the journal: %v", err)
	}
	if err := j.Close(); err != nil {
		t.Fatalf("closing it: %v", err)
	}
	if err := j.Close(); err != nil {
		t.Errorf("closing it again: %v", err)
	}
	var out bytes.Buffer
	j.Logf(&out, "display 1 was put back")
	if !strings.Contains(out.String(), "put back") {
		t.Error("a line logged after the close was lost to the terminal too")
	}
}

// ⛔⛔ AND IT REFUSES A WORK TREE, the way the photographs do. A journal names
// screens, applications and window titles: it describes somebody's desk, and a
// .gitignore is a safety net rather than a barrier.
func TestAJournalRefusesAGitWorkTree(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o700); err != nil {
		t.Fatalf("making a work tree: %v", err)
	}
	inside := filepath.Join(dir, "logs")
	t.Setenv(JournalDirEnv, inside)

	if _, err := JournalPath(time.Now()); err == nil {
		t.Errorf("a journal was accepted inside a work tree at %s", inside)
	}
}

// ⛔ AND WHEN IT CANNOT BE OPENED, WHAT COMES BACK IS STILL USABLE. The caller
// keeps both the trouble and a journal it can log to: a desk that refused to
// appear because a log file could not be created would be a worse fault than
// the one being hunted.
func TestAJournalThatCannotBeOpenedIsStillSafeToUse(t *testing.T) {
	at := time.Date(2026, 9, 20, 19, 8, 40, 0, time.UTC)

	t.Run("nowhere to put it", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o700); err != nil {
			t.Fatalf("making a work tree: %v", err)
		}
		t.Setenv(JournalDirEnv, filepath.Join(dir, "logs"))

		j, err := OpenJournal(at)
		if err == nil {
			t.Error("a work tree was accepted")
		}
		mustStillLog(t, j)
	})

	t.Run("the name is taken", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv(JournalDirEnv, dir)
		// A directory where the file should go: os.OpenFile refuses it, which
		// is the ordinary shape of a disk that will not take the file.
		if err := os.MkdirAll(filepath.Join(dir, "2026-09-20-190840.log"), 0o700); err != nil {
			t.Fatalf("taking the name: %v", err)
		}

		j, err := OpenJournal(at)
		if err == nil {
			t.Error("a directory was opened as a journal")
		}
		mustStillLog(t, j)
	})
}

// mustStillLog is what every failed open has to leave behind.
func mustStillLog(t *testing.T, j *Journal) {
	t.Helper()

	if j == nil {
		t.Fatal("nothing came back; the caller would crash on its first line")
	}
	if j.Path != "" {
		t.Errorf("a journal that was not opened names a file: %q", j.Path)
	}
	var out bytes.Buffer
	j.Logf(&out, "the desk starts anyway")
	if !strings.Contains(out.String(), "starts anyway") {
		t.Error("the terminal lost the line as well")
	}
	if err := j.Close(); err != nil {
		t.Errorf("closing it: %v", err)
	}
}
