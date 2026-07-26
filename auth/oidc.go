// Package auth provides OIDC and OAuth 2.0 authentication for desktop apps.
//
// The package implements the Authorization Code flow with PKCE, which is the
// recommended flow for native/desktop applications. It opens the user's system
// browser for authentication and receives the callback on a temporary localhost
// HTTP server.
//
// Supported providers: Google, Microsoft (Entra ID), Apple, GitHub, Okta,
// Keycloak, and any custom OIDC-compliant provider.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// OIDCConfig configures an authentication flow.
type OIDCConfig struct {
	// Provider is a known provider key ("google", "microsoft", "apple",
	// "github", "okta", "keycloak") or "custom" for arbitrary OIDC providers.
	Provider string

	// Issuer is the OIDC issuer URL. For known providers this is set
	// automatically from the preset. Required for "okta", "keycloak",
	// and "custom" providers.
	Issuer string

	// ClientID is the OAuth 2.0 client identifier.
	ClientID string

	// Scopes to request. Defaults to ["openid", "email", "profile"].
	Scopes []string

	// Timeout for the entire flow. Defaults to 5 minutes.
	Timeout time.Duration
}

// OIDCResult contains the authenticated user's information.
type OIDCResult struct {
	Email        string `json:"email"`
	Name         string `json:"name"`
	Picture      string `json:"picture"`
	Sub          string `json:"sub"` // unique user identifier
	AccessToken  string `json:"-"`
	RefreshToken string `json:"-"`
	IDToken      string `json:"-"`
}

// Authenticate runs the full OIDC Authorization Code + PKCE flow.
// It opens the system browser, waits for the callback, exchanges the
// code for tokens, and returns the user's claims.
func Authenticate(cfg OIDCConfig) (*OIDCResult, error) {
	// Apply defaults.
	if len(cfg.Scopes) == 0 {
		cfg.Scopes = []string{"openid", "email", "profile"}
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Minute
	}

	// Resolve provider preset.
	preset, hasPreset := Providers[cfg.Provider]
	if hasPreset && cfg.Issuer == "" {
		cfg.Issuer = preset.Issuer
	}

	// GitHub uses a custom OAuth flow, not OIDC.
	if cfg.Provider == "github" {
		return authenticateGitHub(cfg)
	}

	if cfg.Issuer == "" {
		return nil, fmt.Errorf("auth: issuer is required for provider %q", cfg.Provider)
	}

	// Step 1: OIDC Discovery.
	disc, err := discover(cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("auth: discovery failed: %w", err)
	}

	// Step 2: Generate PKCE.
	verifier, challenge, err := generatePKCE()
	if err != nil {
		return nil, fmt.Errorf("auth: PKCE generation failed: %w", err)
	}

	// Step 3: Start callback server.
	cb, err := newCallbackServer()
	if err != nil {
		return nil, fmt.Errorf("auth: callback server failed: %w", err)
	}
	defer cb.Close()

	redirectURI := cb.RedirectURI()

	// Step 4: Build authorization URL.
	state, err := randomString(32)
	if err != nil {
		return nil, err
	}
	nonce, err := randomString(32)
	if err != nil {
		return nil, err
	}

	authURL := buildAuthURL(disc.AuthorizationEndpoint, cfg.ClientID, redirectURI,
		cfg.Scopes, state, nonce, challenge)

	// Step 5: Open browser.
	if err := openBrowser(authURL); err != nil {
		return nil, fmt.Errorf("auth: failed to open browser: %w", err)
	}

	// Step 6: Wait for callback.
	code, callbackState, err := cb.WaitForCode(cfg.Timeout)
	if err != nil {
		return nil, fmt.Errorf("auth: callback failed: %w", err)
	}
	if callbackState != state {
		return nil, fmt.Errorf("auth: state mismatch (CSRF protection)")
	}

	// Step 7: Exchange code for tokens.
	tokenResp, err := exchangeCode(disc.TokenEndpoint, cfg.ClientID, code,
		redirectURI, verifier)
	if err != nil {
		return nil, fmt.Errorf("auth: token exchange failed: %w", err)
	}

	// Step 8: Verify the ID token signature and validate its claims. The
	// discovery document's issuer is authoritative for the `iss` check, and its
	// jwks_uri (required by OIDC) is used to verify the signature — fail closed
	// if it is absent rather than accepting an unverifiable token.
	if disc.JwksURI == "" {
		return nil, fmt.Errorf("auth: discovery document is missing jwks_uri; cannot verify ID token signature")
	}
	wantIssuer := disc.Issuer
	if wantIssuer == "" {
		wantIssuer = cfg.Issuer
	}
	result, err := parseIDToken(tokenResp.IDToken, idTokenChecks{
		issuer:   wantIssuer,
		clientID: cfg.ClientID,
		nonce:    nonce,
		jwksURI:  disc.JwksURI,
	})
	if err != nil {
		return nil, fmt.Errorf("auth: ID token validation failed: %w", err)
	}
	result.AccessToken = tokenResp.AccessToken
	result.RefreshToken = tokenResp.RefreshToken
	result.IDToken = tokenResp.IDToken

	return result, nil
}

// discoveryDoc holds the relevant fields from the OIDC discovery document.
type discoveryDoc struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserinfoEndpoint      string `json:"userinfo_endpoint"`
	JwksURI               string `json:"jwks_uri"`
	Issuer                string `json:"issuer"`
}

func discover(issuer string) (*discoveryDoc, error) {
	wellKnown := strings.TrimSuffix(issuer, "/") + "/.well-known/openid-configuration"
	resp, err := httpClient.Get(wellKnown)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discovery returned %d", resp.StatusCode)
	}

	var doc discoveryDoc
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, fmt.Errorf("decode discovery: %w", err)
	}

	// OIDC requires the discovery document's own issuer to match the issuer we
	// requested it from — this defends against a substituted discovery document
	// pointing us at attacker-controlled endpoints.
	if !issuerMatches(doc.Issuer, issuer) {
		return nil, fmt.Errorf("discovery issuer %q does not match requested issuer %q", doc.Issuer, issuer)
	}
	return &doc, nil
}

// issuerMatches reports whether a discovery document's issuer matches the issuer
// we requested. Multi-tenant providers (e.g. Microsoft's /common endpoint)
// publish a templated issuer such as ".../{tenantid}/v2.0"; the concrete tenant
// is only known once the ID token arrives, so a templated issuer is accepted
// here and the concrete value is still pinned later by the token `iss` check.
func issuerMatches(discovered, requested string) bool {
	if strings.Contains(discovered, "{") {
		return true
	}
	return discovered == requested
}

// tokenResponse holds the token endpoint response.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

func exchangeCode(tokenEndpoint, clientID, code, redirectURI, verifier string) (*tokenResponse, error) {
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {clientID},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"code_verifier": {verifier},
	}

	resp, err := httpClient.PostForm(tokenEndpoint, data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange returned %d: %s", resp.StatusCode, string(body))
	}

	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	return &tr, nil
}

// idTokenChecks holds the values an ID token is validated against.
type idTokenChecks struct {
	issuer   string // expected `iss` (from OIDC discovery)
	clientID string // expected to appear in `aud`
	nonce    string // expected `nonce` (replay protection)
	jwksURI  string // JWKS endpoint for signature verification; "" skips it
}

// idTokenClaims are the registered/standard claims we read from an ID token.
type idTokenClaims struct {
	Sub     string          `json:"sub"`
	Email   string          `json:"email"`
	Name    string          `json:"name"`
	Picture string          `json:"picture"`
	Iss     string          `json:"iss"`
	Aud     json.RawMessage `json:"aud"` // string or []string per JWT spec
	Exp     int64           `json:"exp"`
	Nonce   string          `json:"nonce"`
}

// clockSkewLeeway tolerates small clock differences when checking expiry.
const clockSkewLeeway = 2 * time.Minute

// parseIDToken validates a JWT ID token's signature and standard claims.
//
// When checks.jwksURI is set, the token's signature is cryptographically
// verified against the provider's JWKS first (rejecting alg:none and HMAC
// confusion); the signature step is skipped only when no JWKS endpoint is
// available. The registered claims are then validated: `iss` must match the
// discovered issuer, `aud` must contain our client ID, `exp` must be in the
// future, and `nonce` must round-trip the value sent in the auth request.
func parseIDToken(idToken string, checks idTokenChecks) (*OIDCResult, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format")
	}

	if checks.jwksURI != "" {
		if err := verifyJWTSignature(idToken, checks.jwksURI); err != nil {
			return nil, fmt.Errorf("signature verification failed: %w", err)
		}
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode JWT payload: %w", err)
	}

	var claims idTokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("decode claims: %w", err)
	}

	if err := validateClaims(claims, checks); err != nil {
		return nil, err
	}

	return &OIDCResult{
		Sub:     claims.Sub,
		Email:   claims.Email,
		Name:    claims.Name,
		Picture: claims.Picture,
	}, nil
}

// validateClaims enforces the registered-claim checks in idTokenChecks.
func validateClaims(c idTokenClaims, checks idTokenChecks) error {
	if checks.issuer != "" && c.Iss != checks.issuer {
		return fmt.Errorf("issuer mismatch: token %q, expected %q", c.Iss, checks.issuer)
	}
	if checks.clientID != "" && !audienceContains(c.Aud, checks.clientID) {
		return fmt.Errorf("audience does not include client ID")
	}
	if c.Exp == 0 {
		return fmt.Errorf("missing exp claim")
	}
	if time.Now().After(time.Unix(c.Exp, 0).Add(clockSkewLeeway)) {
		return fmt.Errorf("ID token expired")
	}
	if checks.nonce != "" && c.Nonce != checks.nonce {
		return fmt.Errorf("nonce mismatch (replay protection)")
	}
	return nil
}

// audienceContains reports whether the JWT `aud` claim (a string or an array
// of strings) includes the given client ID.
func audienceContains(raw json.RawMessage, clientID string) bool {
	if len(raw) == 0 {
		return false
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		return single == clientID
	}
	var multi []string
	if err := json.Unmarshal(raw, &multi); err == nil {
		for _, a := range multi {
			if a == clientID {
				return true
			}
		}
	}
	return false
}

// PKCE helpers.

func generatePKCE() (verifier, challenge string, err error) {
	verifier, err = randomString(64)
	if err != nil {
		return "", "", err
	}
	h := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(h[:])
	return verifier, challenge, nil
}

func randomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b)[:n], nil
}

func buildAuthURL(endpoint, clientID, redirectURI string, scopes []string, state, nonce, challenge string) string {
	v := url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {redirectURI},
		"scope":                 {strings.Join(scopes, " ")},
		"state":                 {state},
		"nonce":                 {nonce}, // bound into the ID token for replay protection
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"access_type":           {"offline"}, // request refresh token
		"prompt":                {"consent"}, // ensure refresh token for Google
	}
	return endpoint + "?" + v.Encode()
}

var httpClient = &http.Client{Timeout: 30 * time.Second}
