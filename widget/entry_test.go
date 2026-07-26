package widget_test

import (
	"fmt"
	"image"
	"image/color"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/data/binding"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

func TestEntry_Binding(t *testing.T) {
	entry := widget.NewEntry()
	entry.SetText("Init")
	assert.Equal(t, "Init", entry.Text)
	waitForBinding() // this time it is the de-echo before binding

	str := binding.NewString()
	entry.Bind(str)
	waitForBinding()
	assert.Equal(t, "", entry.Text)

	err := str.Set("Updated")
	assert.NoError(t, err)
	waitForBinding()
	assert.Equal(t, "Updated", entry.Text)

	entry.SetText("Typed")
	v, err := str.Get()
	assert.NoError(t, err)
	assert.Equal(t, "Typed", v)

	entry.Unbind()
	waitForBinding()
	assert.Equal(t, "Typed", entry.Text)
}

func TestEntry_Binding_Bounce(t *testing.T) {
	entry := widget.NewEntry()
	entry.SetText("Init")
	assert.Equal(t, "Init", entry.Text)
	waitForBinding() // this time it is the de-echo before binding

	str := binding.NewString()
	entry.Bind(str)
	str.Set("1")
	time.Sleep(10 * time.Millisecond)
	str.Set("2")
	waitForBinding()
	assert.Equal(t, "2", entry.Text)
}

func TestEntry_Binding_Replace(t *testing.T) {
	entry := widget.NewEntry()
	str := binding.NewString()
	_ = str.Set("Content")
	entry.Bind(str)
	waitForBinding()
	assert.Equal(t, "Content", entry.Text)

	entry.FocusGained()
	// Clear select-all triggered by FocusGained
	entry.TypedKey(&event.KeyEvent{Name: event.KeyLeft})
	typeKeys(entry, event.KeyRight, event.KeyRight, keyShiftLeftDown, event.KeyRight, event.KeyRight, keyShiftLeftUp)
	assert.Equal(t, "nt", entry.SelectedText())

	typeOnFocused(entry, "g")
	waitForBinding()
	assert.Equal(t, "Cogent", entry.Text)
}

func TestEntry_Clicked(t *testing.T) {
	entry, window := setupImageTest(t, true)
	c := window.Canvas()

	entry.SetText("MMM\nWWW\n")
	test.AssertRendersToMarkup(t, "entry/tapped_initial.xml", c)

	entry.FocusGained()
	test.AssertRendersToMarkup(t, "entry/tapped_focused.xml", c)

	testCharSize := theme.TextSize()
	pos := geo.NewPos(entryOffset+theme.Padding()+testCharSize*1.5, entryOffset+theme.Padding()+testCharSize/2) // tap in the middle of the 2nd "M"
	clickCanvas(window.Canvas(), pos)
	test.AssertRendersToMarkup(t, "entry/tapped_tapped_2nd_m.xml", c)
	assert.Equal(t, 0, entry.CursorRow)
	assert.Equal(t, 1, entry.CursorColumn)

	pos = geo.NewPos(entryOffset+theme.Padding()+testCharSize*2.5, entryOffset+theme.Padding()+testCharSize/2) // tap near the 3rd "M"
	clickCanvas(window.Canvas(), pos)
	test.AssertRendersToMarkup(t, "entry/tapped_tapped_3rd_m.xml", c)
	assert.Equal(t, 0, entry.CursorRow)
	assert.Equal(t, 3, entry.CursorColumn)

	pos = geo.NewPos(entryOffset+theme.Padding()+testCharSize*4, entryOffset+theme.Padding()+testCharSize/2) // tap after text
	clickCanvas(window.Canvas(), pos)
	test.AssertRendersToMarkup(t, "entry/tapped_tapped_after_last_col.xml", c)
	assert.Equal(t, 0, entry.CursorRow)
	assert.Equal(t, 3, entry.CursorColumn)

	pos = geo.NewPos(entryOffset+testCharSize, entryOffset+testCharSize*4) // tap below rows
	clickCanvas(window.Canvas(), pos)
	test.AssertRendersToMarkup(t, "entry/tapped_tapped_after_last_row.xml", c)
	assert.Equal(t, 2, entry.CursorRow)
	assert.Equal(t, 0, entry.CursorColumn)
}

func TestEntry_CursorColumn(t *testing.T) {
	entry := widget.NewEntry()
	entry.SetText("")
	assert.Equal(t, 0, entry.CursorColumn)

	// only 0 columns, do nothing
	right := &event.KeyEvent{Name: event.KeyRight}
	entry.TypedKey(right)
	assert.Equal(t, 0, entry.CursorColumn)

	// 1, this should increment
	entry.SetText("a")
	entry.TypedKey(right)
	assert.Equal(t, 1, entry.CursorColumn)

	left := &event.KeyEvent{Name: event.KeyLeft}
	entry.TypedKey(left)
	assert.Equal(t, 0, entry.CursorColumn)

	// don't go beyond left
	entry.TypedKey(left)
	assert.Equal(t, 0, entry.CursorColumn)
}

func TestEntry_CursorColumn_Ends(t *testing.T) {
	entry := widget.NewEntry()
	entry.SetText("Hello")
	assert.Equal(t, 0, entry.CursorColumn)

	// down should go to end for last line
	down := &event.KeyEvent{Name: event.KeyDown}
	entry.TypedKey(down)
	assert.Equal(t, 5, entry.CursorColumn)
	assert.Equal(t, 0, entry.CursorRow)

	// up should go to start for first line
	up := &event.KeyEvent{Name: event.KeyUp}
	entry.TypedKey(up)
	assert.Equal(t, 0, entry.CursorColumn)
	assert.Equal(t, 0, entry.CursorRow)
}

func TestEntry_CursorColumn_Jump(t *testing.T) {
	entry := widget.NewMultiLineEntry()
	entry.SetText("a\nbc")

	// go to end of text
	right := &event.KeyEvent{Name: event.KeyRight}
	entry.TypedKey(right)
	entry.TypedKey(right)
	entry.TypedKey(right)
	entry.TypedKey(right)
	assert.Equal(t, 1, entry.CursorRow)
	assert.Equal(t, 2, entry.CursorColumn)

	// go up, to a shorter line
	up := &event.KeyEvent{Name: event.KeyUp}
	entry.TypedKey(up)
	assert.Equal(t, 0, entry.CursorRow)
	assert.Equal(t, 1, entry.CursorColumn)
}

func TestEntry_CursorPosition(t *testing.T) {
	entry := widget.NewEntry()
	entry.TextStyle.Monospace = true
	entry.SetText("mmmmmm")

	right := &event.KeyEvent{Name: event.KeyRight}
	entry.TypedKey(right)
	firstX, firstY := entry.CursorPosition().Components()

	entry.TypedKey(right)
	secondX := entry.CursorPosition().X
	assert.Greater(t, secondX, firstX)

	entry.TypedKey(right)
	entry.TypedKey(right)
	fourthX := entry.CursorPosition().X
	assert.Equal(t, (secondX-firstX)*3, fourthX-firstX)

	entry.SetText("mmmmmm\nmm\nmm")
	entry.CursorRow = 1
	secondY := entry.CursorPosition().Y

	entry.CursorRow = 2
	thirdY := entry.CursorPosition().Y
	assert.Equal(t, secondY-firstY, thirdY-secondY)
}

func TestEntry_Alignment(t *testing.T) {
	entry := widget.NewEntry()
	assert.Equal(t, bellui.TextAlignLeading, entry.Alignment)

	entry.Alignment = bellui.TextAlignTrailing
	entry.SetText("123")

	// Trailing-aligned cursor at end should be further right than at start.
	entry.CursorColumn = len(entry.Text)
	endPos := entry.CursorPosition()
	entry.CursorColumn = 0
	startPos := entry.CursorPosition()
	assert.Greater(t, endPos.X, startPos.X)
}

func TestEntry_Alignment_Center(t *testing.T) {
	entry := widget.NewEntry()
	entry.Alignment = bellui.TextAlignCenter
	entry.SetText("test")
	assert.Equal(t, bellui.TextAlignCenter, entry.Alignment)
}

func TestEntry_Control_Word(t *testing.T) {
	entry := widget.NewMultiLineEntry()
	entry.SetText("a\nbc")
	entry.CursorRow = 0
	entry.CursorColumn = 0

	moveWordModifier := event.KeyModifierShortcutDefault
	if runtime.GOOS == "darwin" {
		moveWordModifier = event.KeyModifierAlt
	}

	// ctrl-right to move on
	nextWord := &desktop.CustomShortcut{KeyName: event.KeyRight, Modifier: moveWordModifier}
	entry.TypedShortcut(nextWord)
	assert.Equal(t, 0, entry.CursorRow)
	assert.Equal(t, 1, entry.CursorColumn)
	entry.TypedShortcut(nextWord)
	assert.Equal(t, 1, entry.CursorRow)
	assert.Equal(t, 0, entry.CursorColumn)
	entry.TypedShortcut(nextWord)
	assert.Equal(t, 1, entry.CursorRow)
	assert.Equal(t, 2, entry.CursorColumn)

	// ctrl-left to move back
	prevWord := &desktop.CustomShortcut{KeyName: event.KeyLeft, Modifier: moveWordModifier}
	entry.TypedShortcut(prevWord)
	assert.Equal(t, 1, entry.CursorRow)
	assert.Equal(t, 0, entry.CursorColumn)
	entry.TypedShortcut(prevWord)
	assert.Equal(t, 0, entry.CursorRow)
	assert.Equal(t, 1, entry.CursorColumn)

	// select word
	entry.SetText("word1 word2 word3")
	entry.CursorRow = 0
	entry.CursorColumn = 3
	selectNextWord := &desktop.CustomShortcut{KeyName: event.KeyRight, Modifier: moveWordModifier | event.KeyModifierShift}
	entry.TypedShortcut(selectNextWord)
	assert.Equal(t, "d1", entry.SelectedText())
	entry.TypedShortcut(selectNextWord)
	assert.Equal(t, "d1 word2", entry.SelectedText())

	// unselect when no shift press
	entry.TypedShortcut(nextWord)
	assert.Equal(t, "", entry.SelectedText())
}

func TestEntry_Control_DeleteWord(t *testing.T) {
	entry := widget.NewMultiLineEntry()
	entry.SetText("Hello world\nhere is a second line")
	entry.CursorRow = 1
	entry.CursorColumn = 10 // right before "second"
	modifier := event.KeyModifierControl
	if runtime.GOOS == "darwin" {
		modifier = event.KeyModifierAlt
	}
	// Ctrl+delete - delete word to right ("second")
	entry.TypedShortcut(&desktop.CustomShortcut{Modifier: modifier, KeyName: event.KeyDelete})
	assert.Equal(t, "Hello world\nhere is a  line", entry.Text)
	assert.Equal(t, 10, entry.CursorColumn)

	entry.CursorColumn = 8 // right before "a"
	// Ctrl+backspace - delete word to left ("is")
	entry.TypedShortcut(&desktop.CustomShortcut{Modifier: modifier, KeyName: event.KeyBackspace})
	assert.Equal(t, "Hello world\nhere a  line", entry.Text)
	assert.Equal(t, 5, entry.CursorColumn)

	// does nothing when nothing left to delete
	entry.SetText("")
	entry.TypedShortcut(&desktop.CustomShortcut{Modifier: modifier, KeyName: event.KeyBackspace})
	assert.Equal(t, "", entry.Text)

	// doesn't crash when trying to delete backward with one space
	entry.SetText(" ")
	entry.CursorRow = 0
	entry.CursorColumn = 1
	entry.TypedShortcut(&desktop.CustomShortcut{Modifier: modifier, KeyName: event.KeyBackspace})
}

func TestEntry_CursorColumn_Wrap(t *testing.T) {
	entry := widget.NewMultiLineEntry()
	entry.SetText("a\nb")
	assert.Equal(t, 0, entry.CursorRow)
	assert.Equal(t, 0, entry.CursorColumn)

	// go to end of line
	right := &event.KeyEvent{Name: event.KeyRight}
	entry.TypedKey(right)
	assert.Equal(t, 0, entry.CursorRow)
	assert.Equal(t, 1, entry.CursorColumn)

	// wrap to new line
	entry.TypedKey(right)
	assert.Equal(t, 1, entry.CursorRow)
	assert.Equal(t, 0, entry.CursorColumn)

	// and back
	left := &event.KeyEvent{Name: event.KeyLeft}
	entry.TypedKey(left)
	assert.Equal(t, 0, entry.CursorRow)
	assert.Equal(t, 1, entry.CursorColumn)
}

func TestEntry_CursorColumn_Wrap2(t *testing.T) {
	entry := widget.NewMultiLineEntry()
	entry.Wrapping = bellui.TextWrapWord
	entry.SetText("1234")
	entry.CursorColumn = 3

	w := test.NewTempWindow(t, entry)
	// Window sizes are 2dp wider than they historically were: the viewport
	// now sits inside the background box (#467), and the extra width keeps
	// the text wrapping at the exact columns this scenario asserts.
	w.Resize(geo.NewSize(68, 64))

	w.Canvas().Focus(entry)
	// Clear select-all triggered by FocusGained
	entry.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	entry.CursorColumn = 3
	typeOnFocused(entry, "a")
	typeOnFocused(entry, "b")
	typeOnFocused(entry, "c")
	assert.Equal(t, 1, entry.CursorColumn)
	assert.Equal(t, 1, entry.CursorRow)
	w.Resize(geo.NewSize(66, 70))
	assert.Equal(t, 1, entry.CursorColumn)
	test.AssertImageMatches(t, "entry/wrap_multi_line_cursor.png", w.Canvas().Capture())

	typeKeys(entry, keyShiftLeftDown, event.KeyLeft, keyShiftLeftUp)
	assert.Equal(t, "c", entry.SelectedText())
	assert.Equal(t, 0, entry.CursorColumn)
	w.Resize(geo.NewSize(60, 78))
	assert.Equal(t, "c", entry.SelectedText())
}

func TestEntry_CursorPasswordRevealer(t *testing.T) {
	pr := widget.NewPasswordEntry().ActionItem.(desktop.Cursorable)
	assert.Equal(t, desktop.DefaultCursor, pr.Cursor())
}

func TestEntry_CursorRow(t *testing.T) {
	entry := widget.NewMultiLineEntry()
	entry.SetText("test")
	assert.Equal(t, 0, entry.CursorRow)

	// only 1 line, do nothing
	down := &event.KeyEvent{Name: event.KeyDown}
	entry.TypedKey(down)
	assert.Equal(t, 0, entry.CursorRow)

	// 2 lines, this should increment
	entry.SetText("test\nrows")
	entry.TypedKey(down)
	assert.Equal(t, 1, entry.CursorRow)

	up := &event.KeyEvent{Name: event.KeyUp}
	entry.TypedKey(up)
	assert.Equal(t, 0, entry.CursorRow)

	// don't go beyond top
	entry.TypedKey(up)
	assert.Equal(t, 0, entry.CursorRow)
}

func TestEntry_Disableable(t *testing.T) {
	entry, window := setupImageTest(t, false)
	c := window.Canvas()

	assert.False(t, entry.Disabled())
	test.AssertRendersToMarkup(t, "entry/disableable_enabled_empty.xml", c)

	entry.Disable()
	assert.True(t, entry.Disabled())
	test.AssertRendersToMarkup(t, "entry/disableable_disabled_empty.xml", c)

	entry.Enable()
	assert.False(t, entry.Disabled())
	test.AssertRendersToMarkup(t, "entry/disableable_enabled_empty.xml", c)

	entry.SetPlaceHolder("Type!")
	assert.False(t, entry.Disabled())
	test.AssertRendersToMarkup(t, "entry/disableable_enabled_placeholder.xml", c)

	entry.Disable()
	assert.True(t, entry.Disabled())
	test.AssertRendersToMarkup(t, "entry/disableable_disabled_placeholder.xml", c)

	entry.Enable()
	assert.False(t, entry.Disabled())
	test.AssertRendersToMarkup(t, "entry/disableable_enabled_placeholder.xml", c)

	entry.SetText("Hello")
	assert.False(t, entry.Disabled())
	test.AssertRendersToMarkup(t, "entry/disableable_enabled_custom_value.xml", c)

	entry.Disable()
	assert.True(t, entry.Disabled())
	test.AssertRendersToMarkup(t, "entry/disableable_disabled_custom_value.xml", c)

	entry.Enable()
	assert.False(t, entry.Disabled())
	test.AssertRendersToMarkup(t, "entry/disableable_enabled_custom_value.xml", c)
}

func TestEntry_Disabled_TextSelection(t *testing.T) {
	entry, window := setupImageTest(t, false)
	entry.SetText("Testing")
	entry.Disable()
	c := window.Canvas()

	assert.True(t, entry.Disabled())
	test.DoubleTap(entry)

	test.AssertImageMatches(t, "entry/disabled_text_selected.png", c.Capture())

	entry.FocusLost()
	test.AssertImageMatches(t, "entry/disabled_text_unselected.png", c.Capture())
}

func TestEntry_EmptySelection(t *testing.T) {
	entry := widget.NewEntry()
	entry.SetText("text")

	// trying to select at the edge
	typeKeys(entry, keyShiftLeftDown, event.KeyLeft, keyShiftLeftUp)
	assert.Equal(t, "", entry.SelectedText())

	typeKeys(entry, event.KeyRight)
	assert.Equal(t, 1, entry.CursorColumn)

	// stop selecting at the edge when nothing is selected
	typeKeys(entry, event.KeyLeft, keyShiftLeftDown, event.KeyRight, event.KeyLeft, keyShiftLeftUp)
	assert.Equal(t, "", entry.SelectedText())
	assert.Equal(t, 0, entry.CursorColumn)

	// check that the selection has been removed
	typeKeys(entry, event.KeyRight, keyShiftLeftDown, event.KeyRight, event.KeyLeft, keyShiftLeftUp)
	assert.Equal(t, "", entry.SelectedText())
	assert.Equal(t, 1, entry.CursorColumn)

	// manually setting to empty selection
	typeKeys(entry, keyShiftLeftDown, event.KeyRight)
	entry.CursorColumn = 1
	entry.Refresh()
	assert.Equal(t, "", entry.SelectedText())
}

func TestEntry_Focus(t *testing.T) {
	entry, window := setupImageTest(t, false)
	c := window.Canvas()

	entry.FocusGained()
	test.AssertRendersToMarkup(t, "entry/focus_gained.xml", c)

	entry.FocusLost()
	test.AssertRendersToMarkup(t, "entry/focus_lost.xml", c)

	window.Canvas().Focus(entry)
	test.AssertRendersToMarkup(t, "entry/focus_gained.xml", c)
}

func TestEntry_FocusWithPopUp(t *testing.T) {
	entry, window := setupImageTest(t, false)
	c := window.Canvas()

	test.TapSecondaryAt(entry, geo.NewPos(1, 1))

	test.AssertRendersToMarkup(t, "entry/focus_with_popup_initial.xml", c)

	test.TapCanvas(c, geo.NewPos(20, 20))
	test.AssertRendersToMarkup(t, "entry/focus_with_popup_entry_selected.xml", c)

	test.TapSecondaryAt(entry, geo.NewPos(1, 1))
	test.AssertRendersToMarkup(t, "entry/focus_with_popup_initial.xml", c)

	test.TapCanvas(c, geo.NewPos(5, 5))
	test.AssertRendersToMarkup(t, "entry/focus_with_popup_dismissed.xml", c)
}

func TestEntry_HidePopUpOnEntry(t *testing.T) {
	entry := widget.NewEntry()
	tapPos := geo.NewPos(1, 1)
	c := bellui.CurrentApp().Driver().CanvasForObject(entry)

	assert.Nil(t, c.Overlays().Top())

	test.TapSecondaryAt(entry, tapPos)
	assert.NotNil(t, c.Overlays().Top())

	test.Type(entry, "KJGFD")
	assert.Nil(t, c.Overlays().Top())
	assert.Equal(t, "KJGFD", entry.Text)
}

func TestEntry_MinSize(t *testing.T) {
	entry := widget.NewEntry()
	min := entry.MinSize()
	entry.SetPlaceHolder("")
	assert.Equal(t, min, entry.MinSize())
	entry.SetText("")
	assert.Equal(t, min, entry.MinSize())
	entry.SetPlaceHolder("Hello")
	assert.Equal(t, entry.MinSize().Width, min.Width)
	assert.Equal(t, entry.MinSize().Height, min.Height)

	assert.Greater(t, min.Width, theme.InnerPadding())
	assert.Greater(t, min.Height, theme.InnerPadding())

	entry.Wrapping = bellui.TextWrapOff
	entry.Scroll = container.ScrollNone
	entry.Refresh()
	assert.Greater(t, entry.MinSize().Width, min.Width)

	min = entry.MinSize()
	entry.ActionItem = canvas.NewCircle(color.Black)
	entry.Refresh()
	assert.Equal(t, min.Add(geo.NewSize(theme.IconInlineSize()+theme.LineSpacing(), 0)), entry.MinSize())
}

func TestEntryMultiline_MinSize(t *testing.T) {
	entry := widget.NewMultiLineEntry()
	min := entry.MinSize()
	entry.SetText("Hello")
	assert.Equal(t, entry.MinSize().Width, min.Width)
	assert.Equal(t, entry.MinSize().Height, min.Height)

	assert.Greater(t, min.Width, theme.InnerPadding())
	assert.Greater(t, min.Height, theme.InnerPadding())

	entry.Wrapping = bellui.TextWrapOff
	entry.Scroll = container.ScrollNone
	entry.Refresh()
	assert.Greater(t, entry.MinSize().Width, min.Width)

	entry.Wrapping = bellui.TextWrapBreak
	entry.Refresh()
	assert.Equal(t, entry.MinSize().Width, min.Width)

	min = entry.MinSize()
	entry.ActionItem = canvas.NewCircle(color.Black)
	entry.Refresh()
	assert.Equal(t, min.Add(geo.NewSize(theme.IconInlineSize()+theme.Padding(), 0)), entry.MinSize())
}

func TestEntryMultiline_SetMinRowsVisible(t *testing.T) {
	entry := widget.NewMultiLineEntry()
	min := entry.MinSize()
	entry.SetText("Hello")
	assert.Equal(t, entry.MinSize().Height, min.Height)

	entry.SetMinRowsVisible(2)
	assert.Less(t, entry.MinSize().Height, min.Height)

	entry.SetMinRowsVisible(5)
	assert.Greater(t, entry.MinSize().Height, min.Height)
}

func TestEntry_MultilineSelect(t *testing.T) {
	e, window := setupSelection(t, false)
	c := window.Canvas()

	// Extend the selection down one row
	typeKeys(e, event.KeyDown)
	test.AssertRendersToMarkup(t, "entry/selection_add_one_row_down.xml", c)
	assert.Equal(t, "sting\nTesti", e.SelectedText())

	typeKeys(e, event.KeyUp)
	test.AssertRendersToMarkup(t, "entry/selection_remove_one_row_up.xml", c)
	assert.Equal(t, "sti", e.SelectedText())

	typeKeys(e, event.KeyUp)
	test.AssertRendersToMarkup(t, "entry/selection_remove_add_one_row_up.xml", c)
	assert.Equal(t, "ng\nTe", e.SelectedText())
}

func TestEntry_MultilineWrapping_DeleteWithBackspace(t *testing.T) {
	entry := widget.NewMultiLineEntry()
	entry.Wrapping = bellui.TextWrapWord
	entry.Resize(geo.NewSize(64, 64))
	test.Type(entry, "line1")
	typeOnFocused(entry, "\nline2")
	typeOnFocused(entry, "\nline3")

	assert.Equal(t, 5, entry.CursorColumn)
	assert.Equal(t, 2, entry.CursorRow)

	for i := 0; i < 4; i++ {
		entry.TypedKey(&event.KeyEvent{Name: event.KeyBackspace})
		assert.Equal(t, 4-i, entry.CursorColumn)
		assert.Equal(t, 2, entry.CursorRow)
	}

	entry.TypedKey(&event.KeyEvent{Name: event.KeyBackspace})
	assert.Equal(t, 0, entry.CursorColumn)
	assert.Equal(t, 2, entry.CursorRow)

	assert.NotPanics(t, func() {
		entry.TypedKey(&event.KeyEvent{Name: event.KeyBackspace})
	})
	assert.Equal(t, 5, entry.CursorColumn)
	assert.Equal(t, 1, entry.CursorRow)
}

func TestEntry_Notify(t *testing.T) {
	entry := widget.NewEntry()
	changed := false

	entry.OnChanged = func(string) {
		changed = true
	}
	entry.SetText("Test")

	assert.True(t, changed)

	changed = false
	entry.TypedKey(&event.KeyEvent{Name: event.KeyDelete})
	assert.True(t, changed)

	changed = false
	entry.CursorColumn = 1
	entry.TypedKey(&event.KeyEvent{Name: event.KeyBackspace})
	assert.True(t, changed)
}

func TestEntry_OnCopy(t *testing.T) {
	e := widget.NewEntry()
	e.SetText("Testing")
	typeKeys(e, event.KeyRight, event.KeyRight, keyShiftLeftDown, event.KeyRight, event.KeyRight, event.KeyRight)

	clipboard := test.NewClipboard()
	shortcut := &bellui.ShortcutCopy{Clipboard: clipboard}
	e.TypedShortcut(shortcut)

	assert.Equal(t, "sti", clipboard.Content())
	assert.Equal(t, "Testing", e.Text)
}

func TestEntry_OnCopy_Password(t *testing.T) {
	e := widget.NewPasswordEntry()
	e.SetText("Testing")
	typeKeys(e, keyShiftLeftDown, event.KeyRight, event.KeyRight, event.KeyRight)

	clipboard := test.NewClipboard()
	shortcut := &bellui.ShortcutCopy{Clipboard: clipboard}
	e.TypedShortcut(shortcut)

	assert.Equal(t, "", clipboard.Content())
	assert.Equal(t, "Testing", e.Text)
}

func TestEntry_OnCut(t *testing.T) {
	e := widget.NewEntry()
	e.SetText("Testing")
	typeKeys(e, event.KeyRight, event.KeyRight, keyShiftLeftDown, event.KeyRight, event.KeyRight, event.KeyRight)

	clipboard := test.NewClipboard()
	shortcut := &bellui.ShortcutCut{Clipboard: clipboard}
	e.TypedShortcut(shortcut)

	assert.Equal(t, "sti", clipboard.Content())
	assert.Equal(t, "Teng", e.Text)
}

func TestEntry_OnCut_Password(t *testing.T) {
	e := widget.NewPasswordEntry()
	e.SetText("Testing")
	typeKeys(e, keyShiftLeftDown, event.KeyRight, event.KeyRight, event.KeyRight)

	clipboard := test.NewClipboard()
	shortcut := &bellui.ShortcutCut{Clipboard: clipboard}
	e.TypedShortcut(shortcut)

	assert.Equal(t, "", clipboard.Content())
	assert.Equal(t, "Testing", e.Text)
}

func TestEntry_OnKeyDown(t *testing.T) {
	entry := widget.NewEntry()

	test.Type(entry, "Hi")

	assert.Equal(t, "Hi", entry.Text)
}

func TestEntry_OnKeyDown_Backspace(t *testing.T) {
	entry := widget.NewEntry()
	entry.SetText("Hi")
	right := &event.KeyEvent{Name: event.KeyRight}
	entry.TypedKey(right)
	entry.TypedKey(right)
	assert.Equal(t, 0, entry.CursorRow)
	assert.Equal(t, 2, entry.CursorColumn)

	backspace := &event.KeyEvent{Name: event.KeyBackspace}
	entry.TypedKey(backspace)

	assert.Equal(t, "H", entry.Text)
	assert.Equal(t, 0, entry.CursorRow)
	assert.Equal(t, 1, entry.CursorColumn)

	entry = widget.NewMultiLineEntry()
	entry.SetText("Line\n2b\n")
	down := &event.KeyEvent{Name: event.KeyDown}
	entry.TypedKey(down)
	entry.TypedKey(right)
	assert.Equal(t, 1, entry.CursorRow)
	assert.Equal(t, 1, entry.CursorColumn)

	entry.TypedKey(backspace)
	assert.Equal(t, "Line\nb\n", entry.Text)
	assert.Equal(t, 1, entry.CursorRow)
	assert.Equal(t, 0, entry.CursorColumn)

	// Word-wrap navigation depends on the exact text-region width. On touch the
	// scrollbar reserves more horizontal space (full-width touch target), so
	// "Line 2b" wraps at a different point and the cursor maps to a different
	// glyph. The wrap geometry is platform-dependent here, not the backspace
	// logic under test, so this sub-case is desktop-only.
	if !bellui.CurrentDevice().IsMobile() {
		entry.CursorRow = 0
		entry.CursorColumn = 0
		entry.Wrapping = bellui.TextWrapWord
		entry.SetText("Line 2b")
		entry.Resize(geo.NewSize(50, 50))
		entry.TypedKey(down)
		entry.TypedKey(right)
		assert.Equal(t, 1, entry.CursorRow)
		assert.Equal(t, 1, entry.CursorColumn)

		entry.TypedKey(backspace)
		assert.Equal(t, "Line b", entry.Text)
		assert.Equal(t, 1, entry.CursorRow)
		assert.Equal(t, 0, entry.CursorColumn)
	}
}

func TestEntry_OnKeyDown_BackspaceBeyondText(t *testing.T) {
	entry := widget.NewEntry()
	entry.SetText("Hi")
	right := &event.KeyEvent{Name: event.KeyRight}
	entry.TypedKey(right)
	entry.TypedKey(right)

	key := &event.KeyEvent{Name: event.KeyBackspace}
	entry.TypedKey(key)
	entry.TypedKey(key)
	entry.TypedKey(key)

	assert.Equal(t, "", entry.Text)
}

func TestEntry_OnKeyDown_BackspaceBeyondTextAndNewLine(t *testing.T) {
	entry := widget.NewMultiLineEntry()
	entry.SetText("H\ni")

	down := &event.KeyEvent{Name: event.KeyDown}
	entry.TypedKey(down)
	right := &event.KeyEvent{Name: event.KeyRight}
	entry.TypedKey(right)

	key := &event.KeyEvent{Name: event.KeyBackspace}
	entry.TypedKey(key)

	assert.Equal(t, 1, entry.CursorRow)
	assert.Equal(t, 0, entry.CursorColumn)
	entry.TypedKey(key)

	assert.Equal(t, 0, entry.CursorRow)
	assert.Equal(t, 1, entry.CursorColumn)
	assert.Equal(t, "H", entry.Text)
}

func TestEntry_OnKeyDown_BackspaceNewline(t *testing.T) {
	entry := widget.NewMultiLineEntry()
	entry.SetText("H\ni")

	down := &event.KeyEvent{Name: event.KeyDown}
	entry.TypedKey(down)

	key := &event.KeyEvent{Name: event.KeyBackspace}
	entry.TypedKey(key)

	assert.Equal(t, "Hi", entry.Text)
}

func TestEntry_OnKeyDown_BackspaceUnicode(t *testing.T) {
	entry := widget.NewEntry()

	test.Type(entry, "è")
	assert.Equal(t, 0, entry.CursorRow)
	assert.Equal(t, 1, entry.CursorColumn)

	bs := &event.KeyEvent{Name: event.KeyBackspace}
	entry.TypedKey(bs)
	assert.Equal(t, "", entry.Text)
	assert.Equal(t, 0, entry.CursorRow)
	assert.Equal(t, 0, entry.CursorColumn)
}

func TestEntry_OnKeyDown_Delete(t *testing.T) {
	entry := widget.NewEntry()
	entry.SetText("Hi")
	right := &event.KeyEvent{Name: event.KeyRight}
	entry.TypedKey(right)
	assert.Equal(t, 0, entry.CursorRow)
	assert.Equal(t, 1, entry.CursorColumn)

	key := &event.KeyEvent{Name: event.KeyDelete}
	entry.TypedKey(key)

	assert.Equal(t, "H", entry.Text)
	assert.Equal(t, 0, entry.CursorRow)
	assert.Equal(t, 1, entry.CursorColumn)
}

func TestEntry_OnKeyDown_DeleteBeyondText(t *testing.T) {
	entry := widget.NewEntry()
	entry.SetText("Hi")

	key := &event.KeyEvent{Name: event.KeyDelete}
	entry.TypedKey(key)
	entry.TypedKey(key)
	entry.TypedKey(key)

	assert.Equal(t, "", entry.Text)
}

func TestEntry_OnKeyDown_DeleteNewline(t *testing.T) {
	entry := widget.NewEntry()
	entry.SetText("H\ni")

	right := &event.KeyEvent{Name: event.KeyRight}
	entry.TypedKey(right)

	key := &event.KeyEvent{Name: event.KeyDelete}
	entry.TypedKey(key)

	assert.Equal(t, "Hi", entry.Text)
}

func TestEntry_OnKeyDown_HomeEnd(t *testing.T) {
	entry := widget.NewEntry()
	entry.SetText("Hi")
	assert.Equal(t, 0, entry.CursorRow)
	assert.Equal(t, 0, entry.CursorColumn)

	end := &event.KeyEvent{Name: event.KeyEnd}
	entry.TypedKey(end)
	assert.Equal(t, 0, entry.CursorRow)
	assert.Equal(t, 2, entry.CursorColumn)

	home := &event.KeyEvent{Name: event.KeyHome}
	entry.TypedKey(home)
	assert.Equal(t, 0, entry.CursorRow)
	assert.Equal(t, 0, entry.CursorColumn)

	if runtime.GOOS == "darwin" {
		endShortcut := &desktop.CustomShortcut{KeyName: event.KeyRight, Modifier: event.KeyModifierSuper}
		entry.TypedShortcut(endShortcut)
		assert.Equal(t, 0, entry.CursorRow)
		assert.Equal(t, 2, entry.CursorColumn)

		homeShortcut := &desktop.CustomShortcut{KeyName: event.KeyLeft, Modifier: event.KeyModifierSuper}
		entry.TypedShortcut(homeShortcut)
		assert.Equal(t, 0, entry.CursorRow)
		assert.Equal(t, 0, entry.CursorColumn)
	}
}

func TestEntry_OnKeyDown_Insert(t *testing.T) {
	entry := widget.NewEntry()

	test.Type(entry, "Hi")
	assert.Equal(t, "Hi", entry.Text)

	left := &event.KeyEvent{Name: event.KeyLeft}
	entry.TypedKey(left)

	typeOnFocused(entry, "o")
	assert.Equal(t, "Hoi", entry.Text)
}

func TestEntry_OnKeyDown_Newline(t *testing.T) {
	entry, window := setupImageTest(t, true)
	c := window.Canvas()

	entry.SetText("Hi")
	assert.Equal(t, 0, entry.CursorRow)
	assert.Equal(t, 0, entry.CursorColumn)
	test.AssertRendersToMarkup(t, "entry/on_key_down_newline_initial.xml", c)

	right := &event.KeyEvent{Name: event.KeyRight}
	entry.TypedKey(right)
	assert.Equal(t, 0, entry.CursorRow)
	assert.Equal(t, 1, entry.CursorColumn)

	key := &event.KeyEvent{Name: event.KeyReturn}
	entry.TypedKey(key)

	assert.Equal(t, "H\ni", entry.Text)
	assert.Equal(t, 1, entry.CursorRow)
	assert.Equal(t, 0, entry.CursorColumn)

	typeOnFocused(entry, "o")
	assert.Equal(t, "H\noi", entry.Text)
	test.AssertRendersToMarkup(t, "entry/on_key_down_newline_typed.xml", c)
}

func TestEntry_OnPaste(t *testing.T) {
	clipboard := test.NewClipboard()
	shortcut := &bellui.ShortcutPaste{Clipboard: clipboard}
	tests := []struct {
		name             string
		entry            *widget.Entry
		clipboardContent string
		wantText         string
		wantRow, wantCol int
	}{
		{
			name:             "singleline: empty content",
			entry:            widget.NewEntry(),
			clipboardContent: "",
			wantText:         "",
			wantRow:          0,
			wantCol:          0,
		},
		{
			name:             "singleline: simple text",
			entry:            widget.NewEntry(),
			clipboardContent: "clipboard content",
			wantText:         "clipboard content",
			wantRow:          0,
			wantCol:          17,
		},
		{
			name:             "singleline: UTF8 text",
			entry:            widget.NewEntry(),
			clipboardContent: "Hié™שרה",
			wantText:         "Hié™שרה",
			wantRow:          0,
			wantCol:          7,
		},
		{
			name:             "singleline: with new line",
			entry:            widget.NewEntry(),
			clipboardContent: "clipboard\ncontent",
			wantText:         "clipboard content",
			wantRow:          0,
			wantCol:          17,
		},
		{
			name:             "singleline: with tab",
			entry:            widget.NewEntry(),
			clipboardContent: "clipboard\tcontent",
			wantText:         "clipboard\tcontent",
			wantRow:          0,
			wantCol:          17,
		},
		{
			name:             "password: with new line",
			entry:            widget.NewPasswordEntry(),
			clipboardContent: "3SB=y+)z\nkHGK(hx6 -e_\"1TZu q^bF3^$u H[:e\"1O.",
			wantText:         `3SB=y+)z kHGK(hx6 -e_"1TZu q^bF3^$u H[:e"1O.`,
			wantRow:          0,
			wantCol:          44,
		},
		{
			name:             "multiline: with new line",
			entry:            widget.NewMultiLineEntry(),
			clipboardContent: "clipboard\ncontent",
			wantText:         "clipboard\ncontent",
			wantRow:          1,
			wantCol:          7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clipboard.SetContent(tt.clipboardContent)
			tt.entry.TypedShortcut(shortcut)
			assert.Equal(t, tt.wantText, tt.entry.Text)
			assert.Equal(t, tt.wantRow, tt.entry.CursorRow)
			assert.Equal(t, tt.wantCol, tt.entry.CursorColumn)
		})
	}
}

func TestEntry_PageUpDown(t *testing.T) {
	t.Run("single line", func(*testing.T) {
		e, window := setupImageTest(t, false)
		c := window.Canvas()

		c.Focus(e)
		e.SetText("Testing")
		test.AssertRendersToMarkup(t, "entry/select_initial.xml", c)

		// move right, press & hold shift and pagedown
		typeKeys(e, event.KeyRight, keyShiftLeftDown, event.KeyPageDown)
		assert.Equal(t, "esting", e.SelectedText())
		assert.Equal(t, 0, e.CursorRow)
		assert.Equal(t, 7, e.CursorColumn)
		test.AssertRendersToMarkup(t, "entry/select_single_line_shift_pagedown.xml", c)

		// while shift is held press pageup
		typeKeys(e, event.KeyPageUp)
		assert.Equal(t, "T", e.SelectedText())
		assert.Equal(t, 0, e.CursorRow)
		assert.Equal(t, 0, e.CursorColumn)
		test.AssertRendersToMarkup(t, "entry/select_single_line_shift_pageup.xml", c)

		// release shift and press pagedown
		typeKeys(e, keyShiftLeftUp, event.KeyPageDown)
		assert.Equal(t, "", e.SelectedText())
		assert.Equal(t, 0, e.CursorRow)
		assert.Equal(t, 7, e.CursorColumn)
		test.AssertRendersToMarkup(t, "entry/select_single_line_pagedown.xml", c)
	})

	t.Run("multi line pages by a screenful", func(t *testing.T) {
		e := widget.NewMultiLineEntry()
		e.Wrapping = bellui.TextWrapOff
		w := test.NewTempWindow(t, e)
		w.Resize(geo.NewSize(220, 140))
		e.Resize(geo.NewSize(200, 100)) // viewport shows only a handful of rows
		w.Canvas().Focus(e)

		var sb strings.Builder
		for i := 0; i < 40; i++ {
			fmt.Fprintf(&sb, "row %02d contents\n", i)
		}
		e.SetText(strings.TrimSuffix(sb.String(), "\n")) // rows 0..39

		e.CursorRow, e.CursorColumn = 0, 3
		e.Refresh()

		// One PageDown pages by a full screenful: more than a single row but not
		// straight to the document end, and it preserves the column.
		typeKeys(e, event.KeyPageDown)
		page := e.CursorRow
		assert.Greater(t, page, 1, "PageDown pages by more than a single row")
		assert.Less(t, page, 39, "PageDown does not jump straight to the document end")
		assert.Equal(t, 3, e.CursorColumn, "PageDown preserves the column")

		// A second PageDown advances by the same page stride.
		typeKeys(e, event.KeyPageDown)
		assert.Equal(t, 2*page, e.CursorRow, "each PageDown advances by the same page")
		assert.Equal(t, 3, e.CursorColumn)

		// PageUp reverses one page.
		typeKeys(e, event.KeyPageUp)
		assert.Equal(t, page, e.CursorRow, "PageUp reverses one page")
		assert.Equal(t, 3, e.CursorColumn)

		// Repeated PageDown clamps at the last row, still preserving the column.
		for i := 0; i < 40; i++ {
			typeKeys(e, event.KeyPageDown)
		}
		assert.Equal(t, 39, e.CursorRow, "PageDown clamps at the last row")
		assert.Equal(t, 3, e.CursorColumn)

		// Repeated PageUp clamps at the first row.
		for i := 0; i < 40; i++ {
			typeKeys(e, event.KeyPageUp)
		}
		assert.Equal(t, 0, e.CursorRow, "PageUp clamps at the first row")
		assert.Equal(t, 3, e.CursorColumn)
	})

	t.Run("multi line shift-selects across a page", func(t *testing.T) {
		e := widget.NewMultiLineEntry()
		e.Wrapping = bellui.TextWrapOff
		w := test.NewTempWindow(t, e)
		w.Resize(geo.NewSize(220, 140))
		e.Resize(geo.NewSize(200, 100))
		w.Canvas().Focus(e)

		var sb strings.Builder
		for i := 0; i < 40; i++ {
			fmt.Fprintf(&sb, "row %02d\n", i)
		}
		e.SetText(strings.TrimSuffix(sb.String(), "\n"))

		e.CursorRow, e.CursorColumn = 0, 0
		e.Refresh()

		// Hold shift and page down: the selection extends from the anchor to the
		// paged caret (a non-empty, multi-row selection well short of the end).
		typeKeys(e, keyShiftLeftDown, event.KeyPageDown)
		assert.NotEmpty(t, e.SelectedText(), "shift+PageDown extends a selection")
		assert.Greater(t, e.CursorRow, 1)
		assert.Less(t, e.CursorRow, 39)

		// Shift+PageUp collapses back to the anchor.
		typeKeys(e, event.KeyPageUp, keyShiftLeftUp)
		assert.Equal(t, 0, e.CursorRow)
		assert.Equal(t, "", e.SelectedText())
	})
}

func TestEntry_DocumentJump(t *testing.T) {
	// Ctrl+Home/Ctrl+End (Cmd+ArrowUp/ArrowDown on macOS) jump to document
	// start/end, with Shift variants extending the selection (#377).
	docStart := event.KeyHome
	docEnd := event.KeyEnd
	if runtime.GOOS == "darwin" {
		docStart = event.KeyUp
		docEnd = event.KeyDown
	}
	jump := func(key event.KeyName, shift bool) *desktop.CustomShortcut {
		mod := event.KeyModifierShortcutDefault
		if shift {
			mod |= event.KeyModifierShift
		}
		return &desktop.CustomShortcut{KeyName: key, Modifier: mod}
	}

	e := widget.NewMultiLineEntry()
	e.SetText("first line\nsecond line\nthird and last line")

	// Ctrl+End jumps to the very end of the document.
	e.CursorRow, e.CursorColumn = 0, 0
	e.TypedShortcut(jump(docEnd, false))
	assert.Equal(t, 2, e.CursorRow)
	assert.Equal(t, len("third and last line"), e.CursorColumn)
	assert.Equal(t, "", e.SelectedText())

	// Ctrl+Home jumps to the very beginning.
	e.TypedShortcut(jump(docStart, false))
	assert.Equal(t, 0, e.CursorRow)
	assert.Equal(t, 0, e.CursorColumn)

	// Ctrl+Shift+End selects from the caret to the document end.
	e.CursorRow, e.CursorColumn = 1, 0
	e.TypedShortcut(jump(docEnd, true))
	assert.Equal(t, 2, e.CursorRow)
	assert.Equal(t, "second line\nthird and last line", e.SelectedText())

	// A plain (non-shift) jump clears the active selection.
	e.TypedShortcut(jump(docStart, false))
	assert.Equal(t, "", e.SelectedText())

	// Ctrl+Shift+Home selects from the caret to the document start.
	e.CursorRow, e.CursorColumn = 1, 0
	e.TypedShortcut(jump(docStart, true))
	assert.Equal(t, 0, e.CursorRow)
	assert.Equal(t, "first line\n", e.SelectedText())
}

func TestEntry_PasteOverSelection(t *testing.T) {
	e := widget.NewEntry()
	e.SetText("Testing")
	typeKeys(e, event.KeyRight, event.KeyRight, keyShiftLeftDown, event.KeyRight, event.KeyRight, event.KeyRight)

	clipboard := test.NewClipboard()
	clipboard.SetContent("Insert")
	shortcut := &bellui.ShortcutPaste{Clipboard: clipboard}
	e.TypedShortcut(shortcut)

	assert.Equal(t, "Insert", clipboard.Content())
	assert.Equal(t, "TeInsertng", e.Text)
}

func TestEntry_PasteUnicode(t *testing.T) {
	e := widget.NewMultiLineEntry()
	e.SetText("line")
	e.CursorColumn = 4

	clipboard := test.NewClipboard()
	clipboard.SetContent("thing {\n\titem: 'val测试'\n}")
	shortcut := &bellui.ShortcutPaste{Clipboard: clipboard}
	e.TypedShortcut(shortcut)

	assert.Equal(t, "thing {\n\titem: 'val测试'\n}", clipboard.Content())
	assert.Equal(t, "linething {\n\titem: 'val测试'\n}", e.Text)

	assert.Equal(t, 2, e.CursorRow)
	assert.Equal(t, 1, e.CursorColumn)
}

// flavorClipboard is a test clipboard implementing the optional flavor
// interfaces (bellui.HTMLClipboard + bellui.ImageClipboard) so Entry's
// assert-and-degrade paste path (#581) can be exercised.
type flavorClipboard struct {
	content string
	html    string
	hasHTML bool
	img     image.Image
	hasImg  bool
}

func (c *flavorClipboard) Content() string      { return c.content }
func (c *flavorClipboard) SetContent(s string)  { c.content = s }
func (c *flavorClipboard) HTML() (string, bool) { return c.html, c.hasHTML }
func (c *flavorClipboard) SetHTML(html, plainFallback string) {
	c.html, c.hasHTML, c.content = html, true, plainFallback
}
func (c *flavorClipboard) Image() (image.Image, bool) { return c.img, c.hasImg }
func (c *flavorClipboard) SetImage(img image.Image)   { c.img, c.hasImg = img, true }

var (
	_ bellui.Clipboard      = (*flavorClipboard)(nil)
	_ bellui.HTMLClipboard  = (*flavorClipboard)(nil)
	_ bellui.ImageClipboard = (*flavorClipboard)(nil)
)

// TestEntry_PasteHTMLFlavor: a clipboard carrying an HTML flavor pastes the
// plain-text fallback into the plain-text Entry — it never parses/strips the
// markup itself (rich HTML paste is #648).
func TestEntry_PasteHTMLFlavor(t *testing.T) {
	e := widget.NewEntry()

	cb := &flavorClipboard{}
	cb.SetHTML("<b>Bold</b> text", "Bold text") // Content() now reports the fallback.
	e.TypedShortcut(&bellui.ShortcutPaste{Clipboard: cb})

	assert.Equal(t, "Bold text", e.Text)
}

// TestEntry_PasteImageFlavorNoOp: an image-only clipboard (Content() == "")
// pastes nothing into a plain Entry and does not panic (image paste is #648).
func TestEntry_PasteImageFlavorNoOp(t *testing.T) {
	e := widget.NewEntry()
	e.SetText("keep")
	e.CursorColumn = 4

	cb := &flavorClipboard{}
	cb.SetImage(image.NewRGBA(image.Rect(0, 0, 1, 1))) // no plain text set.
	e.TypedShortcut(&bellui.ShortcutPaste{Clipboard: cb})

	assert.Equal(t, "keep", e.Text)
}

// TestEntry_PastePlainFlavorClipboard: a flavor-capable clipboard with only
// plain content pastes normally (assertions fall through harmlessly).
func TestEntry_PastePlainFlavorClipboard(t *testing.T) {
	e := widget.NewEntry()
	e.SetText("ab")
	e.CursorColumn = 2

	cb := &flavorClipboard{content: "cd"}
	e.TypedShortcut(&bellui.ShortcutPaste{Clipboard: cb})

	assert.Equal(t, "abcd", e.Text)
}

func TestEntry_Placeholder(t *testing.T) {
	entry := &widget.Entry{Scroll: container.ScrollNone}
	entry.Text = "Text"
	entry.PlaceHolder = "Placehold"

	window := test.NewTempWindow(t, entry)
	defer test.NewApp()
	c := window.Canvas()

	assert.Equal(t, "Text", entry.Text)
	test.AssertRendersToMarkup(t, "entry/placeholder_with_text.xml", c)

	entry.SetText("")
	assert.Equal(t, "", entry.Text)
	test.AssertRendersToMarkup(t, "entry/placeholder_without_text.xml", c)
}

func TestEntry_Select(t *testing.T) {
	for name, tt := range map[string]struct {
		keys          []event.KeyName
		text          string
		setupReverse  bool
		wantMarkup    string
		wantSelection string
		wantText      string
	}{
		"delete single-line": {
			keys:       []event.KeyName{event.KeyDelete},
			wantText:   "Testing\nTeng\nTesting",
			wantMarkup: "entry/selection_delete_single_line.xml",
		},
		"delete multi-line": {
			keys:       []event.KeyName{event.KeyDown, event.KeyDelete},
			wantText:   "Testing\nTeng",
			wantMarkup: "entry/selection_delete_multi_line.xml",
		},
		"delete reverse multi-line": {
			keys:         []event.KeyName{keyShiftLeftDown, event.KeyDown, event.KeyDelete},
			setupReverse: true,
			wantText:     "Testing\nTestisting",
			wantMarkup:   "entry/selection_delete_reverse_multi_line.xml",
		},
		"delete select down with Shift held": {
			keys:          []event.KeyName{keyShiftLeftDown, event.KeyDelete, event.KeyDown},
			wantText:      "Testing\nTeng\nTesting",
			wantSelection: "ng\nTe",
			wantMarkup:    "entry/selection_delete_and_add_down.xml",
		},
		"delete reverse select down with Shift held": {
			keys:          []event.KeyName{keyShiftLeftDown, event.KeyDelete, event.KeyDown},
			setupReverse:  true,
			wantText:      "Testing\nTeng\nTesting",
			wantSelection: "ng\nTe",
			wantMarkup:    "entry/selection_delete_and_add_down.xml",
		},
		"delete select up with Shift held": {
			keys:          []event.KeyName{keyShiftLeftDown, event.KeyDelete, event.KeyUp},
			wantText:      "Testing\nTeng\nTesting",
			wantSelection: "sting\nTe",
			wantMarkup:    "entry/selection_delete_and_add_up.xml",
		},
		"delete reverse select up with Shift held": {
			keys:          []event.KeyName{keyShiftLeftDown, event.KeyDelete, event.KeyUp},
			setupReverse:  true,
			wantText:      "Testing\nTeng\nTesting",
			wantSelection: "sting\nTe",
			wantMarkup:    "entry/selection_delete_and_add_up.xml",
		},
		// The backspace delete behaviour is the same as via delete.
		"backspace single-line": {
			keys:       []event.KeyName{event.KeyBackspace},
			wantText:   "Testing\nTeng\nTesting",
			wantMarkup: "entry/selection_delete_single_line.xml",
		},
		"backspace multi-line": {
			keys:       []event.KeyName{event.KeyDown, event.KeyBackspace},
			wantText:   "Testing\nTeng",
			wantMarkup: "entry/selection_delete_multi_line.xml",
		},
		"backspace reverse multi-line": {
			keys:         []event.KeyName{keyShiftLeftDown, event.KeyDown, event.KeyBackspace},
			setupReverse: true,
			wantText:     "Testing\nTestisting",
			wantMarkup:   "entry/selection_delete_reverse_multi_line.xml",
		},
		"backspace select down with Shift held": {
			keys:          []event.KeyName{keyShiftLeftDown, event.KeyBackspace, event.KeyDown},
			wantText:      "Testing\nTeng\nTesting",
			wantSelection: "ng\nTe",
			wantMarkup:    "entry/selection_delete_and_add_down.xml",
		},
		"backspace reverse select down with Shift held": {
			keys:          []event.KeyName{keyShiftLeftDown, event.KeyBackspace, event.KeyDown},
			setupReverse:  true,
			wantText:      "Testing\nTeng\nTesting",
			wantSelection: "ng\nTe",
			wantMarkup:    "entry/selection_delete_and_add_down.xml",
		},
		"backspace select up with Shift held": {
			keys:          []event.KeyName{keyShiftLeftDown, event.KeyBackspace, event.KeyUp},
			wantText:      "Testing\nTeng\nTesting",
			wantSelection: "sting\nTe",
			wantMarkup:    "entry/selection_delete_and_add_up.xml",
		},
		"backspace reverse select up with Shift held": {
			keys:          []event.KeyName{keyShiftLeftDown, event.KeyBackspace, event.KeyUp},
			setupReverse:  true,
			wantText:      "Testing\nTeng\nTesting",
			wantSelection: "sting\nTe",
			wantMarkup:    "entry/selection_delete_and_add_up.xml",
		},
		// Erase the selection and add a newline at selection start
		"enter": {
			keys:       []event.KeyName{event.KeyEnter},
			wantText:   "Testing\nTe\nng\nTesting",
			wantMarkup: "entry/selection_enter.xml",
		},
		"enter reverse": {
			keys:         []event.KeyName{event.KeyEnter},
			setupReverse: true,
			wantText:     "Testing\nTe\nng\nTesting",
			wantMarkup:   "entry/selection_enter.xml",
		},
		"replace": {
			text:       "hello",
			wantText:   "Testing\nTehellong\nTesting",
			wantMarkup: "entry/selection_replace.xml",
		},
		"replace reverse": {
			text:         "hello",
			setupReverse: true,
			wantText:     "Testing\nTehellong\nTesting",
			wantMarkup:   "entry/selection_replace.xml",
		},
		"deselect and delete": {
			keys:       []event.KeyName{keyShiftLeftUp, event.KeyLeft, event.KeyDelete},
			wantText:   "Testing\nTeting\nTesting",
			wantMarkup: "entry/selection_deselect_delete.xml",
		},
		"deselect and delete holding shift": {
			keys:       []event.KeyName{keyShiftLeftUp, event.KeyLeft, keyShiftLeftDown, event.KeyDelete},
			wantText:   "Testing\nTeting\nTesting",
			wantMarkup: "entry/selection_deselect_delete.xml",
		},
		// ensure that backspace doesn't leave a selection start at the old cursor position
		"deselect and backspace holding shift": {
			keys:       []event.KeyName{keyShiftLeftUp, event.KeyLeft, keyShiftLeftDown, event.KeyBackspace},
			wantText:   "Testing\nTsting\nTesting",
			wantMarkup: "entry/selection_deselect_backspace.xml",
		},
		// clear selection, select a character and while holding shift issue two backspaces
		"deselect, select and double backspace": {
			keys:       []event.KeyName{keyShiftLeftUp, event.KeyRight, event.KeyLeft, keyShiftLeftDown, event.KeyLeft, event.KeyBackspace, event.KeyBackspace},
			wantText:   "Testing\nTeing\nTesting",
			wantMarkup: "entry/selection_deselect_select_backspace.xml",
		},
	} {
		t.Run(name, func(t *testing.T) {
			entry, window := setupSelection(t, tt.setupReverse)
			c := window.Canvas()

			if tt.text != "" {
				typeOnFocused(entry, tt.text)
			} else {
				typeKeys(entry, tt.keys...)
			}
			assert.Equal(t, tt.wantText, entry.Text)
			assert.Equal(t, tt.wantSelection, entry.SelectedText())
			test.AssertRendersToMarkup(t, tt.wantMarkup, c)
		})
	}
}

func TestEntry_SelectAll(t *testing.T) {
	e, window := setupImageTest(t, true)
	c := window.Canvas()

	c.Focus(e)
	e.SetText("First Row\nSecond Row\nThird Row")
	test.AssertRendersToMarkup(t, "entry/select_all_initial.xml", c)

	shortcut := &bellui.ShortcutSelectAll{}
	e.TypedShortcut(shortcut)
	assert.Equal(t, 2, e.CursorRow)
	assert.Equal(t, 9, e.CursorColumn)
	test.AssertRendersToMarkup(t, "entry/select_all_selected.xml", c)
}

func TestEntry_SelectAll_EmptyEntry(t *testing.T) {
	entry := widget.NewEntry()
	entry.TypedShortcut(&bellui.ShortcutSelectAll{})

	assert.Equal(t, "", entry.SelectedText())
}

func TestEntry_SelectEndWithoutShift(t *testing.T) {
	e, window := setupSelection(t, false)
	c := window.Canvas()

	// end after releasing shift
	typeKeys(e, keyShiftLeftUp, event.KeyEnd)
	test.AssertRendersToMarkup(t, "entry/selection_end.xml", c)
	assert.Equal(t, "", e.SelectedText())
}

func TestEntry_SelectHomeEnd(t *testing.T) {
	e, window := setupSelection(t, false)
	c := window.Canvas()

	// Hold shift to continue selection
	typeKeys(e, keyShiftLeftDown)

	// T e[s t i]n g -> end -> // T e[s t i n g]
	typeKeys(e, event.KeyEnd)
	test.AssertRendersToMarkup(t, "entry/selection_add_to_end.xml", c)
	assert.Equal(t, "sting", e.SelectedText())

	// T e[s t i n g] -> home -> [T e]s t i n g
	typeKeys(e, event.KeyHome)
	test.AssertRendersToMarkup(t, "entry/selection_add_to_home.xml", c)
	assert.Equal(t, "Te", e.SelectedText())
}

func TestEntry_SelectHomeWithoutShift(t *testing.T) {
	e, window := setupSelection(t, false)
	c := window.Canvas()

	// home after releasing shift
	typeKeys(e, keyShiftLeftUp, event.KeyHome)
	test.AssertRendersToMarkup(t, "entry/selection_home.xml", c)
	assert.Equal(t, "", e.SelectedText())
}

func TestEntry_SelectSnapDown(t *testing.T) {
	// down snaps to end, but it also moves
	e, window := setupSelection(t, false)
	c := window.Canvas()

	assert.Equal(t, 1, e.CursorRow)
	assert.Equal(t, 5, e.CursorColumn)

	typeKeys(e, keyShiftLeftUp, event.KeyDown)
	assert.Equal(t, 2, e.CursorRow)
	assert.Equal(t, 5, e.CursorColumn)
	test.AssertRendersToMarkup(t, "entry/selection_snap_down.xml", c)
	assert.Equal(t, "", e.SelectedText())
}

func TestEntry_SelectSnapLeft(t *testing.T) {
	e, window := setupSelection(t, false)
	c := window.Canvas()

	assert.Equal(t, 1, e.CursorRow)
	assert.Equal(t, 5, e.CursorColumn)

	typeKeys(e, keyShiftLeftUp, event.KeyLeft)
	assert.Equal(t, 1, e.CursorRow)
	assert.Equal(t, 2, e.CursorColumn)
	test.AssertRendersToMarkup(t, "entry/selection_snap_left.xml", c)
	assert.Equal(t, "", e.SelectedText())
}

func TestEntry_SelectSnapRight(t *testing.T) {
	e, window := setupSelection(t, false)
	c := window.Canvas()

	assert.Equal(t, 1, e.CursorRow)
	assert.Equal(t, 5, e.CursorColumn)

	typeKeys(e, keyShiftLeftUp, event.KeyRight)
	assert.Equal(t, 1, e.CursorRow)
	assert.Equal(t, 5, e.CursorColumn)
	test.AssertRendersToMarkup(t, "entry/selection_snap_right.xml", c)
	assert.Equal(t, "", e.SelectedText())
}

func TestEntry_SelectSnapUp(t *testing.T) {
	// up snaps to start, but it also moves
	e, window := setupSelection(t, false)
	c := window.Canvas()

	assert.Equal(t, 1, e.CursorRow)
	assert.Equal(t, 5, e.CursorColumn)

	typeKeys(e, keyShiftLeftUp, event.KeyUp)
	assert.Equal(t, 0, e.CursorRow)
	assert.Equal(t, 5, e.CursorColumn)
	test.AssertRendersToMarkup(t, "entry/selection_snap_up.xml", c)
	assert.Equal(t, "", e.SelectedText())
}

func TestEntry_Select_TripleTap(t *testing.T) {
	e, _ := setupSelection(t, false)
	e.MultiLine = true
	assert.Equal(t, 1, e.CursorRow)
	assert.Equal(t, "sti", e.SelectedText())
	test.DoubleTap(e)
	time.Sleep(50 * time.Millisecond)
	e.MouseDown(&desktop.MouseEvent{PointEvent: event.PointEvent{Position: geo.NewPos(1, 1)}})
	assert.Equal(t, "Testing", e.SelectedText())
}

func TestEntry_SelectedText(t *testing.T) {
	e, window := setupImageTest(t, false)
	c := window.Canvas()

	c.Focus(e)
	e.SetText("Testing")
	test.AssertRendersToMarkup(t, "entry/select_initial.xml", c)
	assert.Equal(t, "", e.SelectedText())

	// move right, press & hold shift and move right
	typeKeys(e, event.KeyRight, keyShiftLeftDown, event.KeyRight, event.KeyRight)
	assert.Equal(t, "es", e.SelectedText())
	test.AssertRendersToMarkup(t, "entry/select_selected.xml", c)

	// release shift
	typeKeys(e, keyShiftLeftUp)
	// press shift and move
	typeKeys(e, keyShiftLeftDown, event.KeyRight)
	assert.Equal(t, "est", e.SelectedText())
	test.AssertRendersToMarkup(t, "entry/select_add_selection.xml", c)

	// release shift and move right
	typeKeys(e, keyShiftLeftUp, event.KeyRight)
	assert.Equal(t, "", e.SelectedText())
	test.AssertRendersToMarkup(t, "entry/select_move_wo_shift.xml", c)

	// press shift and move left
	typeKeys(e, keyShiftLeftDown, event.KeyLeft, event.KeyLeft)
	assert.Equal(t, "st", e.SelectedText())
	test.AssertRendersToMarkup(t, "entry/select_select_left.xml", c)
}

func TestEntry_SelectionHides(t *testing.T) {
	e, window := setupSelection(t, false)
	c := window.Canvas()

	c.Unfocus()
	test.AssertRendersToMarkup(t, "entry/selection_focus_lost.xml", c)
	assert.Equal(t, "sti", e.SelectedText())

	c.Focus(e)
	test.AssertRendersToMarkup(t, "entry/selection_focus_gained.xml", c)
	assert.Equal(t, "sti", e.SelectedText()) // multi-line entries preserve selection on refocus
}

func TestEntry_SetPlaceHolder(t *testing.T) {
	entry, window := setupImageTest(t, false)
	c := window.Canvas()

	assert.Empty(t, entry.Text)

	entry.SetPlaceHolder("Test")
	assert.Empty(t, entry.Text)
	test.AssertRendersToMarkup(t, "entry/set_placeholder_set.xml", c)

	entry.SetText("Hi")
	assert.Len(t, entry.Text, 2)
	test.AssertRendersToMarkup(t, "entry/set_placeholder_replaced.xml", c)
}

func TestEntry_SetPlaceHolder_ByField(t *testing.T) {
	entry, window := setupImageTest(t, false)
	c := window.Canvas()

	assert.Empty(t, entry.Text)

	entry.PlaceHolder = "Test"
	entry.Refresh()
	assert.Empty(t, entry.Text)
	test.AssertRendersToMarkup(t, "entry/set_placeholder_set.xml", c)

	entry.SetText("Hi")
	assert.Len(t, entry.Text, 2)
	test.AssertRendersToMarkup(t, "entry/set_placeholder_replaced.xml", c)
}

func TestEntry_Disable_KeyDown(t *testing.T) {
	entry := widget.NewEntry()

	test.Type(entry, "H")
	entry.Disable()
	typeOnFocused(entry, "i")
	assert.Equal(t, "H", entry.Text)

	entry.Enable()
	typeOnFocused(entry, "i")
	assert.Equal(t, "Hi", entry.Text)
}

func TestEntry_Disable_OnFocus(t *testing.T) {
	entry, window := setupImageTest(t, false)
	c := window.Canvas()

	entry.Disable()
	entry.FocusGained()
	test.AssertRendersToMarkup(t, "entry/focused_disabled.xml", c)

	entry.Enable()
	entry.FocusGained()
	test.AssertRendersToMarkup(t, "entry/focused_enabled.xml", c)
}

func TestEntry_SetText_EmptyString(t *testing.T) {
	entry := widget.NewEntry()

	assert.Equal(t, 0, entry.CursorColumn)

	test.Type(entry, "test")
	assert.Equal(t, 4, entry.CursorColumn)
	entry.SetText("")
	assert.Equal(t, 0, entry.CursorColumn)

	entry = widget.NewMultiLineEntry()
	test.Type(entry, "test\ntest")

	down := &event.KeyEvent{Name: event.KeyDown}
	entry.TypedKey(down)

	assert.Equal(t, 4, entry.CursorColumn)
	assert.Equal(t, 1, entry.CursorRow)
	entry.SetText("")
	assert.Equal(t, 0, entry.CursorColumn)
	assert.Equal(t, 0, entry.CursorRow)
}

func TestEntry_SetText_Manual(t *testing.T) {
	entry, window := setupImageTest(t, false)
	c := window.Canvas()

	entry.Text = "Test"
	entry.Refresh()
	test.AssertRendersToMarkup(t, "entry/set_text_changed.xml", c)
}

func TestEntry_SetText_Overflow(t *testing.T) {
	entry := widget.NewEntry()

	assert.Equal(t, 0, entry.CursorColumn)

	test.Type(entry, "test")
	assert.Equal(t, 4, entry.CursorColumn)

	entry.SetText("x")
	assert.Equal(t, 1, entry.CursorColumn)

	key := &event.KeyEvent{Name: event.KeyDelete}
	entry.TypedKey(key)

	assert.Equal(t, 1, entry.CursorColumn)
	assert.Equal(t, "x", entry.Text)

	key = &event.KeyEvent{Name: event.KeyBackspace}
	entry.TypedKey(key)

	assert.Equal(t, 0, entry.CursorColumn)
	assert.Equal(t, "", entry.Text)
}

func TestEntry_SetText_Underflow(t *testing.T) {
	entry := widget.NewEntry()
	test.Type(entry, "test")
	assert.Equal(t, 4, entry.CursorColumn)

	entry.Text = ""
	entry.Refresh()
	assert.Equal(t, 0, entry.CursorColumn)

	key := &event.KeyEvent{Name: event.KeyBackspace}
	entry.TypedKey(key)

	assert.Equal(t, 0, entry.CursorColumn)
	assert.Equal(t, "", entry.Text)
}

func TestEntry_SetText_Overflow_Multiline(t *testing.T) {
	entry := widget.NewEntry()
	entry.MultiLine = true

	assert.Equal(t, 0, entry.CursorColumn)
	assert.Equal(t, 0, entry.CursorRow)

	entry.SetText("ab\ncd\nef")
	typeKeys(entry, event.KeyDown, event.KeyDown, event.KeyRight)
	assert.Equal(t, 1, entry.CursorColumn)
	assert.Equal(t, 2, entry.CursorRow)
	entry.SetText("AB\nAAAA")
	assert.Equal(t, 4, entry.CursorColumn)
	assert.Equal(t, 1, entry.CursorRow)
}

func TestEntry_SetTextStyle(t *testing.T) {
	entry, window := setupImageTest(t, false)
	c := window.Canvas()

	entry.Text = "Styled Text"
	entry.TextStyle = bellui.TextStyle{Bold: true}
	entry.Refresh()
	test.AssertRendersToMarkup(t, "entry/set_text_style_bold.xml", c)

	entry.TextStyle = bellui.TextStyle{Monospace: true}
	entry.Refresh()
	test.AssertRendersToMarkup(t, "entry/set_text_style_monospace.xml", c)

	entry.TextStyle = bellui.TextStyle{Italic: true}
	entry.Refresh()
	test.AssertRendersToMarkup(t, "entry/set_text_style_italic.xml", c)
}

func TestEntry_Append(t *testing.T) {
	entry := widget.NewEntry()

	entry.Append("abc")
	assert.Equal(t, "abc", entry.Text)
	entry.Append(" def")
	assert.Equal(t, "abc def", entry.Text)

	entry.SetText("")
	entry.MultiLine = true

	entry.Append("first line")
	entry.Append("\nsecond line")
	assert.Equal(t, "first line\nsecond line", entry.Text)
}

func TestEntry_Submit(t *testing.T) {
	t.Run("Callback", func(t *testing.T) {
		var submission string
		entry := &widget.Entry{
			OnSubmitted: func(s string) {
				submission = s
			},
		}
		t.Run("SingleLine_Enter", func(t *testing.T) {
			entry.MultiLine = false
			entry.SetText("a")
			entry.TypedKey(&event.KeyEvent{Name: event.KeyEnter})
			assert.Equal(t, "a", entry.Text)
			assert.Equal(t, "a", submission)
		})
		t.Run("SingleLine_Return", func(t *testing.T) {
			entry.MultiLine = false
			entry.SetText("b")
			entry.TypedKey(&event.KeyEvent{Name: event.KeyReturn})
			assert.Equal(t, "b", entry.Text)
			assert.Equal(t, "b", submission)
		})
		t.Run("MultiLine_ShiftEnter", func(t *testing.T) {
			entry.MultiLine = true
			entry.SetText("c")
			typeKeys(entry, keyShiftLeftDown, event.KeyReturn, keyShiftLeftUp)
			assert.Equal(t, "c", entry.Text)
			assert.Equal(t, "c", submission)
			entry.SetText("d")
			typeKeys(entry, keyShiftRightDown, event.KeyReturn, keyShiftRightUp)
			assert.Equal(t, "d", entry.Text)
			assert.Equal(t, "d", submission)
		})
		t.Run("MultiLine_ShiftReturn", func(t *testing.T) {
			entry.MultiLine = true
			entry.SetText("e")
			typeKeys(entry, keyShiftLeftDown, event.KeyReturn, keyShiftLeftUp)
			assert.Equal(t, "e", entry.Text)
			assert.Equal(t, "e", submission)
			entry.SetText("f")
			typeKeys(entry, keyShiftRightDown, event.KeyReturn, keyShiftRightUp)
			assert.Equal(t, "f", entry.Text)
			assert.Equal(t, "f", submission)
		})
	})
	t.Run("NoCallback", func(t *testing.T) {
		entry := &widget.Entry{}
		resetEntry := func() {
			entry.SetText("")
		}
		t.Run("SingleLine_Enter", func(t *testing.T) {
			resetEntry()
			entry.MultiLine = false
			entry.SetText("a")
			entry.TypedKey(&event.KeyEvent{Name: event.KeyEnter})
			assert.Equal(t, "a", entry.Text)
		})
		t.Run("SingleLine_Return", func(t *testing.T) {
			resetEntry()
			entry.MultiLine = false
			entry.SetText("b")
			entry.TypedKey(&event.KeyEvent{Name: event.KeyReturn})
			assert.Equal(t, "b", entry.Text)
		})
		t.Run("MultiLine_ShiftEnter", func(t *testing.T) {
			resetEntry()
			entry.MultiLine = true
			entry.SetText("c")
			typeKeys(entry, keyShiftLeftDown, event.KeyReturn, keyShiftLeftUp)
			assert.Equal(t, "\nc", entry.Text)
			entry.SetText("d")
			entry.CursorRow = 0
			entry.CursorColumn = 0
			typeKeys(entry, keyShiftRightDown, event.KeyReturn, keyShiftRightUp)
			assert.Equal(t, "\nd", entry.Text)
		})
		t.Run("MultiLine_ShiftReturn", func(t *testing.T) {
			resetEntry()
			entry.MultiLine = true
			entry.SetText("e")
			typeKeys(entry, keyShiftLeftDown, event.KeyReturn, keyShiftLeftUp)
			assert.Equal(t, "\ne", entry.Text)
			entry.SetText("f")
			entry.CursorRow = 0
			entry.CursorColumn = 0
			typeKeys(entry, keyShiftRightDown, event.KeyReturn, keyShiftRightUp)
			assert.Equal(t, "\nf", entry.Text)
		})
	})
}

// TestEntry_OnEscape: Escape fires the cancel hook — the counterpart of
// OnSubmitted for dismissing an inline editor (#1052).
func TestEntry_OnEscape(t *testing.T) {
	t.Run("Callback", func(t *testing.T) {
		escaped := 0
		entry := widget.NewEntry()
		entry.OnEscape = func() { escaped++ }
		entry.SetText("abc")

		entry.TypedKey(&event.KeyEvent{Name: event.KeyEscape})

		assert.Equal(t, 1, escaped)
		assert.Equal(t, "abc", entry.Text, "Escape must not change the text")
	})
	t.Run("NoCallback", func(t *testing.T) {
		entry := widget.NewEntry()
		entry.SetText("abc")

		entry.TypedKey(&event.KeyEvent{Name: event.KeyEscape}) // must not panic

		assert.Equal(t, "abc", entry.Text)
	})
	t.Run("ReadOnly", func(t *testing.T) {
		// A read-only inline editor still cancels on Escape.
		escaped := 0
		entry := widget.NewEntry()
		entry.OnEscape = func() { escaped++ }
		entry.ReadOnly = true

		entry.TypedKey(&event.KeyEvent{Name: event.KeyEscape})

		assert.Equal(t, 1, escaped)
	})
	t.Run("Disabled", func(t *testing.T) {
		escaped := 0
		entry := widget.NewEntry()
		entry.OnEscape = func() { escaped++ }
		entry.Disable()

		entry.TypedKey(&event.KeyEvent{Name: event.KeyEscape})

		assert.Equal(t, 0, escaped, "a disabled entry receives no key handling")
	})
}

func TestTabable(t *testing.T) {
	entry := &widget.Entry{}
	t.Run("Multiline_Tab_Default", func(t *testing.T) {
		entry.MultiLine = true
		entry.SetText("a")
		typeKeys(entry, event.KeyTab)
		assert.Equal(t, "\ta", entry.Text)
	})
	t.Run("Singleline_Tab_Default", func(t *testing.T) {
		entry.MultiLine = false
		assert.False(t, entry.AcceptsTab())
	})
}

func TestEntry_TappedSecondary(t *testing.T) {
	entry, window := setupImageTest(t, false)
	c := window.Canvas()

	tapPos := geo.NewPos(20, 10)
	test.TapSecondaryAt(entry, tapPos)
	test.AssertRendersToMarkup(t, "entry/tapped_secondary_full_menu.xml", c)
	assert.Len(t, c.Overlays().List(), 1)
	c.Overlays().Remove(c.Overlays().Top())

	entry.Disable()
	test.TapSecondaryAt(entry, tapPos)
	test.AssertRendersToMarkup(t, "entry/tapped_secondary_read_menu.xml", c)
	assert.Len(t, c.Overlays().List(), 1)
	c.Overlays().Remove(c.Overlays().Top())

	entry.Password = true
	entry.Refresh()
	test.TapSecondaryAt(entry, tapPos)
	test.AssertRendersToMarkup(t, "entry/tapped_secondary_no_password_menu.xml", c)
	assert.Nil(t, c.Overlays().Top(), "No popup for disabled password")

	entry.Enable()
	test.TapSecondaryAt(entry, tapPos)
	test.AssertRendersToMarkup(t, "entry/tapped_secondary_password_menu.xml", c)
	assert.Len(t, c.Overlays().List(), 1)
}

func TestEntry_TextWrap(t *testing.T) {
	for name, tt := range map[string]struct {
		multiLine bool
		scroll    container.ScrollDirection
		want      string
		wrap      bellui.TextWrap
	}{
		"single line WrapOff": {
			scroll: container.ScrollNone,
			want:   "entry/wrap_single_line_off.xml",
		},
		"single line Truncate": {
			wrap: bellui.TextWrap(bellui.TextTruncateClip),
			want: "entry/wrap_single_line_truncate.xml",
		},
		"single line Scroll": {
			scroll: container.ScrollHorizontalOnly,
			wrap:   bellui.TextWrapOff,
			want:   "entry/wrap_single_line_truncate.xml",
		},
		// Disallowed - fallback to Scrollling (horizontal)
		"single line WrapBreak": {
			wrap: bellui.TextWrapBreak,
			want: "entry/wrap_single_line_truncate.xml",
		},
		// Disallowed - fallback to Scrolling (horizontal)
		"single line WrapWord": {
			wrap: bellui.TextWrapWord,
			want: "entry/wrap_single_line_truncate.xml",
		},
		"multi line WrapOff": {
			scroll:    container.ScrollNone,
			multiLine: true,
			want:      "entry/wrap_multi_line_off.xml",
		},
		// Disallowed - fallback to TextWrapOff
		"multi line Truncate": {
			multiLine: true,
			wrap:      bellui.TextWrap(bellui.TextTruncateClip),
			want:      "entry/wrap_multi_line_truncate.xml",
		},
		"multi line WrapBreak": {
			multiLine: true,
			wrap:      bellui.TextWrapBreak,
			want:      "entry/wrap_multi_line_wrap_break.xml",
		},
		"multi line WrapWord": {
			multiLine: true,
			wrap:      bellui.TextWrapWord,
			want:      "entry/wrap_multi_line_wrap_word.xml",
		},
	} {
		t.Run(name, func(t *testing.T) {
			e, window := setupImageTest(t, tt.multiLine)
			c := window.Canvas()

			c.Focus(e)
			e.Wrapping = tt.wrap
			e.Scroll = tt.scroll
			if tt.multiLine {
				e.SetText("A long text on short words w/o NLs or LFs.")
			} else {
				e.SetText("Testing Wrapping")
			}
			test.AssertRendersToMarkup(t, tt.want, c)
		})
	}
}

func TestEntry_TextWrap_Changed(t *testing.T) {
	e, window := setupImageTest(t, false)
	c := window.Canvas()

	c.Focus(e)
	e.Wrapping = bellui.TextWrapOff
	e.Scroll = container.ScrollNone
	e.SetText("Testing Wrapping")
	test.AssertRendersToMarkup(t, "entry/wrap_single_line_off.xml", c)

	e.Wrapping = bellui.TextWrap(bellui.TextTruncateClip)
	e.Refresh()
	test.AssertRendersToMarkup(t, "entry/wrap_single_line_truncate.xml", c)

	e.Wrapping = bellui.TextWrapOff
	e.Refresh()
	test.AssertRendersToMarkup(t, "entry/wrap_single_line_off.xml", c)
}

func TestEntry_IconSizeAndPlacement(t *testing.T) {
	entry := widget.NewEntry()
	icon := theme.MailComposeIcon()
	entry.SetIcon(icon)
	entry.SetText("SomeText")
	r := test.TempWidgetRenderer(t, entry)
	r.Layout(entry.MinSize())

	iconObj := r.Objects()[3].(*canvas.Image)
	// Icon should be at the left, with correct size
	assert.NotNil(t, iconObj)
	assert.Equal(t, theme.IconInlineSize(), iconObj.Size().Width)
	assert.Equal(t, theme.IconInlineSize(), iconObj.Size().Height)
	assert.Equal(t, geo.NewPos(theme.InnerPadding(), theme.InnerPadding()), iconObj.Position())
	assert.Equal(t, icon, iconObj.Resource)
}

func TestEntry_SetIcon(t *testing.T) {
	entry := widget.NewEntry()
	assert.Nil(t, entry.Icon)
	icon := theme.MailComposeIcon()
	entry.SetIcon(icon)
	assert.Equal(t, icon, entry.Icon)
}

func TestEntry_Icon_MinSize(t *testing.T) {
	entryWithIcon := widget.NewEntry()
	entryWithIcon.SetIcon(theme.MailComposeIcon())
	entryWithIcon.SetText("Test")
	minSizeWithIcon := entryWithIcon.MinSize()

	entryWithoutIcon := widget.NewEntry()
	entryWithoutIcon.SetText("Test")
	minSizeWithoutIcon := entryWithoutIcon.MinSize()

	assert.Greater(t, minSizeWithIcon.Width, minSizeWithoutIcon.Width)
	assert.Equal(t, minSizeWithIcon.Height, minSizeWithoutIcon.Height)
}

func TestEntry_Icon_MinSize_IncreasesWidth(t *testing.T) {
	entry := widget.NewEntry()
	entry.SetText("Test")
	minSizeWithoutIcon := entry.MinSize()

	entry.SetIcon(theme.MailComposeIcon())
	minSizeWithIcon := entry.MinSize()

	assert.Greater(t, minSizeWithIcon.Width, minSizeWithoutIcon.Width)
	assert.Equal(t, minSizeWithIcon.Height, minSizeWithoutIcon.Height)
}

func TestEntry_Icon_ReplaceAndRemove(t *testing.T) {
	entry := widget.NewEntry()
	icon1 := theme.MailComposeIcon()
	icon2 := theme.InfoIcon()
	entry.SetIcon(icon1)
	assert.Equal(t, icon1, entry.Icon)
	entry.Icon = icon2
	assert.Equal(t, icon2, entry.Icon)
	entry.SetIcon(nil)
	assert.Nil(t, entry.Icon)
}

func TestMultiLineEntry_MinSize(t *testing.T) {
	entry := widget.NewEntry()
	singleMin := entry.MinSize()

	multi := widget.NewMultiLineEntry()
	multiMin := multi.MinSize()

	assert.Equal(t, singleMin.Width, multiMin.Width)
	assert.Greater(t, multiMin.Height, singleMin.Height)

	multi.MultiLine = false
	multi.Refresh()
	multiMin = multi.MinSize()
	assert.Equal(t, singleMin.Height, multiMin.Height)
}

func TestNewEntryWithData(t *testing.T) {
	str := binding.NewString()
	err := str.Set("Init")
	assert.NoError(t, err)

	entry := widget.NewEntryWithData(str)
	waitForBinding()
	assert.Equal(t, "Init", entry.Text)

	entry.SetText("Typed")
	v, err := str.Get()
	assert.NoError(t, err)
	assert.Equal(t, "Typed", v)
}

func TestPasswordEntry_ActionItemSizeAndPlacement(t *testing.T) {
	e := widget.NewEntry()
	b := widget.NewButton("", func() {})
	b.Icon = theme.CancelIcon()
	e.ActionItem = b
	test.TempWidgetRenderer(t, e).Layout(e.MinSize())
	assert.Equal(t, theme.IconInlineSize()+theme.InnerPadding()*2, b.Size().Width)
	assert.Greater(t, b.Size().Height, theme.IconInlineSize())
	assert.Equal(t, geo.NewPos(e.MinSize().Width-theme.InputBorderSize()-b.Size().Width, theme.InputBorderSize()), b.Position())
}

func TestPasswordEntry_Disabled(t *testing.T) {
	entry, _ := setupPasswordTest(t)
	entry.Disable()

	test.Tap(entry.ActionItem.(event.Tappable))
	assert.True(t, entry.Password)

	entry.Enable()
	test.Tap(entry.ActionItem.(event.Tappable))
	assert.False(t, entry.Password)
}

func TestPasswordEntry_NewlineIgnored(t *testing.T) {
	entry := widget.NewPasswordEntry()
	entry.SetText("test")

	checkNewlineIgnored(t, entry)
}

func TestPasswordEntry_Obfuscation(t *testing.T) {
	entry, window := setupPasswordTest(t)
	c := window.Canvas()

	test.Type(entry, "Hié™שרה")
	assert.Equal(t, "Hié™שרה", entry.Text)
	test.AssertRendersToMarkup(t, "password_entry/obfuscation_typed.xml", c)
}

func TestPasswordEntry_Placeholder(t *testing.T) {
	entry, window := setupPasswordTest(t)
	c := window.Canvas()

	test.AssertRendersToMarkup(t, "password_entry/initial.xml", window.Canvas())

	entry.SetPlaceHolder("Password")
	test.AssertRendersToMarkup(t, "password_entry/placeholder_initial.xml", c)

	test.Type(entry, "Hié™שרה")
	assert.Equal(t, "Hié™שרה", entry.Text)
	test.AssertRendersToMarkup(t, "password_entry/placeholder_typed.xml", c)
}

func TestPasswordEntry_Reveal(t *testing.T) {
	test.NewTempApp(t)

	t.Run("NewPasswordEntry constructor", func(t *testing.T) {
		entry := widget.NewPasswordEntry()
		window := test.NewWindow(entry)
		defer window.Close()
		window.Resize(geo.NewSize(150, 100))
		entry.Resize(entry.MinSize().Max(geo.NewSize(130, 0)))
		entry.Move(geo.NewPos(10, 10))
		c := window.Canvas()

		test.AssertRendersToMarkup(t, "password_entry/initial.xml", c)

		c.Focus(entry)
		test.Type(entry, "Secret")
		assert.Equal(t, "Secret", entry.Text)
		test.AssertRendersToMarkup(t, "password_entry/concealed_typed.xml", c)

		// update the Password field
		entry.Password = false
		entry.Refresh()
		assert.Equal(t, "Secret", entry.Text)
		test.AssertRendersToMarkup(t, "password_entry/revealed_toggled.xml", c)
		assert.Equal(t, entry, c.Focused())

		// update the Password field
		entry.Password = true
		entry.Refresh()
		assert.Equal(t, "Secret", entry.Text)
		test.AssertRendersToMarkup(t, "password_entry/concealed_toggled.xml", c)
		assert.Equal(t, entry, c.Focused())

		// tap on action icon
		tapPos := geo.NewPos(140-theme.InnerPadding()-theme.IconInlineSize()/2, 10+entry.Size().Height/2)
		test.TapCanvas(c, tapPos)
		assert.Equal(t, "Secret", entry.Text)
		test.AssertRendersToMarkup(t, "password_entry/revealed_tapped.xml", c)
		assert.Equal(t, entry, c.Focused())

		// tap on action icon
		test.TapCanvas(c, tapPos)
		assert.Equal(t, "Secret", entry.Text)
		test.AssertRendersToMarkup(t, "password_entry/concealed_tapped.xml", c)
		assert.Equal(t, entry, c.Focused())
	})

	// This test cover backward compatibility use case when on an Entry widget
	// the Password field is set to true.
	// In this case the action item will be set when the renderer is created.
	t.Run("Entry with Password field", func(t *testing.T) {
		entry := &widget.Entry{Password: true, Wrapping: bellui.TextWrapWord}
		entry.Refresh()
		window := test.NewWindow(entry)
		defer window.Close()
		window.Resize(geo.NewSize(150, 100))
		entry.Resize(entry.MinSize().Max(geo.NewSize(130, 0)))
		entry.Move(geo.NewPos(10, 10))
		c := window.Canvas()

		test.AssertRendersToMarkup(t, "password_entry/initial.xml", c)

		c.Focus(entry)
		test.Type(entry, "Secret")
		assert.Equal(t, "Secret", entry.Text)
		test.AssertRendersToMarkup(t, "password_entry/concealed_typed.xml", c)

		// update the Password field
		entry.Password = false
		entry.Refresh()
		assert.Equal(t, "Secret", entry.Text)
		test.AssertRendersToMarkup(t, "password_entry/revealed_toggled.xml", c)
		assert.Equal(t, entry, c.Focused())
	})
}

func TestSingleLineEntry_NewlineIgnored(t *testing.T) {
	entry := &widget.Entry{MultiLine: false}
	entry.SetText("test")

	checkNewlineIgnored(t, entry)
}

func TestSingleLineEntry_SelectionSubmitted(t *testing.T) {
	entry := widget.NewEntry()
	entry.SetText("abc")
	assert.Equal(t, "", entry.SelectedText())
	entry.TypedShortcut(&bellui.ShortcutSelectAll{})
	assert.Equal(t, "abc", entry.SelectedText())
	entry.TypedKey(&event.KeyEvent{Name: event.KeyEnter})
	assert.Equal(t, "abc", entry.Text)
}

func TestMultiLineEntry_EnterWithSelection(t *testing.T) {
	entry := widget.NewMultiLineEntry()
	entry.SetText("abc")
	assert.Equal(t, "", entry.SelectedText())
	entry.TypedShortcut(&bellui.ShortcutSelectAll{})
	assert.Equal(t, "abc", entry.SelectedText())
	entry.TypedKey(&event.KeyEvent{Name: event.KeyEnter})
	assert.Equal(t, "\n", entry.Text)
}

func TestEntry_CarriageReturn(t *testing.T) {
	entry := widget.NewMultiLineEntry()
	entry.Wrapping = bellui.TextWrapOff
	entry.Scroll = container.ScrollNone
	entry.SetText("\r\n\r")
	w := test.NewTempWindow(t, entry)
	w.Resize(geo.NewSize(64, 64))
	test.AssertImageMatches(t, "entry/carriage_return_empty.png", w.Canvas().Capture())
	entry.SetText("\rH\re\rl\rl\ro\r\n\rW\ro\rr\rl\rd\r!\r")
	test.AssertImageMatches(t, "entry/carriage_return_text.png", w.Canvas().Capture())
}

func TestEntry_UndoRedo_TypeRune(t *testing.T) {
	entry := widget.NewEntry()

	// Check undo when there is nothing to undo
	entry.TypedShortcut(&bellui.ShortcutUndo{})
	assert.Equal(t, "", entry.Text)

	for _, r := range "abc éàè 123" {
		entry.TypedRune(r)
	}

	assert.Equal(t, "abc éàè 123", entry.Text)

	// Check redo when there is nothing to redo
	entry.TypedShortcut(&bellui.ShortcutRedo{})
	assert.Equal(t, "abc éàè 123", entry.Text)

	entry.TypedShortcut(&bellui.ShortcutUndo{})
	assert.Equal(t, "abc éàè", entry.Text)

	entry.TypedShortcut(&bellui.ShortcutUndo{})
	assert.Equal(t, "abc", entry.Text)

	entry.TypedShortcut(&bellui.ShortcutUndo{})
	assert.Equal(t, "", entry.Text)

	// Check undo when there is nothing to undo
	entry.TypedShortcut(&bellui.ShortcutUndo{})
	assert.Equal(t, "", entry.Text)

	entry.TypedShortcut(&bellui.ShortcutRedo{})
	assert.Equal(t, "abc", entry.Text)

	entry.TypedShortcut(&bellui.ShortcutRedo{})
	assert.Equal(t, "abc éàè", entry.Text)

	entry.TypedShortcut(&bellui.ShortcutRedo{})
	assert.Equal(t, "abc éàè 123", entry.Text)

	// Check redo when there is nothing to redo
	entry.TypedShortcut(&bellui.ShortcutRedo{})
	assert.Equal(t, "abc éàè 123", entry.Text)
}

func TestEntry_UndoRedo_Delete(t *testing.T) {
	entry := widget.NewEntry()

	// Check Undo when there is nothing to undo
	entry.TypedShortcut(&bellui.ShortcutUndo{})
	assert.Equal(t, "", entry.Text)

	for _, r := range "àbcdéf" {
		entry.TypedRune(r)
	}
	assert.Equal(t, "àbcdéf", entry.Text)

	entry.TypedKey(&event.KeyEvent{Name: event.KeyLeft})
	entry.TypedKey(&event.KeyEvent{Name: event.KeyLeft})
	entry.TypedKey(&event.KeyEvent{Name: event.KeyBackspace})
	entry.TypedKey(&event.KeyEvent{Name: event.KeyBackspace})
	entry.TypedKey(&event.KeyEvent{Name: event.KeyDelete})

	assert.Equal(t, "àbf", entry.Text)

	entry.TypedShortcut(&bellui.ShortcutUndo{})
	assert.Equal(t, "àbéf", entry.Text)

	entry.TypedShortcut(&bellui.ShortcutUndo{})
	assert.Equal(t, "àbcdéf", entry.Text)

	entry.TypedShortcut(&bellui.ShortcutRedo{})
	assert.Equal(t, "àbéf", entry.Text)

	entry.TypedShortcut(&bellui.ShortcutRedo{})
	assert.Equal(t, "àbf", entry.Text)
}

func TestEntry_UndoRedo_DeleteWord(t *testing.T) {
	entry := widget.NewMultiLineEntry()

	for _, r := range "Line 1\nline 2" {
		entry.TypedRune(r)
	}
	assert.Equal(t, "Line 1\nline 2", entry.Text)

	moveWordModifier := event.KeyModifierShortcutDefault
	if runtime.GOOS == "darwin" {
		moveWordModifier = event.KeyModifierAlt
	}

	entry.TypedShortcut(&desktop.CustomShortcut{KeyName: event.KeyBackspace, Modifier: moveWordModifier})
	assert.Equal(t, "Line 1\n", entry.Text)

	entry.TypedShortcut(&bellui.ShortcutUndo{})
	assert.Equal(t, "Line 1\nline 2", entry.Text)
}

func TestEntry_UndoRedo_Replace(t *testing.T) {
	entry := widget.NewEntry()

	entry.SetText("àbcdéf")
	typeKeys(entry, event.KeyRight, event.KeyRight, keyShiftLeftDown, event.KeyRight, event.KeyRight, keyShiftLeftUp)
	assert.Equal(t, "cd", entry.SelectedText())

	entry.TypedRune('z')
	assert.Equal(t, "àbzéf", entry.Text)

	entry.TypedShortcut(&bellui.ShortcutUndo{})
	assert.Equal(t, "àbéf", entry.Text)

	entry.TypedShortcut(&bellui.ShortcutUndo{})
	assert.Equal(t, "àbcdéf", entry.Text)

	entry.TypedShortcut(&bellui.ShortcutRedo{})
	assert.Equal(t, "àbéf", entry.Text)

	entry.TypedShortcut(&bellui.ShortcutRedo{})
	assert.Equal(t, "àbzéf", entry.Text)
}

func TestEntry_UndoRedoImage(t *testing.T) {
	e, window := setupImageTest(t, true)
	window.Resize(geo.NewSize(128, 128))
	c := window.Canvas()

	c.Focus(e)
	runes := "The undo/\nredo function allows you to efficiently fix"
	for _, r := range runes {
		e.TypedRune(r)
	}
	test.AssertImageMatches(t, "entry/undo_redo_initial.png", window.Canvas().Capture())

	for _, r := range " mistkaes" {
		e.TypedRune(r)
	}
	test.AssertImageMatches(t, "entry/undo_redo_mistkaes.png", window.Canvas().Capture())

	e.TypedShortcut(&bellui.ShortcutUndo{})
	test.AssertImageMatches(t, "entry/undo_redo_after_undo.png", window.Canvas().Capture())

	for _, r := range " mistakes" {
		e.TypedRune(r)
	}
	test.AssertImageMatches(t, "entry/undo_redo_mistake_corrected.png", window.Canvas().Capture())

	for i := 0; i < 5; i++ {
		e.TypedShortcut(&bellui.ShortcutUndo{})
	}
	test.AssertImageMatches(t, "entry/undo_redo_5undo.png", window.Canvas().Capture())

	for i := 0; i < 5; i++ {
		e.TypedShortcut(&bellui.ShortcutRedo{})
	}
	test.AssertImageMatches(t, "entry/undo_redo_after_redo.png", window.Canvas().Capture())
}

func TestEntry_UndoRedo_Callback(t *testing.T) {
	entry := widget.NewEntry()
	changed := ""
	entry.OnChanged = func(s string) {
		changed = s
	}

	for _, r := range "abc éàè 123" {
		entry.TypedRune(r)
	}

	assert.Equal(t, "abc éàè 123", entry.Text)
	assert.Equal(t, "abc éàè 123", changed)

	entry.TypedShortcut(&bellui.ShortcutUndo{})
	assert.Equal(t, "abc éàè", entry.Text)
	assert.Equal(t, "abc éàè", changed)

	entry.TypedShortcut(&bellui.ShortcutUndo{})
	assert.Equal(t, "abc", entry.Text)
	assert.Equal(t, "abc", changed)

	entry.TypedShortcut(&bellui.ShortcutUndo{})
	assert.Equal(t, "", entry.Text)
	assert.Equal(t, "", changed)

	entry.TypedShortcut(&bellui.ShortcutRedo{})
	assert.Equal(t, "abc", entry.Text)
	assert.Equal(t, "abc", changed)

	entry.TypedShortcut(&bellui.ShortcutRedo{})
	assert.Equal(t, "abc éàè", entry.Text)
	assert.Equal(t, "abc éàè", changed)

	entry.TypedShortcut(&bellui.ShortcutRedo{})
	assert.Equal(t, "abc éàè 123", entry.Text)
	assert.Equal(t, "abc éàè 123", changed)
}

func TestEntry_SelectAPI(t *testing.T) {
	entry := widget.NewMultiLineEntry()
	entry.SetText("Hello\nWorld\nBellui")
	w := test.NewTempWindow(t, entry)
	w.Resize(geo.NewSize(200, 200))

	// Select "llo" from row 0 col 2 to row 0 col 5
	entry.Select(0, 2, 0, 5)
	assert.Equal(t, "llo", entry.SelectedText())

	// Select across lines: "llo\nWor"
	entry.Select(0, 2, 1, 3)
	assert.Equal(t, "llo\nWor", entry.SelectedText())

	// Select entire text
	entry.Select(0, 0, 2, 6)
	assert.Equal(t, "Hello\nWorld\nBellui", entry.SelectedText())

	// Select single character
	entry.Select(1, 0, 1, 1)
	assert.Equal(t, "W", entry.SelectedText())
}

func TestEntry_Select_OutOfRange(t *testing.T) {
	entry := widget.NewMultiLineEntry()
	entry.SetText("Hello\nWorld")
	w := test.NewTempWindow(t, entry)
	w.Resize(geo.NewSize(200, 200))

	// Negative row and col should be clamped to 0
	assert.NotPanics(t, func() {
		entry.Select(-1, -5, 0, 3)
	})
	assert.Equal(t, "Hel", entry.SelectedText())

	// Too-large row should be clamped to last row
	assert.NotPanics(t, func() {
		entry.Select(0, 0, 100, 100)
	})
	assert.Equal(t, "Hello\nWorld", entry.SelectedText())

	// Too-large col on valid row should be clamped to row length
	assert.NotPanics(t, func() {
		entry.Select(0, 0, 0, 999)
	})
	assert.Equal(t, "Hello", entry.SelectedText())
}

func TestEntry_Select_EmptyEntry(t *testing.T) {
	entry := widget.NewMultiLineEntry()
	w := test.NewTempWindow(t, entry)
	w.Resize(geo.NewSize(200, 200))

	assert.NotPanics(t, func() {
		entry.Select(0, 0, 0, 0)
	})
	assert.Equal(t, "", entry.SelectedText())

	assert.NotPanics(t, func() {
		entry.Select(0, 0, 5, 5)
	})
	assert.Equal(t, "", entry.SelectedText())
}

func TestEntry_AddShortcut(t *testing.T) {
	entry := widget.NewEntry()
	entry.SetText("Testing")
	w := test.NewTempWindow(t, entry)
	w.Resize(geo.NewSize(200, 200))
	c := w.Canvas()
	c.Focus(entry)

	fired := false
	shortcut := &desktop.CustomShortcut{KeyName: event.KeyB, Modifier: event.KeyModifierShortcutDefault}
	entry.AddShortcut(shortcut, func(s core.Shortcut) {
		fired = true
	})

	assert.False(t, fired)
	entry.TypedShortcut(shortcut)
	assert.True(t, fired)
}

func TestEntry_KeepSelectionVisible(t *testing.T) {
	entry := widget.NewMultiLineEntry()
	entry.SetText("Hello\nWorld\nBellui")
	entry.KeepSelectionVisible = true
	w := test.NewTempWindow(t, entry)
	w.Resize(geo.NewSize(200, 200))
	c := w.Canvas()

	c.Focus(entry)
	// Clear any select-all from FocusGained (single-line behaviour);
	// multi-line should not select all, but be safe.
	entry.TypedKey(&event.KeyEvent{Name: event.KeyRight})

	// Programmatically select "World"
	entry.Select(1, 0, 1, 5)
	assert.Equal(t, "World", entry.SelectedText())

	// Lose focus
	c.Unfocus()

	// With KeepSelectionVisible the selected text should still be reported
	assert.Equal(t, "World", entry.SelectedText())
}

func TestEntry_MultiLine_FocusGained_NoSelectAll(t *testing.T) {
	// Multi-line entry should NOT select all text on FocusGained
	multi := widget.NewMultiLineEntry()
	multi.SetText("Line1\nLine2\nLine3")
	w := test.NewTempWindow(t, multi)
	w.Resize(geo.NewSize(200, 200))
	c := w.Canvas()

	c.Focus(multi)
	assert.Equal(t, "", multi.SelectedText())

	c.Unfocus()

	// Single-line entry SHOULD select all text on FocusGained
	single := widget.NewEntry()
	single.SetText("Hello")
	w.SetContent(single)
	w.Resize(geo.NewSize(200, 200))

	c.Focus(single)
	assert.Equal(t, "Hello", single.SelectedText())
}

func TestEntry_OnSelectChanged(t *testing.T) {
	entry := widget.NewMultiLineEntry()
	entry.SetText("Hello\nWorld")
	w := test.NewTempWindow(t, entry)
	w.Resize(geo.NewSize(200, 200))

	var lastSelection string
	callCount := 0
	entry.OnSelectChanged = func(sel string) {
		lastSelection = sel
		callCount++
	}

	c := w.Canvas()
	c.Focus(entry)

	// Select() should fire the callback.
	entry.Select(0, 0, 0, 5)
	assert.Equal(t, "Hello", lastSelection)
	assert.Equal(t, 1, callCount)

	// Select() with different range should fire again.
	entry.Select(0, 0, 1, 5)
	assert.Equal(t, "Hello\nWorld", lastSelection)
	assert.Equal(t, 2, callCount)
}

func TestEntry_OnGutterTap(t *testing.T) {
	test.NewApp()
	t.Cleanup(func() { test.NewApp() })

	var tappedLine int
	entry := &widget.Entry{
		MultiLine: true,
		Wrapping:  bellui.TextWrapOff,
		Scroll:    container.ScrollNone,
		LineHighlighter: func(line string, state int) ([]widget.StyleRun, int) {
			return []widget.StyleRun{{Text: line}}, 0
		},
		OnGutterTap: func(line int) {
			tappedLine = line
		},
	}

	w := test.NewTempWindow(t, entry)
	w.Resize(geo.NewSize(300, 200))
	entry.Resize(geo.NewSize(280, 180))
	entry.Move(geo.NewPos(entryOffset, entryOffset))

	entry.SetText("Line one\nLine two\nLine three")

	// Click in the gutter area on the second line.
	// The gutter is on the left side of the entry; clicking at a small X
	// offset (relative to the entry) lands inside it.
	testCharSize := theme.TextSize()
	gutterX := float32(entryOffset) + 5 // well inside the gutter
	lineY := float32(entryOffset) + theme.InnerPadding() + testCharSize*1.5
	clickCanvas(w.Canvas(), geo.NewPos(gutterX, lineY))

	assert.Equal(t, 2, tappedLine, "OnGutterTap should fire with 1-based line number 2")
}

func TestEntry_BreakpointLines(t *testing.T) {
	test.NewApp()
	t.Cleanup(func() { test.NewApp() })

	entry := &widget.Entry{
		MultiLine: true,
		Wrapping:  bellui.TextWrapOff,
		Scroll:    container.ScrollNone,
		LineHighlighter: func(line string, state int) ([]widget.StyleRun, int) {
			return []widget.StyleRun{{Text: line}}, 0
		},
		BreakpointLines: map[int]bool{1: true, 3: true},
	}

	w := test.NewTempWindow(t, entry)
	w.Resize(geo.NewSize(300, 200))
	entry.Resize(geo.NewSize(280, 180))
	entry.Move(geo.NewPos(entryOffset, entryOffset))

	entry.SetText("alpha\nbeta\ngamma\ndelta")

	// Refresh should not panic with breakpoints set.
	assert.NotPanics(t, func() { entry.Refresh() })

	// Update breakpoints and refresh via RefreshGutter.
	entry.BreakpointLines = map[int]bool{2: true}
	assert.NotPanics(t, func() { entry.RefreshGutter() })
}

func TestEntry_HighlightedRow(t *testing.T) {
	test.NewApp()
	t.Cleanup(func() { test.NewApp() })

	entry := &widget.Entry{
		MultiLine: true,
		Wrapping:  bellui.TextWrapOff,
		Scroll:    container.ScrollNone,
		LineHighlighter: func(line string, state int) ([]widget.StyleRun, int) {
			return []widget.StyleRun{{Text: line}}, 0
		},
		HighlightedRow: 2,
	}

	w := test.NewTempWindow(t, entry)
	w.Resize(geo.NewSize(300, 200))
	entry.Resize(geo.NewSize(280, 180))
	entry.Move(geo.NewPos(entryOffset, entryOffset))

	entry.SetText("one\ntwo\nthree")

	// Refresh should not panic with a highlighted row set.
	assert.NotPanics(t, func() { entry.Refresh() })

	// Change highlighted row and use RefreshGutter.
	entry.HighlightedRow = 3
	assert.NotPanics(t, func() { entry.RefreshGutter() })

	// Disable highlighting.
	entry.HighlightedRow = 0
	assert.NotPanics(t, func() { entry.RefreshGutter() })
}

func TestEntry_GutterClickNoSelection(t *testing.T) {
	test.NewApp()
	t.Cleanup(func() { test.NewApp() })

	entry := &widget.Entry{
		MultiLine: true,
		Wrapping:  bellui.TextWrapOff,
		Scroll:    container.ScrollNone,
		LineHighlighter: func(line string, state int) ([]widget.StyleRun, int) {
			return []widget.StyleRun{{Text: line}}, 0
		},
		OnGutterTap: func(line int) {
			// no-op; just need the callback so gutter clicks are intercepted
		},
	}

	w := test.NewTempWindow(t, entry)
	w.Resize(geo.NewSize(300, 200))
	entry.Resize(geo.NewSize(280, 180))
	entry.Move(geo.NewPos(entryOffset, entryOffset))

	entry.SetText("First line\nSecond line\nThird line")
	w.Canvas().Focus(entry)

	// Record cursor position before the gutter click.
	rowBefore := entry.CursorRow
	colBefore := entry.CursorColumn
	selBefore := entry.SelectedText()

	// Click in the gutter area.
	testCharSize := theme.TextSize()
	gutterX := float32(entryOffset) + 5
	lineY := float32(entryOffset) + theme.InnerPadding() + testCharSize*1.5
	clickCanvas(w.Canvas(), geo.NewPos(gutterX, lineY))

	// Cursor and selection should not have changed.
	assert.Equal(t, rowBefore, entry.CursorRow, "gutter click should not move cursor row")
	assert.Equal(t, colBefore, entry.CursorColumn, "gutter click should not move cursor column")
	assert.Equal(t, selBefore, entry.SelectedText(), "gutter click should not change selection")
}

const (
	entryOffset = 10

	keyShiftLeftDown  event.KeyName = "LeftShiftDown"
	keyShiftLeftUp    event.KeyName = "LeftShiftUp"
	keyShiftRightDown event.KeyName = "RightShiftDown"
	keyShiftRightUp   event.KeyName = "RightShiftUp"
)

var typeKeys = func(e *widget.Entry, keys ...event.KeyName) {
	keyDown := func(key *event.KeyEvent) {
		e.KeyDown(key)
		e.TypedKey(key)
	}

	for _, key := range keys {
		switch key {
		case keyShiftLeftDown:
			keyDown(&event.KeyEvent{Name: desktop.KeyShiftLeft})
		case keyShiftLeftUp:
			e.KeyUp(&event.KeyEvent{Name: desktop.KeyShiftLeft})
		case keyShiftRightDown:
			keyDown(&event.KeyEvent{Name: desktop.KeyShiftRight})
		case keyShiftRightUp:
			e.KeyUp(&event.KeyEvent{Name: desktop.KeyShiftRight})
		default:
			keyDown(&event.KeyEvent{Name: key})
			e.KeyUp(&event.KeyEvent{Name: key})
		}
	}
}

func checkNewlineIgnored(t *testing.T, entry *widget.Entry) {
	assert.Equal(t, 0, entry.CursorRow)

	// only 1 line, do nothing
	down := &event.KeyEvent{Name: event.KeyDown}
	entry.TypedKey(down)
	assert.Equal(t, 0, entry.CursorRow)

	// return is ignored, do nothing
	ret := &event.KeyEvent{Name: event.KeyReturn}
	entry.TypedKey(ret)
	assert.Equal(t, 0, entry.CursorRow)

	up := &event.KeyEvent{Name: event.KeyUp}
	entry.TypedKey(up)
	assert.Equal(t, 0, entry.CursorRow)

	// don't go beyond top
	entry.TypedKey(up)
	assert.Equal(t, 0, entry.CursorRow)
}

func setupImageTest(t *testing.T, multiLine bool) (*widget.Entry, bellui.Window) {
	test.NewApp()
	t.Cleanup(func() { test.NewApp() })

	var entry *widget.Entry
	if multiLine {
		entry = &widget.Entry{MultiLine: true, Wrapping: bellui.TextWrapWord, Scroll: container.ScrollNone}
	} else {
		entry = &widget.Entry{Wrapping: bellui.TextWrapOff, Scroll: container.ScrollNone}
	}
	w := test.NewTempWindow(t, entry)
	w.Resize(geo.NewSize(150, 200))

	if multiLine {
		entry.Resize(geo.NewSize(120, 100))
	} else {
		entry.Resize(entry.MinSize().Max(geo.NewSize(120, 0)))
	}
	entry.Move(geo.NewPos(10, 10))

	if multiLine {
		test.AssertRendersToMarkup(t, "entry/initial_multiline.xml", w.Canvas())
	} else {
		test.AssertRendersToMarkup(t, "entry/initial.xml", w.Canvas())
	}

	return entry, w
}

func setupPasswordTest(t *testing.T) (*widget.Entry, bellui.Window) {
	test.NewApp()
	t.Cleanup(func() { test.NewApp() })

	entry := widget.NewPasswordEntry()
	w := test.NewTempWindow(t, entry)
	w.Resize(geo.NewSize(150, 100))

	entry.Resize(entry.MinSize().Max(geo.NewSize(130, 0)))
	entry.Move(geo.NewPos(entryOffset, entryOffset))

	return entry, w
}

// Selects "sti" on line 2 of a new multiline
// T e s t i n g
// T e[s t i]n g
// T e s t i n g
func setupSelection(t *testing.T, reverse bool) (*widget.Entry, bellui.Window) {
	e, window := setupImageTest(t, true)
	e.SetText("Testing\nTesting\nTesting")
	c := window.Canvas()
	c.Focus(e)
	// Clear select-all triggered by FocusGained
	e.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	if reverse {
		e.CursorRow = 1
		e.CursorColumn = 5
		typeKeys(e, keyShiftLeftDown, event.KeyLeft, event.KeyLeft, event.KeyLeft)
		test.AssertRendersToMarkup(t, "entry/selection_initial_reverse.xml", c)
		assert.Equal(t, "sti", e.SelectedText())
	} else {
		e.CursorRow = 1
		e.CursorColumn = 2
		typeKeys(e, keyShiftLeftDown, event.KeyRight, event.KeyRight, event.KeyRight)
		test.AssertRendersToMarkup(t, "entry/selection_initial.xml", c)
		assert.Equal(t, "sti", e.SelectedText())
	}

	return e, window
}

// typeOnFocused types text into an already-focused entry without triggering
// FocusGained (which calls selectAll). Use instead of test.Type when the entry
// is already focused and text should be appended, not replaced.
func typeOnFocused(e *widget.Entry, text string) {
	for _, r := range text {
		e.TypedRune(r)
	}
}

func waitForBinding() {
	time.Sleep(time.Millisecond * 100) // data resolves on background thread
}

// clickCanvas is an analogue of test.TapCanvas that also sends MouseDown/MouseUp events
func clickCanvas(c core.Canvas, pos geo.Position) {
	if o, p := findMouseable(c, pos); o != nil {
		clickPrimary(c, o.(desktop.Mouseable), &event.PointEvent{AbsolutePosition: pos, Position: p})
	}
}

func findMouseable(c core.Canvas, pos geo.Position) (o core.CanvasObject, p geo.Position) {
	matches := func(object core.CanvasObject) bool {
		_, ok := object.(desktop.Mouseable)
		return ok
	}
	o, p, _ = driver.FindObjectAtPositionMatching(pos, matches, c.Overlays().List(), c.Content())
	return o, p
}

func clickPrimary(c core.Canvas, obj desktop.Mouseable, ev *event.PointEvent) {
	handleFocusOnTap(c, obj)
	mouseEvent := &desktop.MouseEvent{
		PointEvent: *ev,
		Button:     desktop.MouseButtonPrimary,
	}
	obj.MouseDown(mouseEvent)
	obj.MouseUp(mouseEvent)
	if tap, ok := obj.(event.Tappable); ok {
		tap.Tapped(ev)
	}
}

func handleFocusOnTap(c core.Canvas, obj any) {
	if c == nil {
		return
	}
	unfocus := true
	if focus, ok := obj.(event.Focusable); ok {
		if dis, ok := obj.(core.Disableable); !ok || !dis.Disabled() {
			unfocus = false
			if focus != c.Focused() {
				unfocus = true
			}
		}
	}
	if unfocus {
		c.Unfocus()
	}
}
