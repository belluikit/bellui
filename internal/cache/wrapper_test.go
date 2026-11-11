package cache_test

import (
	"bytes"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/widget"
)

// labelWrapper stands in the tree for the label it embeds, which was extended
// by the label itself: the shape of #1146 (Player's video{*VideoPlayer}).
type labelWrapper struct{ *widget.Label }

// TestRenderer_WarnsAboutAWrapper pins #1146: a widget whose base widget was
// extended by another object gets a renderer of its own, apart from the one
// the widget's methods use, so it is reported once.
func TestRenderer_WarnsAboutAWrapper(t *testing.T) {
	cache.ResetWrapperWarningsForTests(t)
	var logged bytes.Buffer
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	label := widget.NewLabel("plain")
	cache.Renderer(label)
	assert.Empty(t, logged.String(), "a widget in its own place")

	w := labelWrapper{widget.NewLabel("wrapped")}
	cache.Renderer(w)
	assert.Contains(t, logged.String(), "a cache_test.labelWrapper is in the tree in place of the *widget.Label")
	assert.NotSame(t, cache.Renderer(w.Label), cache.Renderer(w), "the two get separate renderers")

	logged.Reset()
	cache.Renderer(labelWrapper{widget.NewLabel("another")})
	assert.Empty(t, logged.String(), "once per pair of types")
}
