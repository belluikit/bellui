package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// TestReleaseKeepsWhatASwapPutInARoundedClip is #1146's second done-when, in
// player#531's shape: a Stack inside a container with a rounded clip, whose
// child the app swaps for a video widget. A release in the same frame lets go
// of the child swapped out, and keeps the one swapped in, which the walk
// found in the tree.
func TestReleaseKeepsWhatASwapPutInARoundedClip(t *testing.T) {
	test.NewTempApp(t)
	before := widget.NewLabel("loading")
	inner := container.NewStack(before)
	clip := container.New(layout.NewStackLayout(), inner)
	clip.ClipCornerRadius = 12
	c := walkedCanvas(container.NewVBox(widget.NewLabel("title"), clip))
	c.SetPainter(&stubPainter{})
	idleFrame(c)
	_, rendered := cache.CachedRenderer(before)
	require.True(t, rendered, "precondition: the first child has a renderer")

	video := widget.NewButton("video", nil) // stands for the video widget
	inner.Objects = []core.CanvasObject{video}
	inner.Refresh()
	c.Refresh(inner)
	c.ReleaseDropped()
	idleFrame(c)

	_, rendered = cache.CachedRenderer(video)
	assert.True(t, rendered, "the swapped-in widget lost its renderer")
	assert.NotNil(t, cache.GetCanvasForObject(video), "the swapped-in widget lost its canvas")
	_, rendered = cache.CachedRenderer(before)
	assert.False(t, rendered, "the swapped-out child kept its renderer: the release did not run")
}
