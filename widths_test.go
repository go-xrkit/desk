// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import "testing"

// ⛔ THE SENTENCE HAS TO SURVIVE SCREENS OF DIFFERENT WIDTHS, because that is
// now the point: a desk can be one wide screen for a spreadsheet. The old line
// named ONE width and was true only while every screen shared it.
func TestDescribeWidthsSaysWhatWasActuallyMade(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name   string
		widths []int
		want   string
	}{
		{"nothing", nil, "no virtual displays"},
		// The common case still reads as one phrase rather than five.
		{"a ribbon of equals", []int{1920, 1920, 1920, 1920, 1920},
			"5 virtual displays of 1920x1080"},
		{"one wide screen", []int{6400}, "1 virtual display of 6400x1080"},
		// ⭐ The case the change exists for, and the one a person needs to read
		// at a glance: which screen is the wide one.
		{"a wide one and a ribbon", []int{6400, 1920, 1920, 1920},
			"4 virtual displays of 6400x1080 and 3 of 1920x1080"},
		// A width nudged by a pixel is exactly what this must not hide.
		{"one nudged by a pixel", []int{1920, 1921, 1920},
			"3 virtual displays of 1920x1080, 1921x1080 and 1920x1080"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			if got := DescribeWidths(c.widths, 1080); got != c.want {
				t.Errorf("DescribeWidths(%v) =\n  %q\nwant\n  %q", c.widths, got, c.want)
			}
		})
	}
}
