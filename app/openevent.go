package app

import (
	"net/url"
	"strings"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/storage"
)

// openPayload is one buffered "open with" event: exactly one of uri / link is set.
type openPayload struct {
	uri  bellui.URI
	link *url.URL
}

// SetOnFileOpened implements bellui.OpenHandlerApp.
func (a *belluiApp) SetOnFileOpened(fn func(bellui.URI)) {
	a.openMu.Lock()
	a.onFileOpened = fn
	a.flushOpensLocked()
	a.openMu.Unlock()
}

// SetOnURLOpened implements bellui.OpenHandlerApp.
func (a *belluiApp) SetOnURLOpened(fn func(*url.URL)) {
	a.openMu.Lock()
	a.onURLOpened = fn
	a.flushOpensLocked()
	a.openMu.Unlock()
}

// openFile / openURL are the internal delivery entry points used by the
// platform code (launch argv parsing, single-instance forwarding, macOS Apple
// Events, wasm URL). They either deliver on the main goroutine (when the app is
// running and a handler is set) or buffer until both are true.
func (a *belluiApp) openFile(u bellui.URI) {
	if u == nil {
		return
	}
	a.deliverOpen(openPayload{uri: u})
}

func (a *belluiApp) openURL(u *url.URL) {
	if u == nil {
		return
	}
	a.deliverOpen(openPayload{link: u})
}

func (a *belluiApp) deliverOpen(p openPayload) {
	a.openMu.Lock()
	defer a.openMu.Unlock()

	if !a.running || !a.hasHandlerForLocked(p) {
		a.pendingOpens = append(a.pendingOpens, p)
		return
	}
	a.dispatchLocked(p)
}

// dispatchOpenArgs classifies each launch/forwarded argument and delivers the
// ones that name a file or URL. Used by the desktop argv path and by the
// single-instance forwarding callback.
func (a *belluiApp) dispatchOpenArgs(args []string) {
	for _, arg := range args {
		p, ok := classifyOpenArg(arg)
		if !ok {
			continue
		}
		if p.link != nil {
			a.openURL(p.link)
		} else {
			a.openFile(p.uri)
		}
	}
}

// startOpenDelivery marks the app as running and delivers anything buffered
// before Run. It is called from Run just before the driver event loop starts.
func (a *belluiApp) startOpenDelivery() {
	a.openMu.Lock()
	a.running = true
	a.flushOpensLocked()
	a.openMu.Unlock()
}

// flushOpensLocked delivers buffered payloads whose handler is now set. It is a
// no-op until the app is running so pre-Run registrations only buffer. Callers
// must hold openMu.
func (a *belluiApp) flushOpensLocked() {
	if !a.running || len(a.pendingOpens) == 0 {
		return
	}
	kept := a.pendingOpens[:0]
	for _, p := range a.pendingOpens {
		if a.hasHandlerForLocked(p) {
			a.dispatchLocked(p)
		} else {
			kept = append(kept, p)
		}
	}
	a.pendingOpens = kept
}

func (a *belluiApp) hasHandlerForLocked(p openPayload) bool {
	if p.link != nil {
		return a.onURLOpened != nil
	}
	return a.onFileOpened != nil
}

// dispatchLocked queues one payload onto the main goroutine. Callers hold
// openMu; the callback runs later so it never re-enters under the lock.
func (a *belluiApp) dispatchLocked(p openPayload) {
	if p.link != nil {
		cb, link := a.onURLOpened, p.link
		queueOnMain(func() { cb(link) })
		return
	}
	cb, uri := a.onFileOpened, p.uri
	queueOnMain(func() { cb(uri) })
}

// queueOnMain schedules fn on the main goroutine via the running driver. It runs
// on a fresh goroutine so that, when called from Run before the loop is pumping,
// bellui.Do enqueues rather than executing synchronously (see loop.go) - the
// launch payload is thus delivered once the app is actually running.
func queueOnMain(fn func()) {
	go bellui.Do(fn)
}

// classifyOpenArg turns one command-line token into an open payload, or reports
// ok=false for flags and empty strings. A token with a real URL scheme (http,
// mailto, a custom app scheme) becomes a URL; a file:// URI, a bare path or an
// absolute path becomes a file. Windows drive-letter paths (C:\...) parse as a
// single-letter "scheme" and are treated as files, not URLs.
func classifyOpenArg(arg string) (openPayload, bool) {
	if arg == "" || strings.HasPrefix(arg, "-") {
		return openPayload{}, false
	}

	if u, err := url.Parse(arg); err == nil && u.Scheme != "" && !isWindowsDrivePath(arg) {
		if u.Scheme == "file" {
			if uri, err := storage.ParseURI(arg); err == nil {
				return openPayload{uri: uri}, true
			}
			return openPayload{}, false
		}
		return openPayload{link: u}, true
	}

	return openPayload{uri: storage.NewFileURI(arg)}, true
}

// isWindowsDrivePath reports whether arg looks like C:\... or C:/... , which
// url.Parse otherwise reads as scheme "c".
func isWindowsDrivePath(arg string) bool {
	if len(arg) < 2 || arg[1] != ':' {
		return false
	}
	c := arg[0]
	if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
		return false
	}
	return len(arg) == 2 || arg[2] == '\\' || arg[2] == '/'
}
