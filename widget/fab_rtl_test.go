package widget_test

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// extendedFABParts lays out an extended FAB at its own size in a window and
// returns its icon and label as the renderer placed them.
func extendedFABParts(t *testing.T) (*widget.FAB, *canvas.Image, *canvas.Text) {
	t.Helper()
	fab := widget.NewExtendedFAB("Create", theme.ContentAddIcon(), nil)
	w := test.NewTempWindow(t, container.NewCenter(fab))
	w.Resize(geo.NewSize(400, 200))

	var icon *canvas.Image
	var label *canvas.Text
	for _, o := range test.TempWidgetRenderer(t, fab).Objects() {
		switch c := o.(type) {
		case *canvas.Image:
			icon = c
		case *canvas.Text:
			label = c
		}
	}
	require.NotNil(t, icon)
	require.NotNil(t, label)
	return fab, icon, label
}

func right(o core.CanvasObject) float32 { return o.Position().X + o.Size().Width }

func TestExtendedFAB_LTR_IconLeadsLabel(t *testing.T) {
	test.NewTempApp(t)
	_, icon, label := extendedFABParts(t)

	assert.Equal(t, float32(16), icon.Position().X, "leading padding")
	assert.Equal(t, label.MinSize().Width, label.Size().Width, "label box fits its text")
	assert.Equal(t, float32(8), label.Position().X-right(icon), "icon-to-label gap")
}

// Under RTL the icon leads from the right edge and the label follows to its
// left, with the same paddings and gap as LTR; nothing overlaps (#945).
func TestExtendedFAB_RTL_IconLeadsLabelFromTheRight(t *testing.T) {
	test.NewTempApp(t)
	bellui.SetLayoutDirection(bellui.DirectionRTL)
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })

	fab, icon, label := extendedFABParts(t)

	assert.Equal(t, fab.Size().Width-16, right(icon), "leading padding, from the right")
	assert.Equal(t, label.MinSize().Width, label.Size().Width,
		"label box fits its text, so right-aligned text cannot spill onto the icon")
	assert.Equal(t, float32(8), icon.Position().X-right(label), "icon-to-label gap")
	assert.Equal(t, float32(20), label.Position().X, "trailing padding, on the left")
}
