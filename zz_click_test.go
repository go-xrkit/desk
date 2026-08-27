package desk

import "testing"

func TestProbeAdderClick(t *testing.T) {
	p := fanPlan(t, 6, -1, 1)
	d, err := New(p, feedsFor(p))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	added := 0
	d.OnAdd = func() (Feed, error) {
		added++
		return newFakeFeed(p.ScreenW, p.ScreenH, 9), nil
	}
	d.Do(ActionGalleryOpen)
	d.Render()

	g := d.grid
	idx, ok := g.Adder()
	t.Logf("cells=%d screens=%d adder=%d ok=%v", g.Cells(), g.Screens(), idx, ok)
	// Walk the canvas looking for the adder cell, the way a click does.
	found := 0
	var hitX, hitY int
	for y := 0; y < p.ScreenH; y += 7 {
		for x := 0; x < p.ScreenW; x += 7 {
			if i, ok := g.At(x, y); ok && i == idx {
				found++
				if hitX == 0 {
					hitX, hitY = x, y
				}
			}
		}
	}
	t.Logf("adder occupies ~%d sampled points, first at %d,%d", found, hitX, hitY)
	if found == 0 {
		t.Fatal("the adder cell is nowhere on the canvas")
	}
	if !d.Click(hitX, hitY) {
		t.Errorf("Click(%d,%d) on the adder was not taken", hitX, hitY)
	}
	t.Logf("after the click: added=%d screens=%d err=%v", added, d.Plan().Count(), d.Err())
}
