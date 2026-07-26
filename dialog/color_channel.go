package dialog

import (
	"strconv"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	internalwidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

var _ core.Widget = (*colorChannel)(nil)

// colorChannel controls a channel of a color and triggers the callback when changed.
type colorChannel struct {
	widget.BaseWidget
	name      string
	min, max  int
	value     int
	onChanged func(int)
}

// newColorChannel returns a new color channel control for the channel with the given name.
func newColorChannel(name string, min, max, value int, onChanged func(int)) *colorChannel {
	c := &colorChannel{
		name:      name,
		min:       min,
		max:       max,
		value:     clamp(value, min, max),
		onChanged: onChanged,
	}
	c.ExtendBaseWidget(c)
	return c
}

// CreateRenderer is a private method to Bellui which links this widget to its renderer
func (c *colorChannel) CreateRenderer() core.WidgetRenderer {
	label := widget.NewLabelWithStyle(c.name, bellui.TextAlignTrailing, bellui.TextStyle{Bold: true})
	entry := newColorChannelEntry(c)
	slider := &widget.Slider{
		Value:       0.0,
		Min:         float64(c.min),
		Max:         float64(c.max),
		Step:        1.0,
		Orientation: widget.Horizontal,
		OnChanged: func(value float64) {
			c.SetValue(int(value))
		},
	}
	slider.ExtendBaseWidget(slider)
	r := &colorChannelRenderer{
		BaseRenderer: internalwidget.NewBaseRenderer([]core.CanvasObject{
			label,
			slider,
			entry,
		}),
		control: c,
		label:   label,
		entry:   entry,
		slider:  slider,
	}
	r.updateObjects()
	return r
}

// MinSize returns the size that this widget should not shrink below
func (c *colorChannel) MinSize() geo.Size {
	c.ExtendBaseWidget(c)
	return c.BaseWidget.MinSize()
}

// SetValue updates the value in this color widget
func (c *colorChannel) SetValue(value int) {
	value = clamp(value, c.min, c.max)
	if c.value == value {
		return
	}
	c.value = value
	c.Refresh()
	if f := c.onChanged; f != nil {
		f(value)
	}
}

type colorChannelRenderer struct {
	internalwidget.BaseRenderer
	control *colorChannel
	label   *widget.Label
	entry   *colorChannelEntry
	slider  *widget.Slider
}

func (r *colorChannelRenderer) Layout(size geo.Size) {
	lMin := r.label.MinSize()
	eMin := r.entry.MinSize()
	r.label.Move(geo.NewPos(0, (size.Height-lMin.Height)/2))
	r.label.Resize(geo.NewSize(lMin.Width, lMin.Height))
	r.slider.Move(geo.NewPos(lMin.Width, 0))
	r.slider.Resize(geo.NewSize(size.Width-lMin.Width-eMin.Width, size.Height))
	r.entry.Move(geo.NewPos(size.Width-eMin.Width, 0))
	r.entry.Resize(geo.NewSize(eMin.Width, size.Height))
}

func (r *colorChannelRenderer) MinSize() geo.Size {
	lMin := r.label.MinSize()
	sMin := r.slider.MinSize()
	eMin := r.entry.MinSize()
	return geo.NewSize(
		lMin.Width+sMin.Width+eMin.Width,
		geo.Max(lMin.Height, geo.Max(sMin.Height, eMin.Height)),
	)
}

func (r *colorChannelRenderer) Refresh() {
	r.updateObjects()
	r.Layout(r.control.Size())
	canvas.Refresh(r.control)
}

func (r *colorChannelRenderer) updateObjects() {
	r.entry.SetText(strconv.Itoa(r.control.value))
	r.slider.Value = float64(r.control.value)
	r.slider.Refresh()
}

type colorChannelEntry struct {
	userChangeEntry
}

func newColorChannelEntry(c *colorChannel) *colorChannelEntry {
	e := &colorChannelEntry{}
	e.Text = "0"
	e.ExtendBaseWidget(e)
	e.setOnChanged(func(text string) {
		value, err := strconv.Atoi(text)
		if err != nil {
			bellui.LogError("Couldn't parse: "+text, err)
			return
		}
		c.SetValue(value)
	})
	return e
}

func (e *colorChannelEntry) MinSize() geo.Size {
	// Ensure space for 3 digits
	min := bellui.MeasureText("000", theme.TextSize(), bellui.TextStyle{})
	min = min.Add(geo.NewSize(theme.Padding()*6, theme.Padding()*4))
	return min.Max(e.Entry.MinSize())
}

type userChangeEntry struct {
	widget.Entry
	userTyped bool
}

func newUserChangeEntry(text string) *userChangeEntry {
	e := &userChangeEntry{}
	e.Entry.Text = text
	e.ExtendBaseWidget(e)
	return e
}

func (e *userChangeEntry) setOnChanged(onChanged func(s string)) {
	e.Entry.OnChanged = func(text string) {
		if !e.userTyped {
			return
		}

		e.userTyped = false

		if onChanged != nil {
			onChanged(text)
		}
	}
	e.ExtendBaseWidget(e)
}

func (e *userChangeEntry) TypedRune(r rune) {
	e.userTyped = true
	e.Entry.TypedRune(r)
}

func (e *userChangeEntry) TypedKey(ev *event.KeyEvent) {
	e.userTyped = true
	e.Entry.TypedKey(ev)
}
