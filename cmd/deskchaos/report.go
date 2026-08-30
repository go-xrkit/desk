// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// report is what a night of this comes to.
//
// Written down rather than only printed, because the point of a nightly run is
// that nobody watches it. What somebody reads in the morning is either "nothing
// left behind" or a list, and a list is only useful if it says which round,
// which fault and which arrangement -- a defect that needs a seed and a
// headset and four screens to appear is a defect nobody reproduces from
// "something went wrong".
type report struct {
	Seed    int64       `json:"seed"`
	Started time.Time   `json:"started"`
	Took    string      `json:"took"`
	Rounds  []roundSaid `json:"rounds"`
	Found   int         `json:"found"`
	Skipped int         `json:"skipped"`
	Machine machineSaid `json:"machine"`
}

// roundSaid is one round, and what it came to.
type roundSaid struct {
	N        int      `json:"n"`
	Fault    string   `json:"fault"`
	Headset  string   `json:"headset"`
	Settings string   `json:"settings"`
	Found    []string `json:"found,omitempty"`
	Skipped  string   `json:"skipped,omitempty"`
}

// machineSaid is enough about the machine to tell one night from another.
type machineSaid struct {
	Displays int `json:"displays"`
}

// write puts the report where a job can pick it up.
func (r *report) write(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// summary is the one line a person reads first.
func (r *report) summary() string {
	switch {
	case r.Found == 0 && r.Skipped == 0:
		return fmt.Sprintf("nothing left behind over %d rounds, in %s", len(r.Rounds), r.Took)
	case r.Found == 0:
		return fmt.Sprintf("nothing left behind over %d rounds, %d of which the machine was too tired to run, in %s",
			len(r.Rounds), r.Skipped, r.Took)
	default:
		return fmt.Sprintf("%d thing(s) left behind over %d rounds, in %s — seed %d",
			r.Found, len(r.Rounds), r.Took, r.Seed)
	}
}
