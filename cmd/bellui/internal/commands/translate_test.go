package commands

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

const exampleSource = `
package main

func main() {
	lang.X("example", "Example")
	lang.X("multipass", "One" +
		" " +
		"Two" +
		" " +
		"Three")
	lang.X("multiline", ` + "`" + `first
second
third` + "`" + `)
	lang.X("mixer", "prefix\n"+` + "`" + `first
second
third` + "`" + `+"\npostfix")
}
`

func createTestTranslateFiles(t *testing.T, dir, file string) {
	src := filepath.Join(dir, file)

	f, err := os.Create(src)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte(exampleSource)); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestTranslateCommand(t *testing.T) {
	cmd := Translate()
	if cmd.Name != "translate" {
		t.Fatalf("invalid command name: %v", cmd.Name)
	}
}

func TestFindSourcesDefault(t *testing.T) {
	dir := t.TempDir()
	createTestTranslateFiles(t, dir, "foo.go")

	sources, err := findSources(nil, ".go", dir)
	if err != nil {
		t.Fatalf("failed to find sources: %v", err)
	}
	if len(sources) != 1 {
		t.Errorf("wrong number of sources: %v", len(sources))
	}
}

func TestFindSourcesNone(t *testing.T) {
	dir := t.TempDir()
	createTestTranslateFiles(t, dir, "foo.go")

	sources, err := findSources(nil, ".go", filepath.Join(dir, "nonexistent"))
	if err != nil {
		t.Fatalf("failed to find sources: %v", err)
	}
	if len(sources) != 0 {
		t.Errorf("wrong number of sources: %v", len(sources))
	}
}

func TestUpdateTranslationsFile(t *testing.T) {
	src := "foo.go"
	dir := t.TempDir()
	createTestTranslateFiles(t, dir, src)
	srcpath := filepath.Join(dir, src)

	opts := translateOpts{}
	dst := filepath.Join(dir, "en.json")

	if err := updateTranslationsFile(dst, []string{srcpath}, &opts); err != nil {
		t.Fatal(err)
	}

	if err := updateTranslationsFile(dst, []string{srcpath}, &opts); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(dst)
	if err != nil {
		t.Fatal(err)
	}

	translations := make(map[string]any)
	dec := json.NewDecoder(f)
	if err := dec.Decode(&translations); err != nil {
		t.Fatal(err)
	}
	f.Close()

	key := "example"
	val, found := translations[key]
	if !found {
		t.Errorf("failed to find key: %v", key)
	}
	if val != "Example" {
		t.Errorf("invalid value for key: %v: %v", key, val)
	}
}

func TestTranslateFindFilesExt(t *testing.T) {
	src := "foo.go"
	dir := t.TempDir()
	createTestTranslateFiles(t, dir, src)
	files, err := findFilesExt(dir, ".go")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Errorf("invalid number of files: %v", len(files))
	}
	if len(files) == 1 && filepath.Base(files[0]) != src {
		t.Errorf("invalid file: %v", filepath.Base(files[0]))
	}
}

func TestWriteTranslationsFile(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "foo.json")

	if err := writeTranslationsFile([]byte(`{"a":1}`), dst); err != nil {
		t.Fatalf("failed to write translations file: %v", err)
	}

	if err := writeTranslationsFile([]byte(`{"a":2}`), dst); err != nil {
		t.Fatalf("failed to write translations file: %v", err)
	}

	f, err := os.Open(dst)
	if assert.NoError(t, err) {
		defer f.Close()
		b, err := io.ReadAll(f)
		assert.NoError(t, err)
		if assert.GreaterOrEqual(t, len(b), 1) {
			assert.Equal(t, byte('\n'), b[len(b)-1])
		}
	}
}

func TestUpdateTranslationsHash(t *testing.T) {
	src := "foo.go"
	dir := t.TempDir()
	createTestTranslateFiles(t, dir, src)
	srcpath := filepath.Join(dir, src)

	opts := translateOpts{}
	translations := make(map[string]any)
	if err := updateTranslationsHash(translations, []string{srcpath}, &opts); err != nil {
		t.Fatal(err)
	}

	key := "example"
	val, found := translations[key]
	if !found {
		t.Errorf("failed to find key: %v", key)
	}
	if val != "Example" {
		t.Errorf("invalid value for key: %v: %v", key, val)
	}
}

func TestTranslationsVisitor(t *testing.T) {
	src := "foo.go"
	dir := t.TempDir()
	createTestTranslateFiles(t, dir, src)

	fset := token.NewFileSet()
	af, err := parser.ParseFile(fset, filepath.Join(dir, src), nil, parser.AllErrors)
	if err != nil {
		t.Fatalf("failed to parse source: %v", err)
	}

	opts := translateOpts{}
	translations := make(map[string]any)
	ast.Walk(&visitor{opts: &opts, m: translations}, af)

	for _, test := range []struct {
		key  string
		want string
	}{
		{"example", "Example"},
		{"multipass", "One Two Three"},
		{"multiline", "first\nsecond\nthird"},
		{"mixer", "prefix\nfirst\nsecond\nthird\npostfix"},
	} {
		val, found := translations[test.key]
		if !found {
			t.Errorf("failed to find key: %v", test.key)
		}
		if val != test.want {
			t.Errorf("invalid value for key: %v: %v", test.key, val)
		}
	}
}

// forkSeamSource exercises the fork's live-retranslation entry points (pre-rebrand tracker #503):
// lang.BindText plus the binding.NewLocalized* constructors, which the upstream
// scanner silently missed.
const forkSeamSource = `
package main

func main() {
	lang.BindText("Cancel", btn.SetText)
	binding.NewLocalizedString("Save")
	binding.NewLocalizedStringWithKey("app.title", "My Application")
	binding.NewLocalizedPlural("Apples", count)
	binding.NewLocalizedPluralWithKey("basket.count", "{{.Count}} apple(s)", count)
}
`

func TestTranslateForkSeams(t *testing.T) {
	src := "seams.go"
	dir := t.TempDir()
	path := filepath.Join(dir, src)
	if err := os.WriteFile(path, []byte(forkSeamSource), 0o644); err != nil {
		t.Fatal(err)
	}

	fset := token.NewFileSet()
	af, err := parser.ParseFile(fset, path, nil, parser.AllErrors)
	if err != nil {
		t.Fatalf("failed to parse source: %v", err)
	}

	translations := make(map[string]any)
	ast.Walk(&visitor{opts: &translateOpts{}, m: translations}, af)

	// Singular seams: key-only forms fall back to the key, WithKey carries an
	// explicit fallback.
	for _, test := range []struct {
		key  string
		want string
	}{
		{"Cancel", "Cancel"},            // lang.BindText — key doubles as fallback
		{"Save", "Save"},                // NewLocalizedString — key-only
		{"app.title", "My Application"}, // NewLocalizedStringWithKey — explicit fallback
	} {
		val, found := translations[test.key]
		if !found {
			t.Errorf("failed to find key: %v", test.key)
			continue
		}
		if val != test.want {
			t.Errorf("invalid value for key %q: got %v, want %v", test.key, val, test.want)
		}
	}

	// Plural seams must land as a {"other": fallback} object, like N/XN.
	for _, test := range []struct {
		key  string
		want string
	}{
		{"Apples", "Apples"},                    // NewLocalizedPlural — key-only
		{"basket.count", "{{.Count}} apple(s)"}, // NewLocalizedPluralWithKey — explicit fallback
	} {
		val, found := translations[test.key]
		if !found {
			t.Errorf("failed to find plural key: %v", test.key)
			continue
		}
		m, ok := val.(map[string]string)
		if !ok {
			t.Errorf("plural key %q is not a plural object: %T", test.key, val)
			continue
		}
		if m["other"] != test.want {
			t.Errorf("invalid plural value for key %q: got %v, want %v", test.key, m["other"], test.want)
		}
	}
}

// selectSeamSource exercises the grammatical-select seams (pre-rebrand tracker #505). The selector
// is a runtime variable in the common case, and a string literal in the second.
const selectSeamSource = `
package main

func main() {
	lang.LSelect("profile_updated", gender, data)
	lang.LSelect("greeting", "formal", data)
	lang.LSelectPlural("items_shared", gender, count, data)
}
`

func TestTranslateSelectSeams(t *testing.T) {
	src := "selectseams.go"
	dir := t.TempDir()
	path := filepath.Join(dir, src)
	if err := os.WriteFile(path, []byte(selectSeamSource), 0o644); err != nil {
		t.Fatal(err)
	}

	fset := token.NewFileSet()
	af, err := parser.ParseFile(fset, path, nil, parser.AllErrors)
	if err != nil {
		t.Fatalf("failed to parse source: %v", err)
	}

	translations := make(map[string]any)
	ast.Walk(&visitor{opts: &translateOpts{}, m: translations}, af)

	// A variable selector seeds only the mandatory ".other" variant.
	if val, found := translations["profile_updated.other"]; !found {
		t.Errorf("failed to seed profile_updated.other")
	} else if val != "profile_updated.other" {
		t.Errorf("unexpected fallback for profile_updated.other: %v", val)
	}

	// The bare prefix must never be emitted (nothing to clobber).
	if _, found := translations["profile_updated"]; found {
		t.Errorf("bare prefix profile_updated was emitted")
	}

	// A literal selector additionally seeds that explicit variant.
	if _, found := translations["greeting.formal"]; !found {
		t.Errorf("failed to seed literal-selector variant greeting.formal")
	}
	if _, found := translations["greeting.other"]; !found {
		t.Errorf("failed to seed greeting.other")
	}

	// LSelectPlural seeds a plural {"other": …} object, like N/XN.
	val, found := translations["items_shared.other"]
	if !found {
		t.Fatalf("failed to seed items_shared.other")
	}
	m, ok := val.(map[string]string)
	if !ok {
		t.Fatalf("items_shared.other is not a plural object: %T", val)
	}
	if m["other"] != "items_shared.other" {
		t.Errorf("unexpected plural fallback: %v", m["other"])
	}
}

func TestTranslateVisitorNew(t *testing.T) {
	if translateNew(&visitor{}, ast.NewIdent("whatever")) != nil {
		t.Fatalf("failed to get the correct state: nil")
	}

	if translateNew(&visitor{}, ast.NewIdent("lang")) == nil {
		t.Fatalf("failed to get the correct state")
	}

	if translateNew(&visitor{}, ast.NewIdent("binding")) == nil {
		t.Fatalf("failed to get the correct state for binding receiver")
	}
}

func TestTranslateVisitorCall(t *testing.T) {
	if translateCall(&visitor{}, &ast.IfStmt{}) != nil {
		t.Fatalf("failed to get the correct state: nil")
	}

	if translateCall(&visitor{}, ast.NewIdent("X")) == nil {
		t.Fatalf("failed to get the correct state")
	}
}

func TestTranslateVisitorLocalize(t *testing.T) {
	translations := make(map[string]any)
	v := &visitor{
		opts: &translateOpts{},
		m:    translations,
	}
	if translateLocalize(v, &ast.IfStmt{}) != nil {
		t.Fatalf("failed to get the correct state: nil")
	}

	key := "yay"
	if translateLocalize(v, &ast.BasicLit{Value: fmt.Sprintf("%q", key)}) != nil {
		t.Fatalf("failed to get the correct state")
	}

	if v.key != key {
		t.Errorf("invalid key: %q", v.key)
	}
}

func TestTranslateVisitorKey(t *testing.T) {
	translations := make(map[string]any)
	v := &visitor{
		opts: &translateOpts{},
		m:    translations,
	}
	if translateLocalize(v, &ast.IfStmt{}) != nil {
		t.Fatalf("failed to get the correct state: nil")
	}

	key := "whee"
	if translateKey(v, &ast.BasicLit{Value: fmt.Sprintf("%q", key)}) == nil {
		t.Fatalf("failed to get the correct state")
	}

	if v.key != key {
		t.Errorf("invalid key: %q", v.key)
	}
}

func TestTranslateVisitorFallback(t *testing.T) {
	translations := make(map[string]any)
	v := &visitor{
		opts: &translateOpts{},
		m:    translations,
	}
	if translateLocalize(v, &ast.IfStmt{}) != nil {
		t.Fatalf("failed to get the correct state: nil")
	}

	fallback := "WHEE"
	if translateLocalize(v, &ast.BasicLit{Value: fmt.Sprintf("%q", fallback)}) != nil {
		t.Fatalf("failed to get the correct state")
	}

	if v.fallback != fallback {
		t.Errorf("invalid fallback: %q", v.fallback)
	}
}
