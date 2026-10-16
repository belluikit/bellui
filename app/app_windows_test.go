//go:build !ci && !android && !ios

package app

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNotificationTemplateNoInterpolation guards against re-introducing the
// PowerShell command-injection hole (pre-rebrand tracker #287): notification values must be read
// from the environment, never formatted into the script body.
func TestNotificationTemplateNoInterpolation(t *testing.T) {
	assert.NotContains(t, notificationTemplate, "%s",
		"notification values must not be string-interpolated into the script")
	assert.Contains(t, notificationTemplate, "$env:BELLUI_NOTIFY_TITLE")
	assert.Contains(t, notificationTemplate, "$env:BELLUI_NOTIFY_CONTENT")
	assert.Contains(t, notificationTemplate, "$env:BELLUI_NOTIFY_APPID")
	assert.True(t, strings.HasPrefix(notificationTemplate, "$title = $env:BELLUI_NOTIFY_TITLE"))
}
