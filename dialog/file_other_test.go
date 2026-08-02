// Tagged like file_unix.go, whose one "Computer" place this asserts: the wasm
// build's getPlaces (file_wasm.go) returns no places at all, so places[0]
// panicked in-browser (#79).
//go:build !windows && !wasm && !js

package dialog

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The hidden-file convention is owned by storage.IsHidden and tested there.

func TestFileDialog_LoadPlaces(t *testing.T) {
	f := &fileDialog{}
	places := f.getPlaces()

	assert.Len(t, places, 1)
	assert.Equal(t, "Computer", places[0].locName)
}
