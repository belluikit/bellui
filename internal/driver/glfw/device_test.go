//go:build !wasm

package glfw

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/stretchr/testify/assert"
)

func Test_Device(t *testing.T) {
	dev := &glDevice{}

	assert.Equal(t, false, dev.IsMobile())
	assert.Equal(t, bellui.OrientationHorizontalLeft, dev.Orientation())
	assert.Equal(t, true, dev.HasKeyboard())
	assert.Equal(t, false, dev.IsBrowser())
}
