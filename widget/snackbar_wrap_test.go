package widget_test

import (
	"testing"

	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// A short message looks as it did before wrapping existed (#1106).
func TestSnackbar_ShortRenders(t *testing.T) {
	test.NewTempApp(t)
	w := test.NewTempWindow(t, container.NewStack())
	w.Resize(geo.NewSize(360, 200))

	s := widget.NewSnackbar("Saved", "", nil)
	s.Show(w.Canvas())
	defer s.Hide()
	test.AssertRendersToImage(t, "snackbar/short.png", w.Canvas())

	s = widget.NewSnackbar("Like not kept", "Retry", func() {})
	s.Show(w.Canvas())
	defer s.Hide()
	test.AssertRendersToImage(t, "snackbar/short_action.png", w.Canvas())
}
