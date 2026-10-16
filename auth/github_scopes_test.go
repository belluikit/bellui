//go:build !wasm

package auth

import (
	"errors"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// githubAuthorizeScopes runs the GitHub flow up to opening the browser and
// returns the scope parameter of the URL it would have opened.
func githubAuthorizeScopes(t *testing.T, cfg OIDCConfig) string {
	t.Helper()
	var opened string
	orig := openBrowser
	openBrowser = func(u string) error {
		opened = u
		return errors.New("stop before waiting for the callback")
	}
	t.Cleanup(func() { openBrowser = orig })

	cfg.Provider = "github"
	cfg.ClientID = "client"
	_, err := Authenticate(cfg)
	require.Error(t, err)
	require.NotEmpty(t, opened, "the flow must reach the browser")

	u, err := url.Parse(opened)
	require.NoError(t, err)
	return u.Query().Get("scope")
}

// With no scopes set, GitHub gets its own defaults, not the OpenID Connect
// ones, which GitHub does not define (#335).
func TestAuthenticate_GitHubDefaultScopes(t *testing.T) {
	assert.Equal(t, "user:email read:user", githubAuthorizeScopes(t, OIDCConfig{}))
}

func TestAuthenticate_GitHubExplicitScopes(t *testing.T) {
	assert.Equal(t, "repo", githubAuthorizeScopes(t, OIDCConfig{Scopes: []string{"repo"}}))
}
