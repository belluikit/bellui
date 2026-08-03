//go:build wasm || js

package dialog

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/test"

	"github.com/stretchr/testify/assert"
)

// The wasm save override has no picker-close moment: the writer is handed out
// immediately and the "dialog" is the download offered when it is closed.
// Closing the writer is the closest thing to confirming the dialog, so that is
// when SetOnClosed must report — after the result callback, unlike the
// picker-backed paths, and never at all while the writer stays open (#233).
func TestFileSaveOSOverrideReportsCloseAtWriterClose(t *testing.T) {
	test.NewTempApp(t)

	var writer bellui.URIWriteCloser
	d := NewFileSave(func(w bellui.URIWriteCloser, err error) {
		writer = w
		assert.NoError(t, err)
	}, nil)

	closed := 0
	d.SetOnClosed(func() { closed++ })

	assert.True(t, fileSaveOSOverride(d))
	assert.NotNil(t, writer, "the override hands the writer out immediately")
	assert.Zero(t, closed, "no close report while the writer is still open")

	_, err := writer.Write([]byte("payload"))
	assert.NoError(t, err)
	assert.NoError(t, writer.Close())
	assert.Equal(t, 1, closed, "closing the writer reports the close once")
}
