//go:build !tamago && !noos

package test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
)

// AssertNotificationSent allows an app developer to assert that a notification was sent.
// After the content of f has executed this utility will check that the specified notification was sent.
func AssertNotificationSent(t *testing.T, n *bellui.Notification, f func()) {
	require.NotNil(t, f, "function has to be specified")
	require.IsType(t, &app{}, bellui.CurrentApp())
	a := bellui.CurrentApp().(*app)
	a.lastNotification = nil

	f()
	if n == nil {
		assert.Nil(t, a.lastNotification)
		return
	} else if a.lastNotification == nil {
		t.Error("No notification sent")
		return
	}

	assert.Equal(t, n.Title, a.lastNotification.Title)
	assert.Equal(t, n.Content, a.lastNotification.Content)
}
