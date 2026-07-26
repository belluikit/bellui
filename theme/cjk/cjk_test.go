package cjk_test

import (
	"bytes"
	"testing"

	"github.com/belluikit/typesetting/font"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/theme/cjk"
)

func TestBundledFontCoversCJK(t *testing.T) {
	require.NotEmpty(t, cjk.Font.Content(), "bundled font is embedded")

	face, err := font.ParseTTF(bytes.NewReader(cjk.Font.Content()))
	require.NoError(t, err, "bundled font parses")

	// One representative rune per covered script, plus Latin.
	for _, r := range []rune{'世', '你', 'こ', 'カ', '안', 'A'} {
		_, ok := face.NominalGlyph(r)
		assert.True(t, ok, "bundled font should cover %q", string(r))
	}
}

func TestRegisterIsIdempotent(t *testing.T) {
	assert.NotPanics(t, cjk.Register)
	assert.NotPanics(t, cjk.Register)
}
