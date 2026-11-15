package widget_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// A disabled widget takes no focus, from a click, code or assistive tech,
// unless its text stays selectable while disabled (core.SelectableWhenDisabled,
// #1251). A disabled RichEntry used to be focusable from code: the focus
// manager let in any disabled widget with SelectedText. A disabled Entry keeps
// the focus a selection needs, and its text can still be copied.
func TestDisabledFocus_OnlySelectableWidgetsTakeIt(t *testing.T) {
	test.NewTempApp(t)
	before := widget.NewEntry()
	rich := widget.NewRichEntry()
	rich.SetSegments([]widget.RichTextSegment{&widget.TextSegment{Style: widget.RichTextStyleInline, Text: "rich text"}})
	entry := widget.NewEntry()
	entry.SetText("copy me")
	w := test.NewTempWindow(t, container.NewVBox(before, rich, entry))
	w.Resize(geo.NewSize(300, 300))
	c := w.Canvas()

	c.Focus(before)
	rich.Disable()
	c.Focus(rich)
	assert.Equal(t, before, c.Focused(), "a disabled RichEntry took the focus from code")

	entry.Disable()
	c.Focus(entry)
	if bellui.CurrentDevice().IsMobile() {
		// On mobile a disabled Entry is refused all the same: the focus would
		// summon the soft keyboard (core.SelectableWhenDisabled).
		assert.Equal(t, before, c.Focused(), "a disabled Entry took the focus on mobile")
	} else {
		assert.Equal(t, entry, c.Focused(), "a disabled Entry refused the focus its selection needs")
	}
	cb := bellui.CurrentApp().Clipboard()
	entry.TypedShortcut(&bellui.ShortcutSelectAll{})
	entry.TypedShortcut(&bellui.ShortcutCopy{Clipboard: cb})
	assert.Equal(t, "copy me", cb.Content(), "a disabled Entry's text could not be copied")

	rich.Enable()
	c.Focus(rich)
	assert.Equal(t, rich, c.Focused(), "an enabled RichEntry refused the focus")
}
