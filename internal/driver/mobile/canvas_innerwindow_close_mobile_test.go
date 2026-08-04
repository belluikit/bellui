//go:build mobile && (!windows || !ci)

package mobile

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	_ "github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

// Test_windowedDialogFrameCloseButton_TouchTarget locks in the #254 fix: on a
// mobile device the InnerWindow frame buttons must meet the minimum touch
// target, and a tap shifted up by the iOS/Android tapYOffset finger
// compensation (-8dp) must still resolve to the close button rather than the
// inert title bar.
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

	target := theme.Current().Size(theme.SizeNameIconButtonSize)
	btnSize := closeBtn.Size()
	assert.GreaterOrEqual(t, btnSize.Width, target, "frame button width must meet the touch target")
	assert.GreaterOrEqual(t, btnSize.Height, target, "frame button height must meet the touch target")

	center := btnPos.Add(geo.NewPos(btnSize.Width/2, btnSize.Height/2))
	tapAt(c, center)
	assert.Equal(t, 1, closed, "tap on the close button center must fire CloseIntercept")

	// The iOS driver shifts every touch up by 8dp (tapYOffset) before
	// hit-testing; a tap on the visual center must still hit.
	closed = 0
	tapAt(c, center.AddXY(0, -8))
	assert.Equal(t, 1, closed, "tap shifted by the tapYOffset must still hit the close button")
}
