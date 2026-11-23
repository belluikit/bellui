package app_test

import (
	"net/url"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/internal/app"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

// A focusable that cannot be disabled (Hyperlink) inside a disabled container
// takes no focus: Tab skips it and Focus refuses it, so its keys never reach
// it (#1288).
func TestFocusManager_DisabledAncestorBlocksFocus(t *testing.T) {
	test.NewTempApp(t)
	link := func() *widget.Hyperlink {
		u, _ := url.Parse("https://example.com")
		return widget.NewHyperlink("link", u)
	}

	t.Run("AppTabs", func(t *testing.T) {
		before, inside := widget.NewButton("before", nil), link()
		tabs := container.NewAppTabs(container.NewTabItem("a", container.NewVBox(inside)))
		m := app.NewFocusManager(container.NewVBox(before, tabs))
		tabs.Disable()

		assert.False(t, inChain(m, inside), "Tab reaches the link")
		assert.True(t, m.Focus(inside))
		assert.True(t, m.Focused() == nil, "the link took the focus")

		tabs.Enable()
		assert.True(t, inChain(m, inside), "Tab skips the link")
		assert.True(t, m.Focus(inside))
		assert.Equal(t, event.Focusable(inside), m.Focused())
	})

	// ButtonGroup holds only *Button, so no Hyperlink can sit inside one; the
	// other disabled holders are an InnerWindow and a widget the app wrote.
	t.Run("InnerWindow", func(t *testing.T) {
		inside := link()
		win := container.NewInnerWindow("w", container.NewVBox(inside))
		m := app.NewFocusManager(container.NewMultipleWindows(win))
		win.Disable()
		assert.False(t, inChain(m, inside), "Tab reaches the link")
		assert.True(t, m.Focus(inside))
		assert.True(t, m.Focused() == nil, "the link took the focus")
	})

	t.Run("app widget", func(t *testing.T) {
		inside := link()
		holder := &focusable{child: container.NewVBox(inside)}
		m := app.NewFocusManager(container.NewVBox(holder))
		holder.Disable()
		assert.False(t, inChain(m, inside), "Tab reaches the link")
		assert.True(t, m.Focus(inside))
		assert.True(t, m.Focused() == nil, "the link took the focus")
	})

	t.Run("selectable when disabled still focusable", func(t *testing.T) {
		entry := widget.NewEntry()
		tabs := container.NewAppTabs(container.NewTabItem("a", entry))
		m := app.NewFocusManager(tabs)
		tabs.Disable()
		assert.True(t, entry.Disabled())
		assert.False(t, inChain(m, entry), "Tab reaches the disabled entry")
		assert.True(t, m.Focus(entry))
		if bellui.CurrentDevice().IsMobile() {
			// On mobile a disabled text input takes no focus, so it does not
			// summon the soft keyboard: the existing rule, unchanged here.
			assert.True(t, m.Focused() == nil, "a disabled Entry took the focus on mobile")
			return
		}
		assert.True(t, m.Focused() == event.Focusable(entry), "a disabled Entry keeps its text selectable (#1251)")
	})
}

func inChain(m *app.FocusManager, f event.Focusable) bool {
	for _, e := range m.Chain() {
		if e.Focusable == f {
			return true
		}
	}
	return false
}
