//go:build ios

package auth

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework AuthenticationServices -framework UIKit -framework Foundation
#include <stdint.h>
#include <stdlib.h>

void belluiAuthAppleSignIn(uint64_t token, const char *nonce);
void belluiAuthWebSession(uint64_t token, const char *url, const char *scheme);
void belluiAuthWebCancel(uint64_t token);
char *belluiAuthBundleID(void);
*/
import "C"

import (
	"errors"
	"fmt"
	"net/url"
	"time"
	"unsafe"
)

func init() {
	nativeSheets["apple"] = nativeSheet{
		run:             appleSheet,
		hashNonce:       true,
		defaultClientID: bundleID,
	}
	newRedirectFlow = newWebAuthFlow
	loopbackFlows = false
}

//export belluiAuthAppleDone
func belluiAuthAppleDone(token C.uint64_t, status C.int, idToken, code, email, name, msg *C.char) {
	complete(uint64(token), nativeDone{
		status:   int(status),
		idToken:  C.GoString(idToken),
		authCode: C.GoString(code),
		email:    C.GoString(email),
		name:     C.GoString(name),
		msg:      C.GoString(msg),
	})
}

//export belluiAuthWebDone
func belluiAuthWebDone(token C.uint64_t, status C.int, callback, msg *C.char) {
	complete(uint64(token), nativeDone{status: int(status), url: C.GoString(callback), msg: C.GoString(msg)})
}

func bundleID() string {
	c := C.belluiAuthBundleID()
	if c == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(c))
	return C.GoString(c)
}

// appleSheet shows Sign in with Apple. The sheet takes nonce as given, so the
// caller hands it the hash it expects in the token.
func appleSheet(_, nonce string, timeout time.Duration) (*nativeCredential, error) {
	token, ch := register()
	cNonce := C.CString(nonce)
	defer C.free(unsafe.Pointer(cNonce))
	C.belluiAuthAppleSignIn(C.uint64_t(token), cNonce)

	select {
	case d := <-ch:
		switch d.status {
		case nativeOK:
			return &nativeCredential{IDToken: d.idToken, AuthorizationCode: d.authCode, Email: d.email, Name: d.name}, nil
		case nativeCancelled:
			return nil, ErrCancelled
		default:
			return nil, fmt.Errorf("sign in with Apple failed: %s", d.msg)
		}
	case <-time.After(timeout):
		forget(token)
		return nil, fmt.Errorf("authentication timed out after %v", timeout)
	}
}

// webAuthFlow runs the authorization in ASWebAuthenticationSession, which
// catches the redirect to the client's own URL scheme.
type webAuthFlow struct {
	scheme, uri string
}

func newWebAuthFlow(cfg OIDCConfig) (redirectFlow, error) {
	if cfg.Provider != "google" {
		return nil, fmt.Errorf("provider %q has no sign-in flow on iOS: only \"apple\" and \"google\" do", cfg.Provider)
	}
	scheme, uri, err := googleIOSRedirect(cfg.ClientID)
	if err != nil {
		return nil, err
	}
	return webAuthFlow{scheme: scheme, uri: uri}, nil
}

func (f webAuthFlow) RedirectURI() string { return f.uri }

func (f webAuthFlow) Authorize(authURL string, timeout time.Duration) (code, state string, err error) {
	token, ch := register()
	cURL, cScheme := C.CString(authURL), C.CString(f.scheme)
	defer C.free(unsafe.Pointer(cURL))
	defer C.free(unsafe.Pointer(cScheme))
	C.belluiAuthWebSession(C.uint64_t(token), cURL, cScheme)

	var d nativeDone
	select {
	case d = <-ch:
	case <-time.After(timeout):
		forget(token)
		C.belluiAuthWebCancel(C.uint64_t(token))
		return "", "", fmt.Errorf("authentication timed out after %v", timeout)
	}
	switch d.status {
	case nativeOK:
	case nativeCancelled:
		return "", "", ErrCancelled
	default:
		return "", "", fmt.Errorf("browser session failed: %s", d.msg)
	}

	u, err := url.Parse(d.url)
	if err != nil {
		return "", "", fmt.Errorf("redirect: %w", err)
	}
	q := u.Query()
	if e := q.Get("error"); e != "" {
		return "", "", errors.New("provider returned " + e + ": " + q.Get("error_description"))
	}
	return q.Get("code"), q.Get("state"), nil
}

func (f webAuthFlow) Close() {}
