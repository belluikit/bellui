package bellui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/internal/activity"
)

func TestLastUserActivity(t *testing.T) {
	t.Cleanup(activity.Reset)
	activity.Reset()
	assert.True(t, LastUserActivity().IsZero(), "zero until the user gives input")

	activity.Stamp()
	assert.Equal(t, activity.Last(), LastUserActivity())
	assert.False(t, LastUserActivity().IsZero())
}
