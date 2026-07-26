package tiles

import (
	"testing"

	"github.com/belluikit/bellui/x/charts/coord"
)

func TestNewCache_DefaultURL(t *testing.T) {
	c := NewCache("", nil)
	if c == nil {
		t.Fatal("NewCache returned nil")
	}
	if c.baseURL != DefaultTileURL {
		t.Errorf("baseURL = %q, want %q", c.baseURL, DefaultTileURL)
	}
}

func TestNewCache_CustomURL(t *testing.T) {
	custom := "https://example.com/tiles/{z}/{x}/{y}.png"
	c := NewCache(custom, nil)
	if c.baseURL != custom {
		t.Errorf("baseURL = %q, want %q", c.baseURL, custom)
	}
}

func TestGet_NotCached(t *testing.T) {
	// Create a cache that uses a URL that won't resolve (no HTTP calls expected
	// because the goroutine is fire-and-forget; we just check the return value).
	c := NewCache("http://127.0.0.1:1/invalid/{z}/{x}/{y}.png", nil)
	tc := coord.TileCoord{Z: 5, X: 10, Y: 20}

	img := c.Get(tc)
	if img != nil {
		t.Error("Get for uncached tile should return nil")
	}
}

func TestTileURL(t *testing.T) {
	c := NewCache("https://tiles.example.com/{z}/{x}/{y}.png", nil)
	tc := coord.TileCoord{Z: 3, X: 7, Y: 12}

	url := c.tileURL(tc)
	expected := "https://tiles.example.com/3/7/12.png"
	if url != expected {
		t.Errorf("tileURL = %q, want %q", url, expected)
	}
}

func TestNewCache_MaxTilesDefault(t *testing.T) {
	c := NewCache("", nil)
	if c.maxTiles != 256 {
		t.Errorf("maxTiles = %d, want 256", c.maxTiles)
	}
}
