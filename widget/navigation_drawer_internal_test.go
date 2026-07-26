package widget

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
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
	padded, ok := scroll.Content.(*bellui.Container)
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
