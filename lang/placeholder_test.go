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

type inner struct{ Name string }

type outer struct{ *inner }

type ptrStringer struct{ s string }

func (p *ptrStringer) String() string { return "stringer " + p.s }

type stats struct{}

func (stats) Count() int { return 3 }

var (
	five  = 5
	pfive = &five
)

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
		{"field through a nil embedded pointer", "Hi {Name}", outer{}, "Hi {Name}"},
		{"field through an embedded pointer", "Hi {Name}", outer{&inner{Name: "Ada"}}, "Hi Ada"},
		{"pointer value", "{N} items", map[string]any{"N": &five}, "5 items"},
		{"pointer to pointer", "{N} items", map[string]any{"N": &pfive}, "5 items"},
		{"nil pointer value", "{N}", map[string]any{"N": (*int)(nil)}, "<nil>"},
		{"String method on the pointer", "{S}", map[string]any{"S": &ptrStringer{"x"}}, "stringer x"},
		{"pointer field", "{P}", struct{ P *int }{&five}, "5"},
		{"map[any]any", "{Name}", map[any]any{"Name": "Ada"}, "Ada"},
		{"map with int keys", "{Name}", map[int]string{1: "Ada"}, "{Name}"},
		{"method is not called", "{Count} items", stats{}, "{Count} items"},
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

type valueCount struct{}

func (valueCount) Count() int { return 3 }

type pointerCount struct{}

func (*pointerCount) Count() int { return 3 }

// Methods are not called, in either form, whatever the receiver (#667): a
// value comes from a map key or a struct field. text/template called them.
func TestPlaceholders_MethodsAreNotCalled(t *testing.T) {
	for _, data := range []any{valueCount{}, &valueCount{}, &pointerCount{}} {
		assert.Equal(t, "{Count} items", fill("{Count} items", data), "%T", data)
		assert.Equal(t, "{{.Count}} items", fill("{{.Count}} items", data), "%T", data)
	}
}

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &buf
}

type methodOnlyProbe struct{ Other int }

func (methodOnlyProbe) Total() int { return 7 }

// A placeholder left unfilled while a data value has a method of that name is
// logged once per type and name, with the map to pass instead (#667). A name
// that no method has is not logged.
func TestPlaceholders_MethodOnlyLogged(t *testing.T) {
	buf := captureLog(t)
	for range 3 {
		assert.Equal(t, "{Total} / {Missing}", fill("{Total} / {Missing}", methodOnlyProbe{}))
	}
	out := buf.String()
	assert.Equal(t, 1, bytes.Count(buf.Bytes(), []byte("{Total} is left as written")), out)
	assert.Contains(t, out, `map[string]any{"Total": v.Total()}`)
	assert.NotContains(t, out, "Missing")

	buf.Reset()
	fill("{Total}", dataList{map[string]any{}, methodOnlyProbe{}})
	assert.Empty(t, buf.String(), "already logged for this type and name")
}

// The Go template form is deprecated: each spelling is logged once, and still
// filled (#667). The "{Name}" form is not logged.
func TestPlaceholders_DeprecatedFormLogged(t *testing.T) {
	buf := captureLog(t)
	d := map[string]any{"DeprecationProbe": "x"}
	for range 3 {
		assert.Equal(t, "x x", fill("{{.DeprecationProbe}} {DeprecationProbe}", d))
	}
	out := buf.String()
	assert.Equal(t, 1, bytes.Count(buf.Bytes(), []byte(`"{{.DeprecationProbe}}" is deprecated; write {DeprecationProbe}`)), out)
	assert.Equal(t, 1, bytes.Count(buf.Bytes(), []byte("lang: ")), "one line, for the deprecated form only: %s", out)
}

// Several data values can be mixed; the first that has a name fills it, in
// every lang call that takes data (#667). One value behaves as before.
func TestLocalize_SeveralDataValues(t *testing.T) {
	SetLocaleForTests(t, "en")
	type user struct{ Name string }
	ana := user{Name: "Ana"}
	extra := map[string]any{"Count": 3, "Name": "not used"}

	assert.Equal(t, "Ana sent 3", L("{Name} sent {Count}", ana, extra), "struct, then map")
	assert.Equal(t, "not used sent 3", L("{Name} sent {Count}", extra, ana), "the first value wins")
	assert.Equal(t, "Ana sent {Count}", L("{Name} sent {Count}", ana), "one value, as before")
	assert.Equal(t, "Ana sent 3", L("{Name} sent {Count}", nil, ana, extra), "a nil value is skipped")
	assert.Equal(t, "Ana sent 3", LocalizeKey("probe.sent", "{Name} sent {Count}", ana, extra))
	assert.Equal(t, "Ana sent 3 files", LocalizePlural("{Name} sent {Count} files", 3, ana, extra))
	assert.Equal(t, "Ana sent 3 files", LocalizePluralKey("probe.sentFiles", "{Name} sent {Count} files", 3, ana, extra))
	assert.Equal(t, "2 things", LocalizePlural("{PluralCount} things", 2), "go-i18n's PluralCount, with no data")

	SetLocaleForTests(t, "de")
	assert.Equal(t, "Alex hat sein Profil aktualisiert",
		LSelect("profile_updated", "male", user{Name: "Alex"}, extra))
	assert.Equal(t, "Sie hat 3 Elemente geteilt",
		LSelectPlural("items_shared", "female", 3, user{}, map[string]any{"Count": 3}))

	var got string
	remove := BindText("{Name} sent {Count}", func(s string) { got = s }, ana, extra)
	t.Cleanup(remove)
	assert.Equal(t, "Ana sent 3", got)
}
