package auth

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	_ "crypto/sha256" // register SHA-224/256 for crypto.Hash.New
	_ "crypto/sha512" // register SHA-384/512 for crypto.Hash.New
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

// jwk is a single JSON Web Key. Only the public RSA and EC fields needed for
// signature verification are modelled.
type jwk struct {
	Kty string `json:"kty"` // "RSA" or "EC"
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	N   string `json:"n"`   // RSA modulus (base64url)
	E   string `json:"e"`   // RSA exponent (base64url)
	Crv string `json:"crv"` // EC curve ("P-256"/"P-384"/"P-521")
	X   string `json:"x"`   // EC x coordinate (base64url)
	Y   string `json:"y"`   // EC y coordinate (base64url)
}

// jwksDoc is a JSON Web Key Set document.
type jwksDoc struct {
	Keys []jwk `json:"keys"`
}

// jwtHeader is the decoded JOSE header of a JWT.
type jwtHeader struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	Typ string `json:"typ"`
}

// sigParams maps a JWS algorithm to its hash and key family. Only asymmetric
// algorithms are listed; symmetric "HS*" algorithms and "none" are deliberately
// absent so they are rejected — this is the alg:none / algorithm-confusion
// defence (a token must never select an HMAC path against an asymmetric key).
type sigParams struct {
	hash   crypto.Hash
	family string // "RSA", "RSA-PSS", or "EC"
}

var supportedSigAlgs = map[string]sigParams{
	"RS256": {crypto.SHA256, "RSA"},
	"RS384": {crypto.SHA384, "RSA"},
	"RS512": {crypto.SHA512, "RSA"},
	"PS256": {crypto.SHA256, "RSA-PSS"},
	"PS384": {crypto.SHA384, "RSA-PSS"},
	"PS512": {crypto.SHA512, "RSA-PSS"},
	"ES256": {crypto.SHA256, "EC"},
	"ES384": {crypto.SHA384, "EC"},
	"ES512": {crypto.SHA512, "EC"},
}

// verifyJWTSignature verifies idToken's signature using a key fetched from
// jwksURI. It rejects "none", HMAC and any unrecognised algorithm, and requires
// the JWKS key type to match the header algorithm's family.
func verifyJWTSignature(idToken, jwksURI string) error {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return fmt.Errorf("invalid JWT format")
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return fmt.Errorf("decode JWT header: %w", err)
	}
	var hdr jwtHeader
	if err := json.Unmarshal(headerBytes, &hdr); err != nil {
		return fmt.Errorf("decode JWT header: %w", err)
	}

	params, ok := supportedSigAlgs[hdr.Alg]
	if !ok {
		// Covers alg:"none", HS256/384/512 and anything unrecognised. We never
		// fall through to an unverified or symmetric verification path.
		return fmt.Errorf("unsupported or unsafe JWT alg %q", hdr.Alg)
	}

	key, err := defaultJWKS.key(jwksURI, hdr.Kid)
	if err != nil {
		return err
	}

	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return fmt.Errorf("decode JWT signature: %w", err)
	}

	h := params.hash.New()
	h.Write([]byte(parts[0] + "." + parts[1]))
	digest := h.Sum(nil)

	switch params.family {
	case "RSA", "RSA-PSS":
		if key.Kty != "RSA" {
			return fmt.Errorf("JWKS key type %q does not match alg %q", key.Kty, hdr.Alg)
		}
		pub, err := key.rsaPublicKey()
		if err != nil {
			return err
		}
		if params.family == "RSA-PSS" {
			return rsa.VerifyPSS(pub, params.hash, digest, sig, nil)
		}
		return rsa.VerifyPKCS1v15(pub, params.hash, digest, sig)
	case "EC":
		if key.Kty != "EC" {
			return fmt.Errorf("JWKS key type %q does not match alg %q", key.Kty, hdr.Alg)
		}
		return key.verifyECDSA(digest, sig)
	}
	return fmt.Errorf("unsupported alg family %q", params.family)
}

// rsaPublicKey reconstructs the RSA public key from the JWK n/e fields.
func (k jwk) rsaPublicKey() (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("decode RSA modulus: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("decode RSA exponent: %w", err)
	}
	e := new(big.Int).SetBytes(eBytes)
	if !e.IsInt64() || e.Int64() < 2 || e.Int64() > 1<<31-1 {
		return nil, fmt.Errorf("invalid RSA exponent")
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: int(e.Int64())}, nil
}

// verifyECDSA verifies a raw (r||s) JWS ECDSA signature over digest.
func (k jwk) verifyECDSA(digest, sig []byte) error {
	var curve elliptic.Curve
	switch k.Crv {
	case "P-256":
		curve = elliptic.P256()
	case "P-384":
		curve = elliptic.P384()
	case "P-521":
		curve = elliptic.P521()
	default:
		return fmt.Errorf("unsupported EC curve %q", k.Crv)
	}
	xBytes, err := base64.RawURLEncoding.DecodeString(k.X)
	if err != nil {
		return fmt.Errorf("decode EC x: %w", err)
	}
	yBytes, err := base64.RawURLEncoding.DecodeString(k.Y)
	if err != nil {
		return fmt.Errorf("decode EC y: %w", err)
	}

	size := (curve.Params().BitSize + 7) / 8
	if len(xBytes) > size || len(yBytes) > size {
		return fmt.Errorf("invalid EC coordinate length")
	}
	// Assemble the SEC 1 uncompressed point (0x04 || X || Y) and parse it;
	// ParseUncompressedPublicKey also rejects points that are not on the curve.
	uncompressed := make([]byte, 1+2*size)
	uncompressed[0] = 4
	copy(uncompressed[1+size-len(xBytes):1+size], xBytes)
	copy(uncompressed[1+2*size-len(yBytes):], yBytes)
	pub, err := ecdsa.ParseUncompressedPublicKey(curve, uncompressed)
	if err != nil {
		return fmt.Errorf("invalid EC public key: %w", err)
	}

	// JWS ECDSA signatures are the fixed-width r||s concatenation, not ASN.1.
	if len(sig) != 2*size {
		return fmt.Errorf("invalid ECDSA signature length")
	}
	r := new(big.Int).SetBytes(sig[:size])
	s := new(big.Int).SetBytes(sig[size:])
	if !ecdsa.Verify(pub, digest, r, s) {
		return fmt.Errorf("ECDSA signature verification failed")
	}
	return nil
}

// jwksTTL bounds how long a fetched key set is reused before refetching.
const jwksTTL = time.Hour

// jwksCache memoises fetched key sets per JWKS URI.
type jwksCache struct {
	mu      sync.Mutex
	entries map[string]*jwksCacheEntry
}

type jwksCacheEntry struct {
	keys      map[string]jwk // keyed by kid
	fetchedAt time.Time
}

var defaultJWKS = &jwksCache{entries: map[string]*jwksCacheEntry{}}

// key returns the signing key for kid from jwksURI, fetching (and caching) the
// key set as needed. An unknown kid against a cached set triggers a single
// refetch to ride out provider key rotation.
//
// The mutex is held across the network fetch: auth is interactive and
// infrequent, so the simplicity is worth more than fetch concurrency here.
func (c *jwksCache) key(jwksURI, kid string) (jwk, error) {
	if jwksURI == "" {
		return jwk{}, fmt.Errorf("no JWKS URI available to verify the ID token signature")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	entry := c.entries[jwksURI]
	freshlyFetched := false
	if entry == nil || time.Since(entry.fetchedAt) > jwksTTL {
		var err error
		entry, err = c.fetchLocked(jwksURI)
		if err != nil {
			return jwk{}, err
		}
		freshlyFetched = true
	}

	if k, ok := selectKey(entry, kid); ok {
		return k, nil
	}
	if !freshlyFetched {
		// Unknown kid against a cached set — keys may have rotated; refetch once.
		entry, err := c.fetchLocked(jwksURI)
		if err != nil {
			return jwk{}, err
		}
		if k, ok := selectKey(entry, kid); ok {
			return k, nil
		}
	}
	return jwk{}, fmt.Errorf("no signing key for kid %q in JWKS", kid)
}

// fetchLocked fetches and caches the key set. The caller must hold c.mu.
func (c *jwksCache) fetchLocked(jwksURI string) (*jwksCacheEntry, error) {
	resp, err := httpClient.Get(jwksURI)
	if err != nil {
		return nil, fmt.Errorf("fetch JWKS: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS endpoint returned %d", resp.StatusCode)
	}

	var doc jwksDoc
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&doc); err != nil {
		return nil, fmt.Errorf("decode JWKS: %w", err)
	}

	keys := make(map[string]jwk, len(doc.Keys))
	for _, k := range doc.Keys {
		keys[k.Kid] = k
	}
	entry := &jwksCacheEntry{keys: keys, fetchedAt: time.Now()}
	c.entries[jwksURI] = entry
	return entry, nil
}

// selectKey returns the key for kid, or the sole key when the token carries no
// kid and the set holds exactly one key.
func selectKey(e *jwksCacheEntry, kid string) (jwk, bool) {
	if k, ok := e.keys[kid]; ok {
		return k, true
	}
	if kid == "" && len(e.keys) == 1 {
		for _, k := range e.keys {
			return k, true
		}
	}
	return jwk{}, false
}

// reset clears the cache. Used by tests.
func (c *jwksCache) reset() {
	c.mu.Lock()
	c.entries = map[string]*jwksCacheEntry{}
	c.mu.Unlock()
}
