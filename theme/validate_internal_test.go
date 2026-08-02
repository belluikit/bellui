package theme

import "testing"

// Namespaced names are extension-owned (x/editor's "editor.syntax.keyword",
// x/tables' "fytables:rowSelected"): a theme delegating them to the built-in
// palette is the supported fallback pattern (#150), so the typo warning must
// not fire for them — it would log once per extension name per process.
func TestNamespacedThemeName(t *testing.T) {
	for name, want := range map[string]bool{
		"editor.syntax.keyword": true,
		"fytables:rowSelected":  true,
		"forground":             false, // a real typo keeps warning
		"primary":               false,
	} {
		if got := namespacedThemeName(name); got != want {
			t.Errorf("namespacedThemeName(%q) = %v, want %v", name, got, want)
		}
	}
}
