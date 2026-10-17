//go:build !windows || !ci

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

// Escape then Tab leaves a multi-line Entry with a hardware keyboard too (#357).
func Test_driver_EscapeThenTabLeavesMultiLineEntry(t *testing.T) {
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	entry := widget.NewMultiLineEntry()
	button := widget.NewButton("OK", func() {})
	c.SetContent(container.NewVBox(entry, button))
	c.Resize(geo.NewSize(200, 200))

	c.Focus(entry)
	d.typeDownCanvas(c, 0, key.CodeTab, 0)
	assert.Equal(t, entry, c.Focused(), "Tab is typed into the entry")

	d.typeDownCanvas(c, 0, key.CodeEscape, 0)
	d.typeDownCanvas(c, 0, key.CodeTab, 0)
	assert.Equal(t, button, c.Focused(), "Escape then Tab leaves it")

	c.Focus(entry)
	d.typeDownCanvas(c, 0, key.CodeEscape, 0)
	d.typeDownCanvas(c, 0, key.CodeLeftShift, key.ModShift)
	d.typeDownCanvas(c, 0, key.CodeTab, key.ModShift)
	assert.Equal(t, button, c.Focused(), "Escape then Shift+Tab leaves backwards (wrapping to the last widget)")
}
