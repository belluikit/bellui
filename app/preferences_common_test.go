package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Preferences tests that hold on every platform, wasm included — no on-disk
// store, so nothing here reaches the `!wasm` file helpers (#77). The
// filesystem-backed cases live in preferences_test.go.

func TestPreferences_EmptyLoad(t *testing.T) {
	p := newPreferences(&belluiApp{uniqueID: ""})

	count := 0
	p.ReadValues(func(v map[string]any) {
		for range v {
			count++
		}
	})

	assert.Zero(t, count)
}
