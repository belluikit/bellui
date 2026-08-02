//go:build linux && !android && cgo

package webview

import (
	"testing"

	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
	"github.com/belluikit/gocef"
	"github.com/stretchr/testify/assert"
)

// The #126 contract: a live page owns plain Tab; an inert host declines the
// capture so it cannot strand keyboard focus (the WCAG 2.1.2 concern).
func TestOffscreenHostAcceptsTab(t *testing.T) {
	test.NewTempApp(t)

	v := New("about:blank")
	h := newOffscreenHost(v)
	assert.False(t, h.AcceptsTab(), "no engine: an ordinary Tab stop")

	// A zero-value Browser is enough for the capture decision — AcceptsTab
	// only checks liveness, and no CEF call is made.
	v.eng.browser = &gocef.Browser{}
	assert.True(t, h.AcceptsTab(), "live page: the host owns plain Tab")
}

func TestOffscreenHostTabEscape(t *testing.T) {
	assert.True(t, tabEscape(&event.KeyEvent{Name: event.KeyTab, Modifier: event.KeyModifierControl}))
	assert.True(t, tabEscape(&event.KeyEvent{
		Name:     event.KeyTab,
		Modifier: event.KeyModifierControl | event.KeyModifierShift,
	}))
	assert.False(t, tabEscape(&event.KeyEvent{Name: event.KeyTab}), "plain Tab belongs to the page")
	assert.False(t, tabEscape(&event.KeyEvent{Name: event.KeyTab, Modifier: event.KeyModifierShift}),
		"Shift+Tab is backward page traversal, not the exit")
	assert.False(t, tabEscape(&event.KeyEvent{Name: event.KeyA, Modifier: event.KeyModifierControl}))
}

// Ctrl+Tab / Ctrl+Shift+Tab move bellui's focus off the host — the documented
// exit from the page's tab order. The host is deliberately inert here (nil
// browser): the escape must work whether or not a page is alive.
func TestOffscreenHostCtrlTabMovesFocus(t *testing.T) {
	test.NewTempApp(t)

	v := New("about:blank")
	h := newOffscreenHost(v)
	entry := widget.NewEntry()

	w := test.NewTempWindow(t, container.NewVBox(h, entry))
	c := w.Canvas()
	c.Focus(h)
	assert.True(t, h.focused)

	h.KeyDown(&event.KeyEvent{Name: event.KeyTab, Modifier: event.KeyModifierControl})
	assert.Equal(t, entry, c.Focused(), "Ctrl+Tab leaves the page for the next focusable")

	c.Focus(h)
	h.KeyDown(&event.KeyEvent{Name: event.KeyTab, Modifier: event.KeyModifierControl | event.KeyModifierShift})
	assert.Equal(t, entry, c.Focused(), "Ctrl+Shift+Tab exits backwards (wraps to the only other focusable)")
}

// The escape halves never reach the page, and TypedKey ignores everything but
// Tab — with no engine none of these may panic.
func TestOffscreenHostKeyHandlersInert(t *testing.T) {
	test.NewTempApp(t)

	v := New("about:blank")
	h := newOffscreenHost(v)

	h.TypedKey(&event.KeyEvent{Name: event.KeyTab})
	h.TypedKey(&event.KeyEvent{Name: event.KeyA})
	h.KeyDown(&event.KeyEvent{Name: event.KeyTab, Modifier: event.KeyModifierControl})
	h.KeyUp(&event.KeyEvent{Name: event.KeyTab, Modifier: event.KeyModifierControl})
	h.KeyUp(&event.KeyEvent{Name: event.KeyTab})
}
