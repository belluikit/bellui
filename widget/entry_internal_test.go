package widget

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	intWidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// TestEntry_PageScrollsViewport verifies that PageDown/PageUp move the scroll
// viewport (not just the caret) so a large document can be paged through with
// the keyboard (#377).
func TestEntry_PageScrollsViewport(t *testing.T) {
	test.NewTempApp(t)
	e := NewMultiLineEntry()
	e.Wrapping = bellui.TextWrapOff
	w := test.NewTempWindow(t, e)
	w.Resize(geo.NewSize(220, 140))
	e.Resize(geo.NewSize(200, 100)) // viewport shows only a handful of rows
	w.Canvas().Focus(e)

	var sb strings.Builder
	for i := 0; i < 60; i++ {
		fmt.Fprintf(&sb, "row %02d\n", i)
	}
	e.SetText(strings.TrimSuffix(sb.String(), "\n")) // rows 0..59
	e.CursorRow, e.CursorColumn = 0, 0
	e.Refresh()

	assert.NotNil(t, e.scroll)
	assert.Zero(t, e.scroll.Offset.Y, "starts scrolled to the top")

	e.TypedKey(&event.KeyEvent{Name: event.KeyPageDown})
	afterOne := e.scroll.Offset.Y
	assert.Greater(t, afterOne, float32(0), "PageDown scrolls the viewport down")

	e.TypedKey(&event.KeyEvent{Name: event.KeyPageDown})
	afterTwo := e.scroll.Offset.Y
	assert.Greater(t, afterTwo, afterOne, "a second PageDown scrolls further")

	e.TypedKey(&event.KeyEvent{Name: event.KeyPageUp})
	assert.Less(t, e.scroll.Offset.Y, afterTwo, "PageUp scrolls the viewport back up")
}

func clickPrimary(e *Entry, ev *event.PointEvent) {
	mouseEvent := &desktop.MouseEvent{
		PointEvent: *ev,
		Button:     desktop.MouseButtonPrimary,
	}
	e.MouseDown(mouseEvent)
	e.MouseUp(mouseEvent)
	e.Tapped(ev) // in the glfw driver there is a double click delay before Tapped()
}

func TestEntry_Cursor(t *testing.T) {
	entry := NewEntry()
	assert.Equal(t, desktop.TextCursor, entry.Cursor())
}

func TestEntry_HideShadows_SingleVsMultiLine(t *testing.T) {
	// A single-line entry clips overflowing text and must suppress the scroll's
	// edge fade-shadows (#357); a multi-line entry keeps them as a "more
	// content" affordance.
	single := NewEntry()
	cache.Renderer(single) // build the renderer so e.scroll is created
	assert.True(t, single.scroll.HideShadows)

	multi := NewMultiLineEntry()
	cache.Renderer(multi)
	assert.False(t, multi.scroll.HideShadows)
}

func TestEntry_DoubleTapped(t *testing.T) {
	entry := NewEntry()
	entry.Wrapping = bellui.TextWrapOff
	entry.Scroll = intWidget.ScrollNone
	entry.SetText("The quick brown fox\njumped    over the lazy dog\n")
	entry.Resize(entry.MinSize())

	// select the word 'quick'
	ev := getClickPosition("The qui", 0)
	clickPrimary(entry, ev)
	entry.DoubleTapped(ev)
	assert.Equal(t, "quick", entry.SelectedText())

	entry.sel.doubleTappedAtUnixMillis = 0 // make sure we don't register a triple tap next

	// select the whitespace after 'quick'
	ev = getClickPosition("The quick", 0)
	clickPrimary(entry, ev)
	entry.DoubleTapped(ev)
	assert.Equal(t, " ", entry.SelectedText())

	entry.sel.doubleTappedAtUnixMillis = 0

	// select all whitespace after 'jumped'
	ev = getClickPosition("jumped  ", 1)
	clickPrimary(entry, ev)
	entry.DoubleTapped(ev)
	assert.Equal(t, "    ", entry.SelectedText())
}

func TestEntry_DoubleTapped_AfterCol(t *testing.T) {
	entry := NewEntry()
	entry.SetText("A\nB\n")

	window := test.NewWindow(entry)
	defer window.Close()
	window.SetPadded(false)
	window.Resize(entry.MinSize())
	entry.Resize(entry.MinSize())
	c := window.Canvas()

	ev := getClickPosition("", 0)
	clickPrimary(entry, ev)
	assert.Equal(t, entry, c.Focused())

	testCharSize := theme.TextSize()
	pos := geo.NewPos(testCharSize, testCharSize*4) // tap below rows
	ev = &event.PointEvent{Position: pos}
	clickPrimary(entry, ev)
	entry.DoubleTapped(ev)

	assert.Equal(t, "", entry.SelectedText())
}

func TestEntry_DragSelect(t *testing.T) {
	entry := NewEntry()
	entry.Wrapping = bellui.TextWrapOff
	entry.Scroll = intWidget.ScrollNone
	entry.SetText("The quick brown fox jumped\nover the lazy dog\nThe quick\nbrown fox\njumped over the lazy dog\n")
	entry.Resize(entry.MinSize())

	// get position after the letter 'e' on the second row
	ev1 := getClickPosition("ove", 1)
	// get position after the letter 'z' on the second row
	ev2 := getClickPosition("over the laz", 1)

	// mouse down and drag from 'r' to 'z'
	me := &desktop.MouseEvent{PointEvent: *ev1, Button: desktop.MouseButtonPrimary}
	entry.MouseDown(me)
	for ; ev1.Position.X < ev2.Position.X; ev1.Position.X++ {
		de := &event.DragEvent{PointEvent: *ev1, Dragged: geo.NewDelta(1, 0)}
		entry.Dragged(de)
	}
	me = &desktop.MouseEvent{PointEvent: *ev1, Button: desktop.MouseButtonPrimary}
	entry.MouseUp(me)

	assert.Equal(t, "r the laz", entry.SelectedText())
}

func TestEntry_DragSelectLargeStep(t *testing.T) {
	entry := NewEntry()
	entry.Wrapping = bellui.TextWrapOff
	entry.Scroll = intWidget.ScrollNone
	entry.SetText("The quick brown fox jumped\nover the lazy dog\nThe quick\nbrown fox\njumped over the lazy dog\n")
	entry.Resize(entry.MinSize())

	// get position after the letter 'e' on the second row
	ev1 := getClickPosition("ove", 1)
	// get position after the letter 'z' on the second row
	ev2 := getClickPosition("over the laz", 1)

	// mouse down and drag from 'r' to 'z'
	me := &desktop.MouseEvent{PointEvent: *ev1, Button: desktop.MouseButtonPrimary}
	entry.MouseDown(me)

	delta := ev2.Position.Subtract(ev1.Position)
	de := &event.DragEvent{PointEvent: *ev2, Dragged: geo.NewDelta(delta.X, delta.Y)}
	entry.Dragged(de)

	me = &desktop.MouseEvent{PointEvent: *ev2, Button: desktop.MouseButtonPrimary}
	entry.MouseUp(me)

	assert.Equal(t, "r the laz", entry.SelectedText())
}

func TestEntry_DragSelectEmpty(t *testing.T) {
	entry := NewEntry()
	entry.SetText("Testing")

	ev1 := getClickPosition("T", 0)
	ev2 := getClickPosition("Testing", 0)

	// Test empty selection - drag from 'e' to 'e' (empty)
	de := &event.DragEvent{PointEvent: *ev1, Dragged: geo.NewDelta(1, 0)}
	entry.Dragged(de)
	de = &event.DragEvent{PointEvent: *ev1, Dragged: geo.NewDelta(1, 0)}
	entry.Dragged(de)

	assert.True(t, entry.sel.selecting)

	entry.DragEnd()
	assert.Equal(t, "", entry.SelectedText())
	assert.False(t, entry.sel.selecting)

	// Test non-empty selection - drag from 'T' to 'g' (empty)
	ev1 = getClickPosition("", 0)
	de = &event.DragEvent{PointEvent: *ev1, Dragged: geo.NewDelta(1, 0)}
	entry.Dragged(de)
	de = &event.DragEvent{PointEvent: *ev2, Dragged: geo.NewDelta(1, 0)}
	entry.Dragged(de)

	assert.True(t, entry.sel.selecting)

	entry.DragEnd()
	assert.Equal(t, "Testing", entry.SelectedText())
	assert.True(t, entry.sel.selecting)
}

func TestEntry_DragSelectWithScroll(t *testing.T) {
	entry := NewEntry()
	entry.SetText("The quick brown fox jumped over and over the lazy dog.")

	// get position after the letter 'a'.
	ev1 := getClickPosition("The quick brown fox jumped over and over the la", 0)
	// get position after the letter 'u'
	ev2 := getClickPosition("The qu", 0)

	// mouse down and drag from 'a' to 'i'
	me := &desktop.MouseEvent{PointEvent: *ev1, Button: desktop.MouseButtonPrimary}
	entry.MouseDown(me)
	de := &event.DragEvent{PointEvent: *ev1, Dragged: geo.NewDelta(1, 0)}
	entry.Dragged(de)
	de = &event.DragEvent{PointEvent: *ev2, Dragged: geo.NewDelta(1, 0)}
	entry.Dragged(de)
	me = &desktop.MouseEvent{PointEvent: *ev1, Button: desktop.MouseButtonPrimary}
	entry.MouseUp(me)

	assert.Equal(t, "ick brown fox jumped over and over the la", entry.SelectedText())
}

func TestEntry_ExpandSelectionForDoubleTap(t *testing.T) {
	str := []rune(" fish 日本語日  \t  test 本日本 moose  \t")

	// select invalid (before start)
	start, end := getTextWhitespaceRegion(str, -1, false)
	assert.Equal(t, -1, start)
	assert.Equal(t, -1, end)

	// select whitespace at the end of text
	start, end = getTextWhitespaceRegion(str, len(str), false)
	assert.Equal(t, 29, start)
	assert.Equal(t, 32, end)
	start, end = getTextWhitespaceRegion(str, len(str)+100, false)
	assert.Equal(t, 29, start)
	assert.Equal(t, 32, end)

	// select the whitespace
	start, end = getTextWhitespaceRegion(str, 0, false)
	assert.Equal(t, 0, start)
	assert.Equal(t, 1, end)
	// select the whitespace - grab adjacent words
	start, end = getTextWhitespaceRegion(str, 0, true)
	assert.Equal(t, 0, start)
	assert.Equal(t, 5, end)

	// select "fish"
	start, end = getTextWhitespaceRegion(str, 1, false)
	assert.Equal(t, 1, start)
	assert.Equal(t, 5, end)
	start, end = getTextWhitespaceRegion(str, 4, false)
	assert.Equal(t, 1, start)
	assert.Equal(t, 5, end)

	// select "日本語日"
	start, end = getTextWhitespaceRegion(str, 7, false)
	assert.Equal(t, 6, start)
	assert.Equal(t, 10, end)
	start, end = getTextWhitespaceRegion(str, 9, false)
	assert.Equal(t, 6, start)
	assert.Equal(t, 10, end)

	// select "  \t  "
	start, end = getTextWhitespaceRegion(str, 10, false)
	assert.Equal(t, 10, start)
	assert.Equal(t, 15, end)

	// select "  \t"
	start, end = getTextWhitespaceRegion(str, 30, false)
	assert.Equal(t, 29, start)
	assert.Equal(t, len(str), end)
}

func TestEntry_ExpandSelectionWithWordSeparators(t *testing.T) {
	// select "is_a"
	str := []rune("This-is_a-test")
	start, end := getTextWhitespaceRegion(str, 6, false)
	assert.Equal(t, 5, start)
	assert.Equal(t, 9, end)
}

func TestEntry_WordRegionNearest(t *testing.T) {
	// A long-press landing on a whitespace run snaps to the nearest adjacent
	// word instead of selecting the run itself (#958).
	str := []rune("hello   world")

	start, end := getTextWordRegionNearest(str, 5) // first space: "hello" is nearer
	assert.Equal(t, 0, start)
	assert.Equal(t, 5, end)

	start, end = getTextWordRegionNearest(str, 7) // last space: "world" is nearer
	assert.Equal(t, 8, start)
	assert.Equal(t, 13, end)

	start, end = getTextWordRegionNearest(str, 6) // equidistant: the preceding word wins
	assert.Equal(t, 0, start)
	assert.Equal(t, 5, end)

	// A press on a word keeps that word.
	start, end = getTextWordRegionNearest(str, 2)
	assert.Equal(t, 0, start)
	assert.Equal(t, 5, end)
	start, end = getTextWordRegionNearest(str, 9)
	assert.Equal(t, 8, start)
	assert.Equal(t, 13, end)

	// Leading whitespace has no word before it: snap forward.
	str = []rune("  lead")
	start, end = getTextWordRegionNearest(str, 0)
	assert.Equal(t, 2, start)
	assert.Equal(t, 6, end)

	// Trailing whitespace (and a press past the end of the row) snaps back.
	str = []rune("trail  ")
	start, end = getTextWordRegionNearest(str, 6)
	assert.Equal(t, 0, start)
	assert.Equal(t, 5, end)
	start, end = getTextWordRegionNearest(str, 100)
	assert.Equal(t, 0, start)
	assert.Equal(t, 5, end)

	// Punctuation runs snap too (isWordSeparator groups them with whitespace).
	str = []rune("This-is_a-test")
	start, end = getTextWordRegionNearest(str, 4)
	assert.Equal(t, 0, start)
	assert.Equal(t, 4, end)

	// A separator-only row keeps the run selection.
	str = []rune("   ")
	start, end = getTextWordRegionNearest(str, 1)
	assert.Equal(t, 0, start)
	assert.Equal(t, 3, end)

	// Invalid positions pass through.
	start, end = getTextWordRegionNearest(str, -1)
	assert.Equal(t, -1, start)
	assert.Equal(t, -1, end)
	start, end = getTextWordRegionNearest([]rune{}, 0)
	assert.Equal(t, -1, start)
	assert.Equal(t, -1, end)
}

func TestEntry_EraseSelection(t *testing.T) {
	// Selects "sti" on border 2 of a new multiline
	// T e s t i n g
	// T e[s t i]n g
	// T e s t i n g
	e := NewMultiLineEntry()
	e.SetText("Testing\nTesting\nTesting")
	e.CursorRow = 1
	e.CursorColumn = 2
	e.sel.cursorRow, e.sel.cursorRow = e.CursorRow, e.CursorColumn
	keyDown := func(key *event.KeyEvent) {
		e.KeyDown(key)
		e.TypedKey(key)
	}
	keyPress := func(key *event.KeyEvent) {
		keyDown(key)
		e.KeyUp(key)
	}
	keyDown(&event.KeyEvent{Name: desktop.KeyShiftLeft})
	keyPress(&event.KeyEvent{Name: event.KeyRight})
	keyPress(&event.KeyEvent{Name: event.KeyRight})
	keyPress(&event.KeyEvent{Name: event.KeyRight})

	_ = e.Theme()
	e.eraseSelectionAndUpdate()
	e.updateText(e.textProvider().String(), false)
	assert.Equal(t, "Testing\nTeng\nTesting", e.Text)
	a, b := e.sel.selection()
	assert.Equal(t, -1, a)
	assert.Equal(t, -1, b)
}

func TestEntry_CallbackLocking(t *testing.T) {
	e := &Entry{}
	called := 0
	e.OnChanged = func(_ string) {
		called++ // Just to not have an empty critical section.
	}

	_ = e.Theme()
	test.Type(e, "abc123")
	e.selectAll()
	e.TypedKey(&event.KeyEvent{Name: event.KeyBackspace})
	assert.Equal(t, 7, called)
}

func TestEntry_ContentSizeAndPlacementWithIcon(t *testing.T) {
	entry := NewEntry()
	entry.SetIcon(theme.MailComposeIcon())
	entry.SetText("SomeText")
	renderer := entry.CreateRenderer()
	// Leading icon inset plus one input border: the viewport sits inside the
	// background box on the X axis too (#467).
	contentPos := geo.NewPos(theme.LineSpacing()+theme.IconInlineSize()+theme.InputBorderSize(), theme.InputBorderSize())

	renderer.Layout(entry.MinSize())
	// Scrollable content should be positioned after the icon, with correct padding
	assert.Equal(t, contentPos, entry.scroll.Position())

	entry.Wrapping = bellui.TextWrapOff
	entry.Scroll = bellui.ScrollNone
	renderer.Layout(entry.MinSize())
	// Content should be positioned after the icon, with correct padding
	assert.Equal(t, contentPos, entry.content.Position())
}

func TestEntry_MouseClickAndDragOutsideText(t *testing.T) {
	entry := NewEntry()
	entry.SetText("A\nB\n")

	testCharSize := theme.TextSize()
	pos := geo.NewPos(testCharSize, testCharSize*4) // tap below rows
	ev := &event.PointEvent{Position: pos}

	me := &desktop.MouseEvent{PointEvent: *ev, Button: desktop.MouseButtonPrimary}
	entry.MouseDown(me)
	de := &event.DragEvent{PointEvent: *ev, Dragged: geo.NewDelta(1, 0)}
	entry.Dragged(de)
	entry.MouseUp(me)
	assert.False(t, entry.sel.selecting)
}

func TestEntry_MouseDownOnSelect(t *testing.T) {
	entry := NewEntry()
	entry.SetText("Ahnj\nBuki\n")
	entry.TypedShortcut(&bellui.ShortcutSelectAll{})

	testCharSize := theme.TextSize()
	pos := geo.NewPos(testCharSize, testCharSize*4) // tap below rows
	ev := &event.PointEvent{Position: pos}

	me := &desktop.MouseEvent{PointEvent: *ev, Button: desktop.MouseButtonSecondary}
	entry.MouseDown(me)
	entry.MouseUp(me)

	assert.Equal(t, "Ahnj\nBuki\n", entry.SelectedText())

	me = &desktop.MouseEvent{PointEvent: *ev, Button: desktop.MouseButtonPrimary}
	entry.MouseDown(me)
	entry.MouseUp(me)

	assert.Equal(t, "", entry.SelectedText())
}

func TestEntry_PasteFromClipboard(t *testing.T) {
	entry := NewEntry()

	w := test.NewApp().NewWindow("")
	defer w.Close()
	w.SetContent(entry)

	testContent := "test"

	clipboard := bellui.CurrentApp().Clipboard()
	clipboard.SetContent(testContent)

	entry.pasteFromClipboard(clipboard)

	assert.Equal(t, testContent, entry.Text)
}

func TestEntry_PasteFromClipboard_MultilineWrapping(t *testing.T) {
	entry := NewMultiLineEntry()
	entry.Wrapping = bellui.TextWrapWord

	w := test.NewApp().NewWindow("")
	defer w.Close()
	w.SetContent(entry)

	// Use a narrow width that forces "Testing entry" to wrap. The exact width
	// depends on the default font size (14px in the test theme). A width of
	// 86px ensures "Testing entry" wraps between the two words.
	w.Resize(geo.NewSize(86, 64))

	test.Type(entry, "T")
	assert.Equal(t, 0, entry.CursorRow)
	assert.Equal(t, 1, entry.CursorColumn)

	clipboard := bellui.CurrentApp().Clipboard()
	clipboard.SetContent("esting entry")

	entry.pasteFromClipboard(clipboard)

	assert.Equal(t, "Testing entry", entry.Text)
	assert.Equal(t, 1, entry.CursorRow)
	assert.Equal(t, 5, entry.CursorColumn)

	clipboard.SetContent(" paste\ncontent")
	entry.pasteFromClipboard(clipboard)

	assert.Equal(t, "Testing entry paste\ncontent", entry.Text)
	assert.Equal(t, 3, entry.CursorRow) // narrower due to scrollbar space reservation causes extra wrap
	assert.Equal(t, 7, entry.CursorColumn)
}

func TestEntry_PasteFromClipboardValidation(t *testing.T) {
	entry := NewEntry()
	var triggered int
	entry.Validator = func(s string) error {
		triggered++
		return nil
	}

	testContent := "test"
	clipboard := test.NewTempApp(t).Clipboard()
	clipboard.SetContent(testContent)

	entry.pasteFromClipboard(clipboard)
	assert.Equal(t, 2, triggered)
}

func TestEntry_PlaceholderTextStyle(t *testing.T) {
	e := NewEntry()
	e.TextStyle = bellui.TextStyle{Bold: true, Italic: true}

	w := test.NewTempWindow(t, e)
	assert.Equal(t, e.TextStyle, e.placeholder.Segments[0].(*TextSegment).Style.TextStyle)

	w.Canvas().Focus(e)
	assert.Equal(t, e.TextStyle, e.placeholder.Segments[0].(*TextSegment).Style.TextStyle)
}

func TestEntry_Tab(t *testing.T) {
	e := NewEntry()
	e.TextStyle.Monospace = true
	e.SetText("a\n\tb\nc")

	_ = e.Theme()
	r := cache.Renderer(e.textProvider()).(*textRenderer)
	assert.Len(t, r.Objects(), 3)
	assert.Equal(t, "a", r.Objects()[0].(*canvas.Text).Text)
	assert.Equal(t, "\tb", r.Objects()[1].(*canvas.Text).Text)

	w := test.NewTempWindow(t, e)
	w.Resize(geo.NewSize(86, 86))
	w.Canvas().Focus(e)
	test.AssertImageMatches(t, "entry/tab-content.png", w.Canvas().Capture())
}

func TestEntry_TabSelection(t *testing.T) {
	e := NewEntry()
	e.SetText("a\n\tb\nc")
	e.TextStyle.Monospace = true

	e.CursorRow = 1
	e.sel.cursorRow = 1
	e.KeyDown(&event.KeyEvent{Name: desktop.KeyShiftLeft})
	e.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	e.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	e.KeyUp(&event.KeyEvent{Name: desktop.KeyShiftLeft})

	assert.Equal(t, "\tb", e.SelectedText())

	w := test.NewTempWindow(t, e)
	w.Resize(geo.NewSize(86, 86))
	w.Canvas().Focus(e)
	test.AssertImageMatches(t, "entry/tab-select.png", w.Canvas().Capture())
}

func TestEntry_ShiftSelection_ResetOnFocusLost(t *testing.T) {
	e := NewEntry()
	e.SetText("Hello")

	e.KeyDown(&event.KeyEvent{Name: desktop.KeyShiftLeft})
	assert.True(t, e.selectKeyDown)

	e.FocusLost()
	assert.False(t, e.selectKeyDown)
}

func getClickPosition(str string, row int) *event.PointEvent {
	x := bellui.MeasureText(str, theme.TextSize(), bellui.TextStyle{}).Width + theme.Padding()

	rowHeight := bellui.MeasureText("M", theme.TextSize(), bellui.TextStyle{}).Height
	y := float32(row)*rowHeight + rowHeight/2

	// add a couple of pixels, this is currently a workaround for weird mouse to column logic on text with kerning
	pos := geo.NewPos(x+2, y)
	return &event.PointEvent{Position: pos}
}

// TestEntry_SelectAllReShowsContextMenu verifies that choosing "Select all" from
// the touch context menu re-presents the menu over the now-full selection, so
// Cut/Copy stay reachable without a fresh long-press that would move the caret /
// lose the selection (#1128). Desktop (non-touch) does not re-show.
func TestEntry_SelectAllReShowsContextMenu(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")

	entry := NewEntry()
	entry.SetText("Hello World")
	w := test.NewWindow(entry)
	defer w.Close()
	w.Resize(geo.NewSize(200, 100))
	entry.requestFocus()

	selectAllOf := func() *bellui.MenuItem {
		if entry.popUp == nil {
			return nil
		}
		for _, o := range entry.popUp.Items {
			if mi, ok := o.(*menuItem); ok && mi.Item.Label == lang.L("Select all") {
				return mi.Item
			}
		}
		return nil
	}

	// Touch: Select all must re-present the menu over the full selection.
	entry.touchPointer = true
	entry.TappedSecondary(&event.PointEvent{Position: geo.NewPos(10, 10)})
	first := entry.popUp
	assert.NotNil(t, first, "touch context menu shows")
	sa := selectAllOf()
	assert.NotNil(t, sa, "Select all item present")
	sa.Action()
	assert.Equal(t, "Hello World", entry.SelectedText(), "all text selected")
	assert.NotNil(t, entry.popUp, "menu re-presented after touch Select all")
	assert.NotSame(t, first, entry.popUp, "a fresh menu is shown, not the dismissed one")

	// Desktop (non-touch): Select all does not re-show the menu.
	entry.popUp = nil
	entry.touchPointer = false
	entry.TappedSecondary(&event.PointEvent{Position: geo.NewPos(10, 10)})
	before := entry.popUp
	sa = selectAllOf()
	assert.NotNil(t, sa, "Select all item present (desktop)")
	sa.Action()
	assert.Same(t, before, entry.popUp, "desktop Select all does not re-present the menu")
}
