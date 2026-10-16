package commands

import (
	"bytes"
	"go/format"
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
