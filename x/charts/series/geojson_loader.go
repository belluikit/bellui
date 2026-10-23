package series

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/belluikit/bellui/x/charts/internal/geojson"
)

// GeoJSON documents are large — the country outlines the demo uses are 14 MB,
// which took 272 ms to download and 155 ms to parse. Doing that inline in a
// series' Render blocked the render goroutine, and http.DefaultClient has no
// timeout, so a slow or captive network froze the app outright (#35).
//
// geoJSONLoader fetches off the render goroutine on the internal/tiles.Cache
// model: one in-flight request per URL, the parsed result cached, and — unlike
// the original code — failures cached with exponential backoff so an offline
// app degrades to "no overlay" instead of reconnecting on every frame.
const (
	geoJSONTimeout   = 30 * time.Second
	geoJSONMaxBytes  = 64 << 20 // OOM guard; far above any real boundary file
	geoJSONRetryBase = 2 * time.Second
	geoJSONRetryMax  = 5 * time.Minute
)

type geoJSONEntry struct {
	fc       *geojson.FeatureCollection
	loading  bool
	failures int
	retryAt  time.Time
}

type geoJSONLoader struct {
	mu      sync.Mutex
	entries map[string]*geoJSONEntry

	// Seams for tests: no network, no wall clock.
	fetch func(url string) ([]byte, error)
	now   func() time.Time
}

var geoJSONCache = &geoJSONLoader{}

func (l *geoJSONLoader) clock() time.Time {
	if l.now != nil {
		return l.now()
	}
	return time.Now()
}

// Get returns the parsed collection for url. It never blocks: the first call
// starts a background fetch and returns nil, as does every call while that
// fetch is in flight or while a previous failure is still inside its backoff
// window. onLoaded, when non-nil, fires once a fetch settles — successfully or
// not — off the render goroutine, so the caller can request a repaint.
func (l *geoJSONLoader) Get(url string, onLoaded func()) *geojson.FeatureCollection {
	if url == "" {
		return nil
	}

	l.mu.Lock()
	if l.entries == nil {
		l.entries = map[string]*geoJSONEntry{}
	}
	entry, ok := l.entries[url]
	if !ok {
		entry = &geoJSONEntry{}
		l.entries[url] = entry
	}
	switch {
	case entry.fc != nil:
		fc := entry.fc
		l.mu.Unlock()
		return fc
	case entry.loading:
		l.mu.Unlock()
		return nil // already downloading; the repaint will come
	case entry.failures > 0 && l.clock().Before(entry.retryAt):
		l.mu.Unlock()
		return nil // still backing off from an earlier failure
	}
	entry.loading = true
	l.mu.Unlock()

	go l.load(url, onLoaded)
	return nil
}

func (l *geoJSONLoader) load(url string, onLoaded func()) {
	fc, err := l.download(url)
	if err != nil {
		log.Printf("x/charts: choropleth GeoJSON load failed for %s: %v", url, err)
	}

	l.mu.Lock()
	entry := l.entries[url]
	entry.loading = false
	if err == nil {
		entry.fc = fc
		entry.failures = 0
	} else {
		entry.failures++
		entry.retryAt = l.clock().Add(geoJSONBackoff(entry.failures))
	}
	l.mu.Unlock()

	if onLoaded != nil {
		onLoaded()
	}
}

func (l *geoJSONLoader) download(url string) (*geojson.FeatureCollection, error) {
	if l.fetch != nil {
		data, err := l.fetch(url)
		if err != nil {
			return nil, err
		}
		return geojson.Parse(data)
	}

	client := &http.Client{Timeout: geoJSONTimeout}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, geoJSONMaxBytes))
	if err != nil {
		return nil, err
	}
	if len(data) == geoJSONMaxBytes {
		return nil, fmt.Errorf("document exceeds the %d byte limit", geoJSONMaxBytes)
	}
	return geojson.Parse(data)
}

// geoJSONBackoff doubles the retry delay per consecutive failure, capped so a
// long-offline app still recovers promptly once the network returns.
func geoJSONBackoff(failures int) time.Duration {
	d := geoJSONRetryBase
	for i := 1; i < failures && d < geoJSONRetryMax; i++ {
		d *= 2
	}
	if d > geoJSONRetryMax {
		d = geoJSONRetryMax
	}
	return d
}
