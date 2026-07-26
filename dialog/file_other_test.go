//go:build !windows

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
