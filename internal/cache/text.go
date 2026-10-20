package cache

import (
	"image/color"
	"sync"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

var fontSizeCache fontMetricMap

// fontMetricMap is a map guarded by a read-write lock rather than an
// async.Map. A sync.Map hashes its key through an interface, which for this
// struct of strings and floats walks every field generically, and was most of
// the lookup a label pays on every text change; a typed map uses the hash
// function the compiler generates for the key type. A lookup took 121 ns
// before and 67 ns after (#492).
type fontMetricMap struct {
	mu sync.RWMutex
	m  map[fontSizeEntry]*fontMetric
}

func (f *fontMetricMap) Load(k fontSizeEntry) (*fontMetric, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	v, ok := f.m[k]
	return v, ok
}

func (f *fontMetricMap) Store(k fontSizeEntry, v *fontMetric) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.m == nil {
		f.m = make(map[fontSizeEntry]*fontMetric)
	}
	f.m[k] = v
}

func (f *fontMetricMap) Len() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.m)
}

func (f *fontMetricMap) Clear() {
	f.mu.Lock()
	defer f.mu.Unlock()
	clear(f.m)
}

// deleteExpired removes the entries that expired before now.
func (f *fontMetricMap) deleteExpired(now time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for k, v := range f.m {
		if v.isExpired(now) {
			delete(f.m, k)
		}
	}
}

type fontMetric struct {
	expiringCache
	size     geo.Size
	baseLine float32
}

// fontSizeEntry is a comparable cache key for text measurement results.
// TextStyle fields are flattened to resolved values so pointer fields
// do not break map-key equality.
type fontSizeEntry struct {
	text            string
	size            float32
	source          string
	bold            bool
	italic          bool
	monospace       bool
	symbol          bool
	tabWidth        int
	underline       bool
	strikethrough   bool
	weight          bellui.TextWeight
	lineHeightScale float32
	letterSpacing   float32
	language        string
	direction       bellui.TextDirection
}

func newFontSizeEntry(text string, fontSize float32, style bellui.TextStyle, source string) fontSizeEntry {
	return fontSizeEntry{
		text: text, size: fontSize, source: source,
		bold: style.Bold, italic: style.Italic, monospace: style.Monospace,
		symbol: style.Symbol, tabWidth: style.TabWidth, underline: style.Underline,
		strikethrough:   style.Strikethrough,
		weight:          style.ResolvedWeight(),
		lineHeightScale: style.ResolvedLineHeightScale(),
		letterSpacing:   style.ResolvedLetterSpacing(),
		language:        style.Language,
		direction:       style.Direction,
	}
}

// FontCacheEntry is the cache key for rendered text textures.
type FontCacheEntry struct {
	fontSizeEntry

	Canvas core.Canvas
	Color  color.Color
}

// NewFontCacheEntry builds a FontCacheEntry from a text object's properties.
func NewFontCacheEntry(text string, fontSize float32, style bellui.TextStyle, source string, canvas core.Canvas, col color.Color) FontCacheEntry {
	return FontCacheEntry{
		fontSizeEntry: newFontSizeEntry(text, fontSize, style, source),
		Canvas:        canvas,
		Color:         col,
	}
}

// GetFontMetrics looks up a calculated size and baseline required for the specified text parameters.
func GetFontMetrics(text string, fontSize float32, style bellui.TextStyle, source bellui.Resource) (size geo.Size, base float32) {
	name := ""
	if source != nil {
		name = source.Name()
	}
	ent := newFontSizeEntry(text, fontSize, style, name)
	ret, ok := fontSizeCache.Load(ent)
	if !ok {
		return geo.Size{Width: 0, Height: 0}, 0
	}
	ret.setAlive()
	return ret.size, ret.baseLine
}

// SetFontMetrics stores a calculated font size and baseline for parameters that were missing from the cache.
func SetFontMetrics(text string, fontSize float32, style bellui.TextStyle, source bellui.Resource, size geo.Size, base float32) {
	name := ""
	if source != nil {
		name = source.Name()
	}
	ent := newFontSizeEntry(text, fontSize, style, name)
	metric := &fontMetric{size: size, baseLine: base}
	metric.setAlive()
	fontSizeCache.Store(ent, metric)
}

// destroyExpiredFontMetrics destroys expired fontSizeCache entries
func destroyExpiredFontMetrics(now time.Time) {
	fontSizeCache.deleteExpired(now)
}
