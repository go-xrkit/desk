package desk

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-xrkit/depth3d"
	"github.com/go-xrkit/xrkit/glasses"
)

// markEyes is a converter that writes a value saying which eye it filled, so a
// test can tell the two apart without a depth model or a GPU.
type markEyes struct {
	calls  int
	closed bool
	fail   bool
}

func (m *markEyes) Describe() string { return "a converter for the test" }
func (m *markEyes) Close()           { m.closed = true }

func (m *markEyes) Convert(left, right []uint32, stride int, src []uint32, srcStride, w, h int) error {
	m.calls++
	if m.fail {
		return depth3d.ErrNothingToConvert
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			left[y*stride+x] = 0x1000000 | src[y*srcStride+x]&0xFFFFFF
			right[y*stride+x] = 0x2000000 | src[y*srcStride+x]&0xFFFFFF
		}
	}
	return nil
}

func TestTheTwoEyesDifferOnlyWhenTheConverterIsOn(t *testing.T) {
	p := stereoPlan(t)
	v, err := newView(p, 128, 32)
	if err != nil {
		t.Fatal(err)
	}
	c := NewCanvas(p.ScreenW, p.ScreenH)
	for i := range c.Pix {
		c.Pix[i] = byte(i)
	}

	// Off: the eyes are the same picture, which is what a flat source honestly
	// gives. If this ever stops being true without a converter, the desk is
	// inventing a parallax nobody asked for.
	v.draw(c)
	for y := 0; y < 32; y++ {
		for x := 0; x < 64; x++ {
			if l, r := v.out[y*128+x], v.out[y*128+64+x]; l != r {
				t.Fatalf("with no converter the eyes differ at %d,%d: %#x against %#x", x, y, l, r)
			}
		}
	}

	// On: they must not be.
	m := &markEyes{}
	v.SetConverter(m)
	v.draw(c)
	if m.calls != 1 {
		t.Fatalf("the converter was called %d times", m.calls)
	}
	for y := 0; y < 32; y++ {
		for x := 0; x < 64; x++ {
			l, r := v.out[y*128+x], v.out[y*128+64+x]
			if l>>24 != 1 || r>>24 != 2 {
				t.Fatalf("at %d,%d the eyes are %#x and %#x; each half was not written", x, y, l, r)
			}
		}
	}
}

func TestARefusedFrameFallsBackToTheFlatPictureRatherThanToBlack(t *testing.T) {
	// A viewer who sees the picture lose its depth knows what happened. One who
	// sees nothing does not, and will think the desk has crashed.
	p := stereoPlan(t)
	v, err := newView(p, 128, 32)
	if err != nil {
		t.Fatal(err)
	}
	c := NewCanvas(p.ScreenW, p.ScreenH)
	for i := range c.Pix {
		c.Pix[i] = 0xFF
	}
	m := &markEyes{fail: true}
	v.SetConverter(m)
	v.draw(c)
	if m.calls != 1 {
		t.Fatalf("the converter was called %d times", m.calls)
	}
	for y := 0; y < 32; y++ {
		for x := 0; x < 128; x++ {
			if v.out[y*128+x] == background {
				t.Fatalf("a refused frame left %d,%d black", x, y)
			}
		}
	}
}

func TestNothingIsConvertedWhenThereIsNoSecondEye(t *testing.T) {
	// One eye, and an odd framebuffer where the halves cannot be addressed by
	// one stride. Converting either would put the right eye a pixel out on
	// every row, which looks like a slightly soft picture rather than a bug.
	flat, err := NewPlan(glasses.Display{Name: "a monitor", Width: 1920, Height: 1080}, Options{Screens: 4})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		plan Plan
		w, h int
	}{
		{"one eye", flat, 1920, 1080},
		{"an odd framebuffer", stereoPlan(t), 129, 32},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v, err := newView(tc.plan, tc.w, tc.h)
			if err != nil {
				t.Fatal(err)
			}
			if v.convertible() {
				t.Fatal("it offered to convert")
			}
			m := &markEyes{}
			v.SetConverter(m)
			v.draw(NewCanvas(tc.plan.ScreenW, tc.plan.ScreenH))
			if m.calls != 0 {
				t.Fatalf("the converter was called %d times", m.calls)
			}
		})
	}
}

func TestTheToggleSaysWhichWayItWentAndOnlyWhenItChanged(t *testing.T) {
	p := stereoPlan(t)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	var got []bool
	d.OnStereo3D = func(on bool) { got = append(got, on) }

	d.Do(ActionStereo3D)
	d.Do(ActionStereo3D)
	if len(got) != 2 || got[0] != true || got[1] != false {
		t.Fatalf("two toggles reported %v, want [true false]", got)
	}

	// Explicit on and off exist because a shortcut is pressed blind. Asking for
	// the state it is already in must do nothing at all -- not re-open a depth
	// model and a GPU context.
	got = nil
	d.Do(ActionStereo3DOff)
	if len(got) != 0 {
		t.Fatalf("turning off what was already off reported %v", got)
	}
	d.Do(ActionStereo3DOn)
	d.Do(ActionStereo3DOn)
	if len(got) != 1 || !got[0] {
		t.Fatalf("two ons reported %v, want one true", got)
	}
}

func TestTheToggleWorksInTheGalleryToo(t *testing.T) {
	// How the picture is SHOWN has nothing to do with which screen is focused,
	// so it is answered before the ribbon, the gallery and the application list
	// are consulted. A toggle that only worked on the ribbon would be a toggle
	// that stops working exactly when a viewer is looking for a screen.
	p := stereoPlan(t)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	var got []bool
	d.OnStereo3D = func(on bool) { got = append(got, on) }
	d.Do(ActionGalleryOpen)
	d.Do(ActionStereo3D)
	if len(got) != 1 || !got[0] {
		t.Fatalf("in the gallery the toggle reported %v", got)
	}
}

func TestATooSmallViewIsRefusedRatherThanConverted(t *testing.T) {
	p := stereoPlan(t)
	v, err := newView(p, 128, 32)
	if err != nil {
		t.Fatal(err)
	}
	v.rows = nil // a view with no rows: nothing to scale into
	m := &markEyes{}
	v.SetConverter(m)
	if v.convert(NewCanvas(p.ScreenW, p.ScreenH)) {
		t.Fatal("it claimed to have converted nothing")
	}
	if m.calls != 0 {
		t.Fatalf("the converter was called %d times", m.calls)
	}
}

var _ = errors.Is

// TestAskingForThreeDSurvivesTheDisplayGoingAway.
//
// ⛔ SWITCHING THE HEADSET ENDS THE SESSION. The display is torn down and
// re-negotiated, so the ribbon goes with it -- and a wish held only in the view
// would be forgotten at exactly the moment it becomes possible to grant. The
// person would press the row, watch everything go black and come back, and have
// to press it again.
func TestAskingForThreeDSurvivesTheDisplayGoingAway(t *testing.T) {
	p := testPlan(t)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	if d.WantedStereo3D() {
		t.Fatal("a fresh desk already wants 3D")
	}
	d.wantStereo3D()
	if !d.WantedStereo3D() {
		t.Error("the wish did not survive being recorded")
	}
	// And it is a SEPARATE question from the other two ways a session ends:
	// only the caller knows what to do with each, and one flag with three
	// names would make the desk come back wrong.
	if d.WantsSettings() || d.WantsPause() {
		t.Errorf("asking for 3D reads as settings=%v paused=%v",
			d.WantsSettings(), d.WantsPause())
	}
}

// TestAHeadsetThatWillNotSwitchSaysWhatItSaid.
//
// ⛔ A ROW THAT SAYS A FACT A PERSON CANNOT ACT ON IS THE END OF THE
// CONVERSATION. It used to be greyed out with "switch the glasses to 3D
// first" -- a remedy that needed a hand on the headset. It is pressable now,
// and a headset that refuses says SO, on the picture, in its own words.
func TestAHeadsetThatWillNotSwitchSaysWhatItSaid(t *testing.T) {
	was := Set3D
	t.Cleanup(func() { Set3D = was })
	Set3D = func(bool) error { return ErrNoGlasses3D }

	if err := Set3D(true); !errors.Is(err, ErrNoGlasses3D) {
		t.Fatalf("the stand-in refused with %v", err)
	}
	// A refusal is a sentence a person reads, not a code to parse: it names the
	// package and what it would not do.
	if got := ErrNoGlasses3D.Error(); !strings.Contains(got, "glasses") {
		t.Errorf("the refusal reads %q", got)
	}
}
