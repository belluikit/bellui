package editor

import (
	"bytes"
	"log"
	"strings"
	"testing"

	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// captureLog redirects the standard logger to a buffer for the duration of the
// test and returns it.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prevOut, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	})
	return &buf
}

// TestEditor_WarnsWhenNestedInScroll verifies the editor logs a one-time warning
// when a host wraps it in an outer container.Scroll (pre-rebrand tracker #111).
func TestEditor_WarnsWhenNestedInScroll(t *testing.T) {
	test.NewTempApp(t)
	buf := captureLog(t)

	e := NewEditor()
	scroll := container.NewScroll(e)
	w := test.NewWindow(scroll)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(300, 200))

	out := buf.String()
	if !strings.Contains(out, "x/editor: Editor is hosted") {
		t.Errorf("expected the x/editor nested-scroll warning, got %q", out)
	}
	if !strings.Contains(out, "container.Scroll") {
		t.Errorf("warning should name container.Scroll, got %q", out)
	}
}

// TestEditor_WarnsOnlyOnce verifies repeated resizes don't re-log the warning.
func TestEditor_WarnsOnlyOnce(t *testing.T) {
	test.NewTempApp(t)
	buf := captureLog(t)

	e := NewEditor()
	scroll := container.NewScroll(e)
	w := test.NewWindow(scroll)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(300, 200))
	w.Resize(geo.NewSize(320, 240))
	e.Resize(geo.NewSize(400, 400))

	if n := strings.Count(buf.String(), "x/editor: Editor is hosted"); n != 1 {
		t.Errorf("warning should be logged exactly once, logged %d times", n)
	}
}

// TestEditor_NoWarnWhenOwnViewport verifies the editor is silent when it owns its
// viewport (placed directly in the window, not wrapped in a scroll).
func TestEditor_NoWarnWhenOwnViewport(t *testing.T) {
	test.NewTempApp(t)
	buf := captureLog(t)

	e := NewEditor()
	e.SetText("abc\ndef\nghi")
	w := test.NewWindow(e)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(300, 200))

	if strings.Contains(buf.String(), "x/editor:") {
		t.Errorf("editor in its own viewport must not warn, got %q", buf.String())
	}
}
