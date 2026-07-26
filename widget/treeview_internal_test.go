//go:build !mobile

// Hover is desktop pointer semantics; the mobile driver delivers
// long-press instead, so these tests only apply to desktop builds.

package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/test"
)

// Hovering anywhere on a row — including directly over the item's label text —
// must mark the row hovered and show the rename/delete action buttons.
// Regression test: Label used to implement event.Hoverable unconditionally,
// swallowing hover events whenever the pointer was over the item name.
func TestTreeViewItem_HoverOverLabel_ShowsActions(t *testing.T) {
	test.NewTempApp(t)

	tv := NewTreeViewWithNodes([]TreeViewNode{
		{ID: "1", Text: "One"},
		{ID: "2", Text: "Two"},
	})
	tv.AllowRename = true
	tv.AllowDelete = true
	w := test.NewTempWindow(t, tv)
	w.Resize(geo.NewSize(300, 200))

	var item *treeViewItem
	driver.WalkVisibleObjectTree(w.Canvas().Content(), func(o core.CanvasObject, _, _ geo.Position, _ geo.Size) bool {
		if ti, ok := o.(*treeViewItem); ok && item == nil {
			item = ti
		}
		return false
	}, nil)
	require.NotNil(t, item, "no treeViewItem rendered")

	r := test.TempWidgetRenderer(t, item).(*treeViewItemRenderer)
	label, ok := r.content.(*Label)
	require.True(t, ok, "default tree item content should be a Label")

	// Hover directly over the centre of the label text.
	labelPos := bellui.CurrentApp().Driver().AbsolutePositionForObject(label)
	labelCenter := labelPos.Add(geo.NewPos(label.Size().Width/2, label.Size().Height/2))
	test.MoveMouse(w.Canvas(), labelCenter)

	assert.True(t, item.hovered, "row should be hovered when pointer is over the item label")
	assert.True(t, r.renameBtn.Visible(), "rename action should show on hover")
	assert.True(t, r.deleteBtn.Visible(), "delete action should show on hover")

	// Moving onto the just-revealed delete button must keep the row hovered
	// and the buttons visible — otherwise the buttons hide under the pointer
	// and can never be clicked.
	btnPos := bellui.CurrentApp().Driver().AbsolutePositionForObject(r.deleteBtn)
	btnCenter := btnPos.Add(geo.NewPos(r.deleteBtn.Size().Width/2, r.deleteBtn.Size().Height/2))
	test.MoveMouse(w.Canvas(), btnCenter)

	assert.True(t, item.hovered, "row should stay hovered while pointer is over an action button")
	assert.True(t, r.deleteBtn.Visible(), "delete action should stay visible while pointer is over it")

	// Moving off the row hides the actions again.
	test.MoveMouse(w.Canvas(), geo.NewPos(1, 190))
	assert.False(t, item.hovered, "row should not stay hovered after pointer leaves")
	assert.False(t, r.renameBtn.Visible(), "rename action should hide after hover out")
	assert.False(t, r.deleteBtn.Visible(), "delete action should hide after hover out")
}

// Rename/delete are hover-only buttons on the row, so they are also offered
// as accessible actions when the tree allows them (#544) — otherwise
// screen-reader and keyboard users could never reach them.
func TestTreeViewItem_RenameDeleteAccessibleActions(t *testing.T) {
	test.NewTempApp(t)
	tv := NewTreeView()
	tv.SetChildren("", []TreeViewNode{
		{ID: "a", Text: "Alpha"},
		{ID: "b", Text: "Beta"},
	})
	tv.Resize(geo.NewSize(240, 240))

	item := newTreeViewItem(tv, NewLabel(""), false)
	item.update("a", 0, false)

	names := func() []string {
		var out []string
		for _, a := range item.AccessibleActions() {
			out = append(out, a.Name)
		}
		return out
	}

	// Disabled by default: no rename/delete offered, performing is inert.
	assert.NotContains(t, names(), core.ActionRename)
	assert.NotContains(t, names(), core.ActionDelete)
	item.PerformAccessibleAction(core.ActionDelete)
	_, exists := tv.GetNode("a")
	assert.True(t, exists)
	item.PerformAccessibleAction(core.ActionRename)
	assert.False(t, item.renaming)

	tv.AllowRename = true
	tv.AllowDelete = true
	assert.Contains(t, names(), core.ActionRename)
	assert.Contains(t, names(), core.ActionDelete)

	item.PerformAccessibleAction(core.ActionRename)
	assert.True(t, item.renaming, "rename action should start inline rename")

	other := newTreeViewItem(tv, NewLabel(""), false)
	other.update("b", 0, false)
	other.PerformAccessibleAction(core.ActionDelete)
	_, exists = tv.GetNode("b")
	assert.False(t, exists, "delete action should remove the node")
}
