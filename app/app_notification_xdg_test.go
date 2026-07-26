//go:build !ci && !wasm && !android && !ios && !mobile && (linux || openbsd || freebsd || netbsd) && !tinygo && !noos && !tamago

package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseNotifyActionKey(t *testing.T) {
	idx, ok := parseNotifyActionKey("bellui-action-0")
	assert.True(t, ok)
	assert.Equal(t, 0, idx)

	idx, ok = parseNotifyActionKey("bellui-action-12")
	assert.True(t, ok)
	assert.Equal(t, 12, idx)

	_, ok = parseNotifyActionKey("default")
	assert.False(t, ok)

	_, ok = parseNotifyActionKey("bellui-action-x")
	assert.False(t, ok)

	_, ok = parseNotifyActionKey("other-3")
	assert.False(t, ok)
}
