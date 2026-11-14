package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// A disabled FAB draws MD3's disabled look, as Button does, with no hover
// state layer; before #1214 it kept its colours and only the desktop paint
// walk's dimming showed it.
func TestFAB_DisabledLook(t *testing.T) {
	test.NewTempApp(t)
	fab := NewExtendedFAB("Add", theme.ContentAddIcon(), func() {})
	fab.Disable()
	test.NewTempWindow(t, fab)
	r := cache.Renderer(fab).(*fabRenderer)
	v := bellui.CurrentApp().Settings().ThemeVariant()
	th := test.Theme()

	look := func(bg, fg bellui.ThemeColorName) {
		t.Helper()
		assert.Equal(t, th.Color(bg, v), r.bg.FillColor, "container colour")
		assert.Equal(t, fg, r.icon.Resource.(*theme.ThemedResource).ColorName, "icon colour")
		assert.Equal(t, th.Color(fg, v), r.label.Color, "label colour")
	}
	look(theme.ColorNameDisabledButton, theme.ColorNameDisabled)
	fab.HoverIn(&event.HoverEvent{})
	look(theme.ColorNameDisabledButton, theme.ColorNameDisabled)
	fab.HoverOut()

	fab.Enable()
	look(theme.ColorNamePrimaryContainer, theme.ColorNameOnPrimaryContainer)
	fab.Disable()
	look(theme.ColorNameDisabledButton, theme.ColorNameDisabled)
}
