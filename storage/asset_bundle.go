package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/belluikit/bellui"
)

// isWithinDir reports whether path is lexically inside dir.
// Both dir and path must be Clean'd before calling.
func isWithinDir(dir, path string) bool {
	// Ensure dir ends with separator so "/tmp/bundleX" doesn't match "/tmp/bundle".
	prefix := dir + string(filepath.Separator)
	return path == dir || strings.HasPrefix(path, prefix)
}

// AssetBundle provides access to a directory of asset files as [bellui.Resource]
// values. Resources are loaded lazily on first access and cached so that
// repeated calls to [AssetBundle.Resource] with the same name return the
// same [bellui.LazyResource] instance.
//
// AssetBundle is safe for concurrent use.
//
// Since: 1.0
type AssetBundle struct {
	root  string
	cache sync.Map // name -> *bellui.LazyResource
}

// NewAssetBundle creates a new AssetBundle rooted at the given filesystem
// directory. The directory must exist.
//
// Since: 1.0
func NewAssetBundle(dir string) (*AssetBundle, error) {
	clean := filepath.Clean(dir)
	info, err := os.Stat(clean)
	if err != nil {
		return nil, fmt.Errorf("asset bundle directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("asset bundle path is not a directory: %s", clean)
	}
	return &AssetBundle{root: clean}, nil
}

// Resource returns a lazily-loaded [bellui.Resource] for the named asset.
// The name is a path relative to the bundle root, using forward slashes
// as separators (even on Windows). Subdirectories are supported
// (e.g. "icons/cancel.svg").
//
// Returns nil if the file does not exist.
//
// Since: 1.0
func (b *AssetBundle) Resource(name string) bellui.Resource {
	if v, ok := b.cache.Load(name); ok {
		return v.(*bellui.LazyResource)
	}

	path := filepath.Join(b.root, filepath.FromSlash(name))
	// Containment check: ensure the resolved path stays within the bundle root
	// to prevent path traversal attacks (e.g. name = "../../../etc/passwd").
	if !isWithinDir(b.root, path) {
		return nil
	}
	if _, err := os.Stat(path); err != nil {
		return nil
	}

	res := bellui.NewLazyResourceFromPath(path)
	actual, _ := b.cache.LoadOrStore(name, res)
	return actual.(*bellui.LazyResource)
}

// MustResource is like [Resource] but panics if the named asset does not exist.
//
// Since: 1.0
func (b *AssetBundle) MustResource(name string) bellui.Resource {
	res := b.Resource(name)
	if res == nil {
		panic(fmt.Sprintf("asset not found: %s", name))
	}
	return res
}

// List returns the names of all files in the bundle root directory
// (non-recursive). Directory entries are excluded.
//
// Since: 1.0
func (b *AssetBundle) List() ([]string, error) {
	entries, err := os.ReadDir(b.root)
	if err != nil {
		return nil, err
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// ReleaseAll frees all cached resource content. The resources remain valid
// and will re-load their content on the next [bellui.Resource.Content] call.
//
// Since: 1.0
func (b *AssetBundle) ReleaseAll() {
	b.cache.Range(func(_, value any) bool {
		value.(*bellui.LazyResource).Release()
		return true
	})
}
