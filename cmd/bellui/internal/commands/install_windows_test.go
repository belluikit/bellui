//go:build windows

package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeProgID(t *testing.T) {
	assert.Equal(t, "com.example.app", sanitizeProgID("com.example.app"))
	assert.Equal(t, "My_App_1", sanitizeProgID("My App!1"))
	assert.Equal(t, "a_b_c", sanitizeProgID("a/b\\c"))
}
