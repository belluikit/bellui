package auth

import (
	"fmt"
	"time"
)

// redirectFlow shows the user an authorization URL and waits for the redirect
// that brings the code back. On a desktop that is the system browser and a
// loopback listener; on iOS, ASWebAuthenticationSession.
type redirectFlow interface {
	// RedirectURI is the redirect_uri the authorization request names.
	RedirectURI() string
	// Authorize shows authURL and returns the code and state the redirect
	// carries, or ErrCancelled when the user closes the flow.
	Authorize(authURL string, timeout time.Duration) (code, state string, err error)
	Close()
}

// newRedirectFlow makes the redirect flow for cfg. A phone replaces it, since
// it has no browser to open and no loopback redirect its providers accept.
var newRedirectFlow = newLoopbackFlow

// loopbackFlows reports whether the platform runs flows over the loopback
// listener, which the GitHub flow needs.
var loopbackFlows = true

// loopbackFlow opens the system browser and catches the redirect on a
// temporary localhost server.
type loopbackFlow struct{ cb *callbackServer }

func newLoopbackFlow(OIDCConfig) (redirectFlow, error) {
	cb, err := newCallbackServer()
	if err != nil {
		return nil, fmt.Errorf("callback server failed: %w", err)
	}
	return loopbackFlow{cb}, nil
}

func (f loopbackFlow) RedirectURI() string { return f.cb.RedirectURI() }

func (f loopbackFlow) Authorize(authURL string, timeout time.Duration) (code, state string, err error) {
	if err := openBrowser(authURL); err != nil {
		return "", "", fmt.Errorf("failed to open browser: %w", err)
	}
	code, state, err = f.cb.WaitForCode(timeout)
	if err != nil {
		return "", "", fmt.Errorf("callback failed: %w", err)
	}
	return code, state, nil
}

func (f loopbackFlow) Close() { f.cb.Close() }
