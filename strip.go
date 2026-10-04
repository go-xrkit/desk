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
	// drawH is how tall the band is DRAWN, and topY where it starts. They are
	// the view's own height and zero until somebody pushes the band back. See
	// [Strip.SetDrawnHeight].
	drawH, topY int
	// srcWidths is the source width of each screen that is not the shape of
	// the band. See [Strip.SetSourceWidths].
	srcWidths []int
	srcY      []int32
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
	s := &Strip{n: n, viewW: viewW, viewH: viewH, srcW: srcW, srcH: srcH,
		drawH: viewH}

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
		Dst:      stereo.Rect{X: x0, Y: s.topY, W: x1 - x0, H: s.drawH},
		SrcX:     int64(skip) * int64(s.sourceWidth(i)) << fracBits / int64(w),
		SrcXStep: int64(s.sourceWidth(i)) << fracBits / int64(w),
		SrcY:     s.srcY,
	})
}

// Screens is how many screens are on the band.
func (s *Strip) Screens() int { return s.n }

// Width is the whole band, in pixels.
func (s *Strip) Width() int { return s.total }

// Toward is how far the band has moved from the focused screen towards its
// neighbour, in screens: 0 is the focused screen centred, 0.5 half way to the
// next one, -0.25 a quarter of the way back to the last.
//
// It exists so that the turned band ([Fan]) and the flat one agree about where
// the band IS, and it is expressed RELATIVE TO THE FOCUS on purpose. An absolute
// position along the band would need the screens to be in band order, and they
// are not: a ribbon may put screen zero anywhere on the circle, and this desk's
// starts at 210 degrees. Asking "how far past the screen the navigator says we
// are on" needs no such assumption -- and taking each screen's place from the
// ribbon rather than assuming an even spread is what keeps this in step with the
// navigator, which it once was not.
func (s *Strip) Toward(yaw float64, focus int) float64 {
	if focus < 0 || focus >= s.n || s.n < 1 || s.total <= 0 {
		return 0
	}
	// ⛔⛔ ONE SCREEN STILL HAS SOMEWHERE TO GO, and this used to answer zero for
	// every yaw it was ever given. The guard said `s.n < 2` -- "how far towards
	// the neighbour" needs a neighbour -- and on a ring the neighbour is the
	// screen itself, one lap along. Which is exactly the shape wide mode makes:
	// ONE screen, wider than the view, where turning the head is the only way to
	// the rest of it.
	//
	// ⭐ SO HEAD TRACKING DID NOTHING IN WIDE MODE, flat or curved, and said
	// nothing either. Reported: "j'ai activé le suivi de tete mais bien que la
	// camera soit active cela ne faisait rien". The log of that session shows
	// the tracker working -- "following your head, with the camera light on
	// while it does", no darkness, nothing lost -- and the band standing still.
	// Measured: a band of 6448 pixels for a screen of 6400, and Toward
	// answering 0.000 at every yaw from a tenth of a turn to three quarters.
	//
	// ⚠ AND THE WALK BELOW WOULD ANSWER ZERO TOO, so this is not a shortcut past
	// it. With one screen `next` is the focus, the span between them is nothing,
	// and the loop takes its `span <= 0` exit on the first pass. The early
	// return is the only place a one-screen band can be measured at all.
	//
	// One lap IS the step, because the next screen is this one again.
	if s.n == 1 {
		return float64(s.short(s.Offset(yaw)-s.centre[focus])) / float64(s.total)
	}
	// How far past the focused screen's centre the band is, in pixels, the
	// SHORTEST way round -- which is what "past" has to mean on a band that
	// closes. Without it, a desk sitting on the screen either side of the join
	// reports most of a band rather than a little of one.
	d := s.short(s.Offset(yaw) - s.centre[focus])
	step := 1
	if d < 0 {
		step = -1
	}
	// ⛔⛔ AND EACH STEP IS ITS OWN LENGTH. This divided by one average slot,
	// total/n, which is the distance between two neighbours only when every
	// screen is the same width. With an Odyssey G95NC on the band -- 3840 among
	// 1920s -- the step to it is half as long again as the step away from it, so
	// "half a screen along" named a place neither renderer agreed on: [Fan]
	// interpolates between two panels' own centres, and this said something else.
	//
	// Walked screen by screen rather than solved, because the answer may be
	// several screens out while a turn settles, and the screens it crosses are
	// not all the same size.
	rem, at, screens := float64(step*d), focus, 0.0
	for range s.n {
		next := ((at+step)%s.n + s.n) % s.n
		span := float64(step * s.short(s.centre[next]-s.centre[at]))
		if span <= 0 || rem < span {
			if span > 0 {
				screens += rem / span
			}
			break
		}
		rem -= span
		screens++
		at = next
	}
	return float64(step) * screens
}

// short is the shortest signed way round the band, in pixels: positive to the
// right, and never more than half a band either way.
func (s *Strip) short(px int) int {
	px = ((px % s.total) + s.total) % s.total
	if 2*px > s.total {
		px -= s.total
	}
	return px
}

// SetSourceWidths gives screens their own source widths.
//
// A screen mirroring a display this program did not make is not the shape of
// the glasses -- this Mac's panel is 1.547 against the band's 1.778 -- so its
// capture is that shape too, and the arc it takes on the ribbon was worked out
// from the same number. Mapping it through the band's width instead would
// stretch it across a quad that is not its shape, which is the very thing
// giving that screen its own arc was for.
//
// A nil or short slice, or an entry of zero, leaves that screen on the width
// every other screen has.
func (s *Strip) SetSourceWidths(w []int) {
	s.srcWidths = append(s.srcWidths[:0], w...)
}

// sourceWidth is how wide screen i's pixels are.
func (s *Strip) sourceWidth(i int) int {
	if i < 0 || i >= len(s.srcWidths) || s.srcWidths[i] <= 0 {
		return s.srcW
	}
	return s.srcWidths[i]
}

// SetDrawnHeight is how tall the band is drawn, centred in the view. Zero or
// more than the view puts it back to filling the view.
//
// ⛔⛔ PUSHING THE BAND BACK USED TO CHANGE ITS WIDTH AND NOT ITS HEIGHT, which
// is not a distance, it is a squash. Reported from inside the glasses: "les
// touches d'eloigement modifie la largeur de l'ecran mais pas sa hauteur, un
// eloigment doit modifier les deux".
//
// ⭐ MEASURED, three wide screens, the widest panel drawn:
//
//	distance   flat band        curved band
//	1.0x       1920x1080        1600x1080
//	1.5x       1920x1080        1066x720
//	2.0x       1920x1080         800x540
//	3.0x       1920x1080         534x360
//
// The curved renderer divides both axes exactly, because it projects a panel in
// space and a distance is a distance there. The flat one got its horizontal
// scale from the band's own length -- Desk.build divides BandPx by the distance
// -- and its vertical from srcH mapped onto viewH, which nothing divided. So
// three screens of 6400 at distance 2 were half as wide and full height.
//
// ⚠ AND THE STRIP STILL KNOWS NOTHING ABOUT DISTANCE, which is its own doc's
// claim: "how large a screen LOOKS is the optics' business and not this
// package's". It is told a height in pixels, as it is told a band length in
// pixels, and the optics stay with the caller that has them.
func (s *Strip) SetDrawnHeight(h int) {
	if h <= 0 || h > s.viewH {
		h = s.viewH
	}
	s.drawH, s.topY = h, (s.viewH-h)/2
	// The vertical mapping is over the DRAWN rows now, so it is rebuilt with
	// them. It still never changes per frame: the band only slides sideways.
	s.srcY = make([]int32, h)
	for y := range s.srcY {
		s.srcY[y] = int32(int64(y) * int64(s.srcH) / int64(h))
	}
}
