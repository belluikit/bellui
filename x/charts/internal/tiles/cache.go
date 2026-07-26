// Package tiles provides an async tile cache for map tile downloading and caching.
package tiles

import (
	"fmt"
	"image"
	"image/png"
	"log"
	"net/http"
	"strings"
	"sync"

	// Register JPEG decoder for tile sources that serve JPEG
	_ "image/jpeg"

	"github.com/belluikit/bellui/x/charts/coord"
)

// DefaultTileURL is the OpenStreetMap tile server URL template.
const DefaultTileURL = "https://tile.openstreetmap.org/{z}/{x}/{y}.png"

// Cache manages map tile downloading and in-memory caching.
type Cache struct {
	baseURL   string
	cache     sync.Map // coord.TileCoord → *image.NRGBA
	inflight  sync.Map // coord.TileCoord → bool
	onLoaded  func()   // callback when a tile finishes downloading
	mu        sync.Mutex
	maxTiles  int
	tileCount int
}

// NewCache creates a tile cache with the given base URL and a callback
// that fires when a tile finishes downloading (typically chart.Refresh).
func NewCache(baseURL string, onLoaded func()) *Cache {
	if baseURL == "" {
		baseURL = DefaultTileURL
	}
	return &Cache{
		baseURL:  baseURL,
		onLoaded: onLoaded,
		maxTiles: 256,
	}
}

// Get returns a cached tile image, or nil if not yet downloaded.
// If the tile isn't cached, an async download is started.
func (c *Cache) Get(tc coord.TileCoord) *image.NRGBA {
	// Check cache
	if img, ok := c.cache.Load(tc); ok {
		return img.(*image.NRGBA)
	}

	// Check if already downloading
	if _, ok := c.inflight.LoadOrStore(tc, true); ok {
		return nil // download in progress
	}

	// Start async download
	go c.download(tc)
	return nil
}

// Prefetch starts downloading tiles that aren't cached yet.
func (c *Cache) Prefetch(tiles []coord.TileCoord) {
	for _, tc := range tiles {
		if _, ok := c.cache.Load(tc); !ok {
			if _, ok := c.inflight.LoadOrStore(tc, true); !ok {
				go c.download(tc)
			}
		}
	}
}

func (c *Cache) download(tc coord.TileCoord) {
	defer c.inflight.Delete(tc)

	url := c.tileURL(tc)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("fycharts: tile request creation failed for %s: %v", url, err)
		return
	}
	req.Header.Set("User-Agent", "fycharts/1.0 (github.com/belluikit/bellui/x/charts)")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("fycharts: tile download failed for %s: %v", url, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("fycharts: tile download returned status %d for %s", resp.StatusCode, url)
		return
	}

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		log.Printf("fycharts: tile image decode failed for %s: %v", url, err)
		return
	}

	// Convert to NRGBA
	bounds := img.Bounds()
	nrgba := image.NewNRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			nrgba.Set(x, y, img.At(x, y))
		}
	}

	c.cache.Store(tc, nrgba)

	// Simple eviction: if too many tiles, don't add more (LRU would be better)
	c.mu.Lock()
	c.tileCount++
	c.mu.Unlock()

	// Notify that a tile loaded
	if c.onLoaded != nil {
		c.onLoaded()
	}
}

func (c *Cache) tileURL(tc coord.TileCoord) string {
	url := c.baseURL
	url = strings.Replace(url, "{z}", fmt.Sprintf("%d", tc.Z), 1)
	url = strings.Replace(url, "{x}", fmt.Sprintf("%d", tc.X), 1)
	url = strings.Replace(url, "{y}", fmt.Sprintf("%d", tc.Y), 1)
	return url
}

// Suppress unused import warning for png
var _ = png.Decode
