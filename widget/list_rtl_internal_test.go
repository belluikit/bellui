package widget

import (
	"fmt"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The accent bar a List draws on its selected row in the modern look is at
// the row's leading edge: the left left-to-right, the right right-to-left,
// app-wide and as a window's own direction. It used to stay at the left
// (#1506).
func TestList_RTLAccentAtLeadingEdge(t *testing.T) {
	treeDirections(t, func(t *testing.T, rtl bool, show func(core.CanvasObject) bellui.Window) {
		test.ApplyTheme(t, theme.DefaultTheme()) // the modern look draws the accent
		l := NewList(func() int { return 5 }, func() core.CanvasObject { return NewLabel("item") },
			func(i ListItemID, o core.CanvasObject) { o.(*Label).SetText(fmt.Sprintf("item %d", i)) })
		w := show(l)
		l.Select(1)

		var row *listItem
		driver.WalkVisibleObjectTree(w.Canvas().Content(), func(o core.CanvasObject, _, _ geo.Position, _ geo.Size) bool {
			if li, ok := o.(*listItem); ok && li.child.(*Label).Text == "item 1" {
				row = li
			}
			return false
		}, nil)
		require.NotNil(t, row, "row 1 is on screen")
		accent := test.TempWidgetRenderer(t, row).(*listItemRenderer).accent
		require.True(t, accent.Visible(), "the selected row shows its accent")
		want := float32(0)
		if rtl {
			want = row.Size().Width - accent.Size().Width
		}
		assert.InDelta(t, want, accent.Position().X, 0.01, "the accent is at the row's leading edge")
	})
}
