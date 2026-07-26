package painter

import (
	"sort"
	"sync"
)

// FontFamilyInfo describes a discovered font family.
type FontFamilyInfo struct {
	Name           string   // Display name (e.g. "Arial", "Roboto")
	File           string   // Filesystem path to the regular font file
	BoldFile       string   // Filesystem path to the bold variant (may be empty)
	ItalicFile     string   // Filesystem path to the italic variant (may be empty)
	BoldItalicFile string   // Filesystem path to the bold-italic variant (may be empty)
	Styles         []string // Available style names (e.g. "Regular", "Bold", "Italic")
}

var (
	systemFamilies     []FontFamilyInfo
	systemFamiliesOnce sync.Once
)

// SystemFontFamilies returns all font families available on the system, sorted alphabetically.
// Results are cached after the first call. Safe for concurrent use.
func SystemFontFamilies() []FontFamilyInfo {
	systemFamiliesOnce.Do(func() {
		systemFamilies = scanSystemFontFamilies()
		sort.Slice(systemFamilies, func(i, j int) bool {
			return systemFamilies[i].Name < systemFamilies[j].Name
		})
	})
	return systemFamilies
}
