package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// nativeSheet is a platform's own sign-in sheet for one provider: Sign in
// with Apple on iOS, Credential Manager on Android. It hands back a signed ID
// token, which authenticateNative verifies as the browser flow verifies its
// own.
type nativeSheet struct {
	// run shows the sheet and returns what it gave, or ErrCancelled. nonce is
	// the value the request carries, so the token carries it too.
	run func(clientID, nonce string, timeout time.Duration) (*nativeCredential, error)

	// hashNonce sends the SHA-256 of the caller's nonce, hex-encoded, instead
	// of the nonce itself. Apple's sheet takes the nonce that way and puts the
	// same hash in the token.
	hashNonce bool

	// defaultClientID gives the token's audience when the caller sets no
	// ClientID: an Apple token's audience is the app's bundle ID.
	defaultClientID func() string
}

// nativeCredential is what a sheet returned.
type nativeCredential struct {
	IDToken           string
	AuthorizationCode string
	// Email and Name fill in the result when the token lacks them: Apple
	// gives the name only on the first sign-in, and only from the sheet.
	Email string
	Name  string
}

// nativeSheets holds the platform's sheets by provider key. It is empty on a
// desktop; the iOS and Android builds fill it.
var nativeSheets = map[string]nativeSheet{}

// authenticateNative signs in through a native sheet and verifies the ID token
// it returns: the signature against the provider's published keys, then the
// issuer, the audience, the expiry and the nonce.
func authenticateNative(cfg OIDCConfig, sheet nativeSheet, nonce string) (*OIDCResult, error) {
	clientID := cfg.ClientID
	if clientID == "" && sheet.defaultClientID != nil {
		clientID = sheet.defaultClientID()
	}
	if clientID == "" {
		return nil, fmt.Errorf("auth: ClientID is required for provider %q", cfg.Provider)
	}

	// Discovery first, so a network failure shows before the sheet does.
	disc, err := discover(cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("auth: discovery failed: %w", err)
	}
	if disc.JwksURI == "" {
		return nil, fmt.Errorf("auth: discovery document is missing jwks_uri; cannot verify ID token signature")
	}

	sent := nonce
	if sheet.hashNonce {
		sent = sha256Hex(nonce)
	}
	cred, err := sheet.run(clientID, sent, cfg.Timeout)
	if err != nil {
		if errors.Is(err, ErrCancelled) {
			return nil, err
		}
		return nil, fmt.Errorf("auth: %w", err)
	}

	wantIssuer := disc.Issuer
	if wantIssuer == "" {
		wantIssuer = cfg.Issuer
	}
	result, err := parseIDToken(cred.IDToken, idTokenChecks{
		issuer:   wantIssuer,
		clientID: clientID,
		nonce:    sent,
		jwksURI:  disc.JwksURI,
	})
	if err != nil {
		return nil, fmt.Errorf("auth: ID token validation failed: %w", err)
	}
	if result.Email == "" {
		result.Email = cred.Email
	}
	if result.Name == "" {
		result.Name = cred.Name
	}
	result.IDToken = cred.IDToken
	result.AuthorizationCode = cred.AuthorizationCode
	return result, nil
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// googleClientSuffix ends every Google OAuth client ID.
const googleClientSuffix = ".apps.googleusercontent.com"

// googleIOSRedirect gives the redirect a Google iOS client accepts: its client
// ID reversed into a URL scheme, which is also the scheme the browser session
// listens for.
func googleIOSRedirect(clientID string) (scheme, uri string, err error) {
	id, ok := strings.CutSuffix(clientID, googleClientSuffix)
	if !ok || id == "" {
		return "", "", fmt.Errorf("ClientID %q is not a Google client ID (it ends in %s)", clientID, googleClientSuffix)
	}
	scheme = "com.googleusercontent.apps." + id
	return scheme, scheme + ":/oauth2redirect", nil
}
