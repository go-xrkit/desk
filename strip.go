// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"fmt"
	"math"

	"github.com/go-xrkit/xrkit/ribbon"
	"github.com/go-xrkit/xrkit/stereo"
)

// A Strip is the ribbon with its screens left FLAT.
//
// The curved ribbon puts each screen on the surface of a cylinder, which is
// geometrically honest and, worn, buys nothing: a screen the viewer is looking
// straight at is drawn with a bow in it, and the bow argues with the depth the
// glasses are already presenting. Worse, the curve costs a projection — an
// equirectangular panorama and a per-pixel warp, 2.8 ms of a 16.6 ms frame —
// to produce a picture whose whole purpose is to look like a flat screen.
//
// So the screens are laid side by side on a flat band and the band slides. One
// screen is one full view, so a screen at rest fills the viewport exactly, at
// one source pixel per output pixel, with no resampling in either axis and no
// distortion anywhere. Turning the ribbon moves the window along the band.
//
// The band still closes on itself: walking right past the last screen arrives
// at the first, and a screen straddling that seam is drawn as two pieces, one
// against each edge — which is the same thing the panorama did at its own seam.
type Strip struct {
	n          int
	viewW      int
	viewH      int
	gap        int
	pitch      int
	total      int
	srcW, srcH int
	srcY       []int32
	fullscreen bool
}

// DefaultGapPx is the band between two screens, in pixels of the view.
//
// Wide enough that two screens read as two screens rather than as one very
// wide one, and narrow enough that turning the ribbon never shows a gap and
// nothing else.
const DefaultGapPx = 48

// NewStrip lays out n screens of srcW x srcH for a view of viewW x viewH.
func NewStrip(n, srcW, srcH, viewW, viewH, gap int) (*Strip, error) {
	switch {
	case n <= 0:
		return nil, fmt.Errorf("%w: %d screens", ErrNoScreens, n)
	case srcW <= 0 || srcH <= 0:
		return nil, fmt.Errorf("%w: screens of %dx%d", ErrScreens, srcW, srcH)
	case viewW <= 0 || viewH <= 0:
		return nil, fmt.Errorf("%w: a view of %dx%d", ErrScreens, viewW, viewH)
	case gap < 0:
		return nil, fmt.Errorf("%w: a gap of %d", ErrScreens, gap)
	}
	s := &Strip{
		n: n, viewW: viewW, viewH: viewH, gap: gap,
		srcW: srcW, srcH: srcH,
		pitch: viewW + gap,
	}
	s.total = s.pitch * n
	// The vertical mapping never changes: the band only ever slides sideways.
	// So it is built once, and a frame does horizontal work only.
	s.srcY = make([]int32, viewH)
	for y := range s.srcY {
		s.srcY[y] = int32(int64(y) * int64(srcH) / int64(viewH))
	}
	return s, nil
}

// Offset turns a yaw in radians into a position along the band.
//
// The navigator thinks in angles because a ribbon is a circle, and it keeps
// working — the gallery, the focus, the shortest way round — without knowing
// that the picture is flat. A full turn is the whole band.
func (s *Strip) Offset(yaw float64) int {
	frac := yaw / (2 * math.Pi)
	frac -= math.Floor(frac)
	return int(math.Round(frac * float64(s.total)))
}

// Frame appends the blits for one view of the band and returns the extended
// slice. Passing dst[:0] of the previous frame's slice reuses the storage, and
// the frame then allocates nothing at all.
//
// The destination rectangles are in VIEW coordinates and already clipped, so
// the canvas a caller composes into is the size of the picture rather than of
// a panorama.
func (s *Strip) Frame(dst []ribbon.Blit, offset int) []ribbon.Blit {
	offset = ((offset % s.total) + s.total) % s.total
	for i := 0; i < s.n; i++ {
		// Each screen is considered at its own place and one band-width either
		// side of it. That is the whole of the seam: a screen near the end of
		// the band is also just before the beginning, and writing it out this
		// way needs no case analysis about which side of the join we are on.
		base := i*s.pitch - offset
		for _, left := range [3]int{base, base + s.total, base - s.total} {
			if left < s.viewW && left+s.viewW > 0 {
				dst = s.append(dst, i, left)
			}
		}
	}
	return dst
}

// Fullscreen appends the blit for one screen filling the whole view.
func (s *Strip) Fullscreen(dst []ribbon.Blit, i int) ([]ribbon.Blit, error) {
	if i < 0 || i >= s.n {
		return dst, fmt.Errorf("%w: screen %d of %d", ErrScreens, i, s.n)
	}
	return s.append(dst, i, 0), nil
}

// append emits the clipped blit for screen i whose left edge is at left.
//
// It is only ever called with a left edge that leaves something to draw —
// [Strip.Frame] has already established that, and [Strip.Fullscreen] draws at
// zero — so there is no empty case here to guard, and none to leave untested.
func (s *Strip) append(dst []ribbon.Blit, i, left int) []ribbon.Blit {
	x0, x1 := left, left+s.viewW
	skip := 0
	if x0 < 0 {
		skip, x0 = -x0, 0
	}
	if x1 > s.viewW {
		x1 = s.viewW
	}
	return append(dst, ribbon.Blit{
		Screen: i,
		Dst:    stereo.Rect{X: x0, Y: 0, W: x1 - x0, H: s.viewH},
		// One source column per destination column: a screen is exactly as wide
		// as the view, so there is no resampling to do and no error to
		// accumulate across two thousand columns.
		SrcX:     int64(int64(skip)*int64(s.srcW)/int64(s.viewW)) << fracBits,
		SrcXStep: int64(s.srcW) << fracBits / int64(s.viewW),
		SrcY:     s.srcY,
	})
}

// Screens is how many screens are on the band.
func (s *Strip) Screens() int { return s.n }

// Width is the whole band, in pixels.
func (s *Strip) Width() int { return s.total }
