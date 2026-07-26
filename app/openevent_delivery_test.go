//go:build ci

package app

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/storage"
)

// These tests exercise the actual main-goroutine delivery, which needs a driver
// whose event queue runs. The headless test driver (the ci build) runs queued
// functions inline, so delivery completes without a full App.Run loop.

func TestOpenEvent_BufferedUntilRunning(t *testing.T) {
	a := NewWithID("io.bellui.test.openevent-buf").(*belluiApp)

	got := make(chan bellui.URI, 1)
	a.SetOnFileOpened(func(u bellui.URI) { got <- u })

	// Delivered before Run -> must buffer, not fire.
	a.openFile(storage.NewFileURI("/tmp/doc.txt"))
	select {
	case <-got:
		t.Fatal("fired before the app was running")
	case <-time.After(150 * time.Millisecond):
	}

	// Marking the app running flushes the buffer onto the main goroutine.
	a.startOpenDelivery()
	select {
	case u := <-got:
		assert.Contains(t, u.String(), "doc.txt")
	case <-time.After(2 * time.Second):
		t.Fatal("buffered file event never delivered")
	}
}

func TestOpenEvent_HandlerRegisteredAfterPayload(t *testing.T) {
	a := NewWithID("io.bellui.test.openevent-late").(*belluiApp)
	a.startOpenDelivery() // running, but no handler yet

	u, _ := url.Parse("myapp://deeplink?x=1")
	a.openURL(u) // running + no handler -> buffered

	got := make(chan *url.URL, 1)
	a.SetOnURLOpened(func(u *url.URL) { got <- u }) // registration flushes

	select {
	case gotURL := <-got:
		assert.Equal(t, "myapp", gotURL.Scheme)
	case <-time.After(2 * time.Second):
		t.Fatal("late-registered handler never received the buffered URL")
	}
}

func TestOpenEvent_ForwardedArgsDispatch(t *testing.T) {
	a := NewWithID("io.bellui.test.openevent-fwd").(*belluiApp)
	a.startOpenDelivery()

	files := make(chan bellui.URI, 1)
	urls := make(chan *url.URL, 1)
	a.SetOnFileOpened(func(u bellui.URI) { files <- u })
	a.SetOnURLOpened(func(u *url.URL) { urls <- u })

	// Simulate the single-instance transport delivering a forwarded launch.
	a.onForwardedArgs([]string{"--flag", "/tmp/file.md", "myapp://go"})

	select {
	case u := <-files:
		assert.Contains(t, u.String(), "file.md")
	case <-time.After(2 * time.Second):
		t.Fatal("forwarded file not dispatched")
	}
	select {
	case u := <-urls:
		assert.Equal(t, "myapp", u.Scheme)
	case <-time.After(2 * time.Second):
		t.Fatal("forwarded URL not dispatched")
	}
}
