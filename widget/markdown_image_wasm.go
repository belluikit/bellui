// The `ci` tag must NOT gate this file: its counterpart markdown_image_notweb.go
// is `!wasm`, so excluding this one under `wasm && ci` leaves parseMarkdownImage
// undefined and breaks the wasm test build (#73).
//go:build (!android || !ios || !mobile) && wasm

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
