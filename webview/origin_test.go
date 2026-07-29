package webview

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The Windows backend is the one whose engine reports a document URI rather
// than an origin, so it has to derive one — and getting that wrong would widen
// the OriginPolicy silently. These run on every platform because the reduction
// is portable, which is the whole reason it lives outside the backend.
func TestSerializeOrigin(t *testing.T) {
	for _, tt := range []struct {
		uri  string
		want string
	}{
		{"https://example.com/a/b?c=d#e", "https://example.com"},
		{"http://localhost:8080/index.html", "http://localhost:8080"},
		{"HTTPS://Example.COM/", "https://example.com"},

		// A default port is not part of the serialization, so naming the origin
		// with or without it has to mean the same thing.
		{"https://example.com:443/", "https://example.com"},
		{"http://example.com:80/", "http://example.com"},
		{"https://example.com:8443/", "https://example.com:8443"},

		{"http://[::1]:3000/", "http://[::1]:3000"},

		// Local content reports the scheme alone, matching what a
		// WKSecurityOrigin serializes to on the darwin backends.
		{"file:///C:/app/index.html", "file://"},

		// Content with no origin of its own must never be attributed to one:
		// a policy has to name OriginOpaque to accept any of these.
		{"about:blank", OriginOpaque},
		{"data:text/html,<p>hi", OriginOpaque},
		{"blob:https://example.com/1234", OriginOpaque},
		{"javascript:alert(1)", OriginOpaque},
		{"https:///nohost", OriginOpaque},
		{"", OriginOpaque},
		{"::not a url::", OriginOpaque},
	} {
		assert.Equal(t, tt.want, serializeOrigin(tt.uri), "origin of %q", tt.uri)
	}
}

// A policy naming an origin must not be satisfied by a URL that merely starts
// with it — the path is not a security boundary, and a full URL compared as an
// origin would make it one.
func TestSerializeOrigin_PathIsNotPartOfTheOrigin(t *testing.T) {
	policy := TrustOrigins("https://trusted.example")

	assert.True(t, policy(serializeOrigin("https://trusted.example/anything")))
	assert.False(t, policy(serializeOrigin("https://attacker.example/https://trusted.example")))
	assert.False(t, policy(serializeOrigin("https://trusted.example.attacker.test/")))
}
