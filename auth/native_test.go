// The native-sheet tests verify tokens against an in-process httptest server
// for discovery and JWKS, which cannot listen inside a browser (jwks_test.go's
// split, #77).
//go:build !wasm

package auth

import (
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeProvider serves discovery and JWKS for a test issuer and signs tokens
// as that issuer.
type fakeProvider struct {
	issuer string
	key    *rsa.PrivateKey
}

func newFakeProvider(t *testing.T) *fakeProvider {
	t.Helper()
	p := &fakeProvider{key: newRSAKey(t)}
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	p.issuer = srv.URL
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(discoveryDoc{Issuer: srv.URL, JwksURI: srv.URL + "/jwks"})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jwksDoc{Keys: []jwk{rsaJWK("k1", &p.key.PublicKey)}})
	})
	return p
}

func (p *fakeProvider) token(t *testing.T, aud, nonce string, extra map[string]any) string {
	t.Helper()
	claims := map[string]any{
		"iss": p.issuer, "aud": aud, "sub": "user-1", "nonce": nonce,
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	for k, v := range extra {
		claims[k] = v
	}
	return signRS256(t, p.key, "k1", claims)
}

// useSheet registers sheet as provider's native sheet for the test.
func useSheet(t *testing.T, provider string, sheet nativeSheet) {
	t.Helper()
	prev, had := nativeSheets[provider]
	nativeSheets[provider] = sheet
	t.Cleanup(func() {
		if had {
			nativeSheets[provider] = prev
		} else {
			delete(nativeSheets, provider)
		}
	})
}

// A native sheet's token is verified like the browser flow's and returned
// with what the sheet gave beside it. The sheet is given the caller's nonce.
func TestAuthenticate_NativeSheet(t *testing.T) {
	p := newFakeProvider(t)
	var gotClient, gotNonce string
	useSheet(t, "native", nativeSheet{run: func(clientID, nonce string, _ time.Duration) (*nativeCredential, error) {
		gotClient, gotNonce = clientID, nonce
		return &nativeCredential{
			IDToken:           p.token(t, clientID, nonce, map[string]any{"email": "a@example.com"}),
			AuthorizationCode: "code-1",
			Name:              "Ada",
		}, nil
	}})

	r, err := Authenticate(OIDCConfig{Provider: "native", Issuer: p.issuer, ClientID: "client", Nonce: "n-1"})
	require.NoError(t, err)
	assert.Equal(t, "client", gotClient)
	assert.Equal(t, "n-1", gotNonce)
	assert.Equal(t, "user-1", r.Sub)
	assert.Equal(t, "a@example.com", r.Email, "the token's email")
	assert.Equal(t, "Ada", r.Name, "the sheet's name fills in what the token lacks")
	assert.Equal(t, "code-1", r.AuthorizationCode)
	assert.NotEmpty(t, r.IDToken)
}

// Apple's sheet takes the SHA-256 of the nonce and puts that hash in the
// token, so the hash is what is sent and checked. A token carrying the raw
// nonce is refused.
func TestAuthenticate_NativeSheetHashedNonce(t *testing.T) {
	p := newFakeProvider(t)
	inToken := ""
	useSheet(t, "native", nativeSheet{hashNonce: true, run: func(clientID, nonce string, _ time.Duration) (*nativeCredential, error) {
		// shasum -a 256 of "n-1".
		assert.Equal(t, "51aeea8ffa05d2620d35c87463465885e77df13171d5708746df1a9f36d47f35", nonce)
		return &nativeCredential{IDToken: p.token(t, clientID, inToken, nil)}, nil
	}})

	cfg := OIDCConfig{Provider: "native", Issuer: p.issuer, ClientID: "client", Nonce: "n-1"}
	inToken = sha256Hex("n-1")
	_, err := Authenticate(cfg)
	require.NoError(t, err)

	inToken = "n-1"
	_, err = Authenticate(cfg)
	assert.ErrorContains(t, err, "nonce")
}

// Without a ClientID the sheet's default audience is used: an Apple token's
// is the app's bundle ID.
func TestAuthenticate_NativeSheetDefaultClientID(t *testing.T) {
	p := newFakeProvider(t)
	useSheet(t, "native", nativeSheet{
		defaultClientID: func() string { return "com.example.app" },
		run: func(clientID, nonce string, _ time.Duration) (*nativeCredential, error) {
			return &nativeCredential{IDToken: p.token(t, clientID, nonce, nil)}, nil
		},
	})
	r, err := Authenticate(OIDCConfig{Provider: "native", Issuer: p.issuer})
	require.NoError(t, err)
	assert.Equal(t, "user-1", r.Sub)
}

// A token the provider's keys did not sign is refused.
func TestAuthenticate_NativeSheetRejectsForgedToken(t *testing.T) {
	p := newFakeProvider(t)
	other := newRSAKey(t)
	useSheet(t, "native", nativeSheet{run: func(clientID, nonce string, _ time.Duration) (*nativeCredential, error) {
		return &nativeCredential{IDToken: signRS256(t, other, "k1", map[string]any{
			"iss": p.issuer, "aud": clientID, "sub": "user-1", "nonce": nonce,
			"exp": time.Now().Add(time.Hour).Unix(),
		})}, nil
	}})
	_, err := Authenticate(OIDCConfig{Provider: "native", Issuer: p.issuer, ClientID: "client"})
	assert.ErrorContains(t, err, "signature")
}

// A closed sheet is ErrCancelled, told apart with errors.Is from a failure.
func TestAuthenticate_NativeSheetCancelled(t *testing.T) {
	p := newFakeProvider(t)
	useSheet(t, "native", nativeSheet{run: func(string, string, time.Duration) (*nativeCredential, error) {
		return nil, ErrCancelled
	}})
	_, err := Authenticate(OIDCConfig{Provider: "native", Issuer: p.issuer, ClientID: "client"})
	assert.ErrorIs(t, err, ErrCancelled)

	useSheet(t, "native", nativeSheet{run: func(string, string, time.Duration) (*nativeCredential, error) {
		return nil, errors.New("no account")
	}})
	_, err = Authenticate(OIDCConfig{Provider: "native", Issuer: p.issuer, ClientID: "client"})
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrCancelled)
}

// fakeRedirect stands in for a phone's redirect flow.
type fakeRedirect struct{ err error }

func (fakeRedirect) RedirectURI() string { return "com.example:/cb" }

func (f fakeRedirect) Authorize(string, time.Duration) (string, string, error) {
	return "", "", f.err
}
func (fakeRedirect) Close() {}

// A browser session the user closes is ErrCancelled too, unwrapped.
func TestAuthenticate_RedirectFlowCancelled(t *testing.T) {
	p := newFakeProvider(t)
	prev := newRedirectFlow
	newRedirectFlow = func(OIDCConfig) (redirectFlow, error) { return fakeRedirect{err: ErrCancelled}, nil }
	t.Cleanup(func() { newRedirectFlow = prev })

	_, err := Authenticate(OIDCConfig{Provider: "custom", Issuer: p.issuer, ClientID: "client"})
	assert.ErrorIs(t, err, ErrCancelled)
}

// Where the platform has no loopback flow, GitHub's OAuth flow, which needs
// one, says so instead of trying to open a browser.
func TestAuthenticate_GitHubNeedsLoopback(t *testing.T) {
	prev := loopbackFlows
	loopbackFlows = false
	t.Cleanup(func() { loopbackFlows = prev })
	_, err := Authenticate(OIDCConfig{Provider: "github", ClientID: "client"})
	assert.ErrorContains(t, err, "no sign-in flow on this platform")
}

func TestGoogleIOSRedirect(t *testing.T) {
	scheme, uri, err := googleIOSRedirect("123-abc.apps.googleusercontent.com")
	require.NoError(t, err)
	assert.Equal(t, "com.googleusercontent.apps.123-abc", scheme)
	assert.Equal(t, "com.googleusercontent.apps.123-abc:/oauth2redirect", uri)

	_, _, err = googleIOSRedirect("com.googleusercontent.apps.123-abc")
	assert.Error(t, err, "the reversed scheme is not a client ID")
}
