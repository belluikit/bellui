package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/test"
)

// charMinSize keeps its last measurement (#492). It must give a fresh answer
// whenever anything the measurement depends on changes: the text size, the
// style, the concealed character, or the fonts (the cache generation).
func TestRichText_CharMinSizeMemoFollowsItsInputs(t *testing.T) {
	test.NewTempApp(t)
	r := NewRichTextWithText("x")

	small := r.charMinSize(false, bellui.TextStyle{}, 14)
	assert.Equal(t, small, r.charMinSize(false, bellui.TextStyle{}, 14))
	assert.Equal(t, bellui.MeasureText("M", 28, bellui.TextStyle{}), r.charMinSize(false, bellui.TextStyle{}, 28))
	assert.Equal(t, bellui.MeasureText("M", 14, bellui.TextStyle{Monospace: true}), r.charMinSize(false, bellui.TextStyle{Monospace: true}, 14))
	assert.Equal(t, bellui.MeasureText(passwordChar, 14, bellui.TextStyle{}), r.charMinSize(true, bellui.TextStyle{}, 14))

	before := r.charMemo.key.generation
	cache.ResetThemeCaches()
	assert.Equal(t, small, r.charMinSize(false, bellui.TextStyle{}, 14))
	assert.NotEqual(t, before, r.charMemo.key.generation, "a font cache reset must drop the memo")
}
