//go:build darwin && !wasm

package webview

import "sync"

// Callbacks arrive from ObjC carrying a token rather than a Go pointer, which
// cgo would not allow us to hand out. The registry resolves it back.
//
// Everything is delivered on the main thread — the same thread the bellui main
// goroutine sits on — but the registry is still mutex-guarded because Attach
// and Destroy may race a late callback.
//
// It is shared by the two darwin backends: macOS (webview_darwin.go) and iOS
// (webview_ios.go) differ in how their view is hosted, not in how a callback
// finds its way home.
var (
	registryMu   sync.Mutex
	registry     = map[uint64]*WebView{}
	registryNext uint64
)

// registerView assigns w a token and records it, so ObjC callbacks can resolve
// it. It is called once per view, from init.
func registerView(w *WebView) uint64 {
	registryMu.Lock()
	defer registryMu.Unlock()

	registryNext++
	registry[registryNext] = w
	return registryNext
}

// forgetView drops a token, so a callback still in flight when the view was
// destroyed simply finds nothing rather than reaching a released view.
func forgetView(token uint64) {
	registryMu.Lock()
	defer registryMu.Unlock()

	delete(registry, token)
}

func lookupView(token uint64) *WebView {
	registryMu.Lock()
	defer registryMu.Unlock()

	return registry[token]
}
