package theme

import (
	"image/color"
	"sync"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/stretchr/testify/assert"
)

func TestRenderingTheme(t *testing.T) {
	current := &simpleTheme{onlyColor: color.Black}
	assert.Equal(t, current, CurrentlyRenderingWithFallback(current))

	custom := &simpleTheme{onlyColor: color.White}
	PushRenderingTheme(custom)
	assert.Equal(t, custom, CurrentlyRenderingWithFallback(current))

	PopRenderingTheme()
	assert.Equal(t, current, CurrentlyRenderingWithFallback(current))
}

func TestRenderingTheme_EmptyStack(t *testing.T) {
	// With nothing pushed, the fallback should always be returned.
	fallback := &simpleTheme{onlyColor: color.Black}
	assert.Equal(t, fallback, CurrentlyRenderingWithFallback(fallback))

	other := &simpleTheme{onlyColor: color.White}
	assert.Equal(t, other, CurrentlyRenderingWithFallback(other))
}

func TestRenderingTheme_Nested(t *testing.T) {
	fallback := &simpleTheme{onlyColor: color.Black}
	first := &simpleTheme{onlyColor: color.White}
	second := &simpleTheme{onlyColor: color.NRGBA{R: 255, A: 255}}

	PushRenderingTheme(first)
	assert.Equal(t, first, CurrentlyRenderingWithFallback(fallback))

	PushRenderingTheme(second)
	assert.Equal(t, second, CurrentlyRenderingWithFallback(fallback))

	PopRenderingTheme()
	assert.Equal(t, first, CurrentlyRenderingWithFallback(fallback))

	PopRenderingTheme()
	assert.Equal(t, fallback, CurrentlyRenderingWithFallback(fallback))
}

func TestRenderingTheme_PushPopBalance(t *testing.T) {
	// Verify the stack returns to empty after balanced push/pop cycles.
	fallback := &simpleTheme{onlyColor: color.Black}

	for i := 0; i < 10; i++ {
		th := &simpleTheme{onlyColor: color.NRGBA{R: uint8(i), A: 255}}
		PushRenderingTheme(th)
	}
	for i := 0; i < 10; i++ {
		PopRenderingTheme()
	}

	assert.Equal(t, fallback, CurrentlyRenderingWithFallback(fallback))
}

func TestRenderingTheme_RapidPushPop(t *testing.T) {
	// Exercises many sequential push/pop cycles to verify stack integrity.
	fallback := &simpleTheme{onlyColor: color.Black}

	for i := 0; i < 100; i++ {
		th := &simpleTheme{onlyColor: color.NRGBA{R: uint8(i % 256), A: 255}}
		PushRenderingTheme(th)
		assert.Equal(t, th, CurrentlyRenderingWithFallback(fallback))
		PopRenderingTheme()
	}
	assert.Equal(t, fallback, CurrentlyRenderingWithFallback(fallback))
}

func TestRenderingTheme_ConcurrentGoroutines(t *testing.T) {
	// Each goroutine gets its own isolated stack — pushes in one goroutine
	// must not affect reads in another.
	fallback := &simpleTheme{onlyColor: color.Black}

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			th := &simpleTheme{onlyColor: color.NRGBA{R: uint8(id), A: 255}}

			// Before push, each goroutine should see only the fallback.
			assert.Equal(t, fallback, CurrentlyRenderingWithFallback(fallback))

			PushRenderingTheme(th)

			// Each goroutine sees only its own theme.
			got := CurrentlyRenderingWithFallback(fallback)
			assert.Equal(t, th, got, "goroutine %d saw wrong theme", id)

			// Nested push.
			nested := &simpleTheme{onlyColor: color.NRGBA{G: uint8(id), A: 255}}
			PushRenderingTheme(nested)
			assert.Equal(t, nested, CurrentlyRenderingWithFallback(fallback))

			PopRenderingTheme()
			assert.Equal(t, th, CurrentlyRenderingWithFallback(fallback))

			PopRenderingTheme()

			// After pop, back to fallback.
			assert.Equal(t, fallback, CurrentlyRenderingWithFallback(fallback))
		}(g)
	}

	wg.Wait()
}

func TestRenderingTheme_PopOnEmptyIsNoop(t *testing.T) {
	fallback := &simpleTheme{onlyColor: color.Black}

	// Pop with nothing pushed should not panic.
	PopRenderingTheme()

	assert.Equal(t, fallback, CurrentlyRenderingWithFallback(fallback))
}

type simpleTheme struct {
	bellui.Theme

	onlyColor color.Color
}

func (s *simpleTheme) Color(bellui.ThemeColorName, bellui.ThemeVariant) color.Color {
	return s.onlyColor
}
