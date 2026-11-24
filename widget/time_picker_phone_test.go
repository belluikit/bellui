package widget

import (
	"testing"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/stretchr/testify/assert"
)

// TestTimePicker_FitsAPhone pins #1421: at the default theme the 12-hour
// picker is no wider than the 24-hour one (its AM/PM selector sits in the
// header row, not beside the dial), and the time and date-and-time dialogs get
// their full width on a 360 dp screen instead of being squeezed below it.
func TestTimePicker_FitsAPhone(t *testing.T) {
	a := test.NewTempApp(t)
	a.Settings().SetTheme(theme.DefaultTheme())

	h12 := NewTimePicker(9, 30, false, nil).MinSize().Width
	h24 := NewTimePicker(9, 30, true, nil).MinSize().Width
	assert.Equal(t, h24, h12, "the AM/PM selector must not widen the picker")
	assert.Equal(t, float32(304), h12)
	assert.Equal(t, float32(344), NewDateTimePicker(time.Now(), false, nil).MinSize().Width)

	for _, tc := range []struct {
		name string
		show func(core.Canvas)
	}{
		{"time 12h", func(c core.Canvas) { ShowTimePicker(9, 30, false, nil, c) }},
		{"date and time", func(c core.Canvas) { ShowDateTimePicker(time.Now(), false, nil, c) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := test.NewWindow(NewLabel(""))
			defer w.Close()
			w.SetPadded(false)
			w.Resize(geo.NewSize(360, 800))
			c := w.Canvas()
			tc.show(c)
			top := c.Overlays().Top().(*PopUp)
			content := top.Content
			assert.GreaterOrEqual(t, content.Size().Width, content.MinSize().Width, "the dialog squeezes its picker")
			assert.GreaterOrEqual(t, content.Position().X, float32(0))
			assert.LessOrEqual(t, content.Position().X+content.Size().Width, c.Size().Width)
		})
	}
}

// TestTimePicker_12HourHeaderInsideMinSize checks the other side of #1421's
// fix: laid out at its MinSize, every box in the 12-hour picker's header, the
// AM/PM selector included, stays inside the picker and its padding. At the
// default theme the dial sets the width; with small icon buttons the header
// row is wider than the dial, so the second case checks that MinSize measures
// the header too.
func TestTimePicker_12HourHeaderInsideMinSize(t *testing.T) {
	for _, tc := range []struct {
		name string
		th   bellui.Theme
	}{
		{"default theme", theme.DefaultTheme()},
		{"small icon buttons", &smallIconButtonTheme{theme.DefaultTheme()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := test.NewTempApp(t)
			a.Settings().SetTheme(tc.th)
			p := NewTimePicker(21, 5, false, nil)
			sz := p.MinSize()
			p.Resize(sz)
			r := test.TempWidgetRenderer(t, p)
			// The header keeps the margin it starts with on its right side too.
			margin := sz.Width
			for _, o := range r.Objects() {
				if b, ok := o.(*timePickerButton); ok && b.Visible() {
					margin = min(margin, b.Position().X)
				}
			}
			for _, o := range r.Objects() {
				if b, ok := o.(*timePickerButton); ok && b.Visible() {
					assert.LessOrEqual(t, b.Position().X+b.Size().Width, sz.Width-margin,
						"the %q box overruns the picker's padding", b.label.Text)
				}
			}
		})
	}
}

// smallIconButtonTheme shrinks the icon button size, which the time picker
// scales its dial and boxes from, until the header row is wider than the dial.
type smallIconButtonTheme struct{ bellui.Theme }

func (s *smallIconButtonTheme) Size(n bellui.ThemeSizeName) float32 {
	if n == theme.SizeNameIconButtonSize {
		return 8
	}
	return s.Theme.Size(n)
}
