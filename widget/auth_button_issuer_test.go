package widget

import (
	"context"
	"testing"

	"github.com/belluikit/bellui/auth"
	"github.com/stretchr/testify/assert"
)

// The button saves its token through SaveTokenFor with its own Issuer, so two
// buttons for one provider with different issuers keep separate tokens
// (#356); auth decides that a preset issuer keys like no issuer.
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
