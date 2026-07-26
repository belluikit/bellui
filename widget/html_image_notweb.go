//go:build !wasm

package widget

import (
	"golang.org/x/net/html"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/storage"
)

func parseHTMLImage(n *html.Node) []RichTextSegment {
	src := htmlAttr(n, "src")
	title := htmlAttr(n, "alt")
	if title == "" {
		title = htmlAttr(n, "title")
	}
	// A schemeless src (e.g. "/etc/passwd" or a relative path) is NOT turned
	// into a file:// URI: that fallback let untrusted markup read arbitrary
	// local files (#285). Such an image renders as a placeholder; embed an
	// explicit file:// URI if a local image is genuinely intended.
	u, err := storage.ParseURI(src)
	if err != nil {
		u = nil
	}
	return []RichTextSegment{&ImageSegment{Source: u, Title: title, Alignment: bellui.TextAlignCenter}}
}
