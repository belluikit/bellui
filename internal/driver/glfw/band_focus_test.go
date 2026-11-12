//go:build !no_glfw && !mobile

package glfw

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// drawerBelowHeader is a window with a header field above a field the drawer
// covers, and a drawer open below the header with a field of its own.
func drawerBelowHeader(t *testing.T) (w *window, header, covered, inDrawer *widget.Entry, drawer *widget.NavigationDrawer) {
	t.Helper()
	header, covered, inDrawer = widget.NewEntry(), widget.NewEntry(), widget.NewEntry()
	c := newCanvas()
	c.SetContent(container.NewVBox(header, covered))
	c.Resize(geo.NewSize(500, 500))
	w = &window{canvas: c}

	drawer = widget.NewNavigationDrawer([]*widget.NavigationDrawerItem{{Label: "Home", Icon: theme.HomeIcon()}}, nil)
	drawer.Header = inDrawer
	drawer.Modal = true
	drawer.Top = header.Position().Y + header.Size().Height
	require.Greater(t, drawer.Top, float32(0), "precondition: the header is laid out")
	drawer.Show(c)
	return w, header, covered, inDrawer, drawer
}

func (w *window) typeText(s string) {
	for _, r := range s {
		w.processCharInput(r)
	}
}

// With a drawer open below a header, a field in the header takes the focus
// and the keys typed after; Tab visits it and then the drawer (#1172).
func TestWindow_HeaderAboveDrawerTakesFocus(t *testing.T) {
	w, header, covered, inDrawer, drawer := drawerBelowHeader(t)

	w.canvas.Focus(header) // what a tap on it does
	assert.Equal(t, event.Focusable(header), w.canvas.Focused(), "the header field did not take the focus")
	w.typeText("hi")
	assert.Equal(t, "hi", header.Text, "the keys did not reach the header field")

	w.press(event.KeyTab, 0)
	assert.Equal(t, event.Focusable(inDrawer), w.canvas.Focused(), "Tab did not move on to the drawer")
	w.typeText("x")
	assert.Equal(t, "x", inDrawer.Text)
	assert.Equal(t, "hi", header.Text, "the header field kept the keys")

	w.press(event.KeyTab, 0)
	assert.Equal(t, event.Focusable(header), w.canvas.Focused(), "Tab did not come back to the header")
	w.press(event.KeyTab, event.KeyModifierShift)
	assert.Equal(t, event.Focusable(inDrawer), w.canvas.Focused(), "Shift+Tab did not go back to the drawer")

	// The field under the scrim is never reached by Tab, and focusing it from
	// code keeps it from the keys while the drawer is open.
	for range 4 {
		w.press(event.KeyTab, 0)
		assert.NotEqual(t, event.Focusable(covered), w.canvas.Focused(), "Tab reached the covered field")
	}
	w.canvas.Focus(header)
	w.canvas.Focus(covered)
	w.typeText("z")
	assert.Empty(t, covered.Text, "a key reached the field under the scrim")
	assert.Equal(t, "hi", header.Text, "the header field kept the keys")

	// Shown again, the drawer has the keyboard, not the content.
	drawer.Hide()
	drawer.Show(w.canvas)
	assert.Nil(t, w.canvas.Focused(), "the drawer shown again started with the content's focus")
}

// A drawer over the whole height leaves nothing to the content: a field in
// it does not take the keys, as before.
func TestWindow_FullHeightDrawerKeepsTheKeyboard(t *testing.T) {
	w, header, _, _, drawer := drawerBelowHeader(t)
	drawer.Hide()
	drawer.Top = 0
	drawer.Show(w.canvas)

	w.canvas.Focus(header)
	w.typeText("q")
	assert.Empty(t, header.Text, "a key reached a field under a full-height drawer")
}
