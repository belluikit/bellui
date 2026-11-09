package activity

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStampAndLast(t *testing.T) {
	t.Cleanup(Reset)
	Reset()
	assert.True(t, Last().IsZero(), "no input yet reads as the zero time")

	before := time.Now()
	Stamp()
	after := time.Now()
	last := Last()
	assert.False(t, last.Before(before), "the stamp is not before the input")
	assert.False(t, last.After(after), "the stamp is not after the input")
	// A time with a monotonic reading prints it as "m=".
	assert.Contains(t, last.String(), "m=", "the stamp keeps a monotonic reading")

	Reset()
	assert.True(t, Last().IsZero())
}
