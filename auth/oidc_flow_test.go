// The flow needs an in-process httptest server for discovery, which cannot
// listen inside a browser, so the wasm leg would fail before the flow reaches
// the browser (jwks_test.go's split, #77).
//go:build !wasm

package auth

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A caller's nonce is the one the flow sends, so the ID token the provider
// returns carries it and a server that issued it can check it
// (belluikit/player#357). Without one, the flow still sends a random nonce.
func TestAuthenticate_CallerNonce(t *testing.T) {
	srv := httptest.NewServer(nil)
	defer srv.Close()
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"issuer":%q,"authorization_endpoint":%q,"token_endpoint":%q,"jwks_uri":%q}`,
			srv.URL, srv.URL+"/auth", srv.URL+"/token", srv.URL+"/jwks")
	})

	authorizeNonce := func(nonce string) string {
		var opened string
		orig := openBrowser
		openBrowser = func(u string) error {
			opened = u
			return errors.New("stop before waiting for the callback")
		}
		defer func() { openBrowser = orig }()
		_, err := Authenticate(OIDCConfig{Provider: "custom", Issuer: srv.URL, ClientID: "client", Nonce: nonce})
		require.Error(t, err)
		require.NotEmpty(t, opened, "the flow must reach the browser")
		u, err := url.Parse(opened)
		require.NoError(t, err)
		return u.Query().Get("nonce")
	}
	assert.Equal(t, "server-issued-123", authorizeNonce("server-issued-123"))
	random := authorizeNonce("")
	assert.NotEmpty(t, random)
	assert.NotEqual(t, "server-issued-123", random)
}
