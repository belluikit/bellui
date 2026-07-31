//go:build !wasm && !darwin && !ios && !android && !windows && (!linux || !cgo)

package webview

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/belluikit/bellui/test"
	"github.com/stretchr/testify/assert"
)

// These tests lock the degradation contract on targets with no web engine
// binding: Supported is false, Attach reports ErrNotAttached rather than
// panicking, and callbacks are still answered. That guarantee — not any
// particular engine — is what lets a cross-platform app ship one code path
// and degrade to an inert host where there is nothing to bind.

func TestUnsupported_SupportedIsFalse(t *testing.T) {
	assert.False(t, Supported)
}

func TestUnsupported_AttachReportsNotAttached(t *testing.T) {
	a := test.NewTempApp(t)
	w := a.NewWindow("unsupported")

	view := New("https://example.com")
	err := view.Attach(w)
	assert.True(t, errors.Is(err, ErrNotAttached),
		"Attach on an unsupported target must report ErrNotAttached, got %v", err)

	// The view stays usable as an inert object: state reads back, and a
	// result-bearing call is answered exactly once instead of hanging.
	assert.Equal(t, "https://example.com", view.URL())
	called := false
	view.EvalResult("1+1", func(_ json.RawMessage, err error) {
		called = true
		assert.True(t, errors.Is(err, ErrNotAttached))
	})
	assert.True(t, called, "EvalResult must answer immediately when there is no engine")
}
