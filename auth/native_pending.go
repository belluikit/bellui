//go:build ios || android

package auth

import "sync"

// The status a native completion reports, matching signin_ios.m and
// GoNativeActivity.java.
const (
	nativeOK        = 0
	nativeCancelled = 1
	nativeFailed    = 2
	// nativeMissing: the app does not ship the libraries the flow needs.
	nativeMissing = 3
)

// Completions arrive on a platform thread carrying the token their request
// was started with; each waiting call holds a channel for its token.
var (
	pendingMu   sync.Mutex
	pending     = map[uint64]chan nativeDone{}
	pendingNext uint64
)

type nativeDone struct {
	status            int
	idToken, authCode string
	email, name       string
	url, msg          string
}

func register() (uint64, chan nativeDone) {
	pendingMu.Lock()
	defer pendingMu.Unlock()
	pendingNext++
	ch := make(chan nativeDone, 1)
	pending[pendingNext] = ch
	return pendingNext, ch
}

// complete hands d to the call waiting on token. A call that already timed
// out has forgotten its token, and d is dropped.
func complete(token uint64, d nativeDone) {
	pendingMu.Lock()
	ch := pending[token]
	delete(pending, token)
	pendingMu.Unlock()
	if ch != nil {
		ch <- d
	}
}

func forget(token uint64) {
	pendingMu.Lock()
	delete(pending, token)
	pendingMu.Unlock()
}
