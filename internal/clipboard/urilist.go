package clipboard

import (
	"net/url"
	"strings"
)

// This file implements the text/uri-list clipboard flavor (RFC 2483: CRLF
// separated URIs, lines starting with '#' are comments) and the
// x-special/gnome-copied-files variant used by GNOME Files and other Linux
// file managers (an operation line — "copy" or "cut" — followed by
// newline-separated URIs). Pure Go so it is unit-tested on every CI leg,
// like the CF_HTML and DIB codecs.

// EncodeURIList encodes file paths as a text/uri-list payload of
// percent-encoded file:// URIs.
func EncodeURIList(paths []string) []byte {
	var b strings.Builder
	for _, path := range paths {
		b.WriteString(fileURIString(path))
		b.WriteString("\r\n")
	}
	return []byte(b.String())
}

// DecodeURIList returns the file paths of the file:// URIs in a
// text/uri-list payload. Comment lines and non-file URIs are skipped.
func DecodeURIList(data []byte) []string {
	var paths []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, "\r")
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if path, ok := fileURIPath(line); ok {
			paths = append(paths, path)
		}
	}
	return paths
}

// EncodeGnomeCopiedFiles encodes file paths as an
// x-special/gnome-copied-files payload with the given operation
// ("copy" or "cut").
func EncodeGnomeCopiedFiles(op string, paths []string) []byte {
	parts := make([]string, 0, len(paths)+1)
	parts = append(parts, op)
	for _, path := range paths {
		parts = append(parts, fileURIString(path))
	}
	return []byte(strings.Join(parts, "\n"))
}

// DecodeGnomeCopiedFiles returns the operation and file paths of an
// x-special/gnome-copied-files payload. Payloads whose first line is not a
// known operation are rejected — some clipboard owners (xclip) serve their
// data for any requested target, so a text/uri-list payload can arrive here.
func DecodeGnomeCopiedFiles(data []byte) (op string, paths []string) {
	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return "", nil
	}

	op = strings.TrimSpace(strings.TrimRight(lines[0], "\r"))
	if op != "copy" && op != "cut" {
		return "", nil
	}
	for _, line := range lines[1:] {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if line == "" {
			continue
		}
		if path, ok := fileURIPath(line); ok {
			paths = append(paths, path)
		}
	}
	return op, paths
}

// fileURIString renders path as a percent-encoded file:// URI.
func fileURIString(path string) string {
	u := url.URL{Scheme: "file", Path: path}
	return u.String()
}

// fileURIPath returns the decoded path of a file:// URI, or ok=false for any
// other URI.
func fileURIPath(uri string) (string, bool) {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" || u.Path == "" {
		return "", false
	}
	// A non-empty host other than localhost names a remote file (rare);
	// keep local files only.
	if u.Host != "" && u.Host != "localhost" {
		return "", false
	}
	return u.Path, true
}
