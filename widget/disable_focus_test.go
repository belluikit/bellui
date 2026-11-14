package widget_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// A widget disabled while it, or an object inside it, has the focus gives the
// focus up: the focus manager gives a disabled widget none, and keeping it
// would leave the keys typed next with a widget that refuses them (#1221). A
// focus elsewhere stays where it is.
func TestDisable_DropsFocusInside(t *testing.T) {
	self := func(w core.Disableable) func(core.Canvas) event.Focusable {
		return func(core.Canvas) event.Focusable { return w.(event.Focusable) }
	}
	for name, build := range map[string]func() (core.Disableable, func(core.Canvas) event.Focusable){
		"Entry": func() (core.Disableable, func(core.Canvas) event.Focusable) {
			e := widget.NewEntry()
			return e, self(e)
		},
		"Check": func() (core.Disableable, func(core.Canvas) event.Focusable) {
			c := widget.NewCheck("c", nil)
			return c, self(c)
		},
		"Slider": func() (core.Disableable, func(core.Canvas) event.Focusable) {
			s := widget.NewSlider(0, 10)
			return s, self(s)
		},
		"RangeSlider": func() (core.Disableable, func(core.Canvas) event.Focusable) {
			s := widget.NewRangeSlider(0, 10)
			return s, self(s)
		},
		"FontPicker's family select": func() (core.Disableable, func(core.Canvas) event.Focusable) {
			// The picker takes no focus itself, so Tab from nothing lands
			// on its first sub-widget, which it builds in its renderer.
			fp := widget.NewFontPicker(widget.FontPickerConfig{Size: 12}, []widget.FontFamily{{Name: "Sans"}}, nil)
			return fp, func(c core.Canvas) event.Focusable {
				c.FocusNext()
				require.NotNil(t, c.Focused(), "Tab found no sub-widget")
				return c.Focused()
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			test.NewTempApp(t)
			w, focusIn := build()
			other := widget.NewEntry()
			win := test.NewTempWindow(t, container.NewVBox(w.(core.CanvasObject), other))
			c := win.Canvas()
			focus := focusIn(c)

			c.Focus(focus)
			require.Equal(t, focus, c.Focused(), "the widget did not take the focus, so the check would prove nothing")
			w.Disable()
			assert.Nil(t, c.Focused(), "the disabled widget kept the focus")

			w.Enable()
			c.Focus(other)
			w.Disable()
			assert.Equal(t, other, c.Focused(), "disabling a widget took the focus from another")
		})
	}
}
