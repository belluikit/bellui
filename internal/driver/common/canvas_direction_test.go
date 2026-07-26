package common

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
)

func TestCanvas_Direction(t *testing.T) {
	c := &Canvas{}
	c.Initialize(nil, func() {})

	// The default defers to the process-global setting.
	assert.Equal(t, bellui.DirectionAuto, c.Direction())

	c.SetDirection(bellui.DirectionRTL)
	assert.Equal(t, bellui.DirectionRTL, c.Direction())

	// DirectionAuto restores the deferral to the global setting.
	c.SetDirection(bellui.DirectionAuto)
	assert.Equal(t, bellui.DirectionAuto, c.Direction())
}
