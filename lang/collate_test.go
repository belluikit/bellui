package lang

import (
	"sort"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompare_LocaleAware(t *testing.T) {
	// German: "ä" collates with "a", so it sorts before "z".
	SetLocaleForTests(t, "de")
	assert.Negative(t, Compare("äpfel", "zebra"), "de: ä should sort before z")
	assert.Negative(t, Compare("a", "ä"))
	assert.Negative(t, Compare("ä", "b"))
	assert.Zero(t, Compare("gleich", "gleich"))
}

func TestCompare_SwedishOrdersAccentsLast(t *testing.T) {
	// Swedish: å/ä/ö sort after z, unlike German.
	SetLocaleForTests(t, "sv")
	assert.Positive(t, Compare("öl", "zebra"), "sv: ö should sort after z")
	assert.Negative(t, Compare("z", "ä"), "sv: z sorts before ä")
}

func TestCompare_LocaleSwitchInvalidatesCollator(t *testing.T) {
	// The same pooled collator must be discarded and rebuilt when the active
	// locale changes: identical input, different order per locale.
	words := []string{"z", "ä"}

	sortWith := func(locale string) []string {
		SetLocaleForTests(t, locale)
		out := append([]string(nil), words...)
		sort.Slice(out, func(i, j int) bool { return Compare(out[i], out[j]) < 0 })
		return out
	}

	assert.Equal(t, []string{"ä", "z"}, sortWith("de"), "de: ä before z")
	assert.Equal(t, []string{"z", "ä"}, sortWith("sv"), "sv: z before ä")
}

func TestNewCollator_Numeric(t *testing.T) {
	SetLocaleForTests(t, "en")

	plain := NewCollator()
	assert.Positive(t, plain.Compare("file2", "file10"), "lexical: '2' > '1'")

	numeric := NewCollator(CollateNumeric)
	assert.Negative(t, numeric.Compare("file2", "file10"), "numeric: 2 < 10")

	files := []string{"file10", "file2", "file1"}
	numeric.SortStrings(files)
	assert.Equal(t, []string{"file1", "file2", "file10"}, files)
}

func TestCompareFold_CaseAndAccentInsensitive(t *testing.T) {
	SetLocaleForTests(t, "en")
	assert.Zero(t, CompareFold("Apple", "apple"), "case-insensitive")
	assert.Zero(t, CompareFold("café", "cafe"), "accent-insensitive")
	assert.Zero(t, CompareFold("RÉSUMÉ", "resume"))
	assert.NotZero(t, Compare("Apple", "apple"), "plain Compare stays case-sensitive")
}

func TestCompare_ConcurrentSafe(t *testing.T) {
	SetLocaleForTests(t, "de")

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				_ = Compare("äpfel", "zebra")
				_ = CompareFold("Apple", "apple")
			}
		}()
	}
	wg.Wait()
}
