package dialog

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	internalwidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/storage"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// TestFileDialogItem_PressFeedback is #1091: a file or folder in the dialog
// shows the pressed state from the press to the release, over the whole item.
func TestFileDialogItem_PressFeedback(t *testing.T) {
	test.NewTempApp(t)
	f := &fileDialog{}
	item := f.newFileItem(storage.NewFileURI("/tmp/a.txt"), false, false)
	item.Resize(geo.NewSize(80, 90))
	cache.Renderer(item)
	press := internalwidget.PressFeedbackOf(&item.PressTarget)

	item.TouchDown(&mobile.TouchEvent{PointEvent: event.PointEvent{Position: geo.NewPos(10, 20)}})
	assert.True(t, press.Pressed(), "no pressed state while the finger is down")
	assert.Equal(t, item.Size(), press.Ripple.Size())
	assert.Equal(t, test.Theme().Size(theme.SizeNameShapeSmall), press.Ripple.CornerRadius)
	wr, wg, wb, _ := test.Theme().Color(theme.ColorNameOnSurface, bellui.CurrentApp().Settings().ThemeVariant()).RGBA()
	r, g, b, _ := press.Ripple.Color.RGBA()
	assert.Equal(t, [3]uint32{wr >> 8, wg >> 8, wb >> 8}, [3]uint32{r >> 8, g >> 8, b >> 8})
	item.TouchUp(&mobile.TouchEvent{})
	assert.False(t, press.Pressed())

	item.TouchDown(&mobile.TouchEvent{})
	item.TouchCancel(&mobile.TouchEvent{})
	assert.False(t, press.Ripple.Visible(), "a touch that became a scroll left the item pressed")

	item.MouseDown(&desktop.MouseEvent{Button: desktop.MouseButtonPrimary})
	item.HoverOut()
	assert.False(t, press.Pressed(), "a press dragged off the item stayed")
}
