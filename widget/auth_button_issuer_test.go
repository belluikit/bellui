package widget

import (
	"context"
	"testing"

	"github.com/belluikit/bellui/auth"
	"github.com/stretchr/testify/assert"
)

// The button keys its saved token by the app's own issuer, so two buttons for
// one provider with different issuers keep separate tokens (#356). A preset
// issuer is not part of the key, which keeps Google, Microsoft and Apple
// tokens where auth.LoadToken has always found them.
func TestAuthButton_TokenIssuer(t *testing.T) {
	for _, tc := range []struct {
		name     string
		provider string
		issuer   string
		want     string
	}{
		{"google, preset issuer", "google", "", ""},
		{"google, the preset's own issuer", "google", "https://accounts.google.com", ""},
		{"keycloak realm", "keycloak", "https://sso.example.com/realms/a", "https://sso.example.com/realms/a"},
		{"okta tenant", "okta", "https://example.okta.com", "https://example.okta.com"},
		{"custom", "custom", "https://id.example.com", "https://id.example.com"},
		{"github, no issuer", "github", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := &AuthButton{Provider: tc.provider, Issuer: tc.issuer}
			assert.Equal(t, tc.want, b.tokenIssuer())
		})
	}
}

// The button saves through SaveTokenFor with that issuer.
func TestAuthButton_SavesTokenUnderItsIssuer(t *testing.T) {
	prevAuth, prevSave := authenticate, saveToken
	t.Cleanup(func() { authenticate, saveToken = prevAuth, prevSave })
	authenticate = func(auth.OIDCConfig) (*auth.OIDCResult, error) {
		return &auth.OIDCResult{RefreshToken: "refresh", Email: "a@example.com"}, nil
	}
	var saved []string
	saveToken = func(provider, issuer, refresh, email string) error {
		saved = append(saved, provider+" "+issuer+" "+refresh)
		return nil
	}

	realm := &AuthButton{Provider: "keycloak", Issuer: "https://sso.example.com/realms/a"}
	realm.authenticate(context.Background())
	google := &AuthButton{Provider: "google"}
	google.authenticate(context.Background())

	assert.Equal(t, []string{
		"keycloak https://sso.example.com/realms/a refresh",
		"google  refresh",
	}, saved)
}
