package widget

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/test"
)

// Narrow and wide texts of the same byte length: an in-place edit from one to
// the other changes no length the content checks see, only where lines break
// or truncate.
const (
	inplaceNarrow = "iiii iiii iiii iiii iiii iiii"
	inplaceWide   = "WWWW WWWW WWWW WWWW WWWW WWWW"
)

// drawnRows returns the text of each row a RichText's renderer draws.
func drawnRows(rt *RichText) []string {
	var rows []string
	for _, txt := range richTextRenderTexts(rt) {
		rows = append(rows, txt.Text)
	}
	return rows
}

func rowLens(rows []string) []int {
	lens := make([]int, len(rows))
	for i, r := range rows {
		lens[i] = len(r)
	}
	return lens
}

// A widget that edits its RichText segment in place to same-length text of other
// widths must lay it out as a fresh widget showing that text would (#1305).
func TestRichText_InPlaceSameLengthEdit_Relayouts(t *testing.T) {
	u, _ := url.Parse("https://example.com")
	for name, tc := range map[string]struct {
		build func(text string) core.CanvasObject // a widget showing text
		set   func(obj core.CanvasObject, text string)
		rich  func(obj core.CanvasObject) *RichText
	}{
		"Hyperlink.SetText": {
			build: func(text string) core.CanvasObject {
				hl := NewHyperlink(text, u)
				hl.Wrapping = bellui.TextWrapWord
				return hl
			},
			set:  func(obj core.CanvasObject, text string) { obj.(*Hyperlink).SetText(text) },
			rich: func(obj core.CanvasObject) *RichText { return &obj.(*Hyperlink).provider },
		},
		"Entry.SetPlaceHolder": {
			build: func(text string) core.CanvasObject {
				e := NewMultiLineEntry()
				e.Wrapping = bellui.TextWrapWord
				e.SetPlaceHolder(text)
				return e
			},
			set:  func(obj core.CanvasObject, text string) { obj.(*Entry).SetPlaceHolder(text) },
			rich: func(obj core.CanvasObject) *RichText { return obj.(*Entry).placeholderProvider() },
		},
		// The exported field, applied by Refresh through Entry.syncSegments rather
		// than SetPlaceHolder.
		"Entry.PlaceHolder field": {
			build: func(text string) core.CanvasObject {
				e := NewMultiLineEntry()
				e.Wrapping = bellui.TextWrapWord
				e.PlaceHolder = text
				return e
			},
			set: func(obj core.CanvasObject, text string) {
				e := obj.(*Entry)
				e.PlaceHolder = text
				e.Refresh()
			},
			rich: func(obj core.CanvasObject) *RichText { return obj.(*Entry).placeholderProvider() },
		},
		"Select.SetSelected": {
			build: func(text string) core.CanvasObject {
				s := NewSelect([]string{inplaceNarrow, inplaceWide}, nil)
				s.SetSelected(text)
				return s
			},
			set:  func(obj core.CanvasObject, text string) { obj.(*Select).SetSelected(text) },
			rich: func(obj core.CanvasObject) *RichText { return cache.Renderer(obj.(*Select)).(*selectRenderer).label },
		},
		"SearchSelect.SetSelected": {
			build: func(text string) core.CanvasObject {
				s := NewSearchSelect([]string{inplaceNarrow, inplaceWide}, nil)
				s.SetSelected(text)
				return s
			},
			set: func(obj core.CanvasObject, text string) { obj.(*SearchSelect).SetSelected(text) },
			rich: func(obj core.CanvasObject) *RichText {
				return cache.Renderer(obj.(*SearchSelect)).(*searchSelectRenderer).label
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			test.NewTempApp(t)
			show := func(obj core.CanvasObject) {
				w := test.NewTempWindow(t, obj)
				w.SetPadded(false)
				w.Resize(geo.NewSize(120, 300))
				obj.Refresh()
			}

			fresh := tc.build(inplaceWide)
			show(fresh)
			want := drawnRows(tc.rich(fresh))

			edited := tc.build(inplaceNarrow)
			show(edited)
			before := drawnRows(tc.rich(edited))
			tc.set(edited, inplaceWide)

			// A stale layout keeps the old row lengths, so they must differ or a
			// stale and a fresh layout would look the same.
			assert.NotEqual(t, rowLens(want), rowLens(before), "narrow and wide text break alike; this proves nothing")
			assert.Equal(t, want, drawnRows(tc.rich(edited)))
		})
	}
}

// Entry.syncSegments restyles the placeholder segment in place too, so a style
// that changes glyph widths must re-lay it out like a fresh placeholder (#1305).
func TestEntry_PlaceHolderRestyle_Relayouts(t *testing.T) {
	test.NewTempApp(t)
	build := func(style bellui.TextStyle) *Entry {
		e := NewMultiLineEntry()
		e.Wrapping = bellui.TextWrapWord
		e.TextStyle = style
		e.PlaceHolder = inplaceNarrow
		w := test.NewTempWindow(t, e)
		w.SetPadded(false)
		w.Resize(geo.NewSize(120, 300))
		e.Refresh()
		return e
	}
	bold := bellui.TextStyle{Bold: true}

	want := drawnRows(build(bold).placeholderProvider())

	edited := build(bellui.TextStyle{})
	before := drawnRows(edited.placeholderProvider())
	edited.TextStyle = bold
	edited.Refresh()

	assert.NotEqual(t, rowLens(want), rowLens(before), "regular and bold text break alike; this proves nothing")
	assert.Equal(t, want, drawnRows(edited.placeholderProvider()))
}
