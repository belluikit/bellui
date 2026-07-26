package cache

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/stretchr/testify/assert"
)

func TestTextCacheGet(t *testing.T) {
	ResetThemeCaches()
	assert.Equal(t, 0, fontSizeCache.Len())

	bound, base := GetFontMetrics("hi", 10, bellui.TextStyle{}, nil)
	assert.True(t, bound.IsZero())
	assert.Equal(t, float32(0), base)

	SetFontMetrics("hi", 10, bellui.TextStyle{}, nil, geo.NewSize(10, 10), 8)
	assert.Equal(t, 1, fontSizeCache.Len())

	bound, base = GetFontMetrics("hi", 10, bellui.TextStyle{}, nil)
	assert.Equal(t, geo.NewSize(10, 10), bound)
	assert.Equal(t, float32(8), base)
}

// TestThemeCacheGeneration verifies the generation counter is bumped by every
// ResetThemeCaches so a widget-level measurement memo can invalidate in lock-step
// with the global metrics cache (#896).
func TestThemeCacheGeneration(t *testing.T) {
	before := ThemeCacheGeneration()
	ResetThemeCaches()
	assert.Greater(t, ThemeCacheGeneration(), before)
	// A second reset advances it again — memos comparing an old generation always
	// see the change.
	mid := ThemeCacheGeneration()
	ResetThemeCaches()
	assert.Greater(t, ThemeCacheGeneration(), mid)
}

// TestTextCacheKeyShapingFields ensures TextStyle fields that influence shaping
// (and thus measurement) are part of the cache key, so styles differing only by
// one of them do not collide. Covers Direction (#70) and Language (#22).
func TestTextCacheKeyShapingFields(t *testing.T) {
	ResetThemeCaches()

	rtl := bellui.TextStyle{Direction: bellui.TextDirectionRTL}
	serbian := bellui.TextStyle{Language: "sr-Cyrl"}

	SetFontMetrics("hi", 10, bellui.TextStyle{}, nil, geo.NewSize(10, 10), 8)
	SetFontMetrics("hi", 10, rtl, nil, geo.NewSize(20, 20), 16)
	SetFontMetrics("hi", 10, serbian, nil, geo.NewSize(30, 30), 24)

	// Three distinct keys for the same text/size means the fields are keyed.
	assert.Equal(t, 3, fontSizeCache.Len())

	bound, base := GetFontMetrics("hi", 10, rtl, nil)
	assert.Equal(t, geo.NewSize(20, 20), bound, "Direction must not collide with the default style")
	assert.Equal(t, float32(16), base)

	bound, base = GetFontMetrics("hi", 10, serbian, nil)
	assert.Equal(t, geo.NewSize(30, 30), bound, "Language must not collide with the default style")
	assert.Equal(t, float32(24), base)
}

// TestTextCacheKeyDecorations guards that the underline and strikethrough
// decoration bits are part of the cache key, so a toggled decoration is not
// served a stale texture from an otherwise-identical entry (the #70 lesson).
func TestTextCacheKeyDecorations(t *testing.T) {
	ResetThemeCaches()

	plain := bellui.TextStyle{}
	underline := bellui.TextStyle{Underline: true}
	strike := bellui.TextStyle{Strikethrough: true}

	SetFontMetrics("hi", 10, plain, nil, geo.NewSize(10, 10), 8)
	SetFontMetrics("hi", 10, underline, nil, geo.NewSize(11, 11), 9)
	SetFontMetrics("hi", 10, strike, nil, geo.NewSize(12, 12), 10)

	assert.Equal(t, 3, fontSizeCache.Len(), "underline and strikethrough must key distinctly")

	bound, _ := GetFontMetrics("hi", 10, strike, nil)
	assert.Equal(t, geo.NewSize(12, 12), bound, "Strikethrough must not collide with the default style")
}
