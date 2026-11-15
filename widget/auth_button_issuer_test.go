package widget

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/belluikit/bellui/auth"
	"github.com/belluikit/bellui/test"
	"github.com/stretchr/testify/assert"
)

// A tap on a drawn AuthButton starts the sign-in and returns. It used to show
// its press while holding the lock its press shape takes, so the first tap
// hung the main goroutine.
func TestAuthButton_TapDrawnButtonReturns(t *testing.T) {
	test.NewTempApp(t)
	var started atomic.Int32
	prevAuth := authenticate
	t.Cleanup(func() { authenticate = prevAuth })
	authenticate = func(auth.OIDCConfig) (*auth.OIDCResult, error) {
		started.Add(1)
		return nil, context.Canceled
	}
	b := NewAuthButton("google", "id", nil)
	test.NewTempWindow(t, b)

	done := make(chan struct{})
	go func() {
		test.Tap(b)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Tapped did not return: the press and the tap took one lock twice")
	}
	assert.Eventually(t, func() bool { return started.Load() == 1 }, time.Second, 5*time.Millisecond)
}

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
