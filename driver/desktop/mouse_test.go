package desktop

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMouseButton(t *testing.T) {
	// check we're not accidentally adding consts
	assert.NotEqual(t, MouseButtonTertiary, MouseButtonPrimary|MouseButtonSecondary)
}
