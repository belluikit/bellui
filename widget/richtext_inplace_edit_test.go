package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/test"
)

// Two texts of the same byte length whose glyph widths differ a lot, so a
// same-length in-place edit from narrow to wide must re-wrap (#1305).
const (
	inplaceNarrow = "iiii iiii iiii iiii iiii iiii"
	inplaceWide   = "WWWW WWWW WWWW WWWW WWWW WWWW"
)

// rowsOf settles the provider's model and returns how many rows it holds.
func rowsOf(rt *RichText) int {
	rt.updateRowBounds()
	return rt.rows()
}

// A bellui widget that edits its RichText segment in place to text of the same
// byte length must wrap the new text as a fresh widget would: the content
// fingerprint sees only count, identity and length, so the edit site has to
// invalidate the row model itself, as Label.syncSegments does (#1305).
func TestHyperlink_SetText_SameLength_Rewraps(t *testing.T) {
	test.NewTempApp(t)

	fresh := NewHyperlink(inplaceWide, nil)
	fresh.Wrapping = bellui.TextWrapWord
	test.NewTempWindow(t, fresh).Resize(geo.NewSize(120, 300))
	fresh.Resize(geo.NewSize(120, 300))
	want := rowsOf(&fresh.provider)
	require.Greater(t, want, 2, "control: the wide text must wrap onto more rows")

	hl := NewHyperlink(inplaceNarrow, nil)
	hl.Wrapping = bellui.TextWrapWord
	test.NewTempWindow(t, hl).Resize(geo.NewSize(120, 300))
	hl.Resize(geo.NewSize(120, 300))
	hl.Refresh()
	before := rowsOf(&hl.provider)
	require.Less(t, before, want, "control: the narrow text must wrap onto fewer rows")

	hl.SetText(inplaceWide)
	assert.Equal(t, want, rowsOf(&hl.provider))
	assert.Len(t, richTextRenderTexts(&hl.provider), want)
}

func TestEntry_SetPlaceHolder_SameLength_Rewraps(t *testing.T) {
	test.NewTempApp(t)

	build := func(placeholder string) *Entry {
		e := NewMultiLineEntry()
		e.Wrapping = bellui.TextWrapWord
		e.SetPlaceHolder(placeholder)
		test.NewTempWindow(t, e).Resize(geo.NewSize(120, 300))
		e.Resize(geo.NewSize(120, 300))
		e.Refresh()
		return e
	}
	want := rowsOf(build(inplaceWide).placeholderProvider())
	require.Greater(t, want, 2, "control: the wide placeholder must wrap onto more rows")

	e := build(inplaceNarrow)
	require.Less(t, rowsOf(e.placeholderProvider()), want)

	e.SetPlaceHolder(inplaceWide)
	assert.Equal(t, want, rowsOf(e.placeholderProvider()))
}

// Select and SearchSelect truncate their label with an ellipsis, and a stale
// row model keeps the old cut point: a same-length wider selection then
// overflows instead of being cut where a fresh widget would cut it.
func TestSelect_SameLengthSelection_Retruncates(t *testing.T) {
	test.NewTempApp(t)

	build := func(selected string) *Select {
		s := NewSelect([]string{inplaceNarrow, inplaceWide}, nil)
		s.SetSelected(selected)
		test.NewTempWindow(t, s).Resize(geo.NewSize(160, 60))
		s.Resize(geo.NewSize(160, 60))
		s.Refresh()
		return s
	}
	label := func(s *Select) *RichText { return cache.Renderer(s).(*selectRenderer).label }

	fresh := build(inplaceWide)
	want := texts(label(fresh))

	s := build(inplaceNarrow)
	require.NotEqual(t, want, texts(label(s)), "control: narrow and wide must be cut differently")
	s.SetSelected(inplaceWide)
	assert.Equal(t, want, texts(label(s)))
}

func TestSearchSelect_SameLengthSelection_Retruncates(t *testing.T) {
	test.NewTempApp(t)

	build := func(selected string) *SearchSelect {
		s := NewSearchSelect([]string{inplaceNarrow, inplaceWide}, nil)
		s.SetSelected(selected)
		test.NewTempWindow(t, s).Resize(geo.NewSize(160, 60))
		s.Resize(geo.NewSize(160, 60))
		s.Refresh()
		return s
	}

	fresh := build(inplaceWide)
	want := texts(cache.Renderer(fresh).(*searchSelectRenderer).label)

	s := build(inplaceNarrow)
	require.NotEqual(t, want, texts(cache.Renderer(s).(*searchSelectRenderer).label), "control: narrow and wide must be cut differently")
	s.SetSelected(inplaceWide)
	assert.Equal(t, want, texts(cache.Renderer(s).(*searchSelectRenderer).label))
}

// texts returns what the RichText's renderer draws, one string per row.
func texts(rt *RichText) []string {
	var out []string
	for _, txt := range richTextRenderTexts(rt) {
		out = append(out, txt.Text)
	}
	return out
}
