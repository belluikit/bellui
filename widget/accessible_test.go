package widget

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// --- Button ---

func TestButton_AccessibleRole(t *testing.T) {
	b := NewButton("Click Me", nil)
	assert.Equal(t, core.RoleButton, b.AccessibleRole())
}

func TestButton_AccessibleLabel(t *testing.T) {
	b := NewButton("Submit", nil)
	assert.Equal(t, "Submit", b.AccessibleLabel())
}

func TestButton_PerformAccessibleAction(t *testing.T) {
	tapped := false
	b := NewButton("Go", func() { tapped = true })
	b.PerformAccessibleAction(core.ActionPress)
	assert.True(t, tapped)
}

func TestButton_PerformAccessibleAction_Disabled(t *testing.T) {
	tapped := false
	b := NewButton("Go", func() { tapped = true })
	b.Disable()
	b.PerformAccessibleAction(core.ActionPress)
	assert.False(t, tapped)
}

// --- Check ---

func TestCheck_AccessibleRole(t *testing.T) {
	c := NewCheck("Accept", nil)
	assert.Equal(t, core.RoleCheckbox, c.AccessibleRole())
}

func TestCheck_AccessibleLabel(t *testing.T) {
	c := NewCheck("Terms", nil)
	assert.Equal(t, "Terms", c.AccessibleLabel())
}

func TestCheck_AccessibleState(t *testing.T) {
	c := NewCheck("Accept", nil)
	assert.False(t, c.AccessibleState().Has(core.StateChecked))

	c.SetChecked(true)
	assert.True(t, c.AccessibleState().Has(core.StateChecked))

	c.Disable()
	state := c.AccessibleState()
	assert.True(t, state.Has(core.StateChecked))
	assert.True(t, state.Has(core.StateDisabled))
}

func TestCheck_AccessibleState_Partial(t *testing.T) {
	c := NewCheck("Mixed", nil)
	c.Partial = true
	assert.True(t, c.AccessibleState().Has(core.StatePartial))
}

func TestCheck_PerformAccessibleAction(t *testing.T) {
	c := NewCheck("Toggle", nil)
	c.PerformAccessibleAction(core.ActionToggle)
	assert.True(t, c.Checked)
	c.PerformAccessibleAction(core.ActionToggle)
	assert.False(t, c.Checked)
}

// --- Entry ---

func TestEntry_AccessibleRole(t *testing.T) {
	e := NewEntry()
	assert.Equal(t, core.RoleTextBox, e.AccessibleRole())

	e.Password = true
	assert.Equal(t, core.RolePasswordField, e.AccessibleRole())

	e.Password = false
	e.MultiLine = true
	assert.Equal(t, core.RoleMultiLineTextBox, e.AccessibleRole())
}

func TestEntry_AccessibleLabel(t *testing.T) {
	e := NewEntry()
	e.PlaceHolder = "Enter name"
	assert.Equal(t, "Enter name", e.AccessibleLabel())
}

func TestEntry_AccessibleValue(t *testing.T) {
	e := NewEntry()
	e.SetText("hello")
	assert.Equal(t, "hello", e.AccessibleValue())
}

func TestEntry_AccessibleValue_Password(t *testing.T) {
	e := NewPasswordEntry()
	e.SetText("secret")
	assert.Equal(t, "", e.AccessibleValue(), "password values must not be exposed")
}

func TestEntry_AccessibleActions_UndoRedo(t *testing.T) {
	e := NewEntry()
	assert.Empty(t, e.AccessibleActions(), "no history, no actions")

	e.InsertText("hello")
	assert.Equal(t, []string{core.ActionUndo}, a11yActionNames(e.AccessibleActions()))

	e.PerformAccessibleAction(core.ActionUndo)
	assert.Equal(t, "", e.Text)
	assert.Equal(t, []string{core.ActionRedo}, a11yActionNames(e.AccessibleActions()))

	e.PerformAccessibleAction(core.ActionRedo)
	assert.Equal(t, "hello", e.Text)
	assert.Equal(t, []string{core.ActionUndo}, a11yActionNames(e.AccessibleActions()))
}

func TestEntry_AccessibleActions_Gated(t *testing.T) {
	e := NewEntry()
	e.InsertText("hello")
	assert.NotEmpty(t, e.AccessibleActions())

	e.ReadOnly = true
	assert.Empty(t, e.AccessibleActions(), "read-only entries offer no undo/redo")
	e.ReadOnly = false

	e.Disable()
	assert.Empty(t, e.AccessibleActions(), "disabled entries offer no undo/redo")
	e.Enable()

	p := NewPasswordEntry()
	p.InsertText("secret")
	assert.Empty(t, p.AccessibleActions(), "password entries offer no undo/redo")
}

// Undo/redo through the accessible action cancel an in-flight IME composition
// first (#1034): replaying history under a live preedit orphans it, and a
// later IME convergence would commit the orphan at a bogus offset.
func TestEntry_PerformAccessibleAction_UndoCancelsComposition(t *testing.T) {
	e := NewEntry()
	e.InsertText("hello")
	e.Compose(&event.PreeditEvent{Text: "x", CursorPos: 1})

	e.PerformAccessibleAction(core.ActionUndo)
	assert.Equal(t, "", e.Text, "undo applied to the document")
	assert.NotContains(t, e.textProvider().String(), "x", "the preedit was cancelled, not committed")

	e.PerformAccessibleAction(core.ActionRedo)
	assert.Equal(t, "hello", e.Text, "no orphaned preedit leaked into history replay")
}

func TestRichEntry_PerformAccessibleAction_UndoCancelsComposition(t *testing.T) {
	e := NewRichEntry()
	test.Type(e, "hi")
	e.Compose(&event.PreeditEvent{Text: "x", CursorPos: 1})

	e.PerformAccessibleAction(core.ActionUndo)
	assert.Equal(t, "", e.String(), "undo applied to the document")

	e.PerformAccessibleAction(core.ActionRedo)
	assert.Equal(t, "hi", e.String(), "no orphaned preedit leaked into history replay")
}

func TestEntry_PerformAccessibleAction_UnknownIsNoOp(t *testing.T) {
	e := NewEntry()
	e.InsertText("hello")
	e.PerformAccessibleAction("sparkle")
	assert.Equal(t, "hello", e.Text)
}

func TestRichEntry_AccessibleActions_UndoRedo(t *testing.T) {
	e := NewRichEntry()
	assert.Empty(t, e.AccessibleActions(), "no history, no actions")

	test.Type(e, "hi")
	assert.Equal(t, []string{core.ActionUndo}, a11yActionNames(e.AccessibleActions()))

	e.PerformAccessibleAction(core.ActionUndo)
	assert.Equal(t, "", e.String())
	assert.Equal(t, []string{core.ActionRedo}, a11yActionNames(e.AccessibleActions()))

	e.PerformAccessibleAction(core.ActionRedo)
	assert.Equal(t, "hi", e.String())
}

func TestEntry_AccessibleState(t *testing.T) {
	e := NewEntry()
	assert.False(t, e.AccessibleState().Has(core.StateDisabled))
	assert.False(t, e.AccessibleState().Has(core.StateReadOnly))

	e.Disable()
	assert.True(t, e.AccessibleState().Has(core.StateDisabled))

	e.Enable()
	e.ReadOnly = true
	assert.True(t, e.AccessibleState().Has(core.StateReadOnly))
}

// --- Slider ---

func TestSlider_AccessibleRole(t *testing.T) {
	s := NewSlider(0, 100)
	assert.Equal(t, core.RoleSlider, s.AccessibleRole())
}

func TestSlider_AccessibleRange(t *testing.T) {
	s := NewSlider(0, 100)
	s.SetValue(42)

	min, max, cur := s.AccessibleRange()
	assert.Equal(t, 0.0, min)
	assert.Equal(t, 100.0, max)
	assert.Equal(t, 42.0, cur)
}

func TestSlider_AccessibleValue(t *testing.T) {
	s := NewSlider(0, 100)
	s.Step = 1
	s.SetValue(42)
	assert.Equal(t, "42", s.AccessibleValue())

	s.Step = 0.5
	assert.Equal(t, "42.0", s.AccessibleValue())
}

func TestSlider_PerformAccessibleAction(t *testing.T) {
	s := NewSlider(0, 10)
	s.Step = 1
	s.SetValue(5)

	s.PerformAccessibleAction(core.ActionIncrement)
	assert.Equal(t, 6.0, s.Value)

	s.PerformAccessibleAction(core.ActionDecrement)
	assert.Equal(t, 5.0, s.Value)
}

// --- RangeSlider ---

func a11yActionNames(actions []core.AccessibleAction) []string {
	names := make([]string, len(actions))
	for i, a := range actions {
		names[i] = a.Name
	}
	return names
}

// A range slider is a group of two independently adjustable thumb sliders
// (#810): each thumb is its own accessible child, so a screen reader adjusts
// either directly with no active-thumb mode switch.
func TestRangeSlider_AccessibleRole_IsGroup(t *testing.T) {
	s := NewRangeSlider(0, 100)
	assert.Equal(t, core.RoleGroup, s.AccessibleRole())
}

func TestRangeSliderThumb_AccessibleRoleAndActions(t *testing.T) {
	s := NewRangeSlider(0, 100)
	low := &rangeSliderThumb{slider: s, high: false}
	high := &rangeSliderThumb{slider: s, high: true}

	assert.Equal(t, core.RoleSlider, low.AccessibleRole())
	assert.Equal(t, core.RoleSlider, high.AccessibleRole())
	assert.NotEqual(t, low.AccessibleLabel(), high.AccessibleLabel(), "thumbs are distinguishable")

	// Each thumb offers only increment/decrement — no select/switch action.
	names := a11yActionNames(low.AccessibleActions())
	assert.Contains(t, names, core.ActionIncrement)
	assert.Contains(t, names, core.ActionDecrement)
	assert.NotContains(t, names, core.ActionSelect)
}

func TestRangeSliderThumb_AdjustIndependently(t *testing.T) {
	s := NewRangeSlider(0, 100)
	s.Step = 5
	s.SetValues(20, 80)
	low := &rangeSliderThumb{slider: s, high: false}
	high := &rangeSliderThumb{slider: s, high: true}

	// The low thumb adjusts the low value only.
	low.PerformAccessibleAction(core.ActionIncrement)
	assert.Equal(t, 25.0, s.LowValue)
	assert.Equal(t, 80.0, s.HighValue)

	// The high thumb adjusts the high value only — directly, with no mode switch.
	high.PerformAccessibleAction(core.ActionDecrement)
	assert.Equal(t, 75.0, s.HighValue)
	assert.Equal(t, 25.0, s.LowValue, "adjusting the high thumb must not move the low thumb")
}

func TestRangeSliderThumb_ClampsToOtherThumb(t *testing.T) {
	s := NewRangeSlider(0, 100)
	s.Step = 10
	s.SetValues(50, 60)
	low := &rangeSliderThumb{slider: s, high: false}

	// Pushing the low thumb up must stop at the high thumb, not cross it.
	for i := 0; i < 5; i++ {
		low.PerformAccessibleAction(core.ActionIncrement)
	}
	assert.Equal(t, 60.0, s.LowValue)
	assert.Equal(t, 60.0, s.HighValue)
}

func TestRangeSliderThumb_DefaultStep(t *testing.T) {
	s := NewRangeSlider(0, 100) // Step unset -> a11yStep is (max-min)/100 = 1
	s.SetValues(20, 80)
	(&rangeSliderThumb{slider: s, high: false}).PerformAccessibleAction(core.ActionIncrement)
	assert.Equal(t, 21.0, s.LowValue)
}

func TestRangeSliderThumb_RangeAndValue(t *testing.T) {
	s := NewRangeSlider(0, 100)
	s.SetValues(20, 80)
	low := &rangeSliderThumb{slider: s, high: false}
	high := &rangeSliderThumb{slider: s, high: true}

	min, max, cur := low.AccessibleRange()
	assert.Equal(t, 0.0, min)
	assert.Equal(t, 100.0, max)
	assert.Equal(t, 20.0, cur)
	_, _, cur = high.AccessibleRange()
	assert.Equal(t, 80.0, cur)

	assert.Equal(t, "20.0", low.AccessibleValue())
	assert.Equal(t, "80.0", high.AccessibleValue())
}

// The RangeSlider's renderer exposes exactly the two thumb proxies as accessible
// children, so the walker surfaces two adjustable sliders.
func TestRangeSlider_RendererExposesTwoThumbProxies(t *testing.T) {
	test.NewTempApp(t)
	s := NewRangeSlider(0, 100)
	r := test.TempWidgetRenderer(t, s)

	var thumbs int
	for _, o := range r.Objects() {
		if _, ok := o.(*rangeSliderThumb); ok {
			thumbs++
		}
	}
	assert.Equal(t, 2, thumbs, "renderer exposes the low and high thumb proxies")
}

// --- TimePicker ---

func TestTimePickerButton_AccessibleActions_NonValueField(t *testing.T) {
	// An AM/PM toggle has no onAdjust: only the select action.
	b := newTimePickerButton(core.RoleRadioButton, "AM", func() {})
	assert.Equal(t, []string{core.ActionSelect}, a11yActionNames(b.AccessibleActions()))
}

func TestTimePickerButton_AccessibleActions_ValueField(t *testing.T) {
	b := newTimePickerButton(core.RoleTab, "Hour", func() {})
	b.onAdjust = func(int) {}
	names := a11yActionNames(b.AccessibleActions())
	assert.Contains(t, names, core.ActionSelect)
	assert.Contains(t, names, core.ActionIncrement)
	assert.Contains(t, names, core.ActionDecrement)
}

func TestTimePickerButton_PerformAccessibleAction_Adjust(t *testing.T) {
	delta := 0
	b := newTimePickerButton(core.RoleTab, "Hour", func() {})
	b.onAdjust = func(d int) { delta += d }
	b.PerformAccessibleAction(core.ActionIncrement)
	assert.Equal(t, 1, delta)
	b.PerformAccessibleAction(core.ActionDecrement)
	assert.Equal(t, 0, delta)
}

func TestTimePicker_HeaderAdjust_ChangesTime(t *testing.T) {
	test.NewTempApp(t)
	tp := NewTimePicker(10, 30, true, nil)
	r := TempWidgetRendererTimePicker(t, tp)

	r.hourBtn.PerformAccessibleAction(core.ActionIncrement)
	assert.Equal(t, 11, tp.Hour)
	r.hourBtn.PerformAccessibleAction(core.ActionDecrement)
	assert.Equal(t, 10, tp.Hour)

	r.minuteBtn.PerformAccessibleAction(core.ActionIncrement)
	assert.Equal(t, 31, tp.Minute)
	r.minuteBtn.PerformAccessibleAction(core.ActionDecrement)
	assert.Equal(t, 30, tp.Minute)
}

func TestTimePicker_HeaderAdjust_Wraps(t *testing.T) {
	test.NewTempApp(t)
	tp := NewTimePicker(23, 0, true, nil)
	r := TempWidgetRendererTimePicker(t, tp)

	r.hourBtn.PerformAccessibleAction(core.ActionIncrement)
	assert.Equal(t, 0, tp.Hour, "hour wraps 23 -> 0")
	r.minuteBtn.PerformAccessibleAction(core.ActionDecrement)
	assert.Equal(t, 59, tp.Minute, "minute wraps 0 -> 59")
}

func TempWidgetRendererTimePicker(t *testing.T, tp *TimePicker) *timePickerRenderer {
	t.Helper()
	r, ok := test.TempWidgetRenderer(t, tp).(*timePickerRenderer)
	assert.True(t, ok)
	return r
}

// --- Label ---

func TestLabel_AccessibleRole(t *testing.T) {
	l := NewLabel("Hello")
	assert.Equal(t, core.RoleLabel, l.AccessibleRole())
}

func TestLabel_AccessibleLabel(t *testing.T) {
	l := NewLabel("Status: OK")
	assert.Equal(t, "Status: OK", l.AccessibleLabel())
}

// --- Select ---

func TestSelect_AccessibleRole(t *testing.T) {
	s := NewSelect([]string{"A", "B"}, nil)
	assert.Equal(t, core.RoleComboBox, s.AccessibleRole())
}

func TestSelect_AccessibleLabel(t *testing.T) {
	s := NewSelect([]string{"A", "B"}, nil)
	s.PlaceHolder = "Choose"
	assert.Equal(t, "Choose", s.AccessibleLabel())

	s.SetSelected("A")
	assert.Equal(t, "A", s.AccessibleLabel())
}

// --- Switch ---

func TestSwitch_AccessibleRole(t *testing.T) {
	s := NewSwitch(nil)
	assert.Equal(t, core.RoleSwitch, s.AccessibleRole())
}

func TestSwitch_AccessibleState(t *testing.T) {
	s := NewSwitch(nil)
	assert.False(t, s.AccessibleState().Has(core.StateChecked))

	s.On = true
	assert.True(t, s.AccessibleState().Has(core.StateChecked))
}

func TestSwitch_PerformAccessibleAction(t *testing.T) {
	s := NewSwitch(nil)
	s.PerformAccessibleAction(core.ActionToggle)
	assert.True(t, s.On)
}

// --- ProgressBar ---

func TestProgressBar_AccessibleRole(t *testing.T) {
	p := NewProgressBar()
	assert.Equal(t, core.RoleProgressBar, p.AccessibleRole())
}

func TestProgressBar_AccessibleValue(t *testing.T) {
	p := NewProgressBar()
	p.SetValue(0.5)
	assert.Equal(t, "50%", p.AccessibleValue())
}

func TestProgressBar_AccessibleRange(t *testing.T) {
	p := NewProgressBar()
	p.SetValue(0.75)
	min, max, cur := p.AccessibleRange()
	assert.Equal(t, 0.0, min)
	assert.Equal(t, 1.0, max)
	assert.Equal(t, 0.75, cur)
}

// --- SpinBox ---

func TestSpinBox_AccessibleRole(t *testing.T) {
	s := NewSpinBox(0, 10, 1)
	assert.Equal(t, core.RoleSpinButton, s.AccessibleRole())
}

func TestSpinBox_AccessibleRange(t *testing.T) {
	s := NewSpinBox(0, 100, 5)
	s.SetValue(25)
	min, max, cur := s.AccessibleRange()
	assert.Equal(t, 0.0, min)
	assert.Equal(t, 100.0, max)
	assert.Equal(t, 25.0, cur)
}

func TestSpinBox_PerformAccessibleAction(t *testing.T) {
	s := NewSpinBox(0, 10, 1)
	s.SetValue(5)
	s.PerformAccessibleAction(core.ActionIncrement)
	assert.Equal(t, 6.0, s.Value)
	s.PerformAccessibleAction(core.ActionDecrement)
	assert.Equal(t, 5.0, s.Value)
}

// --- Hyperlink ---

func TestHyperlink_AccessibleRole(t *testing.T) {
	h := NewHyperlink("Bellui", nil)
	assert.Equal(t, core.RoleLink, h.AccessibleRole())
}

func TestHyperlink_AccessibleLabel(t *testing.T) {
	h := NewHyperlink("Go to Bellui", nil)
	assert.Equal(t, "Go to Bellui", h.AccessibleLabel())
}

// --- RadioGroup ---

func TestRadioGroup_AccessibleRole(t *testing.T) {
	r := NewRadioGroup([]string{"A", "B"}, nil)
	assert.Equal(t, core.RoleRadioGroup, r.AccessibleRole())
}

// --- Accordion ---

func TestAccordion_AccessibleRole(t *testing.T) {
	a := NewAccordion()
	assert.Equal(t, core.RoleAccordion, a.AccessibleRole())
}

// --- Form ---

func TestForm_AccessibleRole(t *testing.T) {
	f := &Form{}
	assert.Equal(t, core.RoleForm, f.AccessibleRole())
}

// --- Card ---

func TestCard_AccessibleRole(t *testing.T) {
	c := NewCard("Title", "Subtitle", nil)
	assert.Equal(t, core.RoleCard, c.AccessibleRole())
}

func TestCard_AccessibleLabel(t *testing.T) {
	c := NewCard("My Card", "", nil)
	assert.Equal(t, "My Card", c.AccessibleLabel())
}

// --- Toolbar ---

func TestToolbar_AccessibleRole(t *testing.T) {
	tb := NewToolbar()
	assert.Equal(t, core.RoleToolbar, tb.AccessibleRole())
}

// --- List ---

func TestList_AccessibleRole(t *testing.T) {
	l := NewList(func() int { return 0 }, func() core.CanvasObject { return nil }, func(ListItemID, core.CanvasObject) {})
	assert.Equal(t, core.RoleList, l.AccessibleRole())
}

// --- Table ---

func TestTable_AccessibleRole(t *testing.T) {
	tb := NewTable(func() (int, int) { return 0, 0 }, func() core.CanvasObject { return nil }, func(TableCellID, core.CanvasObject) {})
	assert.Equal(t, core.RoleTable, tb.AccessibleRole())
}

func TestTable_AccessibleGridDimensions(t *testing.T) {
	tb := NewTable(func() (int, int) { return 40, 7 }, func() core.CanvasObject { return NewLabel("cell") }, func(TableCellID, core.CanvasObject) {})
	rows, cols := tb.AccessibleGridDimensions()
	assert.Equal(t, 40, rows)
	assert.Equal(t, 7, cols)

	// The dimensions reach the semantic node so backends can report the
	// full extent of a virtualized table (UIA Grid pattern, #487).
	node, ok := core.SnapshotAccessible(tb)
	assert.True(t, ok)
	assert.True(t, node.HasGridDims)
	assert.Equal(t, 40, node.GridRows)
	assert.Equal(t, 7, node.GridCols)

	tb.Length = nil
	rows, cols = tb.AccessibleGridDimensions()
	assert.Zero(t, rows)
	assert.Zero(t, cols)
}

// --- Tree ---

func TestTree_AccessibleRole(t *testing.T) {
	tr := NewTree(nil, nil, func(bool) core.CanvasObject { return nil }, nil)
	assert.Equal(t, core.RoleTree, tr.AccessibleRole())
}

// --- Interface compliance ---

// --- Label override (SetAccessibilityLabel) ---

func TestBaseWidget_SetAccessibilityLabel(t *testing.T) {
	b := NewIconButton(nil, nil)
	assert.Equal(t, "", b.AccessibleLabel())

	b.SetAccessibilityLabel("Delete")
	node, ok := core.SnapshotAccessible(b)
	assert.True(t, ok)
	assert.Equal(t, "Delete", node.Label, "override should win over the empty intrinsic label")

	withText := NewButton("Submit", nil)
	withText.SetAccessibilityLabel("Send the form")
	node, _ = core.SnapshotAccessible(withText)
	assert.Equal(t, "Send the form", node.Label, "override should win over intrinsic text too")

	withText.SetAccessibilityLabel("")
	node, _ = core.SnapshotAccessible(withText)
	assert.Equal(t, "Submit", node.Label, "clearing the override restores the intrinsic label")
}

// --- radioItem ---

func TestRadioItem_Accessible(t *testing.T) {
	item := newRadioItem("A", nil)
	item.Selected = true
	assert.Equal(t, core.RoleRadioButton, item.AccessibleRole())
	assert.Equal(t, "A", item.AccessibleLabel())
	assert.True(t, item.AccessibleState().Has(core.StateChecked))

	item.Disable()
	assert.True(t, item.AccessibleState().Has(core.StateDisabled))
}

func TestRadioItem_PerformAccessibleAction(t *testing.T) {
	var toggled *radioItem
	item := newRadioItem("A", func(i *radioItem) { toggled = i })
	item.PerformAccessibleAction(core.ActionSelect)
	assert.Equal(t, item, toggled)

	toggled = nil
	item.Disable()
	item.PerformAccessibleAction(core.ActionSelect)
	assert.Nil(t, toggled)
}

// --- listItem / gridWrapItem ---

func TestListItem_Accessible(t *testing.T) {
	tapped := false
	li := newListItem(NewLabel("row"), func() { tapped = true })
	assert.Equal(t, core.RoleListItem, li.AccessibleRole())
	assert.Equal(t, core.AccessibleState(0), li.AccessibleState())

	li.PerformAccessibleAction(core.ActionSelect)
	assert.True(t, tapped)
	assert.True(t, li.AccessibleState().Has(core.StateSelected))
}

func TestGridWrapItem_Accessible(t *testing.T) {
	tapped := false
	gw := newGridWrapItem(NewLabel("cell"), func() { tapped = true })
	assert.Equal(t, core.RoleListItem, gw.AccessibleRole())

	gw.PerformAccessibleAction(core.ActionSelect)
	assert.True(t, tapped)
	assert.True(t, gw.AccessibleState().Has(core.StateSelected))
}

func TestGridWrap_AccessibleRole(t *testing.T) {
	g := NewGridWrap(func() int { return 0 }, nil, nil)
	assert.Equal(t, core.RoleList, g.AccessibleRole())
}

// --- segmentWidget ---

func TestSegmentWidget_Accessible(t *testing.T) {
	tapped := false
	seg := &segmentWidget{text: "Day", selected: true, onTap: func() { tapped = true }}
	assert.Equal(t, core.RoleTab, seg.AccessibleRole())
	assert.Equal(t, "Day", seg.AccessibleLabel())
	assert.True(t, seg.AccessibleState().Has(core.StateSelected))

	seg.PerformAccessibleAction(core.ActionSelect)
	assert.True(t, tapped)
}

// --- menuItem ---

func TestMenuItem_Accessible(t *testing.T) {
	triggered := false
	belluiItem := bellui.NewMenuItem("Copy", func() { triggered = true })
	belluiItem.Checked = true
	parent := NewMenu(bellui.NewMenu("", belluiItem))
	item := parent.Items[0].(*menuItem)

	assert.Equal(t, core.RoleMenuItem, item.AccessibleRole())
	assert.Equal(t, "Copy", item.AccessibleLabel())
	assert.True(t, item.AccessibleState().Has(core.StateChecked))

	item.PerformAccessibleAction(core.ActionPress)
	assert.True(t, triggered)

	belluiItem.Disabled = true
	triggered = false
	assert.True(t, item.AccessibleState().Has(core.StateDisabled))
	item.PerformAccessibleAction(core.ActionPress)
	assert.False(t, triggered)
}

// --- navigation items ---

func TestNavBarItem_Accessible(t *testing.T) {
	tapped := false
	w := newNavBarItem(&NavigationBarItem{Label: "Home"}, true, func() { tapped = true })
	assert.Equal(t, core.RoleTab, w.AccessibleRole())
	assert.Equal(t, "Home", w.AccessibleLabel())
	assert.True(t, w.AccessibleState().Has(core.StateSelected))

	w.PerformAccessibleAction(core.ActionSelect)
	assert.True(t, tapped)
}

// --- PopUp ---

func TestPopUp_Accessible(t *testing.T) {
	c := test.Canvas()

	modal := NewModalPopUp(NewLabel("dialog body"), c)
	assert.Equal(t, core.RoleDialog, modal.AccessibleRole())
	assert.True(t, modal.AccessibleState().Has(core.StateModal))
	assert.Empty(t, modal.AccessibleActions(), "plain modal pop-ups are not dismissable from outside")

	plain := NewPopUp(NewLabel("popup body"), c)
	assert.Equal(t, core.RoleGroup, plain.AccessibleRole())
	assert.False(t, plain.AccessibleState().Has(core.StateModal))
	assert.Len(t, plain.AccessibleActions(), 1)

	plain.Show()
	plain.PerformAccessibleAction(core.ActionDismiss)
	assert.False(t, plain.Visible())
}

// --- pickers ---

func TestDatePicker_Accessible(t *testing.T) {
	d := NewDatePicker(time.Date(2026, 7, 4, 0, 0, 0, 0, time.UTC), nil)
	assert.Equal(t, core.RoleGroup, d.AccessibleRole())
	assert.NotEmpty(t, d.AccessibleLabel())
	assert.NotEmpty(t, d.AccessibleValue())

	min, max, cur := d.AccessibleRange()
	assert.Zero(t, min)
	assert.Zero(t, max)
	assert.Zero(t, cur)

	node, ok := core.SnapshotAccessible(d)
	assert.True(t, ok)
	assert.False(t, node.HasRange, "value-only widgets must not report a numeric range")
}

func TestTimePicker_Accessible(t *testing.T) {
	p := NewTimePicker(14, 30, true, nil)
	assert.Equal(t, core.RoleGroup, p.AccessibleRole())
	assert.NotEmpty(t, p.AccessibleValue())
}

func TestDateTimePicker_Accessible(t *testing.T) {
	p := NewDateTimePicker(time.Date(2026, 7, 4, 14, 30, 0, 0, time.UTC), true, nil)
	assert.Equal(t, core.RoleGroup, p.AccessibleRole())
	assert.NotEmpty(t, p.AccessibleValue())
}

// --- Table cells / Tree nodes (#524) ---

func TestTableCellHost_Accessible(t *testing.T) {
	test.NewTempApp(t)
	table := NewTable(
		func() (int, int) { return 5, 5 },
		func() core.CanvasObject { return NewLabel("cell") },
		func(TableCellID, core.CanvasObject) {},
	)
	table.Resize(geo.NewSize(200, 200))

	host := newTableCellHost(table, NewLabel("cell"))
	host.id = TableCellID{Row: 2, Col: 3}

	assert.Equal(t, core.RoleTableCell, host.AccessibleRole())
	row, col := host.AccessibleCellPosition()
	assert.Equal(t, 2, row)
	assert.Equal(t, 3, col)
	assert.False(t, host.AccessibleState().Has(core.StateSelected))

	host.PerformAccessibleAction(core.ActionSelect)
	assert.Equal(t, &TableCellID{Row: 2, Col: 3}, table.selectedCell)
	assert.True(t, host.AccessibleState().Has(core.StateSelected))

	node, ok := core.SnapshotAccessible(host)
	assert.True(t, ok)
	assert.True(t, node.HasCellPosition)
	assert.Equal(t, 2, node.CellRow)
	assert.Equal(t, 3, node.CellCol)
}

func TestTreeNode_Accessible(t *testing.T) {
	test.NewTempApp(t)
	tree := NewTreeWithStrings(map[string][]string{
		"":  {"a"},
		"a": {"b"},
	})
	tree.Resize(geo.NewSize(200, 200))

	branchNode := &treeNode{tree: tree, uid: "a", depth: 0, isBranch: true}
	assert.Equal(t, core.RoleTreeItem, branchNode.AccessibleRole())
	assert.Equal(t, 1, branchNode.AccessibleLevel())
	assert.False(t, branchNode.AccessibleState().Has(core.StateExpanded))

	branchNode.PerformAccessibleAction(core.ActionExpand)
	assert.True(t, tree.IsBranchOpen("a"))
	assert.True(t, branchNode.AccessibleState().Has(core.StateExpanded))
	// The expand/collapse action offered tracks the open state.
	names := []string{}
	for _, a := range branchNode.AccessibleActions() {
		names = append(names, a.Name)
	}
	assert.Contains(t, names, core.ActionCollapse)
	assert.NotContains(t, names, core.ActionExpand)

	branchNode.PerformAccessibleAction(core.ActionCollapse)
	assert.False(t, tree.IsBranchOpen("a"))

	leafNode := &treeNode{tree: tree, uid: "b", depth: 1, isBranch: false}
	assert.Equal(t, 2, leafNode.AccessibleLevel())
	leafNode.PerformAccessibleAction(core.ActionExpand) // no-op on leaves
	assert.False(t, tree.IsBranchOpen("b"))

	leafNode.PerformAccessibleAction(core.ActionSelect)
	assert.True(t, leafNode.AccessibleState().Has(core.StateSelected))

	node, ok := core.SnapshotAccessible(leafNode)
	assert.True(t, ok)
	assert.Equal(t, 2, node.Level)
}

// --- passwordRevealer (#531) ---

func TestPasswordRevealer_Accessible(t *testing.T) {
	test.NewTempApp(t)
	entry := NewPasswordEntry()
	entry.SetText("secret")
	pr := entry.ActionItem.(*passwordRevealer)

	assert.Equal(t, core.RoleButton, pr.AccessibleRole())
	assert.Equal(t, "Show password", pr.AccessibleLabel())

	pr.PerformAccessibleAction(core.ActionPress)
	assert.False(t, entry.Password, "press must reveal the password")
	assert.Equal(t, "Hide password", pr.AccessibleLabel())

	entry.Disable()
	assert.True(t, pr.AccessibleState().Has(core.StateDisabled))
	pr.PerformAccessibleAction(core.ActionPress)
	assert.False(t, entry.Password, "press must stay inert while disabled")
}

// --- TreeView rows (#532) ---

func TestTreeViewItem_Accessible(t *testing.T) {
	test.NewTempApp(t)
	tv := NewTreeView()
	tv.SetChildren("", []TreeViewNode{
		{ID: "a", Text: "Branch A"},
		{ID: "leaf", Text: "Leaf"},
	})
	tv.SetChildren("a", []TreeViewNode{{ID: "a1", Text: "Child"}})
	tv.Resize(geo.NewSize(240, 240))

	branch := newTreeViewItem(tv, NewLabel(""), true)
	branch.update("a", 0, true)

	assert.Equal(t, core.RoleTreeItem, branch.AccessibleRole())
	assert.Equal(t, "Branch A", branch.AccessibleLabel())
	assert.Equal(t, 1, branch.AccessibleLevel())
	assert.False(t, branch.AccessibleState().Has(core.StateExpanded))

	branch.PerformAccessibleAction(core.ActionExpand)
	assert.True(t, tv.IsBranchOpen("a"))
	assert.True(t, branch.AccessibleState().Has(core.StateExpanded))
	names := []string{}
	for _, a := range branch.AccessibleActions() {
		names = append(names, a.Name)
	}
	assert.Contains(t, names, core.ActionCollapse)
	assert.NotContains(t, names, core.ActionExpand)

	branch.PerformAccessibleAction(core.ActionCollapse)
	assert.False(t, tv.IsBranchOpen("a"))

	leaf := newTreeViewItem(tv, NewLabel(""), false)
	leaf.update("a1", 1, false)
	assert.Equal(t, 2, leaf.AccessibleLevel())
	assert.Equal(t, "Child", leaf.AccessibleLabel())

	leaf.PerformAccessibleAction(core.ActionSelect)
	assert.Equal(t, TreeViewNodeID("a1"), tv.Selected())
	assert.True(t, leaf.AccessibleState().Has(core.StateSelected))

	node, ok := core.SnapshotAccessible(leaf)
	assert.True(t, ok)
	assert.Equal(t, 2, node.Level)
}

func TestTreeViewItem_AccessibleLabel_CustomContent(t *testing.T) {
	test.NewTempApp(t)
	tv := NewTreeView()
	tv.CreateNode = func(branch bool) core.CanvasObject { return NewLabel("custom") }
	tv.SetChildren("", []TreeViewNode{{ID: "a", Text: "Node A"}})

	item := newTreeViewItem(tv, NewLabel("custom"), false)
	item.update("a", 0, false)
	assert.Equal(t, "", item.AccessibleLabel(),
		"custom CreateNode content provides its own semantics as child nodes")
}

// --- misc containers ---

func TestStatusBar_AccessibleRole(t *testing.T) {
	assert.Equal(t, core.RoleToolbar, NewStatusBar().AccessibleRole())
}

func TestCarousel_AccessibleRole(t *testing.T) {
	assert.Equal(t, core.RoleGroup, NewCarousel().AccessibleRole())
}

func TestInterfaceCompliance(t *testing.T) {
	// Verify compile-time interface checks for key widgets.
	var _ core.Accessible = (*Button)(nil)
	var _ core.Accessible = (*Entry)(nil)
	var _ core.Accessible = (*Check)(nil)
	var _ core.Accessible = (*Slider)(nil)
	var _ core.Accessible = (*Label)(nil)
	var _ core.Accessible = (*Select)(nil)
	var _ core.Accessible = (*Switch)(nil)
	var _ core.Accessible = (*Hyperlink)(nil)
	var _ core.Accessible = (*ProgressBar)(nil)
	var _ core.Accessible = (*SpinBox)(nil)
	var _ core.Accessible = (*RadioGroup)(nil)
	var _ core.Accessible = (*Accordion)(nil)
	var _ core.Accessible = (*Form)(nil)
	var _ core.Accessible = (*Card)(nil)
	var _ core.Accessible = (*Toolbar)(nil)
	var _ core.Accessible = (*List)(nil)
	var _ core.Accessible = (*Table)(nil)
	var _ core.Accessible = (*Tree)(nil)
	var _ core.Accessible = (*Menu)(nil)
	var _ core.Accessible = (*NavigationBar)(nil)
	var _ core.Accessible = (*NavigationRail)(nil)
	var _ core.Accessible = (*TopAppBar)(nil)
	var _ core.Accessible = (*BottomAppBar)(nil)
	var _ core.Accessible = (*FAB)(nil)
	var _ core.Accessible = (*IconButton)(nil)
	var _ core.Accessible = (*Chip)(nil)
	var _ core.Accessible = (*Badge)(nil)
	var _ core.Accessible = (*Toast)(nil)
	var _ core.Accessible = (*SegmentedButton)(nil)
	var _ core.Accessible = (*SplitButton)(nil)
	var _ core.Accessible = (*Tooltip)(nil)
	var _ core.Accessible = (*RichText)(nil)
	var _ core.Accessible = (*RangeSlider)(nil)
	var _ core.Accessible = (*PopUpMenu)(nil)
	var _ core.Accessible = (*SearchField)(nil)
	var _ core.Accessible = (*TreeView)(nil)
	var _ core.Accessible = (*PopUp)(nil)
	var _ core.Accessible = (*GridWrap)(nil)
	var _ core.Accessible = (*DatePicker)(nil)
	var _ core.Accessible = (*TimePicker)(nil)
	var _ core.Accessible = (*DateTimePicker)(nil)
	var _ core.Accessible = (*Carousel)(nil)
	var _ core.Accessible = (*StatusBar)(nil)
	var _ core.Accessible = (*ScrollIndicator)(nil)
	var _ core.Accessible = (*radioItem)(nil)
	var _ core.Accessible = (*listItem)(nil)
	var _ core.Accessible = (*gridWrapItem)(nil)
	var _ core.Accessible = (*segmentWidget)(nil)
	var _ core.Accessible = (*menuItem)(nil)
	var _ core.Accessible = (*navBarItemWidget)(nil)
	var _ core.Accessible = (*navRailItemWidget)(nil)
	var _ core.Accessible = (*drawerItemWidget)(nil)

	var _ core.AccessibleLabelOverrider = (*BaseWidget)(nil)

	var _ core.Accessible = (*tableCellHost)(nil)
	var _ core.AccessibleGridItem = (*tableCellHost)(nil)
	var _ core.Accessible = (*treeNode)(nil)
	var _ core.AccessibleLeveler = (*treeNode)(nil)
	var _ core.AccessibleStateful = (*treeNode)(nil)
	var _ core.AccessibleActioner = (*treeNode)(nil)
	var _ core.Accessible = (*treeViewItem)(nil)
	var _ core.AccessibleLeveler = (*treeViewItem)(nil)
	var _ core.AccessibleStateful = (*treeViewItem)(nil)
	var _ core.AccessibleActioner = (*treeViewItem)(nil)

	// Verify optional interfaces
	var _ core.AccessibleActioner = (*Button)(nil)
	var _ core.AccessibleActioner = (*Check)(nil)
	var _ core.AccessibleActioner = (*Slider)(nil)
	var _ core.AccessibleActioner = (*Switch)(nil)
	var _ core.AccessibleActioner = (*Hyperlink)(nil)
	var _ core.AccessibleActioner = (*SpinBox)(nil)
	var _ core.AccessibleActioner = (*FAB)(nil)
	var _ core.AccessibleActioner = (*IconButton)(nil)
	var _ core.AccessibleActioner = (*SplitButton)(nil)

	var _ core.AccessibleValuer = (*Slider)(nil)
	var _ core.AccessibleValuer = (*SpinBox)(nil)
	var _ core.AccessibleValuer = (*ProgressBar)(nil)
	var _ core.AccessibleValuer = (*Entry)(nil)
	// A RangeSlider is a group; its two thumb proxies are the adjustable sliders.
	var _ core.AccessibleValuer = (*rangeSliderThumb)(nil)
	var _ core.AccessibleActioner = (*rangeSliderThumb)(nil)

	var _ core.AccessibleStateful = (*Check)(nil)
	var _ core.AccessibleStateful = (*Switch)(nil)
	var _ core.AccessibleStateful = (*Entry)(nil)
	var _ core.AccessibleStateful = (*Select)(nil)
}
