package mobile

import (
	"github.com/belluikit/bellui"
)

// Declare conformity with Clipboard interface
var _ bellui.Clipboard = mobileClipboard{}

func NewClipboard() bellui.Clipboard {
	return mobileClipboard{}
}

// mobileClipboard represents the system mobileClipboard
type mobileClipboard struct{}
