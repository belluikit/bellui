package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
)

func TestRegisterNotifyCallbacks_NoCallbacks(t *testing.T) {
	a := &belluiApp{}
	// A notification with neither a click handler nor actions needs no tracking.
	token := a.registerNotifyCallbacks(&bellui.NotificationOptions{Title: "T", Content: "C"})
	assert.Equal(t, uint64(0), token)
	assert.Empty(t, a.notifyHandlers)
}

func TestRegisterNotifyCallbacks_StoresAndForgets(t *testing.T) {
	a := &belluiApp{}
	opts := &bellui.NotificationOptions{
		Title:    "T",
		OnTapped: func() {},
		Actions: []bellui.NotificationAction{
			{Label: "Reply", Callback: func() {}},
			{Label: "Ignore", Callback: func() {}},
		},
	}

	t1 := a.registerNotifyCallbacks(opts)
	t2 := a.registerNotifyCallbacks(opts)
	assert.NotZero(t, t1)
	assert.NotEqual(t, t1, t2, "tokens must be unique")
	assert.Len(t, a.notifyHandlers, 2)

	cb := a.notifyHandlers[t1]
	assert.NotNil(t, cb.onTapped)
	assert.Len(t, cb.actions, 2)

	a.forgetNotify(t1)
	assert.Len(t, a.notifyHandlers, 1)
	a.forgetNotify(0) // no-op, must not panic or remove anything
	assert.Len(t, a.notifyHandlers, 1)
}

func TestRegisterNotifyCallbacks_ActionsOnly(t *testing.T) {
	a := &belluiApp{}
	// Actions but no OnTapped should still be tracked.
	token := a.registerNotifyCallbacks(&bellui.NotificationOptions{
		Actions: []bellui.NotificationAction{{Label: "Go", Callback: func() {}}},
	})
	assert.NotZero(t, token)
	assert.Nil(t, a.notifyHandlers[token].onTapped)
	assert.Len(t, a.notifyHandlers[token].actions, 1)
}

func TestPlainNotification(t *testing.T) {
	n := plainNotification(&bellui.NotificationOptions{Title: "Hi", Content: "There", OnTapped: func() {}})
	assert.Equal(t, "Hi", n.Title)
	assert.Equal(t, "There", n.Content)
}
