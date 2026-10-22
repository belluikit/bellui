package lang

import (
	"errors"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
)

func readCatalog(t *testing.T, src string) (map[string]any, error) {
	t.Helper()
	var v any
	err := unmarshalCatalog([]byte(src), &v)
	if err != nil {
		return nil, err
	}
	return v.(map[string]any), nil
}

func TestCatalog_Read(t *testing.T) {
	m, err := readCatalog(t, `
# A comment.
messages = {
  OK: "OK"
  "Create Folder": "Ordner erstellen"
  "file.count": {
    one = "{Count} Datei"
    other = "{Count} Dateien"
    description = "Files in the folder"
  }
  profile: {
    female: "{Name} hat ihr Profil aktualisiert"
  }
}
`)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"OK":            "OK",
		"Create Folder": "Ordner erstellen",
		"file.count": map[string]any{
			"one": "{Count} Datei", "other": "{Count} Dateien", "description": "Files in the folder",
		},
		"profile": map[string]any{"female": "{Name} hat ihr Profil aktualisiert"},
	}, m)

	m, err = readCatalog(t, "messages = {\n  plural {\n    one = \"a\"\n    other = \"b\"\n  }\n}\n")
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"plural": map[string]any{"one": "a", "other": "b"}}, m, "the name { … } block form")

	m, err = readCatalog(t, "")
	require.NoError(t, err)
	assert.Empty(t, m)
}

func TestCatalog_ReadErrors(t *testing.T) {
	for _, src := range []string{
		`messages = {`,
		`other = { OK: "OK" }`,
		`messages = "OK"`,
		`messages = { OK: 1 }`,
		`messages = { OK: "a" OK: "b" }`,
		"messages = { OK: \"a\" }\nmessages = { No: \"b\" }",
	} {
		_, err := readCatalog(t, src)
		assert.Error(t, err, src)
	}
}

// A namespace block reads as the "."-joined IDs a flat catalog has.
func TestCatalog_Namespace(t *testing.T) {
	SetLocaleForTests(t, "en")
	require.NoError(t, AddTranslationsForLocale([]byte(`messages = {
		catalog_ns: { greeting: "Hi {Name}" }
	}`), "en"))
	assert.Equal(t, "Hi Ada", X("catalog_ns.greeting", "", map[string]any{"Name": "Ada"}))
}

// JSON catalogs are not read since #522.
func TestCatalog_JSONRejected(t *testing.T) {
	err := AddTranslations(bellui.NewStaticResource("en.json", []byte(`{"OK": "OK"}`)))
	assert.True(t, errors.Is(err, errJSONCatalog), "%v", err)
	assert.Error(t, AddTranslations(bellui.NewStaticResource("en.yaml", []byte(`OK: OK`))))
}

var catalogPlaceholder = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)\}`)

func placeholderNames(s string) string {
	var names []string
	for _, m := range catalogPlaceholder.FindAllStringSubmatch(s, -1) {
		names = append(names, m[1])
	}
	sort.Strings(names)
	return strings.Join(names, ",")
}

// TestCatalog_PlaceholdersMatchSource: every bundled translation uses the
// placeholders of the English source message, in every plural form; and none
// uses the Go template form (#522).
func TestCatalog_PlaceholdersMatchSource(t *testing.T) {
	read := func(name string) map[string]any {
		data, err := fs.ReadFile(translations, path.Join("translations", name))
		require.NoError(t, err)
		m, err := readCatalog(t, string(data))
		require.NoError(t, err, name)
		return m
	}
	forms := func(v any) map[string]string {
		switch v := v.(type) {
		case string:
			return map[string]string{"other": v}
		case map[string]any:
			out := map[string]string{}
			for k, f := range v {
				if k != "description" && k != "hash" {
					out[k] = f.(string)
				}
			}
			return out
		}
		return nil
	}

	source := read("base.en.pxf")
	entries, err := fs.ReadDir(translations, "translations")
	require.NoError(t, err)
	checked := 0
	for _, e := range entries {
		for id, v := range read(e.Name()) {
			src, ok := source[id]
			if !ok {
				t.Errorf("%s: %q is not in base.en.pxf", e.Name(), id)
				continue
			}
			want := placeholderNames(forms(src)["other"])
			for form, text := range forms(v) {
				assert.NotContains(t, text, "{{", "%s %q %s", e.Name(), id, form)
				assert.Equal(t, want, placeholderNames(text), "%s %q %s: %q", e.Name(), id, form, text)
				checked++
			}
		}
	}
	assert.Greater(t, checked, 1000, "the bundled catalogs were read")
	t.Logf("checked %d messages in %d catalogs", checked, len(entries))
}
