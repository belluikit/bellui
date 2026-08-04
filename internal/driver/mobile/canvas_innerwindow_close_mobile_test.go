//go:build mobile && (!windows || !ci)

package mobile

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	_ "github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

// Test_windowedDialogFrameCloseButton_TouchTarget locks in the #254 fix: on a
// mobile device the InnerWindow frame buttons keep the icon-button visual but
// expose the full touch target around it (visual + padding ring, 48dp on the
// default theme — the MD3/Android floor). A tap shifted up by the iOS/Android
// tapYOffset finger compensation (-8dp) and a tap on the ring outside the
// visual must both resolve to the close button rather than the inert title
// bar.
func Test_windowedDialogFrameCloseButton_TouchTarget(t *testing.T) {
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	c.SetContent(widget.NewLabel("content behind"))
	c.Resize(geo.NewSize(600, 600))

	closed := 0
	pop := showWindowedDialogFrame(c, func() { closed++ })

	closeBtn, btnPos := frameCloseButton(pop)
	if closeBtn == nil {
		t.Fatal("no frame buttons found in the InnerWindow title bar")
	}

	th := theme.Current()
	visual := th.Size(theme.SizeNameIconButtonSize)
	pad := th.Size(theme.SizeNamePadding)
	target := visual + pad*2

	btnSize := closeBtn.Size()
	assert.Equal(t, geo.NewSquareSize(visual), btnSize, "the visual button must keep the icon-button size")

	// The ring around the visual belongs to the touch target: the tappable
	// object resolved there must span at least the platform floor.
	ringPos := btnPos.AddXY(btnSize.Width+pad/2, btnSize.Height/2)
	ring, _, _ := c.FindObjectAtPositionMatching(ringPos, func(object core.CanvasObject) bool {
		_, ok := object.(event.Tappable)
		return ok
	})
	if assert.NotNil(t, ring, "the ring outside the visual button must resolve to a tappable object") {
		assert.GreaterOrEqual(t, ring.Size().Width, target, "touch-target width must meet the platform floor")
		assert.GreaterOrEqual(t, ring.Size().Height, target, "touch-target height must meet the platform floor")
	}

	center := btnPos.Add(geo.NewPos(btnSize.Width/2, btnSize.Height/2))
	tapAt(c, center)
	assert.Equal(t, 1, closed, "tap on the close button center must fire CloseIntercept")

	// The iOS driver shifts every touch up by 8dp (tapYOffset) before
	// hit-testing; a tap on the visual center must still hit.
	closed = 0
	tapAt(c, center.AddXY(0, -8))
	assert.Equal(t, 1, closed, "tap shifted by the tapYOffset must still hit the close button")

	// A tap on the ring — inside the touch target, outside the visual — must
	// activate the button too.
	closed = 0
	tapAt(c, ringPos)
	assert.Equal(t, 1, closed, "tap on the touch-target ring must hit the close button")
}
