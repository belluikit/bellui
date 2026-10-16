package widget

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/storage"
	"github.com/belluikit/bellui/test"
)

// tickFrame makes the current frame due right now and runs one driver tick,
// advancing playback by exactly one frame (the advance schedules the next
// frame in the future, so the bounded catch-up loop stops after one step).
func tickFrame(g *AnimatedGif) {
	g.nextFrameAt = time.Now()
	g.tick()
}

func TestNewAnimatedGif(t *testing.T) {
	test.NewTempApp(t)

	g, err := NewAnimatedGif(storage.NewFileURI("./testdata/gif/earth.gif"))
	require.NoError(t, err)
	require.NotNil(t, g.src)
	assert.Equal(t, 44, len(g.src.Image))
	assert.NotNil(t, g.dst.Image, "first frame shown before Start")

	w := test.NewWindow(g)
	defer w.Close()
	w.Resize(geo.NewSize(128, 128))

	g.Start()
	assert.Equal(t, -1, g.remaining, "earth.gif loops forever")
	assert.True(t, g.playing)
	assert.NotNil(t, g.ticker, "frame driver armed")

	tickFrame(g)
	assert.Equal(t, 1, g.frameIndex, "tick advances one frame when due")
	tickFrame(g)
	assert.Equal(t, 2, g.frameIndex)

	g.Stop()
	assert.False(t, g.playing)
	assert.Nil(t, g.ticker, "Stop releases the frame driver")
	assert.Equal(t, 2, g.frameIndex, "last frame remains visible")
}

func TestAnimatedGif_MinSize(t *testing.T) {
	r, err := os.ReadFile("./testdata/gif/earth.gif")
	require.NoError(t, err)

	res := bellui.NewStaticResource("earth.gif", r)
	g, err := NewAnimatedGifFromResource(res)
	require.NoError(t, err)
	assert.True(t, g.min.IsZero())

	g.SetMinSize(geo.NewSize(10.0, 10.0))
	assert.Equal(t, float32(10), g.MinSize().Width)
	assert.Equal(t, float32(10), g.MinSize().Height)
}

func TestAnimatedGif_PlaysOnce(t *testing.T) {
	test.NewTempApp(t)

	g, err := NewAnimatedGif(storage.NewFileURI("./testdata/gif/earth-once.gif"))
	require.NoError(t, err)

	g.Start()
	assert.Equal(t, 1, g.remaining, "LoopCount -1 plays the frames once")

	for i := 0; i < len(g.src.Image) && g.playing; i++ {
		tickFrame(g)
	}
	assert.False(t, g.playing, "playback completes after one pass")
	assert.Nil(t, g.ticker, "completed playback releases the frame driver")
	assert.Equal(t, len(g.src.Image)-1, g.frameIndex, "last frame remains visible")

	// Start after completion restarts from the first frame.
	g.Start()
	assert.True(t, g.playing)
	assert.Equal(t, 0, g.frameIndex)
	g.Stop()
}

func TestAnimatedGif_CatchUpResync(t *testing.T) {
	test.NewTempApp(t)

	g, err := NewAnimatedGif(storage.NewFileURI("./testdata/gif/earth.gif"))
	require.NoError(t, err)
	g.Start()

	// Far behind (e.g. re-shown after a long eviction): one tick advances at
	// most a full cycle, then resynchronizes to now instead of replaying the
	// backlog forever.
	g.nextFrameAt = time.Now().Add(-time.Hour)
	g.tick()
	assert.True(t, g.playing)
	assert.True(t, g.nextFrameAt.After(time.Now().Add(-time.Minute)), "resynchronized to now")
	g.Stop()
}

// TestAnimatedGif_RendererLifecycle pins the pre-rebrand tracker #28-style lifecycle: a renderer
// Destroy (discarded widget, or cache eviction of a hidden one) stops the
// frame driver so its closure no longer holds the widget alive, and
// CreateRenderer re-arms playback on re-show.
func TestAnimatedGif_RendererLifecycle(t *testing.T) {
	test.NewTempApp(t)

	g, err := NewAnimatedGif(storage.NewFileURI("./testdata/gif/earth.gif"))
	require.NoError(t, err)

	g.Start()
	require.NotNil(t, g.ticker)

	cache.Renderer(g).Destroy()
	assert.Nil(t, g.ticker, "Destroy must stop the frame driver")
	assert.True(t, g.playing, "desired playing state survives eviction")

	g.CreateRenderer()
	assert.NotNil(t, g.ticker, "re-show re-arms the frame driver")

	g.Stop()
	cache.Renderer(g).Destroy()
	g.CreateRenderer()
	assert.Nil(t, g.ticker, "a stopped gif does not re-arm on re-show")
}

func TestAnimatedGif_LoadStopsPlayback(t *testing.T) {
	test.NewTempApp(t)

	g, err := NewAnimatedGif(storage.NewFileURI("./testdata/gif/earth.gif"))
	require.NoError(t, err)
	g.Start()
	require.True(t, g.playing)

	require.NoError(t, g.Load(nil))
	assert.False(t, g.playing)
	assert.Nil(t, g.ticker)
	assert.Nil(t, g.src)
	assert.Nil(t, g.dst.Image)

	// Start with nothing loaded is a safe no-op.
	g.Start()
	assert.False(t, g.playing)
}
