// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-appdirs/outdir"
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

// Say writes one line that is never silenced -- what the desk is waiting for,
// why it stopped, what it could not put back.
//
// ⛔⛔ THE LINE THAT SAYS WHY WAS THE ONE MISSING FROM THE FILE. The first run
// written with a journal ended on "CGGetActiveDisplayList counted 0 displays"
// and the file did not hold it: the failure went straight to standard error
// while everything leading up to it was kept. A record of a run that keeps
// everything except the reason it stopped is worse than none, because it reads
// as a run that simply ended.
//
// out is where a person sees it -- standard output for news, standard error
// for trouble -- and is never nil: -quiet is about chatter.
func (j *Journal) Say(out io.Writer, format string, a ...any) {
	// A trailing newline is trimmed rather than doubled. These lines were
	// fmt.Printf calls carrying their own, in a file with fifty of them, and a
	// conversion that had to edit every format string by hand is a conversion
	// that silently mangles one. Both spellings say the same thing here.
	line := strings.TrimSuffix(fmt.Sprintf(format, a...), "\n")
	if out != nil {
		fmt.Fprintln(out, line)
	}
	j.keep(line)
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
	j.keep(line)
}

// keep puts one line in the file, with the time.
//
// The time is in the file ONLY: a terminal has the person watching it, and a
// file read days later has nothing else to say when a line happened -- which is
// the witnesses' whole question, both known victims having been damaged within
// the first second.
func (j *Journal) keep(line string) {
	if j == nil || j.w == nil {
		return
	}
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
