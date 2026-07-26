// Package clipboard provides pure-Go codecs for platform clipboard payloads:
// the Windows CF_HTML ("HTML Format") header block and device-independent
// bitmaps (CF_DIB). They are kept free of platform dependencies so every CI
// leg unit-tests the encoding the native bridges rely on.
package clipboard

import (
	"bytes"
	"fmt"
	"strconv"
)

const (
	cfHTMLPrefix = "<html>\r\n<body>\r\n<!--StartFragment-->"
	cfHTMLSuffix = "<!--EndFragment-->\r\n</body>\r\n</html>"

	startFragmentMarker = "<!--StartFragment-->"
	endFragmentMarker   = "<!--EndFragment-->"
)

// EncodeCFHTML wraps an HTML fragment in the Windows "HTML Format" clipboard
// payload: a fixed-width offset header followed by a full document embedding
// the fragment. All offsets are byte offsets into the returned payload.
func EncodeCFHTML(fragment string) []byte {
	// The header length is constant because the offsets are zero-padded to a
	// fixed width, so offsets can be computed before formatting.
	const headerTemplate = "Version:0.9\r\nStartHTML:%010d\r\nEndHTML:%010d\r\nStartFragment:%010d\r\nEndFragment:%010d\r\n"
	headerLen := len(fmt.Sprintf(headerTemplate, 0, 0, 0, 0))

	startHTML := headerLen
	startFragment := startHTML + len(cfHTMLPrefix)
	endFragment := startFragment + len(fragment)
	endHTML := endFragment + len(cfHTMLSuffix)

	var buf bytes.Buffer
	buf.Grow(endHTML)
	fmt.Fprintf(&buf, headerTemplate, startHTML, endHTML, startFragment, endFragment)
	buf.WriteString(cfHTMLPrefix)
	buf.WriteString(fragment)
	buf.WriteString(cfHTMLSuffix)
	return buf.Bytes()
}

// DecodeCFHTML extracts the HTML fragment from a Windows "HTML Format"
// clipboard payload. It prefers the StartFragment/EndFragment byte offsets,
// falling back to the fragment comment markers and finally the
// StartHTML/EndHTML offsets, tolerating the header variations real producers
// emit (extra lines such as SourceURL, LF-only line endings, other versions).
// It returns ("", false) when the payload cannot be understood.
func DecodeCFHTML(payload []byte) (string, bool) {
	start, startOK := cfHTMLOffset(payload, "StartFragment:")
	end, endOK := cfHTMLOffset(payload, "EndFragment:")
	if startOK && endOK && start <= end && end <= len(payload) {
		return string(payload[start:end]), true
	}

	if start := bytes.Index(payload, []byte(startFragmentMarker)); start >= 0 {
		start += len(startFragmentMarker)
		if end := bytes.LastIndex(payload, []byte(endFragmentMarker)); end >= start {
			return string(payload[start:end]), true
		}
	}

	start, startOK = cfHTMLOffset(payload, "StartHTML:")
	end, endOK = cfHTMLOffset(payload, "EndHTML:")
	if startOK && endOK && start <= end && end <= len(payload) {
		return string(payload[start:end]), true
	}
	return "", false
}

// cfHTMLOffset scans the payload's header block for "key" and parses the
// decimal byte offset following it. Some producers write -1 for absent
// offsets, which is reported as not found.
func cfHTMLOffset(payload []byte, key string) (int, bool) {
	idx := bytes.Index(payload, []byte(key))
	if idx < 0 {
		return 0, false
	}
	num := payload[idx+len(key):]
	if end := bytes.IndexAny(num, "\r\n"); end >= 0 {
		num = num[:end]
	}
	val, err := strconv.Atoi(string(bytes.TrimSpace(num)))
	if err != nil || val < 0 {
		return 0, false
	}
	return val, true
}
