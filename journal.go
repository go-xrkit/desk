// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/go-fsctl/outdir"
)

// JournalDirEnv points the journal somewhere else, like [PhotoDirEnv] does for
// photographs.
const JournalDirEnv = "XRDESK_LOG_DIR"

// JournalPath is where this run writes what it says.
//
// ⛔⛔ AN APPLICATION LAUNCHED FROM THE FINDER HAS NO STANDARD OUTPUT. macOS
// connects it to nothing, so every line this program prints about what it is
// doing is DISCARDED the moment it is used the ordinary way -- double-clicked,
// from the Dock, from Login Items. Only a run started in a terminal leaves a
// trace, and a fault that appears in ordinary use is exactly the one nobody
// starts a terminal for.
//
// That is not a comfort feature. The witness strings exist to say how many are
// damaged at once, once, in a line; the one desk they most need to speak on is
// the one whose journal goes to /dev/null.
//
// The same barrier as the photographs, and for a weaker but real version of the
// same reason: a journal names screens, applications and window titles, so it
// is a description of somebody's desk and must not land where a `git add` can
// reach it.
func JournalPath(at time.Time) (string, error) {
	dir, err := outdir.Ensure(outdir.Spec{
		App: "xrdesk",
		Env: JournalDirEnv,
		Sub: "logs",
	})
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, at.Format("2006-01-02-150405")+".log"), nil
}

// Journal is where a run writes what it says. The zero value writes nowhere and
// is what a run gets when the file could not be opened.
type Journal struct {
	// Path is the file being written, empty when there is none. Reported so a
	// person can be told where to look -- a file a program wrote and did not
	// name is a file nobody finds.
	Path string

	w io.WriteCloser
}

// OpenJournal starts a journal for a run.
//
// ⛔ A JOURNAL THAT COULD NOT BE OPENED IS NOT A REASON TO REFUSE TO START. The
// desk's job is to put screens in front of somebody; losing the record of what
// it said is worth a line on standard error, not a run that never happens. So
// this returns a working Journal and the trouble, and the caller may use both.
func OpenJournal(at time.Time) (*Journal, error) {
	path, err := JournalPath(at)
	if err != nil {
		return &Journal{}, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return &Journal{}, err
	}
	return &Journal{Path: path, w: f}, nil
}

// Logf writes one line to the journal and to out, so a run started from a
// terminal still says everything on the screen in front of the person who
// started it.
//
// out may be nil, which is what -quiet wants: the file still gets the line,
// because quiet is about not filling somebody's terminal and never about
// throwing the evidence away.
func (j *Journal) Logf(out io.Writer, format string, a ...any) {
	line := fmt.Sprintf(format, a...)
	if out != nil {
		fmt.Fprintf(out, "  %s\n", line)
	}
	if j == nil || j.w == nil {
		return
	}
	// The time, in the file only: a terminal has the person watching it, a file
	// read days later has nothing else to say when a line happened. And the
	// witnesses' whole question is WHEN -- both known victims were damaged
	// within the first second.
	fmt.Fprintf(j.w, "%s  %s\n", time.Now().Format("15:04:05.000"), line)
}

// Close finishes the file. Writing continues to work afterwards, to nowhere,
// so a line logged during shutdown cannot panic.
func (j *Journal) Close() error {
	if j == nil || j.w == nil {
		return nil
	}
	w := j.w
	j.w = nil
	return w.Close()
}
