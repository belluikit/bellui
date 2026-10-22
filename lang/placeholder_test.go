package lang

import (
	"bytes"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func fill(src string, data any) string {
	s, _ := parsePlaceholders(src).Execute(data)
	return s
}

func TestPlaceholders(t *testing.T) {
	type person struct {
		Name    string
		Age     int
		private string
	}
	m := map[string]any{"Name": "Ada", "Count": 3, "Value": 2.5}

	for _, tt := range []struct {
		name, src string
		data      any
		want      string
	}{
		{"named", "Hello {Name}", m, "Hello Ada"},
		{"several", "{Name} has {Count} items, {Value} each", m, "Ada has 3 items, 2.5 each"},
		{"Go template field", "Hello {{.Name}}", m, "Hello Ada"},
		{"Go template field with spaces", "Hello {{ .Name }}", m, "Hello Ada"},
		{"both forms", "{Name} / {{.Name}}", m, "Ada / Ada"},
		{"struct", "{Name} is {Age}", person{Name: "Ada", Age: 36}, "Ada is 36"},
		{"struct pointer", "{Name}", &person{Name: "Ada"}, "Ada"},
		{"unexported field", "{private}", person{private: "x"}, "{private}"},
		{"map[string]string", "{Name}", map[string]string{"Name": "Ada"}, "Ada"},
		{"map[string]int", "{Count}", map[string]int{"Count": 4}, "4"},
		{"named map type", "{K}", map[string]float64{"K": 1.5}, "1.5"},
		{"plural count", "{PluralCount} files", map[string]any{"PluralCount": 2}, "2 files"},
		{"unknown name", "Hello {Who}", m, "Hello {Who}"},
		{"unknown Go template field", "Hello {{.Who}}", m, "Hello {{.Who}}"},
		{"nil data", "Hello {Name}", nil, "Hello {Name}"},
		{"no placeholders", "Plain text", m, "Plain text"},
		{"empty", "", m, ""},
		{"not a name", "{} { Name } {1x} {a-b}", m, "{} { Name } {1x} {a-b}"},
		{"unclosed", "Hello {Name", m, "Hello {Name"},
		{"unicode around", "Grüße, {Name}! 👋", m, "Grüße, Ada! 👋"},
		{"other data", "{Name}", 42, "{Name}"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, fill(tt.src, tt.data))
		})
	}
}

// Template actions other than a plain field are left as written, and logged
// once.
func TestPlaceholders_UnsupportedAction(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	src := "{{if .Name}}x{{end}} {Name}"
	assert.Equal(t, "{{if .Name}}x{{end}} Ada", fill(src, map[string]any{"Name": "Ada"}))
	assert.Equal(t, "{{if .Name}}x{{end}} Ada", fill(src, map[string]any{"Name": "Ada"}))
	assert.Equal(t, 1, bytes.Count(buf.Bytes(), []byte(`"{{if .Name}}"`)), buf.String())
}

// The bundled catalogs' placeholders are filled, in the plural form the count
// selects.
func TestLocalize_PlaceholdersInCatalog(t *testing.T) {
	SetLocaleForTests(t, "de")
	assert.Equal(t, "Sie hat 1 Element geteilt", XN("items_shared.female", "", 1, map[string]any{"Count": 1}))
	assert.Equal(t, "Sie hat 3 Elemente geteilt", XN("items_shared.female", "", 3, map[string]any{"Count": 3}))
	assert.Equal(t, "Alex hat sein Profil aktualisiert", X("profile_updated.male", "", map[string]any{"Name": "Alex"}))
}
