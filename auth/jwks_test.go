// Every positive path here fetches JWKS/discovery documents from an
// in-process httptest server, which cannot listen inside a browser — and
// without it the negative tests would pass for the wrong reason (any fetch
// error also fails verification).
// (#77's split convention; #79 Phase B.)
//go:build !wasm

package auth

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- test JWT signing helpers ---

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func signingInput(header, claims map[string]any) string {
	hb, _ := json.Marshal(header)
	cb, _ := json.Marshal(claims)
	return b64(hb) + "." + b64(cb)
}

func signRS256(t *testing.T, priv *rsa.PrivateKey, kid string, claims map[string]any) string {
	t.Helper()
	si := signingInput(map[string]any{"alg": "RS256", "typ": "JWT", "kid": kid}, claims)
	sum := sha256.Sum256([]byte(si))
	sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, sum[:])
	require.NoError(t, err)
	return si + "." + b64(sig)
}

func signES256(t *testing.T, priv *ecdsa.PrivateKey, kid string, claims map[string]any) string {
	t.Helper()
	si := signingInput(map[string]any{"alg": "ES256", "typ": "JWT", "kid": kid}, claims)
	sum := sha256.Sum256([]byte(si))
	r, s, err := ecdsa.Sign(rand.Reader, priv, sum[:])
	require.NoError(t, err)
	out := make([]byte, 64) // P-256: 32-byte r || 32-byte s
	r.FillBytes(out[:32])
	s.FillBytes(out[32:])
	return si + "." + b64(out)
}

func signHS256(secret []byte, kid string, claims map[string]any) string {
	si := signingInput(map[string]any{"alg": "HS256", "typ": "JWT", "kid": kid}, claims)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(si))
	return si + "." + b64(mac.Sum(nil))
}

// --- test JWK / JWKS server helpers ---

func rsaJWK(kid string, pub *rsa.PublicKey) jwk {
	return jwk{
		Kty: "RSA", Kid: kid, Alg: "RS256",
		N: b64(pub.N.Bytes()),
		E: b64(big.NewInt(int64(pub.E)).Bytes()),
	}
}

func ecJWK(t *testing.T, kid string, pub *ecdsa.PublicKey) jwk {
	t.Helper()
	raw, err := pub.Bytes() // 0x04 || X(32) || Y(32) for P-256
	require.NoError(t, err)
	return jwk{Kty: "EC", Kid: kid, Alg: "ES256", Crv: "P-256", X: b64(raw[1:33]), Y: b64(raw[33:65])}
}

func jwksServer(t *testing.T, keys ...jwk) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jwksDoc{Keys: keys})
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func newRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return k
}

func newECKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return k
}

// --- signature verification tests ---

func TestVerifyJWTSignature_RS256(t *testing.T) {
	defaultJWKS.reset()
	priv := newRSAKey(t)
	uri := jwksServer(t, rsaJWK("k1", &priv.PublicKey))
	tok := signRS256(t, priv, "k1", validClaims())

	assert.NoError(t, verifyJWTSignature(tok, uri))
}

func TestVerifyJWTSignature_ES256(t *testing.T) {
	defaultJWKS.reset()
	priv := newECKey(t)
	uri := jwksServer(t, ecJWK(t, "k1", &priv.PublicKey))
	tok := signES256(t, priv, "k1", validClaims())

	assert.NoError(t, verifyJWTSignature(tok, uri))
}

func TestVerifyJWTSignature_RejectsNone(t *testing.T) {
	defaultJWKS.reset()
	priv := newRSAKey(t)
	uri := jwksServer(t, rsaJWK("k1", &priv.PublicKey))

	// alg:none is never acceptable, even with a valid JWKS available.
	assert.Error(t, verifyJWTSignature(makeJWT(t, validClaims()), uri))
}

func TestVerifyJWTSignature_RejectsHMACConfusion(t *testing.T) {
	defaultJWKS.reset()
	priv := newRSAKey(t)
	uri := jwksServer(t, rsaJWK("k1", &priv.PublicKey))

	// Classic confusion: forge an HS256 token using the public modulus as the
	// HMAC secret. It must be rejected because HS* is not a supported alg.
	tok := signHS256(priv.PublicKey.N.Bytes(), "k1", validClaims())
	assert.Error(t, verifyJWTSignature(tok, uri))
}

func TestVerifyJWTSignature_TamperedPayload(t *testing.T) {
	defaultJWKS.reset()
	priv := newRSAKey(t)
	uri := jwksServer(t, rsaJWK("k1", &priv.PublicKey))
	tok := signRS256(t, priv, "k1", validClaims())

	// Swap in a different payload, keep the original signature.
	parts := splitJWT(tok)
	forged := map[string]any{"sub": "attacker"}
	cb, _ := json.Marshal(forged)
	tampered := parts[0] + "." + b64(cb) + "." + parts[2]
	assert.Error(t, verifyJWTSignature(tampered, uri))
}

func TestVerifyJWTSignature_WrongKey(t *testing.T) {
	defaultJWKS.reset()
	signer := newRSAKey(t)
	other := newRSAKey(t)
	// Server publishes a different key under the same kid.
	uri := jwksServer(t, rsaJWK("k1", &other.PublicKey))
	tok := signRS256(t, signer, "k1", validClaims())

	assert.Error(t, verifyJWTSignature(tok, uri))
}

func TestVerifyJWTSignature_KeyTypeMismatch(t *testing.T) {
	defaultJWKS.reset()
	priv := newRSAKey(t)
	ec := newECKey(t)
	// Header says RS256 but the JWKS key under that kid is an EC key.
	uri := jwksServer(t, ecJWK(t, "k1", &ec.PublicKey))
	tok := signRS256(t, priv, "k1", validClaims())

	assert.Error(t, verifyJWTSignature(tok, uri))
}

func TestVerifyJWTSignature_UnknownKid(t *testing.T) {
	defaultJWKS.reset()
	priv := newRSAKey(t)
	uri := jwksServer(t, rsaJWK("k1", &priv.PublicKey))
	tok := signRS256(t, priv, "k2", validClaims()) // kid not in the set

	assert.Error(t, verifyJWTSignature(tok, uri))
}

func TestJWKSCache_RefetchesOnRotation(t *testing.T) {
	defaultJWKS.reset()
	k1 := newRSAKey(t)
	k2 := newRSAKey(t)

	var mu sync.Mutex
	current := []jwk{rsaJWK("k1", &k1.PublicKey)}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		keys := current
		mu.Unlock()
		_ = json.NewEncoder(w).Encode(jwksDoc{Keys: keys})
	}))
	t.Cleanup(srv.Close)

	// Prime the cache with k1.
	require.NoError(t, verifyJWTSignature(signRS256(t, k1, "k1", validClaims()), srv.URL))

	// Provider rotates to k2; a token signed with the new kid forces a refetch.
	mu.Lock()
	current = []jwk{rsaJWK("k2", &k2.PublicKey)}
	mu.Unlock()
	assert.NoError(t, verifyJWTSignature(signRS256(t, k2, "k2", validClaims()), srv.URL))
}

// --- parseIDToken integration ---

func TestParseIDToken_VerifiesSignature(t *testing.T) {
	defaultJWKS.reset()
	priv := newRSAKey(t)
	uri := jwksServer(t, rsaJWK("k1", &priv.PublicKey))
	checks := validChecks()
	checks.jwksURI = uri

	res, err := parseIDToken(signRS256(t, priv, "k1", mapClaims(validClaims())), checks)
	require.NoError(t, err)
	assert.Equal(t, "user-1", res.Sub)
}

func TestParseIDToken_RejectsBadSignature(t *testing.T) {
	defaultJWKS.reset()
	signer := newRSAKey(t)
	other := newRSAKey(t)
	uri := jwksServer(t, rsaJWK("k1", &other.PublicKey))
	checks := validChecks()
	checks.jwksURI = uri

	_, err := parseIDToken(signRS256(t, signer, "k1", mapClaims(validClaims())), checks)
	assert.Error(t, err)
}

func TestParseIDToken_RejectsNoneWhenJWKSPresent(t *testing.T) {
	defaultJWKS.reset()
	priv := newRSAKey(t)
	checks := validChecks()
	checks.jwksURI = jwksServer(t, rsaJWK("k1", &priv.PublicKey))

	_, err := parseIDToken(makeJWT(t, validClaims()), checks)
	assert.Error(t, err)
}

// --- discovery issuer match ---

func discoveryServer(t *testing.T, doc discoveryDoc) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(doc)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestDiscover_IssuerMatch(t *testing.T) {
	// A server whose discovery issuer equals its own URL (the requested issuer).
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(discoveryDoc{Issuer: srv.URL, JwksURI: srv.URL + "/jwks"})
	})

	doc, err := discover(srv.URL)
	require.NoError(t, err)
	assert.Equal(t, srv.URL+"/jwks", doc.JwksURI)
}

func TestDiscover_IssuerMismatch(t *testing.T) {
	uri := discoveryServer(t, discoveryDoc{Issuer: "https://evil.example.com"})
	_, err := discover(uri)
	assert.Error(t, err, "a discovery issuer that doesn't match the requested issuer is rejected")
}

func TestDiscover_TemplatedIssuerTolerated(t *testing.T) {
	// A multi-tenant templated issuer (Microsoft /common style) is accepted.
	uri := discoveryServer(t, discoveryDoc{
		Issuer:  "https://login.microsoftonline.com/{tenantid}/v2.0",
		JwksURI: "https://login.microsoftonline.com/common/discovery/v2.0/keys",
	})
	_, err := discover(uri)
	assert.NoError(t, err)
}

func TestIssuerMatches(t *testing.T) {
	assert.True(t, issuerMatches("https://a.example", "https://a.example"))
	assert.False(t, issuerMatches("https://a.example", "https://b.example"))
	assert.True(t, issuerMatches("https://x/{tenantid}/v2.0", "https://x/common/v2.0"))
}

// --- helpers shared with these tests ---

func splitJWT(tok string) [3]string {
	var out [3]string
	i := 0
	start := 0
	for j := 0; j < len(tok) && i < 3; j++ {
		if tok[j] == '.' {
			out[i] = tok[start:j]
			i++
			start = j + 1
		}
	}
	out[2] = tok[start:]
	return out
}

// mapClaims converts the validClaims map (used across oidc_test.go) into the
// any-typed map signRS256 expects, normalising the exp value.
func mapClaims(c map[string]any) map[string]any {
	out := make(map[string]any, len(c))
	for k, v := range c {
		out[k] = v
	}
	if _, ok := out["exp"]; !ok {
		out["exp"] = time.Now().Add(time.Hour).Unix()
	}
	return out
}
