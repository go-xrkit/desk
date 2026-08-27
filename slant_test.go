// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package desk

import (
	"math"
	"testing"

	"github.com/go-xrkit/xrkit/stereo"
)

// TestASplayOfNothingIsTheFlatBand.
//
// The pin the whole feature hangs from, and the reason the arrangement is a
// CHAIN of panels hinged edge to edge rather than panels tangent to a circle: a
// chain has a zero. With no splay the chain is one flat plane at `distance`,
// every panel square on, and every panel exactly the rectangle this package drew
// before -- viewW/distance wide, viewH/distance tall, side by side.
//
// A circle has no zero, so there would have been no state in which the new
// geometry had to agree with the old one, and no way to tell a projection that
// is right from one that merely looks plausible.
func TestASplayOfNothingIsTheFlatBand(t *testing.T) {
	const viewW, viewH = 1920, 1200
	const fov = 45.0

	for _, d := range []float64{1, 2, 4} {
		hw, panelH, f := slantOptics(fov, viewW, viewW, viewH)
		for _, j := range []int{-1, 0, 1} {
			lx, lz, rx, rz := slantChain(j, 0, hw, d, 0)
			s, ok := slantOf(nil, j+1, lx, lz, rx, rz, panelH, f, viewW, viewH, viewW, viewH)
			if !ok {
				if j != 0 && d == 1 {
					continue // a neighbour at distance 1 is off the canvas, rightly
				}
				t.Fatalf("distance %g, panel %d: no projection", d, j)
			}
			// Width and height follow the distance, exactly as the flat band's
			// pixel scale does -- and a panel is one width along from the last,
			// which is what "side by side" means.
			panelW, wantH := int(viewW/d), int(viewH/d)
			x := (viewW-panelW)/2 + j*panelW
			wantX, wantW := x, panelW
			if wantX < 0 {
				wantW, wantX = wantW+wantX, 0
			}
			if wantX+wantW > viewW {
				wantW = viewW - wantX
			}
			if s.Dst.W != wantW || s.Dst.H != wantH {
				t.Errorf("distance %g, panel %d: %dx%d, want %dx%d",
					d, j, s.Dst.W, s.Dst.H, wantW, wantH)
			}
			if s.Dst.X != wantX {
				t.Errorf("distance %g, panel %d: at x=%d, want %d",
					d, j, s.Dst.X, wantX)
			}
			// Square on: every column the same height, source evenly spaced.
			h0 := s.Cols[0].Y1 - s.Cols[0].Y0
			for i, col := range s.Cols {
				if col.Y1-col.Y0 != h0 {
					t.Fatalf("distance %g, panel %d, column %d is %d tall and the "+
						"first %d: with no splay nothing is turned",
						d, j, i, col.Y1-col.Y0, h0)
				}
				// The source column, counted from the panel's own left edge --
				// which for a panel clipped on the left is not column zero.
				skipped := max(-x, 0)
				if want := int32((float64(i+skipped) + 0.5) * d); col.Src < want-1 || col.Src > want+1 {
					t.Fatalf("distance %g, panel %d, column %d reads source %d, "+
						"want about %d", d, j, i, col.Src, want)
				}
			}
		}
	}
}

// TestASplayedNeighbourIsStretchedTowardsTheEdge.
//
// What a turned panel looks like in a rectilinear projection, which is the one
// the glasses impose. Each of these is a separate way of getting a rotation
// wrong.
//
// The stretch is the counter-intuitive part and it is correct: off to the side of
// a FLAT projection a panel is wider and its outer edge taller, the way a monitor
// at the edge of a wide-angle photograph is. The intuition that says the far edge
// should be shorter belongs to human vision, which is not a flat panel. Writing
// that expectation down first, and being wrong, is how this got settled.
func TestASplayedNeighbourIsStretchedTowardsTheEdge(t *testing.T) {
	const viewW, viewH = 1920, 1200
	const fov, dist, splay = 45.0, 3.0, 20.0
	hw, panelH, f := slantOptics(fov, viewW, viewW, viewH)

	flat := project(t, 0, 0, hw, panelH, f, dist, viewW, viewH)
	right := project(t, 1, splay, hw, panelH, f, dist, viewW, viewH)
	left := project(t, -1, splay, hw, panelH, f, dist, viewW, viewH)

	// Turned away and set further along the chain, so it is off to the side.
	if right.Dst.X <= flat.Dst.X {
		t.Errorf("the right-hand neighbour starts at x=%d and the middle one at %d",
			right.Dst.X, flat.Dst.X)
	}
	// The outer edge is the taller one: its depth along the view axis is smaller,
	// and a flat projection divides by depth.
	inner := right.Cols[0].Y1 - right.Cols[0].Y0
	outer := right.Cols[len(right.Cols)-1].Y1 - right.Cols[len(right.Cols)-1].Y0
	if outer <= inner {
		t.Errorf("the outer edge is %d tall and the inner %d: nothing is turned",
			outer, inner)
	}
	// Perspective-correct: the inner columns are further away, so each covers
	// MORE of the source. A linear map would make the two gaps equal, and the
	// picture would slide along the surface as the angle grew.
	// Over ten columns at each end, because one column apart the difference is
	// smaller than a source pixel and the gradient is invisible to integers.
	near := right.Cols[10].Src - right.Cols[0].Src
	far := right.Cols[len(right.Cols)-1].Src - right.Cols[len(right.Cols)-11].Src
	if near <= far {
		t.Errorf("the source advances by %d over ten columns at the inner edge "+
			"and %d at the outer", near, far)
	}

	// And the chain is symmetric: the panel on the left is the mirror of the one
	// on the right, which is what says the arithmetic is not lopsided.
	lInner := left.Cols[len(left.Cols)-1].Y1 - left.Cols[len(left.Cols)-1].Y0
	lOuter := left.Cols[0].Y1 - left.Cols[0].Y0
	if lInner != inner || lOuter != outer {
		t.Errorf("the mirror is not a mirror: %d/%d against %d/%d",
			lOuter, lInner, outer, inner)
	}
	if left.Dst.W != right.Dst.W {
		t.Errorf("the left panel is %d wide and the right %d",
			left.Dst.W, right.Dst.W)
	}
}

// TestAWiderSplayTurnsThePanelFurther: the knob does something, monotonically,
// and the middle panel never moves.
//
// The last part is what a person cares about: whatever the angle, the screen they
// are reading is square on and the same size. A splay that crept into the middle
// panel would be a setting that changes the one thing it must not touch.
func TestAWiderSplayTurnsThePanelFurther(t *testing.T) {
	const viewW, viewH = 1920, 1200
	const fov, dist = 45.0, 3.0
	hw, panelH, f := slantOptics(fov, viewW, viewW, viewH)

	var lastRatio float64
	var middle Slant
	for i, splay := range []float64{0, 10, 20, 30} {
		mid := project(t, 0, splay, hw, panelH, f, dist, viewW, viewH)
		if i == 0 {
			middle = mid
		} else if mid.Dst != middle.Dst {
			t.Errorf("splay %g moved the middle panel to %+v from %+v",
				splay, mid.Dst, middle.Dst)
		}

		n := project(t, 1, splay, hw, panelH, f, dist, viewW, viewH)
		inner := float64(n.Cols[0].Y1 - n.Cols[0].Y0)
		outer := float64(n.Cols[len(n.Cols)-1].Y1 - n.Cols[len(n.Cols)-1].Y0)
		ratio := outer / inner
		if i == 0 {
			if ratio != 1 {
				t.Errorf("with no splay the neighbour is keystoned: %g", ratio)
			}
		} else if ratio <= lastRatio {
			t.Errorf("splay %g keystones by %g, no more than the %g before it",
				splay, ratio, lastRatio)
		}
		lastRatio = ratio
	}
}

// TestTheChainTurnsWithTheViewer: the scroll is the viewer's own rotation, so a
// turn of the angle to the next panel brings that panel to the middle.
//
// It is the whole of how the band scrolls in this arrangement: the chain is fixed
// and the head moves, which is also why the panel a person turns to face comes
// out square on when the splay matches the angle between them -- the same thing
// that happens at a desk of three angled monitors.
func TestTheChainTurnsWithTheViewer(t *testing.T) {
	const viewW, viewH = 1920, 1200
	const fov, dist, splay = 45.0, 3.0, 15.0
	hw, panelH, f := slantOptics(fov, viewW, viewW, viewH)

	// Where the next panel's centre is, as an angle.
	lx, lz, rx, rz := slantChain(1, splay, hw, dist, 0)
	turn := math.Atan2((lx+rx)/2, (lz+rz)/2)
	if turn <= 0 {
		t.Fatalf("the next panel is at %g radians", turn)
	}

	before := project(t, 1, splay, hw, panelH, f, dist, viewW, viewH)
	lx, lz, rx, rz = slantChain(1, splay, hw, dist, turn)
	after, ok := slantOf(nil, 1, lx, lz, rx, rz, panelH, f, viewW, viewH, viewW, viewH)
	if !ok {
		t.Fatal("the panel disappeared when the viewer turned to it")
	}
	// It was off to the side and now it is in the middle.
	beforeMid := before.Dst.X + before.Dst.W/2
	afterMid := after.Dst.X + after.Dst.W/2
	if abs(afterMid-viewW/2) >= abs(beforeMid-viewW/2) {
		t.Errorf("turning to the panel left it at %d, from %d, in a %d view",
			afterMid, beforeMid, viewW)
	}
	// And facing it, it is very nearly square on: the two edges within a few
	// pixels of each other rather than the keystone it had off to the side.
	in := after.Cols[0].Y1 - after.Cols[0].Y0
	out := after.Cols[len(after.Cols)-1].Y1 - after.Cols[len(after.Cols)-1].Y0
	if d := abs(int(in - out)); d > 4 {
		t.Errorf("facing it, the edges are %d and %d tall", in, out)
	}
}

// TestASlantRefusesWhatCannotBeDrawn: every way a projection has of being
// nothing, and none of them a panic.
func TestASlantRefusesWhatCannotBeDrawn(t *testing.T) {
	const viewW, viewH = 640, 480
	hw, panelH, f := slantOptics(45, viewW, 100, 100)
	for _, c := range []struct {
		what                     string
		lx, lz, rx, rz           float64
		panelH, f                float64
		viewW, viewH, srcW, srcH int
	}{
		{"no view", -1, 3, 1, 3, panelH, f, 0, viewH, 100, 100},
		{"no view height", -1, 3, 1, 3, panelH, f, viewW, 0, 100, 100},
		{"no source", -1, 3, 1, 3, panelH, f, viewW, viewH, 0, 100},
		{"no source height", -1, 3, 1, 3, panelH, f, viewW, viewH, 100, 0},
		{"no height", -1, 3, 1, 3, 0, f, viewW, viewH, 100, 100},
		{"no focal length", -1, 3, 1, 3, panelH, 0, viewW, viewH, 100, 100},
		{"behind the viewer", -1, -3, 1, -3, panelH, f, viewW, viewH, 100, 100},
		{"one edge behind", -1, 3, 1, -1, panelH, f, viewW, viewH, 100, 100},
		{"inside out", 1, 3, -1, 3, panelH, f, viewW, viewH, 100, 100},
		{"edge on", 0, 3, 0.01, 5, panelH, f, viewW, viewH, 100, 100},
		{"far off to the side", 40, 1, 42, 1, panelH, f, viewW, viewH, 100, 100},
	} {
		if _, ok := slantOf(nil, 0, c.lx, c.lz, c.rx, c.rz, c.panelH, c.f,
			c.viewW, c.viewH, c.srcW, c.srcH); ok {
			t.Errorf("%s: projected anyway", c.what)
		}
	}
	_ = hw
}

// TestASlantIsClippedToTheCanvas: nothing is ever written outside it, and a
// panel too tall for it is cut rather than squashed into it.
//
// The rows are per COLUMN and not per panel, which is what makes this
// expressible: a turned panel is taller on its near side, so the height that has
// to be cut is different in every column. A whole-rectangle clip would have to
// choose one.
func TestASlantIsClippedToTheCanvas(t *testing.T) {
	const viewW, viewH = 800, 400

	// A source twice as tall as the canvas's shape, so at distance one the panel
	// is twice the canvas height and every column has to be cut.
	hw, panelH, f := slantOptics(45, viewW, 800, 800)
	tall := project(t, 0, 20, hw, panelH, f, 1, viewW, viewH)
	if tall.Dst.X < 0 || tall.Dst.Y < 0 ||
		tall.Dst.X+tall.Dst.W > viewW || tall.Dst.Y+tall.Dst.H > viewH {
		t.Errorf("the bounding box %+v is outside the %dx%d canvas",
			tall.Dst, viewW, viewH)
	}
	cut := 0
	for i, col := range tall.Cols {
		if col.Y0 < 0 || col.Y1 > int32(viewH) {
			t.Errorf("column %d covers %d..%d, outside the canvas", i, col.Y0, col.Y1)
		}
		if col.Src >= 800 {
			t.Errorf("column %d reads source column %d of 800", i, col.Src)
		}
		if col.Y0 == 0 && col.Y1 == int32(viewH) {
			cut++
		}
	}
	if cut == 0 {
		t.Fatal("a panel twice the canvas height had no column cut, so this is " +
			"not looking at what it says it is")
	}

	// And the same panel further away fits, so nothing is cut: a clip that always
	// fires is indistinguishable from a bug that always fires.
	fits := project(t, 0, 20, hw, panelH, f, 3, viewW, viewH)
	for i, col := range fits.Cols {
		if col.Y0 == 0 && col.Y1 == int32(viewH) {
			t.Errorf("at distance three, column %d still reaches both edges", i)
			break
		}
	}
	if fits.Dst.H >= viewH {
		t.Errorf("at distance three the panel is %d tall in a %d canvas",
			fits.Dst.H, viewH)
	}

	// A panel half off the side is half a panel, and its columns still read
	// source columns that exist.
	half := project(t, 1, 0, hw, panelH, f, 1.5, viewW, viewH)
	if half.Dst.X+half.Dst.W > viewW {
		t.Errorf("the neighbour reaches x=%d in a %d canvas",
			half.Dst.X+half.Dst.W, viewW)
	}
	const wholeNeighbour = viewW * 2 / 3 // its width at distance 1.5
	if half.Dst.W >= wholeNeighbour {
		t.Errorf("the neighbour is %d wide, so it was not clipped at all", half.Dst.W)
	}
}

// TestASlantReusesItsBuffer: the columns are appended to the caller's scratch, so
// a frame drawing three turned screens allocates nothing.
func TestASlantReusesItsBuffer(t *testing.T) {
	const viewW, viewH = 1920, 1200
	hw, panelH, f := slantOptics(45, viewW, viewW, viewH)
	scratch := make([]SlantCol, 0, 4096)

	first := projectInto(t, scratch, 0, 15, hw, panelH, f, 3, viewW, viewH)
	second := projectInto(t, scratch, 1, 15, hw, panelH, f, 3, viewW, viewH)
	if &first.Cols[0] != &second.Cols[0] {
		t.Error("the second projection allocated a new buffer instead of reusing " +
			"the one it was given -- which is also why a slant must be drawn " +
			"before the next is asked for")
	}
	if cap(scratch) != 4096 {
		t.Errorf("the scratch grew to %d", cap(scratch))
	}
}

// TestSlantOpticsPinsOneScreenToOneView, and falls back when the headset's field
// of view is not known.
func TestSlantOpticsPinsOneScreenToOneView(t *testing.T) {
	const viewW, viewH = 1920, 1200
	hw, panelH, f := slantOptics(45, viewW, viewW, viewH)

	// The pin: a panel straight ahead at distance one is exactly the view.
	if got := 2 * f * hw; math.Abs(got-viewW) > 1 {
		t.Errorf("a screen at distance one projects %g wide in a %d view", got, viewW)
	}
	if got := f * panelH; math.Abs(got-viewH) > 1 {
		t.Errorf("a screen at distance one projects %g tall in a %d view", got, viewH)
	}

	// A shape that is not the view's: the panel keeps the SOURCE's aspect, so a
	// square screen stays square rather than being stretched to the canvas.
	_, sqH, sqF := slantOptics(45, viewW, 1000, 1000)
	if got := sqF * sqH; math.Abs(got-float64(viewW)) > 1 {
		t.Errorf("a square screen projects %g tall in a %d-wide view", got, viewW)
	}

	// Unknown, and nonsense, both fall back to the default rather than dividing
	// by a tangent of nothing.
	want, _, _ := slantOptics(DefaultFOVDeg, viewW, viewW, viewH)
	for _, fov := range []float64{0, -10, 180, 400} {
		if got, _, _ := slantOptics(fov, viewW, viewW, viewH); got != want {
			t.Errorf("a field of view of %g gave a half-width of %g, want the "+
				"default's %g", fov, got, want)
		}
	}
}

// project is one panel of the chain, projected, with a fatal error if it cannot
// be -- which every case here expects to be able to.
func project(t *testing.T, j int, splay, hw, panelH, f, dist float64,
	viewW, viewH int) Slant {
	t.Helper()
	return projectInto(t, nil, j, splay, hw, panelH, f, dist, viewW, viewH)
}

func projectInto(t *testing.T, scratch []SlantCol, j int, splay, hw, panelH, f, dist float64,
	viewW, viewH int) Slant {
	t.Helper()
	lx, lz, rx, rz := slantChain(j, splay, hw, dist, 0)
	s, ok := slantOf(scratch, j, lx, lz, rx, rz, panelH, f, viewW, viewH, viewW, viewH)
	if !ok {
		t.Fatalf("panel %d at splay %g, distance %g: no projection", j, splay, dist)
	}
	return s
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// TestASlantOfSomethingTooThinToSeeIsStillDrawn.
//
// A screen four thousand pixels wide and one tall is a legitimate thing for a
// capture to hand over -- a status strip, or a display mode somebody set by
// mistake -- and the panel keeps the source's aspect, so its world height is a
// four-thousandth of its width and its projection is a fifth of a pixel.
//
// It is drawn as one or two rows, not dropped. Every panel is at eye level, so
// its projection straddles the middle row of the canvas whatever its height:
// there is no empty column to guard against, and the guard that used to be here
// was unreachable and is gone.
func TestASlantOfSomethingTooThinToSeeIsStillDrawn(t *testing.T) {
	const viewW, viewH = 800, 400
	hw, panelH, f := slantOptics(45, viewW, 4000, 1)

	for _, d := range []float64{1, 4, 40} {
		lx, lz, rx, rz := slantChain(0, 0, hw, d, 0)
		s, ok := slantOf(nil, 0, lx, lz, rx, rz, panelH, f, viewW, viewH, 4000, 1)
		if !ok {
			t.Fatalf("distance %g: a one-pixel screen was dropped", d)
		}
		if s.Dst.H < 1 || s.Dst.H > 2 {
			t.Errorf("distance %g: it is %d rows tall", d, s.Dst.H)
		}
		// And across the middle of the canvas, where a screen at eye level goes.
		if mid := viewH / 2; s.Dst.Y > mid || s.Dst.Y+s.Dst.H < mid {
			t.Errorf("distance %g: rows %d..%d do not include the middle row %d",
				d, s.Dst.Y, s.Dst.Y+s.Dst.H, mid)
		}
		for i, col := range s.Cols {
			if col.Y1 <= col.Y0 {
				t.Fatalf("distance %g, column %d covers no row at all", d, i)
			}
		}
	}
}

// TestCanvasSlantDrawsTheTurnedPicture.
//
// The drawing half. It is the slow path -- a pixel at a time, because a
// trapezoid's columns each have their own height and there is no run to copy --
// so what has to be proved is that it writes the right pixels in the right
// places and NOTHING outside them.
func TestCanvasSlantDrawsTheTurnedPicture(t *testing.T) {
	const viewW, viewH = 400, 200
	const srcW, srcH = 200, 100

	// A source whose every column is its own colour, so a pixel says which source
	// column it came from.
	src := Source{Pix: make([]byte, srcW*srcH*4), W: srcW, H: srcH, Stride: srcW * 4}
	for y := range srcH {
		for x := range srcW {
			o := (y*srcW + x) * 4
			src.Pix[o] = byte(x)
			src.Pix[o+1] = byte(y)
			src.Pix[o+2] = 0x40
			src.Pix[o+3] = 0xFF
		}
	}

	hw, panelH, f := slantOptics(45, viewW, srcW, srcH)
	s := project(t, 1, 20, hw, panelH, f, 2, viewW, viewH)

	c := NewCanvas(viewW, viewH)
	c.Slant(s, src)

	painted := 0
	for i, col := range s.Cols {
		x := s.Dst.X + i
		for y := range viewH {
			o := (y*viewW + x) * 4
			got := c.Pix[o : o+4]
			inside := col.Src >= 0 && int32(y) >= col.Y0 && int32(y) < col.Y1
			if !inside {
				if got[3] != 0 {
					t.Fatalf("column %d row %d is outside the panel and was "+
						"written: %v", i, y, got)
				}
				continue
			}
			painted++
			// The pixel came from this column's source column, and from the row
			// its own height maps to.
			wantX := byte(col.Src)
			wantY := byte(int(int32(y)-col.Y0) * srcH / int(col.Y1-col.Y0))
			if got[0] != wantX || got[1] != wantY {
				t.Fatalf("column %d row %d shows source %d,%d; want %d,%d",
					i, y, got[0], got[1], wantX, wantY)
			}
		}
	}
	if painted == 0 {
		t.Fatal("nothing was drawn at all")
	}

	// Nothing outside the bounding box, on either side.
	for _, x := range []int{s.Dst.X - 1, s.Dst.X + s.Dst.W} {
		if x < 0 || x >= viewW {
			continue
		}
		for y := range viewH {
			if o := (y*viewW + x) * 4; c.Pix[o+3] != 0 {
				t.Fatalf("the column at x=%d, outside the panel, was written", x)
			}
		}
	}
}

// TestCanvasSlantRefusesWhatItCannotDraw: every guard, and none of them a panic
// or a write.
//
// A slant that does not match its own column list, or one whose box is off the
// canvas, is a caller's mistake -- and the cost of getting it wrong here is
// writing over another screen's pixels or past the end of the buffer, so it is
// checked rather than trusted.
func TestCanvasSlantRefusesWhatItCannotDraw(t *testing.T) {
	const viewW, viewH = 100, 80
	src := Source{Pix: make([]byte, 40*20*4), W: 40, H: 20, Stride: 40 * 4}
	good := Slant{
		Dst:  rectOf(10, 10, 3, 20),
		Cols: []SlantCol{{Src: 0, Y0: 10, Y1: 30}, {Src: 1, Y0: 10, Y1: 30}, {Src: 2, Y0: 10, Y1: 30}},
	}
	for _, c := range []struct {
		what string
		s    Slant
		src  Source
	}{
		{"no width", Slant{Dst: rectOf(0, 0, 0, 10), Cols: nil}, src},
		{"no height", Slant{Dst: rectOf(0, 0, 10, 0), Cols: good.Cols}, src},
		{"no source", good, Source{}},
		{"a column list of the wrong length",
			Slant{Dst: rectOf(0, 0, 3, 20), Cols: good.Cols[:2]}, src},
		{"off the left", Slant{Dst: rectOf(-1, 0, 3, 20), Cols: good.Cols}, src},
		{"off the top", Slant{Dst: rectOf(0, -1, 3, 20), Cols: good.Cols}, src},
		{"off the right", Slant{Dst: rectOf(viewW-1, 0, 3, 20), Cols: good.Cols}, src},
		{"off the bottom", Slant{Dst: rectOf(0, viewH-1, 3, 20), Cols: good.Cols}, src},
	} {
		canvas := NewCanvas(viewW, viewH)
		canvas.Slant(c.s, c.src)
		for i, b := range canvas.Pix {
			if b != 0 {
				t.Errorf("%s: byte %d was written", c.what, i)
				break
			}
		}
	}

	// A source row that is not there -- a short buffer, which a capture that
	// stopped mid-frame produces -- is skipped rather than read past the end.
	short := Source{Pix: make([]byte, 4), W: 40, H: 20, Stride: 40 * 4}
	canvas := NewCanvas(viewW, viewH)
	canvas.Slant(good, short)
	for i, b := range canvas.Pix {
		if b != 0 {
			t.Errorf("a short source wrote byte %d", i)
			break
		}
	}
}

// rectOf is a destination rectangle, spelled short because these tests are full
// of them.
func rectOf(x, y, w, h int) stereo.Rect {
	return stereo.Rect{X: x, Y: y, W: w, H: h}
}
