//go:build !no_emoji

package theme

import (
	_ "embed"

	"github.com/belluikit/bellui"
)

//go:embed font/EmojiOneColor.otf
var emojiFontData []byte

var emoji = &bellui.StaticResource{
	StaticName:    "EmojiOneColor.otf",
	StaticContent: emojiFontData,
}
