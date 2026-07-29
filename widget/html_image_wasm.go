// The `ci` tag must NOT gate this file: its counterpart html_image_notweb.go is
// `!wasm`, so excluding this one under `wasm && ci` leaves parseHTMLImage
// undefined and breaks the wasm test build (#73).
//go:build (!android || !ios || !mobile) && wasm

package widget

import (
	"strings"
	"syscall/js"

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
	u, err := storage.ParseURI(src)
	if err != nil {
		if !strings.HasPrefix(src, "/") {
			src = "/" + src
		}
		origin := js.Global().Get("location").Get("origin").String()
		u, err = storage.ParseURI(origin + src)
		if err != nil {
			bellui.LogError("Can't load image in HTML", err)
			return []RichTextSegment{}
		}
	}
	return []RichTextSegment{&ImageSegment{Source: u, Title: title, Alignment: bellui.TextAlignCenter}}
}
