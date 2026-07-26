//go:build !ci && (!android || !ios || !mobile) && wasm

package widget

import (
	"strings"
	"syscall/js"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/storage"
	"github.com/yuin/goldmark/ast"
)

func parseMarkdownImage(t *ast.Image) []RichTextSegment {
	dest := string(t.Destination)
	u, err := storage.ParseURI(dest)
	if err != nil {
		if !strings.HasPrefix(dest, "/") {
			dest = "/" + dest
		}
		origin := js.Global().Get("location").Get("origin").String()
		u, err = storage.ParseURI(origin + dest)
		if err != nil {
			bellui.LogError("Can't load image in markdown", err)
			return []RichTextSegment{}
		}
	}
	return []RichTextSegment{&ImageSegment{Source: u, Title: string(t.Title), Alignment: bellui.TextAlignCenter}}
}
