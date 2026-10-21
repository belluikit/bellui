package widget

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	internalWidget "github.com/belluikit/bellui/internal/widget"
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
