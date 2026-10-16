package dialog

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// reportCancel must pass a typed nil result for every dialog mode. The
// multi-select shape was missing, so cancelling a canvas-served multi-select
// dialog panicked with an interface-conversion error (#253) — found live in
// the #239 iPad pass, where mobile serves multi-select through the canvas
// dialog (pre-rebrand tracker #716).
func TestFileDialog_CancelReportsEveryCallbackShape(t *testing.T) {
	test.NewTempApp(t)
	w := test.NewWindow(widget.NewLabel("x"))
	defer w.Close()

	t.Run("multi-select open", func(t *testing.T) {
		called := false
		d := NewFileOpenMultiple(func(readers []bellui.URIReadCloser, err error) {
			called = true
			assert.Nil(t, readers)
			assert.NoError(t, err)
		}, w)
		closed := false
		d.SetOnClosed(func() { closed = true })
		d.Show()
		require.NotNil(t, d.dialog, "multi-select must be served by the canvas dialog")

		require.NotPanics(t, func() { d.Dismiss() })
		assert.True(t, called, "cancel reports a nil result to the multi callback")
		assert.True(t, closed)
	})

	t.Run("single open", func(t *testing.T) {
		called := false
		d := NewFileOpen(func(r bellui.URIReadCloser, err error) {
			called = true
			assert.Nil(t, r)
			assert.NoError(t, err)
		}, w)
		d.Show()
		require.NotNil(t, d.dialog)
		require.NotPanics(t, func() { d.Dismiss() })
		assert.True(t, called)
	})

	t.Run("folder open", func(t *testing.T) {
		called := false
		d := NewFolderOpen(func(l bellui.ListableURI, err error) {
			called = true
			assert.Nil(t, l)
			assert.NoError(t, err)
		}, w)
		d.Show()
		require.NotNil(t, d.dialog)
		require.NotPanics(t, func() { d.Dismiss() })
		assert.True(t, called)
	})

	t.Run("save", func(t *testing.T) {
		called := false
		d := NewFileSave(func(wr bellui.URIWriteCloser, err error) {
			called = true
			assert.Nil(t, wr)
			assert.NoError(t, err)
		}, w)
		d.Show()
		require.NotNil(t, d.dialog)
		require.NotPanics(t, func() { d.Dismiss() })
		assert.True(t, called)
	})
}
