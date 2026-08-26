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
// glasses are already presenting. It also costs a projection — an
// equirectangular panorama and a per-pixel warp, 2.8 ms of a 16.6 ms frame — to
// produce a picture whose whole purpose is to look like a flat screen.
//
// So the screens are laid side by side on a flat band and the band slides.
//
// The scale is the only thing that has to be decided, and the plan decides it:
// ONE VIEW IS ONE FIELD OF VIEW. Everything else follows from the ribbon's own
// placement — where each screen sits, how wide it is, and therefore how much
// space is left between two of them. There is no separate gap to set and get
// wrong, because the gap is not this type's to invent.
//
// The band closes on itself: walking right past the last screen arrives at the
// first, and a screen straddling that join is drawn as two pieces, one against
// each edge.
type Strip struct {
	n int
	// centre and width place each screen along the band, in pixels, from the
	// ribbon's own arrangement rather than from an assumption that the screens
	// are evenly spread and all one size. Guessing put the band half a screen
	// out of step with the navigator, which is invisible until you wear it.
	centre []int
	width  []int

	viewW, viewH int
	total        int
	srcW, srcH   int
	srcY         []int32
}

// NewStrip lays the ribbon's screens out flat for a view of viewW x viewH.
//
// totalPx is how long the whole band is, in pixels. That single number sets the
// scale: a screen's arc becomes its width, and the arc between two of them
// becomes the space between them. Nothing here needs a field of view — the band
// is flat, so how large a screen LOOKS is the optics' business and not this
// package's.
func NewStrip(placed []ribbon.Placed, totalPx, srcW, srcH, viewW, viewH int) (*Strip, error) {
	n := len(placed)
	switch {
	case n <= 0:
		return nil, fmt.Errorf("%w: %d screens", ErrNoScreens, n)
	case srcW <= 0 || srcH <= 0:
		return nil, fmt.Errorf("%w: screens of %dx%d", ErrScreens, srcW, srcH)
	case viewW <= 0 || viewH <= 0:
		return nil, fmt.Errorf("%w: a view of %dx%d", ErrScreens, viewW, viewH)
	case totalPx <= 0:
		return nil, fmt.Errorf("%w: a band of %d pixels", ErrScreens, totalPx)
	}
	s := &Strip{n: n, viewW: viewW, viewH: viewH, srcW: srcW, srcH: srcH}

	s.total = totalPx
	pxPerRad := float64(totalPx) / (2 * math.Pi)
	s.centre = make([]int, n)
	s.width = make([]int, n)
	for i, p := range placed {
		frac := p.Centre / (2 * math.Pi)
		frac -= math.Floor(frac)
		s.centre[i] = int(math.Round(frac * float64(s.total)))
		s.width[i] = int(math.Round(p.Span() * pxPerRad))
		if s.width[i] <= 0 {
			return nil, fmt.Errorf("%w: screen %d spans %g radians, which is no pixels at all",
				ErrScreens, i, p.Span())
		}
	}
	// The vertical mapping never changes: the band only ever slides sideways.
	// So it is built once, and a frame does horizontal work only.
	s.srcY = make([]int32, viewH)
	for y := range s.srcY {
		s.srcY[y] = int32(int64(y) * int64(srcH) / int64(viewH))
	}
	return s, nil
}

// Offset turns a yaw in radians into the point on the band the viewer faces.
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
// a panorama it would then have to be a projection of.
func (s *Strip) Frame(dst []ribbon.Blit, offset int) []ribbon.Blit {
	offset = ((offset % s.total) + s.total) % s.total
	for i := 0; i < s.n; i++ {
		// Each screen is considered at its own place and one band-width either
		// side of it. That is the whole of the join: a screen near the end of
		// the band is also just before the beginning, and writing it this way
		// needs no case analysis about which side of it we are on.
		base := s.centre[i] - s.width[i]/2 - offset + s.viewW/2
		for _, left := range [3]int{base, base + s.total, base - s.total} {
			if left < s.viewW && left+s.width[i] > 0 {
				dst = s.append(dst, i, left, s.width[i])
			}
		}
	}
	return dst
}

// Fullscreen appends the blit for one screen filling the whole view.
//
// It is a real promotion only when the screen is narrower than the view. At one
// screen per view it changes nothing, which is the right answer rather than a
// missing feature: there is nowhere for a screen already filling the glasses to
// grow to.
func (s *Strip) Fullscreen(dst []ribbon.Blit, i int) ([]ribbon.Blit, error) {
	if i < 0 || i >= s.n {
		return dst, fmt.Errorf("%w: screen %d of %d", ErrScreens, i, s.n)
	}
	return s.append(dst, i, 0, s.viewW), nil
}

// append emits the clipped blit for screen i, w pixels wide with its left edge
// at left.
//
// It is only ever called with a left edge that leaves something to draw —
// [Strip.Frame] has already established that, and [Strip.Fullscreen] draws the
// whole view — so there is no empty case here to guard, and none to leave
// untested.
func (s *Strip) append(dst []ribbon.Blit, i, left, w int) []ribbon.Blit {
	x0, x1 := left, left+w
	skip := 0
	if x0 < 0 {
		skip, x0 = -x0, 0
	}
	if x1 > s.viewW {
		x1 = s.viewW
	}
	return append(dst, ribbon.Blit{
		Screen:   i,
		Dst:      stereo.Rect{X: x0, Y: 0, W: x1 - x0, H: s.viewH},
		SrcX:     int64(skip) * int64(s.srcW) << fracBits / int64(w),
		SrcXStep: int64(s.srcW) << fracBits / int64(w),
		SrcY:     s.srcY,
	})
}

// Screens is how many screens are on the band.
func (s *Strip) Screens() int { return s.n }

// Width is the whole band, in pixels.
func (s *Strip) Width() int { return s.total }
