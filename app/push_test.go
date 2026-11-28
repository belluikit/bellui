package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
)

func TestDecodePush(t *testing.T) {
	n := decodePush([]byte(`{"aps":{"alert":{"title":"Invite","body":"Join Ana"},"sound":"default"},"data":{"room":"42"}}`))
	require.NotNil(t, n)
	assert.Equal(t, "Invite", n.Title)
	assert.Equal(t, "Join Ana", n.Body)
	assert.Equal(t, map[string]any{"room": "42"}, n.Payload["data"])
	assert.Contains(t, n.Payload, "aps", "the whole payload is kept, aps included")

	// APNs allows the alert as a plain string: that is the body.
	n = decodePush([]byte(`{"aps":{"alert":"Hello"}}`))
	require.NotNil(t, n)
	assert.Empty(t, n.Title)
	assert.Equal(t, "Hello", n.Body)

	// No alert at all (a badge-only push) still delivers the payload.
	n = decodePush([]byte(`{"aps":{"badge":3},"k":"v"}`))
	require.NotNil(t, n)
	assert.Empty(t, n.Title+n.Body)
	assert.Equal(t, "v", n.Payload["k"])

	assert.Nil(t, decodePush([]byte(`not json`)))
	assert.Nil(t, decodePush([]byte(`null`)))
	assert.Nil(t, decodePush([]byte(`[1]`)))
}

// A tap that arrives before anything can take it is held: before the app
// runs, and while it runs with no callback.
func TestPushOpened_HeldWithoutCallbackOrRun(t *testing.T) {
	a := &belluiApp{}
	a.pushOpened(&bellui.PushNotification{Body: "cold"})
	a.setOnPushOpened(func(*bellui.PushNotification) { t.Error("delivered before the app ran") })
	require.Len(t, a.pendingOpens, 1, "a callback alone does not deliver before Run")

	a.setOnPushOpened(nil)
	a.pushOpened(nil) // nothing to hold
	assert.Len(t, a.pendingOpens, 1)
	assert.Equal(t, "cold", a.pendingOpens[0].push.Body)
}

// With no foreground callback the push is shown, and the answer comes at once.
func TestPushReceived_NoCallbackShows(t *testing.T) {
	a := &belluiApp{}
	var answers []bool
	a.pushReceived(&bellui.PushNotification{}, func(show bool) { answers = append(answers, show) })
	assert.Equal(t, []bool{true}, answers)

	a.setOnPushReceived(func(*bellui.PushNotification) bool { return false })
	assert.NotNil(t, a.onPushReceived)
	a.setOnPushReceived(nil)
	a.pushReceived(&bellui.PushNotification{}, func(show bool) { answers = append(answers, show) })
	assert.Equal(t, []bool{true, true}, answers, "clearing the callback shows again")
}
