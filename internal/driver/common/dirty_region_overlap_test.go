package common

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/geo"
)

func TestInDirtyRegion(t *testing.T) {
	across := func(rPos geo.Position, rSize geo.Size) bool {
		return InDirtyRegion(geo.NewPos(-10, -10), geo.NewSize(50, 50), rPos, rSize)
	}
	assert.False(t, across(geo.Position{}, geo.Size{}), "an empty region holds nothing, even under a rect across it (#605)")
	assert.False(t, across(geo.NewPos(5, 5), geo.NewSize(0, 20)), "nor does a zero-width one")
	assert.True(t, across(geo.NewPos(5, 5), geo.NewSize(20, 20)), "a region the rect covers")
	assert.False(t, across(geo.NewPos(40, 5), geo.NewSize(20, 20)), "a region beside it")
}
