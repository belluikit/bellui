package posthog

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewWithHost(t *testing.T) {
	s := NewWithHost("k", "https://eu.i.posthog.com/")
	assert.Equal(t, "https://eu.i.posthog.com/batch/", s.url)
}
