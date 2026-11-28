//go:build ci

package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
)

// The hold-until-handler rule for a tapped push (#1517), through the real
// main-goroutine delivery: a tap that launches the app arrives before Run and
// before the app sets its callback, and must reach the callback once both
// have happened, in order, and only once.
func TestPushOpened_ColdLaunchTapHeldUntilCallback(t *testing.T) {
	a := NewWithID("io.bellui.test.push-cold").(*belluiApp)

	a.pushOpened(&bellui.PushNotification{Body: "first"})  // the launching tap
	a.startOpenDelivery()                                  // Run: no callback yet
	a.pushOpened(&bellui.PushNotification{Body: "second"}) // running, still none

	got := make(chan string, 4)
	a.setOnPushOpened(func(n *bellui.PushNotification) { got <- n.Body })

	for _, want := range []string{"first", "second"} {
		select {
		case b := <-got:
			assert.Equal(t, want, b)
		case <-time.After(2 * time.Second):
			t.Fatalf("held tap %q never delivered", want)
		}
	}

	// With the callback set, a tap while running goes straight through.
	a.pushOpened(&bellui.PushNotification{Body: "warm"})
	select {
	case b := <-got:
		assert.Equal(t, "warm", b)
	case <-time.After(2 * time.Second):
		t.Fatal("tap while running never delivered")
	}
	select {
	case b := <-got:
		t.Fatalf("delivered twice: %q", b)
	case <-time.After(150 * time.Millisecond):
	}
	assert.Empty(t, a.pendingOpens)
}

// A tapped push does not take a file or URL callback's place, nor they its.
func TestPushOpened_NotDeliveredToOpenCallbacks(t *testing.T) {
	a := NewWithID("io.bellui.test.push-kinds").(*belluiApp)
	a.startOpenDelivery()
	a.SetOnFileOpened(func(bellui.URI) { t.Error("push delivered as a file") })
	a.pushOpened(&bellui.PushNotification{Body: "x"})
	require.Len(t, a.pendingOpens, 1, "held for the push callback")
}

// The foreground callback's answer is what the system is told.
func TestPushReceived_CallbackDecides(t *testing.T) {
	a := NewWithID("io.bellui.test.push-fg").(*belluiApp)
	a.setOnPushReceived(func(n *bellui.PushNotification) bool { return n.Title == "show me" })

	for _, c := range []struct {
		title string
		show  bool
	}{{"show me", true}, {"hide me", false}} {
		answer := make(chan bool, 2)
		a.pushReceived(&bellui.PushNotification{Title: c.title}, func(show bool) { answer <- show })
		select {
		case show := <-answer:
			assert.Equal(t, c.show, show, c.title)
		case <-time.After(2 * time.Second):
			t.Fatalf("%s: no answer", c.title)
		}
	}
}
