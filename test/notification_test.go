package test

import (
	"testing"

	"github.com/belluikit/bellui"

	"github.com/stretchr/testify/assert"
)

func TestAssertNotificationSent(t *testing.T) {
	n := bellui.NewNotification("Test Title", "Some content")
	myApp := bellui.CurrentApp()

	AssertNotificationSent(t, n, func() {
		bellui.SendNotification(n)
	})
	assert.Equal(t, myApp, bellui.CurrentApp())
}

func TestAssertNotificationSent_Nil(t *testing.T) {
	AssertNotificationSent(t, nil, func() {
		// don't send anything
	})
}

func TestAssertNotificationSent_NotSent(t *testing.T) {
	tt := &testing.T{}

	AssertNotificationSent(tt, &bellui.Notification{}, func() {
		// don't send anything
	})
	assert.True(t, tt.Failed(), "notification assert should fail if no notification was sent")
}

func TestSendNotificationWithOptions(t *testing.T) {
	NewTempApp(t)

	tapped := false
	opts := &bellui.NotificationOptions{
		Title:    "Rich Title",
		Content:  "Rich content",
		OnTapped: func() { tapped = true },
		Actions:  []bellui.NotificationAction{{Label: "Do it", Callback: func() {}}},
	}

	// The test app implements bellui.NotifierWithOptionsApp, so the rich form is
	// delivered and the free function reports true. It also records the reduced
	// plain notification, so AssertNotificationSent still passes.
	delivered := bellui.SendNotificationWithOptions(opts)
	assert.True(t, delivered)

	AssertNotificationSent(t, bellui.NewNotification("Rich Title", "Rich content"), func() {
		bellui.SendNotificationWithOptions(opts)
	})
	assert.False(t, tapped, "callbacks are only fired by the OS, not on send")
}
