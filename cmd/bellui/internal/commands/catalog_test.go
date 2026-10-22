package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCatalog_FormatAndRead(t *testing.T) {
	m := map[string]any{
		"OK":            "OK",
		"Create Folder": "Ordner erstellen",
		"quote":         `say "hi" \ back`,
		"multi":         "line one\nline two",
		"unicode":       "Grüße 👋 日本",
		"file.count":    map[string]string{"one": "{Count} Datei", "other": "{Count} Dateien"},
		"described":     map[string]any{"other": "x", "description": "for translators"},
		"profile":       map[string]any{"female": "{Name} hat ihr Profil aktualisiert"},
	}
	out := formatCatalog(m)
	assert.Equal(t, `messages = {
  "Create Folder": "Ordner erstellen"
  OK: "OK"
  described: {
    description = "for translators"
    other = "x"
  }
  "file.count": {
    one = "{Count} Datei"
    other = "{Count} Dateien"
  }
  multi: "line one\nline two"
  profile: {
    female: "{Name} hat ihr Profil aktualisiert"
  }
  quote: "say \"hi\" \\ back"
  unicode: "Grüße 👋 日本"
}
`, string(out))

	back, err := readCatalog(out)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"OK":            "OK",
		"Create Folder": "Ordner erstellen",
		"quote":         `say "hi" \ back`,
		"multi":         "line one\nline two",
		"unicode":       "Grüße 👋 日本",
		"file.count":    map[string]any{"one": "{Count} Datei", "other": "{Count} Dateien"},
		"described":     map[string]any{"other": "x", "description": "for translators"},
		"profile":       map[string]any{"female": "{Name} hat ihr Profil aktualisiert"},
	}, back)
}

// A message block may be written without "=", as "name { … }"; the toolkit
// reads that form too.
func TestCatalog_ReadBlockForm(t *testing.T) {
	m, err := readCatalog([]byte("messages = {\n  plural {\n    one = \"a\"\n    other = \"b\"\n  }\n}\n"))
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"plural": map[string]any{"one": "a", "other": "b"}}, m)
}

func TestCatalog_ReadErrors(t *testing.T) {
	for _, src := range []string{
		`messages = {`,
		`other = { OK: "OK" }`,
		`messages = "OK"`,
		`messages = { OK: 1 }`,
		`messages = { OK: "a" OK: "b" }`,
	} {
		_, err := readCatalog([]byte(src))
		assert.Error(t, err, src)
	}
}

func TestNamedPlaceholders(t *testing.T) {
	assert.Equal(t, "{Count} files", namedPlaceholders("{{.Count}} files"))
	assert.Equal(t, "{Name} and {Count}", namedPlaceholders("{{ .Name }} and {{.Count}}"))
	assert.Equal(t, "{{if .X}}x{{end}}", namedPlaceholders("{{if .X}}x{{end}}"))
	assert.Equal(t, "{Name}", namedPlaceholders("{Name}"))
	assert.Equal(t, map[string]bool{"Count": true, "Name": true}, placeholderSet("{{.Count}} by {Name}"))
}

// Extracting into an existing PXF catalog adds the new messages, keeps the
// existing ones as they are (comments aside), and writes named placeholders
// for a source string in the Go template form (#522).
func TestUpdateTranslationsFile_ExistingPXF(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "main.go")
	require.NoError(t, os.WriteFile(src, []byte(`package main

func main() {
	lang.L("Kept")
	lang.L("Hello {{.Name}}")
	lang.X("files", "{{.Count}} files")
	lang.N("{Count} apples", 2)
}
`), 0o600))
	dst := filepath.Join(dir, "de.pxf")
	require.NoError(t, os.WriteFile(dst, []byte(`messages = {
  Kept: "Behalten"
  "old.plural": {
    one = "{Count} alt"
    other = "{Count} alte"
  }
}
`), 0o600))

	require.NoError(t, updateTranslationsFile(dst, []string{src}, &translateOpts{}))

	data, err := os.ReadFile(dst)
	require.NoError(t, err)
	got, err := readCatalog(data)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"Kept":            "Behalten",
		"old.plural":      map[string]any{"one": "{Count} alt", "other": "{Count} alte"},
		"Hello {{.Name}}": "Hello {Name}",
		"files":           "{Count} files",
		"{Count} apples":  map[string]any{"other": "{Count} apples"},
	}, got)
}

func TestUpdateTranslationsFile_BadCatalog(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "main.go")
	require.NoError(t, os.WriteFile(src, []byte("package main\n\nfunc main() { lang.L(\"A\") }\n"), 0o600))
	dst := filepath.Join(dir, "en.pxf")
	require.NoError(t, os.WriteFile(dst, []byte(`{"A": "A"}`), 0o600))

	assert.Error(t, updateTranslationsFile(dst, []string{src}, &translateOpts{}), "a JSON body in a .pxf file is not read")
	data, _ := os.ReadFile(dst)
	assert.Equal(t, `{"A": "A"}`, string(data), "the file is left alone")
}
