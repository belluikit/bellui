package lang

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatList_English(t *testing.T) {
	setupLang("en")
	assert.Equal(t, "", FormatList(nil))
	assert.Equal(t, "", FormatList([]string{}))
	assert.Equal(t, "apples", FormatList([]string{"apples"}))
	assert.Equal(t, "apples and oranges", FormatList([]string{"apples", "oranges"}))
	// Oxford comma before the conjunction is locale data, present for en.
	assert.Equal(t, "apples, oranges, and pears", FormatList([]string{"apples", "oranges", "pears"}))
	assert.Equal(t, "a, b, c, and d", FormatList([]string{"a", "b", "c", "d"}))
}

func TestFormatList_German(t *testing.T) {
	setupLang("de")
	// German joins the last item with "und" and no Oxford comma.
	assert.Equal(t, "Äpfel und Orangen", FormatList([]string{"Äpfel", "Orangen"}))
	assert.Equal(t, "Äpfel, Orangen und Birnen", FormatList([]string{"Äpfel", "Orangen", "Birnen"}))
}

func TestFormatList_Locale(t *testing.T) {
	items := []string{"a", "b", "c"}

	setupLang("en")
	assert.Equal(t, "a, b, and c", FormatList(items))

	setupLang("de")
	assert.Equal(t, "a, b und c", FormatList(items))
}

// TestFormatList_LocaleCoverage locks in the CLDR list connectors for every
// bundled locale (#860) — the conjunction word and separator are per-locale
// data, so a regression here means the wrong language is rendered. Values are
// the CLDR "standard" list patterns; " "/" " are the non-breaking and
// narrow-no-break spaces CLDR mandates for cs/fr.
func TestFormatList_LocaleCoverage(t *testing.T) {
	for _, tt := range []struct {
		locale string
		want   string
	}{
		{"en", "a, b, and c"},
		{"de", "a, b und c"},
		{"cs", "a, b a c"},
		{"el", "a, b και c"},
		{"es", "a, b y c"},
		{"fr", "a, b et c"},
		{"ja", "a、b、c"},
		{"pl", "a, b i c"},
		{"pt", "a, b e c"},
		{"pt-BR", "a, b e c"},
		{"ru", "a, b и c"},
		{"sv", "a, b och c"},
		{"ta", "a, b மற்றும் c"},
		{"uk", "a, b і c"},
		{"zh-Hans", "a、b和c"},
	} {
		t.Run(tt.locale, func(t *testing.T) {
			setupLang(tt.locale)
			assert.Equal(t, tt.want, FormatList([]string{"a", "b", "c"}))
		})
	}
}

// TestFormatUnit_LocaleCoverage spot-checks locales whose unit labels are NOT
// the international abbreviation — Cyrillic км, Greek χλμ., Chinese 公里, Tamil
// கி.மீ., French narrow-NBSP (#860).
func TestFormatUnit_LocaleCoverage(t *testing.T) {
	for _, tt := range []struct {
		locale string
		want   string
	}{
		{"ru", "5 км"},
		{"uk", "5 км"},
		{"el", "5 χλμ."},
		{"zh-Hans", "5公里"},
		{"ta", "5 கி.மீ."},
		{"fr", "5 km"},
	} {
		t.Run(tt.locale, func(t *testing.T) {
			setupLang(tt.locale)
			assert.Equal(t, tt.want, FormatUnit(5, UnitKilometer))
		})
	}
}

func TestFormatList_ItemWithPlaceholderNotResubstituted(t *testing.T) {
	setupLang("en")
	// An item literally containing "{1}" must survive verbatim.
	assert.Equal(t, "{1} and z", FormatList([]string{"{1}", "z"}))
}
