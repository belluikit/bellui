package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// phoneInsets are a phone canvas's system insets in one orientation.
type phoneInsets struct{ top, left, right float32 }

var (
	portrait      = geo.NewSize(360, 800)
	landscape     = geo.NewSize(800, 360)
	portraitIns   = phoneInsets{top: 24}             // a status bar
	landscapeLeft = phoneInsets{left: 30, right: 10} // a notch on the left
)

// rotatingCanvas reports insets that a test changes as it rotates the
// canvas, the way a phone moves its notch from the top to a side.
type rotatingCanvas struct {
	core.Canvas
	ins *phoneInsets
}

func (c rotatingCanvas) InteractiveArea() (geo.Position, geo.Size) {
	return geo.NewPos(c.ins.left, c.ins.top), c.Size().SubtractWidthHeight(c.ins.left+c.ins.right, c.ins.top)
}

// rotate turns the device: the insets change, then the canvas is resized, as
// the drivers report a rotation.
func rotate(w bellui.Window, c rotatingCanvas, size geo.Size, ins phoneInsets) {
	*c.ins = ins
	w.Resize(size)
}

// assertRests checks the drawer rests against its edge of the canvas as it
// is now, with the height below Top and the width it would open with here.
func assertRests(t *testing.T, d *NavigationDrawer, c rotatingCanvas, atRight bool) {
	t.Helper()
	cs := c.Size()
	notch := c.ins.left
	if atRight {
		notch = c.ins.right
	}
	wantWidth := min(360+notch, cs.Width-drawerScrimStrip)

	pos, size := drawerSurface(t, d)
	if atRight {
		assert.Equal(t, cs.Width, pos.X+size.Width, "against the right edge")
	} else {
		assert.Equal(t, float32(0), pos.X, "against the left edge")
	}
	assert.Equal(t, d.Top, pos.Y, "from Top")
	assert.Equal(t, geo.NewSize(wantWidth, cs.Height-d.Top), size, "the width and height for this canvas")
	if d.Modal {
		assert.Equal(t, geo.NewSize(cs.Width, cs.Height-d.Top), drawerScrim(t, d).Size(), "the scrim covers the canvas")
	}
	_, rowLeft, _ := drawerRowsAbs(t, d)
	if atRight {
		rowRight := rowLeft + drawerRows(t, d)[0].Size().Width
		assert.LessOrEqual(t, rowRight, cs.Width-c.ins.right, "the destination clears the right notch")
	} else {
		assert.GreaterOrEqual(t, rowLeft, c.ins.left, "the destination clears the left notch")
	}
	assert.True(t, d.IsOpen())
}

// An open drawer rotating with the device stays against the edge it opened
// on, spans the new height and keeps clear of the new insets. It used to keep
// the geometry it opened with: an end drawer turned to landscape sat near the
// left edge, and one turned to portrait was off the canvas with only its
// scrim left (#1467).
func TestNavigationDrawer_Rotates(t *testing.T) {
	items := []*NavigationDrawerItem{{Label: "Home", Icon: theme.HomeIcon()}}
	for _, tc := range []struct {
		name     string
		side     DrawerSide
		modal    bool
		top      float32
		from, to geo.Size
		fromIns  phoneInsets
		toIns    phoneInsets
	}{
		{"end, portrait to landscape", DrawerSideEnd, true, 0, portrait, landscape, portraitIns, landscapeLeft},
		{"end, landscape to portrait", DrawerSideEnd, true, 0, landscape, portrait, landscapeLeft, portraitIns},
		{"start, portrait to landscape", DrawerSideStart, true, 0, portrait, landscape, portraitIns, landscapeLeft},
		{"start, landscape to portrait", DrawerSideStart, true, 0, landscape, portrait, landscapeLeft, portraitIns},
		{"end below a header, landscape to portrait", DrawerSideEnd, true, 56, landscape, portrait, landscapeLeft, portraitIns},
		{"standard end, landscape to portrait", DrawerSideEnd, false, 0, landscape, portrait, landscapeLeft, portraitIns},
		{"standard start, portrait to landscape", DrawerSideStart, false, 0, portrait, landscape, portraitIns, landscapeLeft},
	} {
		t.Run(tc.name, func(t *testing.T) {
			test.NewTempApp(t)
			w := test.NewTempWindow(t, NewLabel("content"))
			ins := tc.fromIns
			c := rotatingCanvas{Canvas: w.Canvas(), ins: &ins}
			w.Resize(tc.from)

			d := NewNavigationDrawer(items, nil)
			d.Side = tc.side
			d.Modal = tc.modal
			d.Top = tc.top
			d.Show(c)
			defer d.Hide()
			atRight := tc.side == DrawerSideEnd
			assertRests(t, d, c, atRight)

			rotate(w, c, tc.to, tc.toIns)
			assertRests(t, d, c, atRight)

			rotate(w, c, tc.from, tc.fromIns)
			assertRests(t, d, c, atRight)
		})
	}
}

// A drawer rotated in the middle of its slide in ends at once where it rests
// on the new canvas, rather than finishing the slide to the old x.
func TestNavigationDrawer_RotatesWhileSlidingIn(t *testing.T) {
	a := test.NewTempApp(t)
	drv := &carouselAnimDriver{Driver: a.Driver()}
	bellui.SetCurrentApp(&carouselAnimApp{App: a, drv: drv})
	t.Cleanup(func() { bellui.SetCurrentApp(a) })

	w := test.NewTempWindow(t, NewLabel("content"))
	ins := landscapeLeft
	c := rotatingCanvas{Canvas: w.Canvas(), ins: &ins}
	w.Resize(landscape)
	d := NewNavigationDrawer([]*NavigationDrawerItem{{Label: "Home", Icon: theme.HomeIcon()}}, nil)
	d.Side = DrawerSideEnd
	d.Show(c)
	t.Cleanup(d.hideNow)
	slide := d.slide
	require.NotNil(t, slide)
	slide.Tick(0.5)

	rotate(w, c, portrait, portraitIns)
	assert.Contains(t, drv.stopped, slide, "the slide in stops")
	assert.Nil(t, d.slide)
	assertRests(t, d, c, true)
}

// A drawer rotated while it slides out still leaves past its edge of the new
// canvas. The slide used to aim for the old edge, which on a wider canvas is
// in the middle of the screen.
func TestNavigationDrawer_RotatesWhileSlidingOut(t *testing.T) {
	a := test.NewTempApp(t)
	drv := &carouselAnimDriver{Driver: a.Driver()}
	bellui.SetCurrentApp(&carouselAnimApp{App: a, drv: drv})
	t.Cleanup(func() { bellui.SetCurrentApp(a) })

	w := test.NewTempWindow(t, NewLabel("content"))
	ins := portraitIns
	c := rotatingCanvas{Canvas: w.Canvas(), ins: &ins}
	w.Resize(portrait)
	d := NewNavigationDrawer([]*NavigationDrawerItem{{Label: "Home", Icon: theme.HomeIcon()}}, nil)
	d.Side = DrawerSideEnd
	d.Show(c)
	d.slide.Tick(1)
	t.Cleanup(d.hideNow)

	p := d.popUp
	d.Hide()
	slide := d.slide
	require.NotNil(t, slide)
	slide.Tick(0.5)
	rotate(w, c, landscape, landscapeLeft)
	slide.Tick(1)
	assert.Equal(t, landscape.Width, p.innerPos.X, "ends beyond the new right edge")
	assert.True(t, p.isHiding)
	assert.False(t, d.IsOpen())
}

// A change of insets alone, such as a landscape phone turned the other way
// round that moves its notch to the other side, places the drawer again on
// the next refresh, though the canvas keeps its size.
func TestNavigationDrawer_InsetsChangeAlone(t *testing.T) {
	test.NewTempApp(t)
	w := test.NewTempWindow(t, NewLabel("content"))
	ins := landscapeLeft
	c := rotatingCanvas{Canvas: w.Canvas(), ins: &ins}
	w.Resize(landscape)
	d := NewNavigationDrawer([]*NavigationDrawerItem{{Label: "Home", Icon: theme.HomeIcon()}}, nil)
	d.Show(c)
	defer d.Hide()
	assertRests(t, d, c, false)

	ins = phoneInsets{left: 10, right: 30}
	d.popUp.Refresh()
	assertRests(t, d, c, false)
	_, size := drawerSurface(t, d)
	assert.Equal(t, float32(370), size.Width, "the surface spans the smaller left inset only")
}
