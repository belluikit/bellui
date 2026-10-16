//go:build !wasm

package widget

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/storage"
	"github.com/yuin/goldmark/ast"
)

func parseMarkdownImage(t *ast.Image) []RichTextSegment {
	// A schemeless destination (e.g. "/etc/passwd" or a relative path) is NOT
	// turned into a file:// URI: that fallback let untrusted markup read
	// arbitrary local files (pre-rebrand tracker #285). Such an image renders as a placeholder;
	// embed an explicit file:// URI if a local image is genuinely intended.
	dest := string(t.Destination)
	u, err := storage.ParseURI(dest)
	if err != nil {
		u = nil
	}
	return []RichTextSegment{&ImageSegment{Source: u, Title: string(t.Title), Alignment: bellui.TextAlignCenter}}
}
