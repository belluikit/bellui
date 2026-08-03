package dialog_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/dialog"
	"github.com/belluikit/bellui/geo"
	intdriver "github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// Escape is how a user cancels a dialog, and cancelling has to reach the
// dialog's own close path — the callback carries the answer.
func TestConfirm_EscapeCancels(t *testing.T) {
	test.NewTempApp(t)
	w := test.NewTempWindow(t, widget.NewLabel("content"))
	w.Resize(geo.NewSize(400, 300))

	answered := 0
	var answer bool
	d := dialog.NewConfirm("Delete?", "Really delete it?", func(ok bool) {
		answered++
		answer = ok
	}, w)
	d.Show()
	require.Len(t, w.Canvas().Overlays().List(), 1, "the dialog should be up")

	require.True(t, intdriver.HandleEscape(w.Canvas()), "Escape should close the dialog")
	assert.Equal(t, 1, answered, "the dialog's callback must run — cancelling is an answer")
	assert.False(t, answer, "Escape means no")
	assert.Empty(t, w.Canvas().Overlays().List(), "the dialog should be gone")
}

func TestInformation_EscapeCloses(t *testing.T) {
	test.NewTempApp(t)
	w := test.NewTempWindow(t, widget.NewLabel("content"))
	w.Resize(geo.NewSize(400, 300))

	d := dialog.NewInformation("Done", "That worked.", w)
	closed := 0
	d.SetOnClosed(func() { closed++ })
	d.Show()
	require.Len(t, w.Canvas().Overlays().List(), 1)

	require.True(t, intdriver.HandleEscape(w.Canvas()))
	assert.Equal(t, 1, closed)
	assert.Empty(t, w.Canvas().Overlays().List())
}
