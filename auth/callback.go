package auth

import (
	"context"
	"fmt"
	"html"
	"net"
	"net/http"
	"time"
)

// callbackServer is a temporary localhost HTTP server that receives the
// OAuth callback with the authorization code.
type callbackServer struct {
	listener net.Listener
	server   *http.Server
	codeCh   chan callbackResult
}

type callbackResult struct {
	Code  string
	State string
	Err   error
}

func newCallbackServer() (*callbackServer, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}

	cb := &callbackServer{
		listener: listener,
		codeCh:   make(chan callbackResult, 1),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", cb.handleCallback)
	cb.server = &http.Server{Handler: mux}

	go cb.server.Serve(listener)

	return cb, nil
}

// RedirectURI returns the localhost callback URL for the OAuth flow.
func (cb *callbackServer) RedirectURI() string {
	return fmt.Sprintf("http://127.0.0.1:%d/callback", cb.Port())
}

// Port returns the dynamically assigned port.
func (cb *callbackServer) Port() int {
	return cb.listener.Addr().(*net.TCPAddr).Port
}

// WaitForCode blocks until the callback is received or the timeout expires.
func (cb *callbackServer) WaitForCode(timeout time.Duration) (code, state string, err error) {
	select {
	case result := <-cb.codeCh:
		return result.Code, result.State, result.Err
	case <-time.After(timeout):
		return "", "", fmt.Errorf("authentication timed out after %v", timeout)
	}
}

// Close shuts down the callback server.
func (cb *callbackServer) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cb.server.Shutdown(ctx)
}

func (cb *callbackServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	errParam := r.URL.Query().Get("error")
	errDesc := r.URL.Query().Get("error_description")

	if errParam != "" {
		msg := errParam
		if errDesc != "" {
			msg = errParam + ": " + errDesc
		}
		cb.codeCh <- callbackResult{Err: fmt.Errorf("%s", msg)}
		w.Header().Set("Content-Type", "text/html")
		// The error params are attacker-influenced; escape before reflecting.
		fmt.Fprintf(w, `<html><body><h2>Authentication Failed</h2><p>%s</p><p>You can close this window.</p></body></html>`, html.EscapeString(msg))
		return
	}

	if code == "" {
		cb.codeCh <- callbackResult{Err: fmt.Errorf("no authorization code in callback")}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><body><h2>Error</h2><p>No authorization code received.</p></body></html>`)
		return
	}

	cb.codeCh <- callbackResult{Code: code, State: state}
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, `<html><body><h2>Authentication Successful</h2><p>You can close this window and return to the application.</p></body></html>`)
}
