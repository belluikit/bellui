package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// TestEntry_FilledIndicator pins #494: a filled entry draws no outline but an
// active indicator along the bottom of its box, one input border thick and
// two when focused; the outlined style keeps its outline.
func TestEntry_FilledIndicator(t *testing.T) {
	test.NewTempApp(t)
	e := NewEntry()
	w := test.NewWindow(e)
	defer w.Close()
	e.Resize(geo.NewSize(200, 40))
	r := test.TempWidgetRenderer(t, e).(*entryRenderer)
	border := e.Theme().Size(theme.SizeNameInputBorder)

	assert.Zero(t, r.border.StrokeWidth, "no outline")
	assert.Equal(t, geo.NewSize(200-border*2, border), r.border.Size())
	assert.Equal(t, geo.NewPos(border, 40-border*2), r.border.Position(), "along the box's bottom edge")
	assert.Zero(t, r.box.CornerRadius, "square bottom corners")
	assert.Positive(t, r.box.TopLeftCornerRadius)

	w.Canvas().Focus(e)
	assert.Equal(t, border*2, r.border.Size().Height, "focused: twice as thick")
	assert.Equal(t, 40-border*3, r.border.Position().Y)

	e.Style = EntryStyleOutlined
	e.Refresh()
	assert.Equal(t, border, r.border.StrokeWidth*0.5, "focused outline")
	assert.Equal(t, geo.NewSize(200-border-.5, 40-border-.5), r.border.Size())
	assert.Positive(t, r.box.CornerRadius)
	assert.Zero(t, r.box.TopLeftCornerRadius)
}
