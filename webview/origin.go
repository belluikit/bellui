package webview

import (
	"net/url"
	"strings"
)

// serializeOrigin reduces a document URI to the security origin the bridge's
// [OriginPolicy] is written against — see [OriginOpaque] for the shape.
//
// It exists for backends whose engine hands over the sending frame's *URI*
// rather than an origin object. macOS gets a WKSecurityOrigin, the web target
// gets the browser's own event.origin and Android reads window.location.origin,
// so those three never need this; WebView2 reports a URI, and turning it into
// an origin is exactly the step that must not be skipped — a policy compared
// against a full URL would match on a path, which is not a security boundary.
//
// It is deliberately an allowlist. A scheme that is not http, https or file
// carries no origin a policy could meaningfully name (data:, about:blank,
// javascript:, and blob:, whose origin is the URL it wraps), so all of them
// report opaque and a policy has to name [OriginOpaque] to accept them.
func serializeOrigin(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return OriginOpaque
	}

	scheme := strings.ToLower(u.Scheme)
	if scheme == "file" {
		return "file://"
	}
	if scheme != "http" && scheme != "https" {
		return OriginOpaque
	}

	host := strings.ToLower(u.Hostname())
	if host == "" {
		return OriginOpaque
	}
	if strings.Contains(host, ":") { // IPv6 literal, serialized in brackets
		host = "[" + host + "]"
	}

	// The default port for the scheme is not part of the serialization, so
	// https://example.com and https://example.com:443 name the same origin.
	if port := u.Port(); port != "" && !defaultPort(scheme, port) {
		return scheme + "://" + host + ":" + port
	}
	return scheme + "://" + host
}

func defaultPort(scheme, port string) bool {
	return (scheme == "http" && port == "80") || (scheme == "https" && port == "443")
}
