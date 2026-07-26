package widget

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

func TestToast_Show(t *testing.T) {
	w := test.NewTempWindow(t, NewLabel("Content"))
	c := w.Canvas()

	toast := newToast("Hello", MediumImportance, c)
	toast.Duration = 50 * time.Millisecond
	toast.Show()
	t.Cleanup(toast.Dismiss)

	assert.NotNil(t, toast.popUp)
}

func TestToast_Dismiss(t *testing.T) {
	w := test.NewTempWindow(t, NewLabel("Content"))
	c := w.Canvas()

	toast := newToast("Hello", MediumImportance, c)
	toast.Show()
	assert.NotNil(t, toast.popUp)

	toast.Dismiss()
	toast.mu.Lock()
	assert.Nil(t, toast.popUp)
	toast.mu.Unlock()
}

func TestToast_DoubleDismiss(t *testing.T) {
	w := test.NewTempWindow(t, NewLabel("Content"))
	c := w.Canvas()

	toast := newToast("Hello", MediumImportance, c)
	toast.Show()
	toast.Dismiss()
	toast.Dismiss() // should not panic
}

func TestToast_Colors(t *testing.T) {
	tests := []struct {
		importance Importance
		fg         string
		bg         string
	}{
		{MediumImportance, string(theme.ColorNameForeground), string(theme.ColorNameOverlayBackground)},
		{DangerImportance, string(theme.ColorNameForegroundOnError), string(theme.ColorNameError)},
		{SuccessImportance, string(theme.ColorNameForegroundOnSuccess), string(theme.ColorNameSuccess)},
		{WarningImportance, string(theme.ColorNameForegroundOnWarning), string(theme.ColorNameWarning)},
	}
	for _, tt := range tests {
		toast := newToast("test", tt.importance, nil)
		fg, bg := toast.colors()
		assert.Equal(t, tt.fg, string(fg))
		assert.Equal(t, tt.bg, string(bg))
	}
}

func TestToast_UsesCanvasText(t *testing.T) {
	w := test.NewTempWindow(t, NewLabel("Content"))
	c := w.Canvas()

	for _, imp := range []Importance{MediumImportance, DangerImportance, SuccessImportance, WarningImportance} {
		toast := newToast("Test text", imp, c)
		toast.Show()

		if toast.popUp != nil {
			found := false
			content := toast.popUp.Content.(*bellui.Container)
			for _, obj := range content.Objects {
				if txt, ok := obj.(*canvas.Text); ok {
					assert.Equal(t, "Test text", txt.Text)
					assert.NotNil(t, txt.Color, "Text should have a color set")
					found = true
				}
			}
			assert.True(t, found, "Should find canvas.Text in toast for importance %d", imp)
		}
		toast.Dismiss()
	}
}

func TestShowToast(t *testing.T) {
	w := test.NewTempWindow(t, NewLabel("Content"))
	c := w.Canvas()

	toast := ShowToast("Hello", c)
	t.Cleanup(toast.Dismiss)
}

func TestShowToastWithImportance(t *testing.T) {
	w := test.NewTempWindow(t, NewLabel("Content"))
	c := w.Canvas()

	for _, imp := range []Importance{WarningImportance, DangerImportance, SuccessImportance} {
		toast := ShowToastWithImportance("Message", imp, c)
		t.Cleanup(toast.Dismiss)
	}
}

func TestToast_NilCanvas(t *testing.T) {
	toast := newToast("Hello", MediumImportance, nil)
	toast.Show() // should not panic
	assert.Nil(t, toast.popUp)
}

func TestToast_DoesNotBlockContentBeneath(t *testing.T) {
	tapped := 0
	btn := NewButton("Tap", func() { tapped++ })
	w := test.NewTempWindow(t, btn)
	w.Resize(geo.NewSize(200, 200))
	c := w.Canvas()

	toast := newToast("Transient", MediumImportance, c)
	toast.Duration = time.Minute
	toast.Show()
	t.Cleanup(toast.Dismiss)

	// A tap aimed at the button beneath the toast reaches it (#569)...
	btnCenter := btn.Position().Add(geo.NewPos(btn.Size().Width/2, btn.Size().Height/2))
	test.TapCanvas(c, btnCenter)
	assert.Equal(t, 1, tapped, "tap passes through the transient toast")

	// ...and does not dismiss the toast: it persists for its duration.
	assert.True(t, toast.popUp.Visible(), "toast persists through outside taps")
}

func TestToast_DoesNotStealFocus(t *testing.T) {
	entry := NewEntry()
	w := test.NewTempWindow(t, entry)
	w.Resize(geo.NewSize(200, 200))
	c := w.Canvas()

	c.Focus(entry)
	toast := newToast("Transient", MediumImportance, c)
	toast.Duration = time.Minute
	toast.Show()
	t.Cleanup(toast.Dismiss)

	// Typing while the toast is visible still reaches the focused entry:
	// the pass-through pop-up never owns the canvas focus routing (#569).
	test.Type(entry, "hi")
	assert.Equal(t, "hi", entry.Text)
	assert.Equal(t, entry, c.Focused())
}
