package dialog

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

func Test_colorAdvancedPicker_HexValidation(t *testing.T) {
	test.NewTempApp(t)

	color := newColorAdvancedPicker(theme.Color(theme.ColorNamePrimary), nil)
	r := test.TempWidgetRenderer(t, color).(*colorPickerRenderer)
	hex := r.hex

	// A valid hex string validates cleanly.
	hex.SetText("#ff0000")
	assert.NoError(t, hex.Validate())

	// Invalid input now enters the entry's error state rather than being
	// accepted silently (#293).
	hex.SetText("nonsense")
	assert.Error(t, hex.Validate())
}

func Test_colorGreyscalePicker_Layout(t *testing.T) {
	test.NewTempApp(t)

	color := newColorGreyscalePicker(nil)

	window := test.NewTempWindow(t, container.NewCenter(color))
	window.Resize(color.MinSize().Max(geo.NewSize(360, 60)))

	test.AssertRendersToImage(t, "color/picker_layout_greyscale.png", window.Canvas())
}

func Test_colorBasicPicker_Layout(t *testing.T) {
	test.NewTempApp(t)

	color := newColorBasicPicker(nil)

	window := test.NewTempWindow(t, container.NewCenter(color))
	window.Resize(color.MinSize().Max(geo.NewSize(360, 60)))

	test.AssertRendersToImage(t, "color/picker_layout_basic.png", window.Canvas())
}

func Test_colorRecentPicker_Layout(t *testing.T) {
	a := test.NewTempApp(t)

	// Inject recent preferences
	bellui.SetPref(a.Preferences(), "color_recents", "#0000FF,#008000,#FF0000")

	color := newColorRecentPicker(nil)

	window := test.NewTempWindow(t, container.NewCenter(color))
	window.Resize(color.MinSize().Max(geo.NewSize(360, 60)))

	test.AssertRendersToImage(t, "color/picker_layout_recent.png", window.Canvas())
}

func Test_colorAdvancedPicker_Layout(t *testing.T) {
	test.NewTempApp(t)

	color := newColorAdvancedPicker(theme.Color(theme.ColorNamePrimary), nil)

	color.Refresh()

	window := test.NewTempWindow(t, container.NewCenter(color))
	window.Resize(color.MinSize().Max(geo.NewSize(200, 200)))

	test.AssertRendersToImage(t, "color/picker_layout_advanced.png", window.Canvas())
}
