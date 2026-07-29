//go:build wasm

package webview

import (
	"encoding/json"
	"strconv"
	"strings"
	"syscall/js"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/geo"
)

// These tests need a real DOM, so they run under a browser rather than node:
//
//	go install github.com/agnivade/wasmbrowsertest@latest
//	GOOS=js GOARCH=wasm go test -exec="$(go env GOPATH)/bin/wasmbrowsertest" ./webview/
//
// Content is loaded with LoadHTML, whose srcdoc document inherits the embedder's
// origin — so the same-origin paths (bridge injection, Eval, the message
// round-trip) are exercised for real, not simulated.

// newDOMView attaches a view over a stand-in canvas, since the backend
// positions itself against the canvas bellui's driver would have created.
func newDOMView(t *testing.T, enableBridge bool) *WebView {
	t.Helper()

	doc := js.Global().Get("document")
	canvas := doc.Call("createElement", "canvas")
	doc.Get("body").Call("appendChild", canvas)

	w := New("")
	if enableBridge {
		w.EnableBridge(TrustAllOrigins())
	}
	require.NoError(t, w.attachRaw(nil))

	t.Cleanup(func() {
		w.Destroy()
		if p := canvas.Get("parentNode"); p.Truthy() {
			p.Call("removeChild", canvas)
		}
	})
	return w
}

// loadAndWait loads literal content and waits for the frame's load event, which
// is also when the bridge shim is installed.
func loadAndWait(t *testing.T, w *WebView, html string) {
	t.Helper()

	loaded := make(chan struct{}, 1)
	prev := w.OnLoadChanged
	w.OnLoadChanged = func(e LoadEvent) {
		if prev != nil {
			prev(e)
		}
		if e.State == LoadStateComplete {
			select {
			case loaded <- struct{}{}:
			default:
			}
		}
	}
	w.LoadHTML(html, "")

	select {
	case <-loaded:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the frame to load")
	}
}

func TestWasm_AttachBuildsAClippedFrame(t *testing.T) {
	w := newDOMView(t, false)

	assert.True(t, w.attachedRaw(), "a view with a frame reports attached")
	assert.Equal(t, "IFRAME", w.frame.Get("tagName").String())
	// The wrapper is what gives the host its native clip; without overflow
	// hidden the content would spill out of an enclosing Scroll.
	assert.Equal(t, "hidden", w.clip.Get("style").Call("getPropertyValue", "overflow").String())
	assert.True(t, w.frame.Get("parentNode").Equal(w.clip))
}

func TestWasm_DestroyRemovesTheFrameAndIsIdempotent(t *testing.T) {
	w := newDOMView(t, true)
	clip := w.clip

	w.Destroy()
	assert.False(t, w.attachedRaw(), "a destroyed view is not attached")
	assert.False(t, clip.Get("parentNode").Truthy(), "the wrapper is out of the document")

	w.Destroy() // must be safe without a live view
}

func TestWasm_SetGeometryPlacesAndClips(t *testing.T) {
	w := newDOMView(t, false)

	// The canvas is the origin, so measure against where it actually sits
	// rather than assuming the viewport corner.
	origin := w.canvas.Call("getBoundingClientRect")
	ox, oy := origin.Get("left").Float(), origin.Get("top").Float()
	dpr := js.Global().Get("devicePixelRatio").Float()

	rect := geo.Rectangle{Pos: geo.NewPos(10, 20), Size: geo.NewSize(200, 100)}
	clip := geo.Rectangle{Pos: geo.NewPos(0, 40), Size: geo.NewSize(150, 50)}
	w.SetGeometry(rect, clip, float32(dpr)) // scale == dpr, so 1 point == 1 CSS px

	style := func(el js.Value, prop string) float64 {
		v := el.Get("style").Call("getPropertyValue", prop).String()
		f, err := strconv.ParseFloat(strings.TrimSuffix(v, "px"), 64)
		require.NoError(t, err, "parsing %s=%q", prop, v)
		return f
	}

	assert.InDelta(t, ox+0, style(w.clip, "left"), 0.01)
	assert.InDelta(t, oy+40, style(w.clip, "top"), 0.01)
	assert.InDelta(t, 150, style(w.clip, "width"), 0.01)
	assert.InDelta(t, 50, style(w.clip, "height"), 0.01)

	// The frame is placed relative to the wrapper, so the part outside the clip
	// is masked rather than drawn.
	assert.InDelta(t, 10, style(w.frame, "left"), 0.01)
	assert.InDelta(t, 20-40, style(w.frame, "top"), 0.01)
	assert.InDelta(t, 200, style(w.frame, "width"), 0.01)
	assert.InDelta(t, 100, style(w.frame, "height"), 0.01)
}

func TestWasm_SetVisibleTogglesDisplay(t *testing.T) {
	w := newDOMView(t, false)

	w.SetVisible(false)
	assert.Equal(t, "none", w.clip.Get("style").Call("getPropertyValue", "display").String())
	w.SetVisible(true)
	assert.Equal(t, "block", w.clip.Get("style").Call("getPropertyValue", "display").String())
}

// TestWasm_BridgeOffInstallsNothing is the opt-in invariant: with the bridge
// disabled the page must have no window.bellui at all, not an ignored one.
func TestWasm_BridgeOffInstallsNothing(t *testing.T) {
	w := newDOMView(t, false)
	require.Empty(t, w.shim, "no shim is prepared when the bridge is off")

	loadAndWait(t, w, "<html><body>hi</body></html>")

	res, err := w.evalIn("typeof window.bellui")
	require.NoError(t, err)
	assert.Equal(t, "undefined", res.String(), "the page has no bridge object")
}

func TestWasm_EvalAndEvalResultReachSameOriginContent(t *testing.T) {
	w := newDOMView(t, false)
	loadAndWait(t, w, "<html><body><div id='x'>before</div></body></html>")

	w.Eval("document.getElementById('x').textContent = 'after';")

	got := make(chan json.RawMessage, 1)
	w.EvalResult("document.getElementById('x').textContent", func(v json.RawMessage, err error) {
		require.NoError(t, err)
		got <- v
	})
	select {
	case v := <-got:
		assert.JSONEq(t, `"after"`, string(v))
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the eval result")
	}
}

func TestWasm_EvalResultReportsAThrow(t *testing.T) {
	w := newDOMView(t, false)
	loadAndWait(t, w, "<html><body></body></html>")

	done := make(chan error, 1)
	w.EvalResult("throw new Error('boom')", func(_ json.RawMessage, err error) {
		done <- err
	})
	select {
	case err := <-done:
		require.Error(t, err, "a throwing script reports an error, it does not hang")
		assert.Contains(t, err.Error(), "boom")
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the eval error")
	}
}

// TestWasm_PageToBelluiMessage exercises the whole inbound path for real: the
// shim is injected into the srcdoc document, the page posts, the parent's
// message listener attributes the origin, and the origin policy admits it.
func TestWasm_PageToBelluiMessage(t *testing.T) {
	w := newDOMView(t, true)
	require.NotEmpty(t, w.shim, "the bridge prepares a shim when enabled")

	msgs := make(chan Message, 1)
	w.OnMessage = func(m Message) { msgs <- m }

	loadAndWait(t, w, `<html><body><script>
		window.bellui.post({hello: "world"});
	</script></body></html>`)

	select {
	case m := <-msgs:
		var payload struct {
			Hello string `json:"hello"`
		}
		require.NoError(t, m.Decode(&payload))
		assert.Equal(t, "world", payload.Hello)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the page message")
	}
}

// TestWasm_ForeignFrameCannotImpersonateThePage locks the check that makes the
// origin gate meaningful here: window receives every frame's messages, so a
// message from anything but our own frame must be ignored outright.
func TestWasm_ForeignFrameCannotImpersonateThePage(t *testing.T) {
	w := newDOMView(t, true)

	got := make(chan Message, 1)
	w.OnMessage = func(m Message) { got <- m }

	loadAndWait(t, w, "<html><body></body></html>")

	// Post from the top-level window rather than the view's frame. It carries a
	// trusted origin and a well-formed envelope, so only the source check can
	// reject it.
	js.Global().Call("postMessage", `{"v":1,"kind":"post","payload":{"spoofed":true}}`, "*")

	select {
	case m := <-got:
		t.Fatalf("a message from another window was accepted: %s", m.Payload)
	case <-time.After(500 * time.Millisecond):
	}
}

func TestWasm_FocusBridge(t *testing.T) {
	w := newDOMView(t, false)
	loadAndWait(t, w, "<html><body></body></html>")

	assert.False(t, w.HasFocus(), "a fresh frame does not hold the focus")

	if !w.TakeFocus() {
		t.Skip("the browser did not give the frame focus (window not focused)")
	}
	assert.True(t, w.HasFocus())

	assert.True(t, w.ResignFocus(), "the frame gives the focus back")
	assert.False(t, w.HasFocus())
}

// TestWasm_CanGoBackIsHonestlyFalse records the documented platform limit: an
// iframe shares the top-level session history, so this cannot be answered.
func TestWasm_CanGoBackIsHonestlyFalse(t *testing.T) {
	w := newDOMView(t, false)
	loadAndWait(t, w, "<html><body></body></html>")

	assert.False(t, w.CanGoBack())
	assert.False(t, w.CanGoForward())
}

func TestWasm_OnNavigateVetoesAProgrammaticLoad(t *testing.T) {
	w := newDOMView(t, false)

	w.OnNavigate = func(NavigationRequest) bool { return false }

	failed := make(chan LoadEvent, 1)
	w.OnLoadChanged = func(e LoadEvent) {
		if e.State == LoadStateFailed {
			failed <- e
		}
	}
	w.LoadURL("https://example.com/blocked")

	select {
	case e := <-failed:
		assert.ErrorIs(t, e.Err, ErrNavigationCancelled)
		assert.Empty(t, w.frame.Get("src").String(), "the frame never navigated")
	case <-time.After(2 * time.Second):
		t.Fatal("a refused navigation must report a cancelled load")
	}
}

func TestEscapeAttr(t *testing.T) {
	assert.Equal(t, "https://example.com/a&amp;b", escapeAttr("https://example.com/a&b"))
	assert.Equal(t,
		"&quot;&gt;&lt;script&gt;alert(1)&lt;/script&gt;",
		escapeAttr(`"><script>alert(1)</script>`),
		"a base URL cannot close its attribute and inject markup")
}
