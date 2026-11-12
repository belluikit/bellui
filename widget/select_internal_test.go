package widget

import (
	"testing"

	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/stretchr/testify/assert"
)

func TestSelect_SetOptions(t *testing.T) {
	sel := NewSelect([]string{"10", "11", "12"}, func(s string) {})
	test.Tap(sel)

	assert.NotNil(t, sel.popUp)
	assert.Len(t, sel.popUp.Items, 3)
	assert.Equal(t, "10", sel.popUp.Items[0].(*menuItem).Item.Label)

	sel.popUp.Hide()
	sel.SetOptions([]string{"15", "16", "17"})

	test.Tap(sel)
	assert.NotNil(t, sel.popUp)
	assert.Len(t, sel.popUp.Items, 3)
	assert.Equal(t, "16", sel.popUp.Items[1].(*menuItem).Item.Label)

	sel.popUp.Hide()
	sel.Options = []string{"20", "21"}
	sel.Refresh()

	test.Tap(sel)
	assert.NotNil(t, sel.popUp)
	assert.Len(t, sel.popUp.Items, 2)
	assert.Equal(t, "20", sel.popUp.Items[0].(*menuItem).Item.Label)
}

// TestSelect_PressFeedback is #1091: a select shows its pressed state from the
// press, over the whole field, where it used to run a ripple only after the
// release. A renderer made again after the cache dropped the old one shows it
// too.
func TestSelect_PressFeedback(t *testing.T) {
	test.NewTempApp(t)
	sel := NewSelect([]string{"one"}, func(string) {})
	sel.Resize(geo.NewSize(150, sel.MinSize().Height))
	assertPressCycle(t, sel, &sel.pressTarget, sel.HoverOut)
	assertRowShape(t, sel, &sel.pressTarget, test.Theme().Size(theme.SizeNameInputRadius))
	assertTapFromCode(t, &sel.pressTarget, func() { sel.TypedKey(&event.KeyEvent{Name: event.KeySpace}) })
	if sel.popUp != nil {
		sel.popUp.Hide()
	}

	render1 := cache.Renderer(sel)
	cache.DestroyRenderer(sel)
	sel.Refresh()
	assert.NotSame(t, render1, cache.Renderer(sel))
	sel.TouchDown(&mobile.TouchEvent{})
	assert.True(t, widget.PressFeedbackOf(&sel.pressTarget).Pressed(), "the new renderer shows no press")
	sel.TouchCancel(nil)

	sel.Disable()
	sel.TouchDown(&mobile.TouchEvent{})
	assert.False(t, widget.PressFeedbackOf(&sel.pressTarget).Pressed(), "a disabled select showed a press")
}
