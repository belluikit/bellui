package commands

import (
	"bytes"
	"go/format"
	"path"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/cmd/bellui/internal/metadata"
	"github.com/belluikit/bellui/cmd/bellui/internal/templates"
)

// bellui init writes a main.go that builds: ShowAndRun is a function in the
// bellui package, not a Window method (#337).
func TestHelloWorldTemplateUsesShowAndRunFunction(t *testing.T) {
	data := &metadata.BelluiApp{}
	data.Details.ID = "com.example.hello"
	data.Details.Name = "Hello"

	var buf bytes.Buffer
	require.NoError(t, templates.HelloWorld.Execute(&buf, &data))

	_, err := format.Source(buf.Bytes())
	require.NoError(t, err, "the generated main.go must be valid Go:\n%s", buf.String())

	out := buf.String()
	assert.Contains(t, out, "bellui.ShowAndRun(w)")
	assert.NotContains(t, out, "w.ShowAndRun()")
}

// The main.go bellui init writes embeds the catalog init creates: a pattern
// matching no file fails the build ("no matching files found").
func TestHelloWorldTemplateEmbedsInitCatalog(t *testing.T) {
	data := &metadata.BelluiApp{}
	var buf bytes.Buffer
	require.NoError(t, templates.HelloWorld.Execute(&buf, &data))

	m := regexp.MustCompile(`(?m)^//go:embed (\S+)$`).FindStringSubmatch(buf.String())
	require.NotNil(t, m, "main.go embeds the translations:\n%s", buf.String())
	ok, err := path.Match(m[1], initCatalog)
	require.NoError(t, err)
	assert.True(t, ok, "//go:embed %s does not match %s", m[1], initCatalog)
}
