package auth

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeJWT builds an unsigned JWT (alg "none") with the given claims. We never
// verify the signature, so the value of the third segment is irrelevant.
func makeJWT(t *testing.T, claims map[string]any) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload, err := json.Marshal(claims)
	require.NoError(t, err)
	return header + "." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}

func validClaims() map[string]any {
	return map[string]any{
		"sub":   "user-1",
		"email": "user@example.com",
		"name":  "User One",
		"iss":   "https://issuer.example.com",
		"aud":   "client-abc",
		"exp":   time.Now().Add(time.Hour).Unix(),
		"nonce": "nonce-xyz",
	}
}

func validChecks() idTokenChecks {
	return idTokenChecks{
		issuer:   "https://issuer.example.com",
		clientID: "client-abc",
		nonce:    "nonce-xyz",
	}
}

func TestParseIDToken_Valid(t *testing.T) {
	tok := makeJWT(t, validClaims())
	res, err := parseIDToken(tok, validChecks())
	require.NoError(t, err)
	assert.Equal(t, "user-1", res.Sub)
	assert.Equal(t, "user@example.com", res.Email)
	assert.Equal(t, "User One", res.Name)
}

func TestParseIDToken_AudienceArray(t *testing.T) {
	claims := validClaims()
	claims["aud"] = []string{"other", "client-abc"}
	res, err := parseIDToken(makeJWT(t, claims), validChecks())
	require.NoError(t, err)
	assert.Equal(t, "user-1", res.Sub)
}

func TestParseIDToken_Rejections(t *testing.T) {
	tests := []struct {
		name string
		mut  func(map[string]any)
	}{
		{"issuer mismatch", func(c map[string]any) { c["iss"] = "https://evil.example.com" }},
		{"audience mismatch", func(c map[string]any) { c["aud"] = "someone-else" }},
		{"expired", func(c map[string]any) { c["exp"] = time.Now().Add(-time.Hour).Unix() }},
		{"missing exp", func(c map[string]any) { delete(c, "exp") }},
		{"nonce mismatch", func(c map[string]any) { c["nonce"] = "wrong" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := validClaims()
			tt.mut(claims)
			_, err := parseIDToken(makeJWT(t, claims), validChecks())
			assert.Error(t, err)
		})
	}
}

func TestParseIDToken_MalformedJWT(t *testing.T) {
	_, err := parseIDToken("not.a.jwt.token", validChecks())
	assert.Error(t, err)

	_, err = parseIDToken("only-one-segment", validChecks())
	assert.Error(t, err)
}

func TestAudienceContains(t *testing.T) {
	assert.True(t, audienceContains(json.RawMessage(`"abc"`), "abc"))
	assert.False(t, audienceContains(json.RawMessage(`"abc"`), "xyz"))
	assert.True(t, audienceContains(json.RawMessage(`["a","abc"]`), "abc"))
	assert.False(t, audienceContains(json.RawMessage(`["a","b"]`), "abc"))
	assert.False(t, audienceContains(nil, "abc"))
}

func TestBuildAuthURL_IncludesNonce(t *testing.T) {
	u := buildAuthURL("https://issuer/auth", "client", "http://127.0.0.1:1/callback",
		[]string{"openid"}, "state-1", "nonce-1", "challenge-1")
	assert.True(t, strings.Contains(u, "nonce=nonce-1"), "auth URL should carry the nonce: %s", u)
	assert.True(t, strings.Contains(u, "state=state-1"))
}
