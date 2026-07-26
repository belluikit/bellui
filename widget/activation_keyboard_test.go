package widget_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// activationRecord is one core.OnActivation callback: the activated object and
// whether it came from the keyboard.
type activationRecord struct {
	obj         core.CanvasObject
	viaKeyboard bool
}

// captureActivations installs a core.OnActivation hook recording every
// activation, restoring the previous hook on cleanup. It is the analytics
// autocapture seam (#700/#834) observed directly.
func captureActivations(t *testing.T) *[]activationRecord {
	t.Helper()
	prev := core.OnActivation
	var recs []activationRecord
	core.OnActivation = func(obj core.CanvasObject, viaKeyboard bool) {
		recs = append(recs, activationRecord{obj, viaKeyboard})
	}
	t.Cleanup(func() { core.OnActivation = prev })
	return &recs
}

func roleOf(obj core.CanvasObject) core.AccessibleRole {
	if acc, ok := obj.(core.Accessible); ok {
		return acc.AccessibleRole()
	}
	return core.RoleNone
}

// TestCheck_KeyboardActivationSignal verifies Check fires the central activation
// signal on a space rune (it activates via TypedRune, not TypedKey, so the
// driver's Activatable key-hook does not cover it) (#834).
func TestCheck_KeyboardActivationSignal(t *testing.T) {
	test.NewTempApp(t)
	recs := captureActivations(t)

	c := widget.NewCheck("opt", nil)
	c.TypedRune(' ')

	if assert.Len(t, *recs, 1, "space rune fires one activation") {
		assert.Equal(t, core.CanvasObject(c), (*recs)[0].obj, "reports the Check itself")
		assert.True(t, (*recs)[0].viaKeyboard, "flagged keyboard")
		assert.True(t, c.Checked, "and actually toggled")
	}

	// A disabled Check neither toggles nor signals.
	*recs = nil
	c.Disable()
	c.TypedRune(' ')
	assert.Empty(t, *recs, "disabled Check does not signal")
}

// TestSwitch_KeyboardActivation verifies the Switch is now keyboard-operable
// (Space toggles) and declares itself Activatable so the driver fires the signal
// in parity with a pointer tap (#834).
func TestSwitch_KeyboardActivation(t *testing.T) {
	test.NewTempApp(t)

	var changed []bool
	s := widget.NewSwitch(func(on bool) { changed = append(changed, on) })

	assert.True(t, s.ActivatedByKey(&event.KeyEvent{Name: event.KeySpace}),
		"space activates the switch (drives the driver signal)")
	assert.False(t, s.ActivatedByKey(&event.KeyEvent{Name: event.KeyReturn}),
		"only space activates")

	s.TypedKey(&event.KeyEvent{Name: event.KeySpace})
	assert.True(t, s.On, "space toggles the switch on")
	s.TypedKey(&event.KeyEvent{Name: event.KeySpace})
	assert.False(t, s.On, "space toggles it back off")
	assert.Equal(t, []bool{true, false}, changed, "OnChanged fired per toggle")

	// Focus is tracked (drives the focus indicator) without panicking.
	s.FocusGained()
	s.FocusLost()
}

// TestList_KeyboardActivationReportsItem verifies List keyboard activation
// reports the realized item (RoleListItem) rather than the List (RoleList), so
// autocapture's role→verb mapping matches the pointer path (#834).
func TestList_KeyboardActivationReportsItem(t *testing.T) {
	test.NewTempApp(t)
	recs := captureActivations(t)

	list := widget.NewList(
		func() int { return 20 },
		func() core.CanvasObject { return widget.NewLabel("template") },
		func(id widget.ListItemID, o core.CanvasObject) { o.(*widget.Label).SetText("row") },
	)
	w := test.NewTempWindow(t, list)
	w.Resize(geo.NewSize(200, 300))
	w.Canvas().Focus(list)

	list.TypedKey(&event.KeyEvent{Name: event.KeySpace})

	if assert.Len(t, *recs, 1, "space fires one activation") {
		assert.True(t, (*recs)[0].viaKeyboard)
		assert.Equal(t, core.RoleListItem, roleOf((*recs)[0].obj),
			"reports the item, not the container")
	}
}

// TestTree_KeyboardActivationReportsItem verifies Tree keyboard activation
// reports the realized node (RoleTreeItem) (#834).
func TestTree_KeyboardActivationReportsItem(t *testing.T) {
	test.NewTempApp(t)
	recs := captureActivations(t)

	tree := widget.NewTree(
		func(uid widget.TreeNodeID) []widget.TreeNodeID {
			if uid == "" {
				return []widget.TreeNodeID{"a", "b"}
			}
			return nil
		},
		func(uid widget.TreeNodeID) bool { return uid == "" },
		func(bool) core.CanvasObject { return widget.NewLabel("template") },
		func(uid widget.TreeNodeID, branch bool, o core.CanvasObject) {
			o.(*widget.Label).SetText(string(uid))
		},
	)
	w := test.NewTempWindow(t, tree)
	w.Resize(geo.NewSize(200, 300))
	w.Canvas().Focus(tree)
	tree.Select("a") // set the focus row to a realized leaf

	*recs = nil
	tree.TypedKey(&event.KeyEvent{Name: event.KeySpace})

	if assert.Len(t, *recs, 1, "space fires one activation") {
		assert.True(t, (*recs)[0].viaKeyboard)
		assert.Equal(t, core.RoleTreeItem, roleOf((*recs)[0].obj),
			"reports the node, not the container")
	}
}

// TestTable_KeyboardActivationReportsCell verifies Table keyboard activation
// (Space on the focused cell) reports the realized cell object (RoleTableCell),
// not the container, so autocapture captures it at cell granularity (#851).
func TestTable_KeyboardActivationReportsCell(t *testing.T) {
	test.NewTempApp(t)
	recs := captureActivations(t)

	table := widget.NewTable(
		func() (int, int) { return 5, 5 },
		func() core.CanvasObject { return widget.NewLabel("template") },
		func(id widget.TableCellID, o core.CanvasObject) { o.(*widget.Label).SetText("x") },
	)
	w := test.NewTempWindow(t, table)
	w.Resize(geo.NewSize(300, 300))
	w.Canvas().Focus(table)

	table.TypedKey(&event.KeyEvent{Name: event.KeySpace})

	if assert.Len(t, *recs, 1, "space fires one activation") {
		assert.True(t, (*recs)[0].viaKeyboard)
		assert.Equal(t, core.RoleTableCell, roleOf((*recs)[0].obj),
			"reports the cell, not the container")
	}
}

// TestTable_PointerActivationReportsCell verifies a tap on a Table cell reports
// the realized cell object (RoleTableCell), matching the keyboard path so both
// activation paths agree at cell granularity (#851).
func TestTable_PointerActivationReportsCell(t *testing.T) {
	test.NewTempApp(t)
	recs := captureActivations(t)

	table := widget.NewTable(
		func() (int, int) { return 5, 5 },
		func() core.CanvasObject { return widget.NewLabel("template") },
		func(id widget.TableCellID, o core.CanvasObject) { o.(*widget.Label).SetText("x") },
	)
	w := test.NewTempWindow(t, table)
	w.Resize(geo.NewSize(300, 300))

	table.Tapped(&event.PointEvent{Position: geo.NewPos(20, 20)})

	if assert.Len(t, *recs, 1, "tap fires one activation") {
		assert.False(t, (*recs)[0].viaKeyboard, "flagged as pointer")
		assert.Equal(t, core.RoleTableCell, roleOf((*recs)[0].obj),
			"reports the cell, not the container")
	}
}
