package gsportal

import (
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/event"
)

func TestPortalTrigger(t *testing.T) {
	assert.Equal(t, "CTRL+ALT+k",
		PortalTrigger(event.KeyModifierControl|event.KeyModifierAlt, event.KeyName("K")))
	assert.Equal(t, "CTRL+SHIFT+SUPER+a",
		PortalTrigger(event.KeyModifierControl|event.KeyModifierShift|event.KeyModifierSuper, event.KeyName("A")))
	assert.Equal(t, "SHIFT+F5",
		PortalTrigger(event.KeyModifierShift, event.KeyName("F5")))
	assert.Equal(t, "Space",
		PortalTrigger(0, event.KeyName("Space")))
	// digits and multi-char names pass through unchanged
	assert.Equal(t, "CTRL+1", PortalTrigger(event.KeyModifierControl, event.KeyName("1")))
}

func TestSenderToken(t *testing.T) {
	assert.Equal(t, "1_42", senderToken([]string{":1.42"}))
	assert.Equal(t, "1_100_5", senderToken([]string{":1.100.5", ":other"}))
	assert.Equal(t, "", senderToken(nil))
}

func TestRequestPath(t *testing.T) {
	assert.Equal(t,
		dbus.ObjectPath("/org/freedesktop/portal/desktop/request/1_42/bellui3"),
		requestPath("1_42", "bellui3"))
}

func TestRouteActivated_FiresHandlerViaDeliver(t *testing.T) {
	fired := make(chan string, 1)
	c := &Client{
		entries: map[string]*entry{
			"CustomDesktop:Control+Alt+K": {id: "CustomDesktop:Control+Alt+K", handler: func() { fired <- "hit" }},
		},
		// deliver runs the handler inline (a real client hops it to the main goroutine)
		deliver: func(fn func()) { fn() },
	}

	// Activated(o session, s shortcut_id, t timestamp, a{sv} options)
	c.routeActivated(&dbus.Signal{
		Name: gsIface + ".Activated",
		Body: []interface{}{dbus.ObjectPath("/session/1"), "CustomDesktop:Control+Alt+K", uint64(123), map[string]dbus.Variant{}},
	})
	select {
	case <-fired:
	case <-time.After(time.Second):
		t.Fatal("handler was not fired for a matching Activated signal")
	}

	// Unknown shortcut id → no handler, no panic.
	c.routeActivated(&dbus.Signal{
		Name: gsIface + ".Activated",
		Body: []interface{}{dbus.ObjectPath("/session/1"), "CustomDesktop:Unknown", uint64(1), map[string]dbus.Variant{}},
	})
	// Malformed body (too short) → ignored, no panic.
	c.routeActivated(&dbus.Signal{Name: gsIface + ".Activated", Body: []interface{}{dbus.ObjectPath("/s")}})
	select {
	case <-fired:
		t.Fatal("handler fired for a non-matching / malformed signal")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestRouteResponse_DeliversToWaiter(t *testing.T) {
	reqPath := requestPath("1_42", "bellui1")
	ch := make(chan responseMsg, 1)
	c := &Client{responses: map[dbus.ObjectPath]chan responseMsg{reqPath: ch}}

	c.routeResponse(&dbus.Signal{
		Name: requestIface + ".Response",
		Path: reqPath,
		Body: []interface{}{uint32(0), map[string]dbus.Variant{"session_handle": dbus.MakeVariant("/session/1")}},
	})

	select {
	case msg := <-ch:
		assert.Equal(t, uint32(0), msg.code)
		assert.Equal(t, "/session/1", msg.results["session_handle"].Value())
	case <-time.After(time.Second):
		t.Fatal("response was not delivered to the waiter")
	}

	// Response for an unknown request path → dropped, no panic.
	c.routeResponse(&dbus.Signal{
		Name: requestIface + ".Response",
		Path: dbus.ObjectPath("/unknown"),
		Body: []interface{}{uint32(0), map[string]dbus.Variant{}},
	})
}
