//go:build (!windows || !ci) && !wasm

package mobile

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver/mobile/event/key"
	"github.com/belluikit/bellui/widget"
)

// A hardware keyboard's Ctrl+Tab leaves a multi-line Entry, which keeps plain
// Tab for itself (#340).
func Test_driver_CtrlTabLeavesMultiLineEntry(t *testing.T) {
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	entry := widget.NewMultiLineEntry()
	button := widget.NewButton("OK", func() {})
	c.SetContent(container.NewVBox(entry, button))
	c.Resize(geo.NewSize(200, 200))

	c.Focus(entry)
	d.typeDownCanvas(c, '\t', key.CodeTab, 0)
	assert.Equal(t, entry, c.Focused(), "plain Tab stays in the entry")

	d.typeDownCanvas(c, 0, key.CodeTab, key.ModControl)
	assert.Equal(t, button, c.Focused(), "Ctrl+Tab leaves the entry")

	c.Focus(entry)
	d.typeDownCanvas(c, 0, key.CodeTab, key.ModControl|key.ModShift)
	assert.Equal(t, button, c.Focused(), "Ctrl+Shift+Tab leaves backwards (wraps to the only other focusable)")
}
