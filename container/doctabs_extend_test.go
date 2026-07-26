package container

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type extendedDocTabs struct {
	DocTabs
}

func newExtendedDocTabs(items ...*TabItem) *extendedDocTabs {
	ret := &extendedDocTabs{}
	ret.ExtendBaseWidget(ret)

	ret.Items = items
	return ret
}

func TestDocTabs_Extended_Tapped(t *testing.T) {
	tabs := newExtendedDocTabs(
		NewTabItem("Test1", widget.NewLabel("Test1")),
		NewTabItem("Test2", widget.NewLabel("Test2")),
	)
	tabs.Resize(geo.NewSize(150, 150)) // Ensure DocTabs is big enough to show both tab buttons
	r := test.TempWidgetRenderer(t, tabs).(*docTabsRenderer)

	buttons := r.bar.Objects[0].(*Scroll).Content.(*bellui.Container).Objects
	tab1 := buttons[0].(*tabButton)
	tab2 := buttons[1].(*tabButton)
	require.Equal(t, 0, tabs.SelectedIndex())
	require.Equal(t, theme.Color(theme.ColorNamePrimary), test.TempWidgetRenderer(t, tab1).(*tabButtonRenderer).label.Color)

	tab2.Tapped(&event.PointEvent{})
	assert.Equal(t, 1, tabs.SelectedIndex())
	require.Equal(t, theme.Color(theme.ColorNameForeground), test.TempWidgetRenderer(t, tab1).(*tabButtonRenderer).label.Color)
	require.Equal(t, theme.Color(theme.ColorNamePrimary), test.TempWidgetRenderer(t, tab2).(*tabButtonRenderer).label.Color)
	assert.False(t, tabs.Items[0].Content.Visible())
	assert.True(t, tabs.Items[1].Content.Visible())

	tab2.Tapped(&event.PointEvent{})
	assert.Equal(t, 1, tabs.SelectedIndex())
	require.Equal(t, theme.Color(theme.ColorNameForeground), test.TempWidgetRenderer(t, tab1).(*tabButtonRenderer).label.Color)
	require.Equal(t, theme.Color(theme.ColorNamePrimary), test.TempWidgetRenderer(t, tab2).(*tabButtonRenderer).label.Color)
	assert.False(t, tabs.Items[0].Content.Visible())
	assert.True(t, tabs.Items[1].Content.Visible())

	tab1.Tapped(&event.PointEvent{})
	assert.Equal(t, 0, tabs.SelectedIndex())
	require.Equal(t, theme.Color(theme.ColorNamePrimary), test.TempWidgetRenderer(t, tab1).(*tabButtonRenderer).label.Color)
	require.Equal(t, theme.Color(theme.ColorNameForeground), test.TempWidgetRenderer(t, tab2).(*tabButtonRenderer).label.Color)
	assert.True(t, tabs.Items[0].Content.Visible())
	assert.False(t, tabs.Items[1].Content.Visible())
}
