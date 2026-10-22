package lang

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBindText(t *testing.T) {
	SetLocale("en")
	t.Cleanup(func() { SetLocale("") })

	var got string
	remove := BindText("Cancel", func(s string) { got = s })

	// Applied immediately with the current translation.
	assert.Equal(t, "Cancel", got)

	// Re-applied on a live locale change.
	SetLocale("fr-FR")
	assert.Equal(t, "Annuler", got)

	SetLocale("en")
	assert.Equal(t, "Cancel", got)

	// After remove, further locale changes are ignored.
	remove()
	SetLocale("fr-FR")
	assert.Equal(t, "Cancel", got)
}

func TestBindText_TemplateData(t *testing.T) {
	SetLocale("en")
	t.Cleanup(func() { SetLocale("") })

	var got string
	remove := BindText("Hello {Name}", func(s string) { got = s }, map[string]any{"Name": "Ada"})
	t.Cleanup(remove)

	// Unknown key falls back to the key text with template data filled in.
	assert.Equal(t, "Hello Ada", got)
}
