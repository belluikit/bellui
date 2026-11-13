package widget

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	internalWidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// drawerRows digs the destination row widgets out of a shown drawer's popup so
// a test can drive a real tap through them.
func drawerRows(t *testing.T, d *NavigationDrawer) []*drawerItemWidget {
	t.Helper()
	scroll, ok := d.popUp.Content.(*internalWidget.Scroll)
	require.True(t, ok, "drawer popup content should be a scroll")
	inset, ok := scroll.Content.(*bellui.Container)
	require.True(t, ok)
	padded, ok := inset.Objects[0].(*bellui.Container)
	require.True(t, ok)
	content, ok := padded.Objects[0].(*bellui.Container)
	require.True(t, ok)

	var rows []*drawerItemWidget
	for _, o := range content.Objects {
		if row, ok := o.(*drawerItemWidget); ok {
			rows = append(rows, row)
		}
	}
	return rows
}

// The captions are sized as well as placed: an unsized text has no box for
// partial repaint to find its glyphs in, which cut "File Transfer" in half in
// the gallery (#602), and screen readers get no bounds for it.
func TestNavigationDrawer_CaptionsAreSized(t *testing.T) {
	test.NewTempApp(t)
	items := []*NavigationDrawerItem{
		{Label: "Home", Icon: theme.HomeIcon()},
		{Label: "File Transfer", Icon: theme.UploadIcon()},
	}
	d := NewNavigationDrawer(items, nil)
	w := test.NewTempWindow(t, NewLabel("content"))
	w.Resize(geo.NewSize(400, 600))
	d.Show(w.Canvas())

	rows := drawerRows(t, d)
	require.Len(t, rows, 2)
	for _, row := range rows {
		assert.Equal(t, row.label.MinSize(), row.label.Size(), row.label.Text)
	}
}

// An item may leave its Icon out: it draws its label alone, starting where
// the icon would, rather than panicking on the first frame (#1179).
func TestNavigationDrawer_ItemWithoutIcon(t *testing.T) {
	test.NewTempApp(t)
	items := []*NavigationDrawerItem{
		{Label: "Home", Icon: theme.HomeIcon()},
		{Label: "Settings"},
		{Label: "Account"},
	}
	d := NewNavigationDrawer(items, nil)
	d.Selected = 1
	w := test.NewTempWindow(t, NewLabel("content"))
	w.Resize(geo.NewSize(400, 600))
	d.Show(w.Canvas())
	w.Canvas().Capture()

	rows := drawerRows(t, d)
	require.Len(t, rows, 3)
	for _, row := range rows {
		row.Refresh()
	}
	assert.True(t, rows[0].icon.Visible())
	assert.Equal(t, float32(52), rows[0].label.Position().X)
	for _, row := range rows[1:] {
		assert.False(t, row.icon.Visible(), row.label.Text)
		assert.Equal(t, float32(16), row.label.Position().X, row.label.Text)
	}
}

func TestNavigationDrawer_ItemTap(t *testing.T) {
	test.NewTempApp(t)
	var got, calls int
	items := []*NavigationDrawerItem{
		{Label: "Home", Icon: theme.HomeIcon()},
		{Label: "Settings", Icon: theme.SettingsIcon()},
		{Label: "Account", Icon: theme.AccountIcon()},
	}
	d := NewNavigationDrawer(items, func(i int) { got = i; calls++ })

	w := test.NewTempWindow(t, NewLabel("content"))
	w.Resize(geo.NewSize(400, 600))
	d.Show(w.Canvas())
	require.True(t, d.IsOpen())

	rows := drawerRows(t, d)
	require.Len(t, rows, 3)

	// Tapping a destination selects it, fires OnChanged and dismisses the drawer.
	rows[2].Tapped(&event.PointEvent{})
	assert.Equal(t, 2, d.Selected)
	assert.Equal(t, 2, got)
	assert.Equal(t, 1, calls)
	assert.False(t, d.IsOpen(), "selecting a destination closes the drawer")
}

// TestNavigationDrawer_ScrollsLongLists ensures the drawer content is wrapped
// in a scroll container so destination lists taller than the canvas remain
// reachable on small windows.
func TestNavigationDrawer_ScrollsLongLists(t *testing.T) {
	test.NewTempApp(t)

	items := make([]*NavigationDrawerItem, 30)
	for i := range items {
		items[i] = &NavigationDrawerItem{Label: fmt.Sprintf("Item %d", i)}
	}

	w := test.NewTempWindow(t, NewLabel("content"))
	w.Resize(geo.NewSize(400, 300))

	drawer := NewNavigationDrawer(items, nil)
	drawer.Show(w.Canvas())
	defer drawer.Hide()

	scroll, ok := drawer.popUp.Content.(*internalWidget.Scroll)
	if !ok {
		t.Fatalf("expected drawer popup content to be a scroll container, got %T", drawer.popUp.Content)
	}
	if scroll.Content.MinSize().Height <= w.Canvas().Size().Height {
		t.Fatal("expected the 30-item list to exceed the canvas height")
	}
}

// insetCanvas reports system insets the way a phone canvas does: a status bar
// at the top, a home indicator at the bottom and a notch on the left.
type insetCanvas struct {
	core.Canvas
	top, bottom, left float32
}

func (c insetCanvas) InteractiveArea() (geo.Position, geo.Size) {
	return geo.NewPos(c.left, c.top), c.Size().SubtractWidthHeight(c.left, c.top+c.bottom)
}

// drawerRowsAbs returns the absolute top and bottom of the drawer's first and
// last destination, with the drawer scrolled to its end.
func drawerRowsAbs(t *testing.T, d *NavigationDrawer) (firstTop, firstLeft, lastBottom float32) {
	t.Helper()
	rows := drawerRows(t, d)
	require.NotEmpty(t, rows)
	drv := bellui.CurrentApp().Driver()
	first := drv.AbsolutePositionForObject(rows[0])

	scroll := d.popUp.Content.(*internalWidget.Scroll)
	scroll.ScrollToBottom()
	last := rows[len(rows)-1]
	lastPos := drv.AbsolutePositionForObject(last)
	return first.Y, first.X, lastPos.Y + last.Size().Height
}

// The drawer's destinations start below the status bar and can scroll clear
// of the home indicator; the surface itself still covers the whole edge (#608).
func TestNavigationDrawer_KeepsDestinationsInsideInsets(t *testing.T) {
	test.NewTempApp(t)
	items := make([]*NavigationDrawerItem, 30)
	for i := range items {
		items[i] = &NavigationDrawerItem{Label: fmt.Sprintf("Item %d", i), Icon: theme.HomeIcon()}
	}
	w := test.NewTempWindow(t, NewLabel("content"))
	w.Resize(geo.NewSize(400, 600))

	// Reference: the same drawer on a canvas without insets. The test canvas
	// reports a small inset of its own, so the reference zeroes it.
	plain := NewNavigationDrawer(items, nil)
	plain.Show(insetCanvas{Canvas: w.Canvas()})
	plainTop, plainLeft, plainBottom := drawerRowsAbs(t, plain)
	plainSurfaceY := plain.popUp.Content.Position().Y
	plain.Hide()

	c := insetCanvas{Canvas: w.Canvas(), top: 48, bottom: 24, left: 30}
	d := NewNavigationDrawer(items, nil)
	d.Show(c)
	defer d.Hide()
	top, left, bottom := drawerRowsAbs(t, d)

	assert.Equal(t, plainTop+c.top, top, "first destination sits below the status bar")
	assert.Equal(t, plainLeft+c.left, left, "destinations clear a left notch")
	assert.Equal(t, plainBottom-c.bottom, bottom, "last destination scrolls clear of the home indicator")
	assert.Equal(t, plainSurfaceY, d.popUp.Content.Position().Y,
		"the drawer surface still starts at the top edge")
}

// edgeCanvas reports a notch on either side, as a phone in landscape does.
type edgeCanvas struct {
	core.Canvas
	left, right float32
}

func (c edgeCanvas) InteractiveArea() (geo.Position, geo.Size) {
	return geo.NewPos(c.left, 0), c.Size().SubtractWidthHeight(c.left+c.right, 0)
}

// drawerSurface is the drawer's surface on the canvas: its background, which
// runs to the edges the content's insets keep the destinations clear of.
func drawerSurface(t *testing.T, d *NavigationDrawer) (pos geo.Position, size geo.Size) {
	t.Helper()
	switch r := cache.Renderer(d.popUp).(type) {
	case *modalPopUpRenderer:
		return r.background.Position(), r.background.Size()
	case *popUpRenderer:
		return r.background.Position(), r.background.Size()
	}
	t.Fatalf("unexpected drawer renderer %T", cache.Renderer(d.popUp))
	return
}

func drawerScrim(t *testing.T, d *NavigationDrawer) *canvas.Rectangle {
	t.Helper()
	r, ok := cache.Renderer(d.popUp).(*modalPopUpRenderer)
	require.True(t, ok, "a modal drawer has a scrim")
	return r.underlay
}

// A drawer with neither Side nor Top set comes to rest where the drawer from
// before #1171 sat: at the left edge, full height, its scrim over the whole
// canvas, and a tap on the scrim anywhere closes it.
func TestNavigationDrawer_DefaultsUnchanged(t *testing.T) {
	test.NewTempApp(t)
	items := []*NavigationDrawerItem{{Label: "Home", Icon: theme.HomeIcon()}}
	header := NewButton("header", nil)
	w := test.NewTempWindow(t, &bellui.Container{Layout: layout.NewVBoxLayout(), Objects: []core.CanvasObject{header}})
	w.Resize(geo.NewSize(800, 600))
	c := w.Canvas()

	d := NewNavigationDrawer(items, nil)
	d.Show(c)
	pos, size := drawerSurface(t, d)
	assert.Equal(t, geo.NewPos(0, 0), pos)
	assert.Equal(t, c.Size().Height, size.Height)
	scrim := drawerScrim(t, d)
	assert.Equal(t, geo.NewPos(0, 0), scrim.Position())
	assert.Equal(t, c.Size(), scrim.Size())

	// Over the header, outside the drawer: the scrim takes the tap.
	tapped := false
	header.OnTapped = func() { tapped = true }
	at := bellui.CurrentApp().Driver().AbsolutePositionForObject(header)
	require.Greater(t, at.X+header.Size().Width, size.Width, "the header reaches past the drawer")
	test.TapCanvas(c, geo.NewPos(at.X+header.Size().Width-2, at.Y+2))
	assert.False(t, tapped, "the scrim covers the header")
	assert.False(t, d.IsOpen(), "a tap on the scrim closes the drawer")
}

// An end drawer sits against the end edge: the right in a left-to-right
// layout, the left in a right-to-left one, where a start drawer moves to the
// right. The notch on its own side moves with it.
func TestNavigationDrawer_Side(t *testing.T) {
	test.NewTempApp(t)
	items := []*NavigationDrawerItem{{Label: "Home", Icon: theme.HomeIcon()}}
	w := test.NewTempWindow(t, NewLabel("content"))
	w.Resize(geo.NewSize(800, 400))
	c := edgeCanvas{Canvas: w.Canvas(), left: 30, right: 40}
	width := c.Size().Width

	for _, tc := range []struct {
		name  string
		dir   bellui.Direction
		side  DrawerSide
		right bool
	}{
		{"start ltr", bellui.DirectionLTR, DrawerSideStart, false},
		{"end ltr", bellui.DirectionLTR, DrawerSideEnd, true},
		{"start rtl", bellui.DirectionRTL, DrawerSideStart, true},
		{"end rtl", bellui.DirectionRTL, DrawerSideEnd, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bellui.SetLayoutDirection(tc.dir)
			t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })

			d := NewNavigationDrawer(items, nil)
			d.Side = tc.side
			d.Show(c)
			defer d.Hide()

			pos, size := drawerSurface(t, d)
			_, rowLeft, _ := drawerRowsAbs(t, d)
			rowRight := rowLeft + drawerRows(t, d)[0].Size().Width
			if tc.right {
				assert.Equal(t, width, pos.X+size.Width, "against the right edge")
				assert.Equal(t, 360+c.right, size.Width, "the surface spans the right notch")
				assert.LessOrEqual(t, rowRight, width-c.right, "the destination clears the right notch")
				assert.Less(t, rowLeft-pos.X, c.left, "and no left notch is kept clear")
			} else {
				assert.Equal(t, float32(0), pos.X, "against the left edge")
				assert.Equal(t, 360+c.left, size.Width, "the surface spans the left notch")
				assert.GreaterOrEqual(t, rowLeft, c.left, "the destination clears the left notch")
			}
		})
	}
}

// With Top set, the drawer and its scrim start there. A tap above Top reaches
// the header button and leaves the drawer open; a tap on the scrim below
// closes it.
func TestNavigationDrawer_Top(t *testing.T) {
	test.NewTempApp(t)
	items := []*NavigationDrawerItem{{Label: "Home", Icon: theme.HomeIcon()}}
	tapped := 0
	header := NewButton("header", func() { tapped++ })
	w := test.NewTempWindow(t, &bellui.Container{Layout: layout.NewVBoxLayout(), Objects: []core.CanvasObject{header}})
	w.Resize(geo.NewSize(800, 600))
	c := w.Canvas()
	at := bellui.CurrentApp().Driver().AbsolutePositionForObject(header)
	top := at.Y + header.Size().Height

	d := NewNavigationDrawer(items, nil)
	d.Side = DrawerSideEnd
	d.Top = top
	d.Show(c)

	pos, size := drawerSurface(t, d)
	assert.Equal(t, top, pos.Y, "the drawer starts at Top")
	assert.Equal(t, c.Size().Height-top, size.Height, "and runs to the bottom")
	scrim := drawerScrim(t, d)
	assert.Equal(t, geo.NewPos(0, top), scrim.Position(), "the scrim starts at Top")
	assert.Equal(t, geo.NewSize(c.Size().Width, c.Size().Height-top), scrim.Size())

	// Above Top, over the drawer's column too: the header gets the tap.
	test.TapCanvas(c, geo.NewPos(c.Size().Width-20, at.Y+header.Size().Height/2))
	test.TapCanvas(c, geo.NewPos(20, at.Y+header.Size().Height/2))
	assert.Equal(t, 2, tapped, "taps above Top reach the header")
	assert.True(t, d.IsOpen(), "and leave the drawer open")

	// Below Top, outside the drawer: the scrim closes it.
	test.TapCanvas(c, geo.NewPos(20, top+20))
	assert.Equal(t, 2, tapped)
	assert.False(t, d.IsOpen(), "a tap on the scrim closes the drawer")
}

// Top past the status bar leaves the status-bar inset out of the drawer: its
// destinations start at Top, not a status bar's height below it.
func TestNavigationDrawer_TopPastStatusBar(t *testing.T) {
	test.NewTempApp(t)
	items := []*NavigationDrawerItem{{Label: "Home", Icon: theme.HomeIcon()}}
	w := test.NewTempWindow(t, NewLabel("content"))
	w.Resize(geo.NewSize(400, 600))

	plain := NewNavigationDrawer(items, nil)
	plain.Show(insetCanvas{Canvas: w.Canvas()})
	plainTop, _, _ := drawerRowsAbs(t, plain)
	plain.Hide()

	c := insetCanvas{Canvas: w.Canvas(), top: 48}
	d := NewNavigationDrawer(items, nil)
	d.Top = 100
	d.Show(c)
	defer d.Hide()
	rowTop, _, _ := drawerRowsAbs(t, d)
	assert.Equal(t, plainTop+100, rowTop)

	d.Top = 20 // under the status bar: the rest of the bar still applies
	d.Show(c)
	rowTop, _, _ = drawerRowsAbs(t, d)
	assert.Equal(t, plainTop+48, rowTop)
}

// With animations on, a drawer slides in from beyond its edge to its place;
// with them off, it appears there (#1171).
func TestNavigationDrawer_SlidesIn(t *testing.T) {
	items := []*NavigationDrawerItem{{Label: "Home", Icon: theme.HomeIcon()}}
	show := func(t *testing.T, noAnim bool, dir bellui.Direction, side DrawerSide) (*NavigationDrawer, *carouselAnimDriver, float32) {
		t.Helper()
		a := test.NewTempApp(t)
		drv := &carouselAnimDriver{Driver: a.Driver()}
		bellui.SetCurrentApp(&carouselAnimApp{App: a, drv: drv, noAnim: noAnim})
		t.Cleanup(func() { bellui.SetCurrentApp(a) })
		bellui.SetLayoutDirection(dir)
		t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })

		w := test.NewTempWindow(t, NewLabel("content"))
		w.Resize(geo.NewSize(800, 400))
		d := NewNavigationDrawer(items, nil)
		d.Side = side
		d.Top = 50
		d.Show(insetCanvas{Canvas: w.Canvas()})
		t.Cleanup(d.Hide)
		return d, drv, w.Canvas().Size().Width
	}
	surfaceX := func(t *testing.T, d *NavigationDrawer) float32 {
		t.Helper()
		pos, _ := drawerSurface(t, d)
		assert.Equal(t, float32(50), pos.Y, "the slide keeps Top")
		return pos.X
	}

	t.Run("end ltr slides from the right", func(t *testing.T) {
		d, drv, width := show(t, false, bellui.DirectionLTR, DrawerSideEnd)
		require.NotNil(t, d.slide)
		assert.Contains(t, drv.started, d.slide)
		d.slide.Tick(0)
		assert.Equal(t, width, surfaceX(t, d), "starts beyond the right edge")
		d.slide.Tick(0.5)
		mid := surfaceX(t, d)
		assert.Less(t, mid, width)
		assert.Greater(t, mid, width-360)
		d.slide.Tick(1)
		assert.Equal(t, width-360, surfaceX(t, d), "ends against the right edge")
	})

	t.Run("end rtl slides from the left", func(t *testing.T) {
		d, _, _ := show(t, false, bellui.DirectionRTL, DrawerSideEnd)
		require.NotNil(t, d.slide)
		d.slide.Tick(0)
		assert.Equal(t, float32(-360), surfaceX(t, d), "starts beyond the left edge")
		d.slide.Tick(1)
		assert.Equal(t, float32(0), surfaceX(t, d), "ends against the left edge")
	})

	t.Run("animations off: in place", func(t *testing.T) {
		d, drv, width := show(t, true, bellui.DirectionLTR, DrawerSideEnd)
		assert.Nil(t, d.slide)
		assert.Empty(t, drv.started)
		assert.Equal(t, width-360, surfaceX(t, d))
	})

	t.Run("start ltr slides from the left", func(t *testing.T) {
		d, _, _ := show(t, false, bellui.DirectionLTR, DrawerSideStart)
		require.NotNil(t, d.slide)
		d.slide.Tick(0)
		assert.Equal(t, float32(-360), surfaceX(t, d), "starts beyond the left edge")
		d.slide.Tick(1)
		assert.Equal(t, float32(0), surfaceX(t, d), "ends against the left edge")
	})

	t.Run("start rtl slides from the right", func(t *testing.T) {
		d, _, width := show(t, false, bellui.DirectionRTL, DrawerSideStart)
		require.NotNil(t, d.slide)
		d.slide.Tick(0)
		assert.Equal(t, width, surfaceX(t, d), "starts beyond the right edge")
		d.slide.Tick(1)
		assert.Equal(t, width-360, surfaceX(t, d), "ends against the right edge")
	})

	t.Run("animations off, start: in place", func(t *testing.T) {
		d, drv, _ := show(t, true, bellui.DirectionLTR, DrawerSideStart)
		assert.Nil(t, d.slide)
		assert.Empty(t, drv.started)
		assert.Equal(t, float32(0), surfaceX(t, d))
	})

	t.Run("hide stops the slide in", func(t *testing.T) {
		d, drv, _ := show(t, false, bellui.DirectionLTR, DrawerSideEnd)
		slide := d.slide
		d.Hide()
		assert.Contains(t, drv.stopped, slide)
		assert.NotSame(t, slide, d.slide, "the slide out replaces it")
	})
}

// With animations on, a drawer closing by any path slides from where it is
// out beyond the edge it opened against, then leaves the canvas; with them
// off, it goes at once (#1186).
func TestNavigationDrawer_SlidesOut(t *testing.T) {
	items := []*NavigationDrawerItem{{Label: "Home", Icon: theme.HomeIcon()}}
	show := func(t *testing.T, noAnim bool, dir bellui.Direction, side DrawerSide) (*NavigationDrawer, *carouselAnimDriver, core.Canvas) {
		t.Helper()
		a := test.NewTempApp(t)
		drv := &carouselAnimDriver{Driver: a.Driver()}
		bellui.SetCurrentApp(&carouselAnimApp{App: a, drv: drv, noAnim: noAnim})
		t.Cleanup(func() { bellui.SetCurrentApp(a) })
		bellui.SetLayoutDirection(dir)
		t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })

		w := test.NewTempWindow(t, NewLabel("content"))
		w.Resize(geo.NewSize(800, 400))
		d := NewNavigationDrawer(items, nil)
		d.Side = side
		d.Top = 50
		d.Show(insetCanvas{Canvas: w.Canvas()})
		if d.slide != nil {
			d.slide.Tick(1)
		}
		t.Cleanup(d.hideNow)
		return d, drv, w.Canvas()
	}
	// closingX is where the sliding-out surface is.
	closingX := func(t *testing.T, p *PopUp) float32 {
		t.Helper()
		assert.Equal(t, float32(50), p.innerPos.Y, "the slide keeps Top")
		return p.innerPos.X
	}
	onCanvas := func(c core.Canvas, p *PopUp) bool {
		for _, o := range c.Overlays().List() {
			if o == p {
				return true
			}
		}
		return false
	}

	t.Run("end ltr slides to the right", func(t *testing.T) {
		d, drv, c := show(t, false, bellui.DirectionLTR, DrawerSideEnd)
		p := d.popUp
		width := c.Size().Width
		d.Hide()
		assert.False(t, d.IsOpen(), "closed from the moment the slide begins")
		require.NotNil(t, d.slide)
		assert.Contains(t, drv.started, d.slide)
		slide := d.slide
		slide.Tick(0)
		assert.Equal(t, width-360, closingX(t, p), "starts where it is")
		slide.Tick(0.5)
		mid := closingX(t, p)
		assert.Greater(t, mid, width-360)
		assert.Less(t, mid, width)
		assert.True(t, onCanvas(c, p), "still drawn while it slides")
		slide.Tick(1)
		assert.Equal(t, width, closingX(t, p), "ends beyond the right edge")
		assert.Nil(t, d.slide)
		assert.Nil(t, d.closing)
		assert.True(t, p.isHiding, "then its pop-up hides")
	})

	t.Run("start ltr slides to the left", func(t *testing.T) {
		d, _, _ := show(t, false, bellui.DirectionLTR, DrawerSideStart)
		p := d.popUp
		d.Hide()
		require.NotNil(t, d.slide)
		d.slide.Tick(0)
		assert.Equal(t, float32(0), closingX(t, p))
		d.slide.Tick(1)
		assert.Equal(t, float32(-360), closingX(t, p), "ends beyond the left edge")
	})

	t.Run("start rtl slides to the right", func(t *testing.T) {
		d, _, c := show(t, false, bellui.DirectionRTL, DrawerSideStart)
		p := d.popUp
		d.Hide()
		require.NotNil(t, d.slide)
		d.slide.Tick(1)
		assert.Equal(t, c.Size().Width, closingX(t, p), "ends beyond the right edge")
	})

	t.Run("end rtl slides to the left", func(t *testing.T) {
		d, _, _ := show(t, false, bellui.DirectionRTL, DrawerSideEnd)
		p := d.popUp
		d.Hide()
		require.NotNil(t, d.slide)
		d.slide.Tick(1)
		assert.Equal(t, float32(-360), closingX(t, p), "ends beyond the left edge")
	})

	t.Run("closing mid slide in starts from where it got to", func(t *testing.T) {
		d, _, c := show(t, false, bellui.DirectionLTR, DrawerSideEnd)
		d.hideNow()
		d.Show(insetCanvas{Canvas: c})
		p := d.popUp
		d.slide.Tick(0.5)
		at := p.innerPos.X
		d.Hide()
		d.slide.Tick(0)
		assert.Equal(t, at, closingX(t, p))
	})

	dismissals := map[string]func(t *testing.T, p *PopUp){
		"scrim tap":     func(_ *testing.T, p *PopUp) { p.Tapped(&event.PointEvent{Position: geo.NewPos(10, 200)}) },
		"escape":        func(t *testing.T, p *PopUp) { assert.True(t, p.DismissOnEscape()) },
		"a11y dismiss":  func(_ *testing.T, p *PopUp) { p.PerformAccessibleAction(core.ActionDismiss) },
		"toggle":        nil,
		"item selected": nil,
	}
	for name, dismiss := range dismissals {
		t.Run(name+" slides out", func(t *testing.T) {
			d, _, _ := show(t, false, bellui.DirectionLTR, DrawerSideEnd)
			p := d.popUp
			switch name {
			case "toggle":
				d.Toggle(p.Canvas)
			case "item selected":
				drawerRows(t, d)[0].Tapped(&event.PointEvent{})
			default:
				dismiss(t, p)
			}
			assert.False(t, d.IsOpen())
			assert.Same(t, p, d.closing, "it slides rather than fading in place")
			assert.True(t, p.isHiding, "a second dismissal is absorbed")
			assert.False(t, p.DismissOnEscape(), "Escape goes to what is beneath")
		})
	}

	t.Run("a destination tapped while sliding out is not chosen", func(t *testing.T) {
		d, _, _ := show(t, false, bellui.DirectionLTR, DrawerSideEnd)
		calls := 0
		d.OnChanged = func(int) { calls++ }
		row := drawerRows(t, d)[0]
		d.Hide()
		row.Tapped(&event.PointEvent{})
		assert.Zero(t, calls)
	})

	t.Run("OnChanged hiding it too keeps the slide", func(t *testing.T) {
		d, _, _ := show(t, false, bellui.DirectionLTR, DrawerSideEnd)
		p := d.popUp
		d.OnChanged = func(int) { d.Hide() }
		drawerRows(t, d)[0].Tapped(&event.PointEvent{})
		assert.Same(t, p, d.closing, "the second Hide does not cut it short")
		require.NotNil(t, d.slide)
		d.slide.Tick(1)
		assert.Nil(t, d.closing)
	})

	t.Run("toggle while sliding out reopens", func(t *testing.T) {
		d, _, c := show(t, false, bellui.DirectionLTR, DrawerSideEnd)
		old := d.popUp
		d.Hide()
		closing := d.slide
		d.Toggle(c)
		assert.True(t, d.IsOpen())
		assert.NotSame(t, old, d.popUp)
		assert.Nil(t, d.closing)
		assert.True(t, old.isHiding, "the closing pop-up goes")
		closing.Tick(1)
		assert.NotEqual(t, c.Size().Width, old.innerPos.X, "a stopped slide out no longer moves it")
		assert.True(t, onCanvas(c, d.popUp))
	})

	t.Run("animations off: gone at once", func(t *testing.T) {
		d, drv, c := show(t, true, bellui.DirectionLTR, DrawerSideEnd)
		p := d.popUp
		d.Hide()
		assert.Nil(t, d.slide)
		assert.Nil(t, d.closing)
		assert.Empty(t, drv.started)
		assert.False(t, onCanvas(c, p))
	})
}
