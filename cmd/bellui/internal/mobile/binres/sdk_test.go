// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package binres

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPackedTableMatchesSDK re-packs the pinned SDK platform's resources.arsc
// and requires the result to match the checked-in arsc.go, catching a strip
// edit (or SDKPlatform bump) committed without `go generate`. No CI runner is
// guaranteed the pinned platform (the macos runner has an ANDROID_HOME but not
// android-35), so it skips unless the platform jar is present — it gates the
// machines that can regenerate, which are the ones that introduce the drift.
// The comparison is on the uncompressed table so a Go-release gzip change
// cannot fail it.
func TestPackedTableMatchesSDK(t *testing.T) {
	jar, err := sdkAPIResourcesPath()
	if err != nil {
		t.Skipf("cannot re-pack %s to compare: %v", SDKPlatform, err)
	}
	if _, err := os.Stat(jar); err != nil {
		t.Skipf("%s not installed; cannot re-pack to compare: %v", SDKPlatform, err)
	}
	r := require.New(t)

	fresh, err := PackResources()
	r.NoError(err)

	gunzip := func(b []byte) []byte {
		zr, err := gzip.NewReader(bytes.NewReader(b))
		r.NoError(err)
		defer zr.Close()
		raw, err := io.ReadAll(zr)
		r.NoError(err)
		return raw
	}
	r.True(bytes.Equal(gunzip(arsc), gunzip(fresh)),
		"embedded arsc.go is stale for %s: run `go generate` in internal/mobile/binres", SDKPlatform)
}

// platformsFixture points ANDROID_HOME at a temp SDK whose platforms/ holds
// exactly the named directories, and returns that platforms/ path so a test
// can drop non-directory entries beside them.
func platformsFixture(t *testing.T, names ...string) string {
	t.Helper()
	sdkdir := t.TempDir()
	platforms := filepath.Join(sdkdir, "platforms")
	require.NoError(t, os.MkdirAll(platforms, 0o755))
	for _, name := range names {
		require.NoError(t, os.MkdirAll(filepath.Join(platforms, name), 0o755))
	}
	t.Setenv("ANDROID_HOME", sdkdir)
	return platforms
}

// TestSDKPlatformPinConsistency keeps SDKPlatformVersion tied to SDKPlatform:
// the version must be the API level SDKPlatform names. SDKPlatformCodename is
// not derivable from either, so it rides the goldens instead — TestEncode
// compares against aapt output recorded for exactly this pin.
func TestSDKPlatformPinConsistency(t *testing.T) {
	level, ok := platformAPILevel(SDKPlatform)
	require.True(t, ok, "SDKPlatform %q has no parsable API level", SDKPlatform)
	require.Equal(t, SDKPlatformVersion, level,
		"SDKPlatformVersion must match the API level of SDKPlatform %q", SDKPlatform)
}

func TestLatestAPIResourcesPath(t *testing.T) {
	latest := func(t *testing.T) string {
		p, err := LatestAPIResourcesPath()
		require.NoError(t, err)
		return filepath.Base(filepath.Dir(p))
	}

	// android-9 sorts after android-36 lexically; the pick must be numeric (#296).
	t.Run("numeric beats lexical", func(t *testing.T) {
		platformsFixture(t, "android-36", "android-9", "android-34")
		require.Equal(t, "android-36", latest(t))
	})

	t.Run("previews and extension SDKs rank behind numbered releases", func(t *testing.T) {
		platformsFixture(t, "android-VanillaIceCream", "android-35", "android-36-ext4", "android-9")
		require.Equal(t, "android-35", latest(t))
	})

	// A minor release is a stable platform, numbered just after its API
	// level; extension SDKs and previews stay behind it (#674).
	for _, tc := range []struct {
		installed []string
		want      string
	}{
		{[]string{"android-35", "android-36.1", "android-36-ext4"}, "android-36.1"},
		{[]string{"android-36", "android-36.1"}, "android-36.1"},
		{[]string{"android-36.1", "android-36.2"}, "android-36.2"},
		{[]string{"android-36.10", "android-36.9"}, "android-36.10"},
		{[]string{"android-36.1", "android-37"}, "android-37"},
		{[]string{"android-36-ext4"}, "android-36-ext4"},
		{[]string{"android-36.x", "android-35"}, "android-35"},
	} {
		t.Run("minor releases "+strings.Join(tc.installed, ","), func(t *testing.T) {
			platformsFixture(t, tc.installed...)
			require.Equal(t, tc.want, latest(t))
		})
	}

	t.Run("preview wins only when nothing numbered is installed", func(t *testing.T) {
		platformsFixture(t, "android-Baklava", "android-VanillaIceCream")
		require.Equal(t, "android-VanillaIceCream", latest(t))
	})

	t.Run("stray entries are not platforms", func(t *testing.T) {
		platforms := platformsFixture(t, "android-34", "tools")
		// a file is never a platform, not even one named like the newest one
		for _, name := range []string{".DS_Store", "android-99"} {
			require.NoError(t, os.WriteFile(filepath.Join(platforms, name), nil, 0o644))
		}
		require.Equal(t, "android-34", latest(t))
	})

	t.Run("empty platforms dir errors", func(t *testing.T) {
		platformsFixture(t)
		_, err := LatestAPIResourcesPath()
		require.ErrorContains(t, err, "no SDK platforms")
	})
}

// stripFixture returns a package shaped the way PackResources' stripping
// requires: type pool attr/id/style/string/dimen, one spec per type in order.
func stripFixture() *Package {
	pkg := &Package{typePool: &Pool{}, keyPool: &Pool{}}
	for i, name := range []string{"attr", "id", "style", "string", "dimen"} {
		pkg.typePool.ref(name)
		pkg.specs = append(pkg.specs, &TypeSpec{id: uint8(i) + 1})
	}
	return pkg
}

// TestStripKeyPool locks the key-blanking rule itself, which is otherwise
// guarded only by tests that need the pinned platform installed
// (TestPackedTableMatchesSDK) or that reach it through the embedded table
// (TestConfigChangesFlagNamesSurviveStrip, configChanges names only).
func TestStripKeyPool(t *testing.T) {
	pkg := stripFixture()
	shared := pkg.keyPool.ref("screenLayout")
	keptOnly := pkg.keyPool.ref("configChanges")
	droppedOnly := pkg.keyPool.ref("^attr-private/windowFrame")
	unreferenced := pkg.keyPool.ref("orphan")

	// attr and id are kept specs; string holds a dropped (nil'd) entry beside
	// a surviving one; dimen is a spec that goes away wholesale and happens to
	// reference the same key slot as the kept id entry — the #299 shape.
	pkg.specs[0].types = []*Type{{entries: []*Entry{{key: keptOnly}}}}
	pkg.specs[1].types = []*Type{{entries: []*Entry{{key: shared}}}}
	pkg.specs[3].types = []*Type{{entries: []*Entry{nil, {key: keptOnly}}}}
	pkg.specs[4].types = []*Type{{entries: []*Entry{{key: shared}, {key: droppedOnly}}}}

	stripKeyPool(pkg)

	r := require.New(t)
	r.Equal("screenLayout", shared.Resolve(pkg.keyPool),
		"a key a kept entry references must survive a dropped spec sharing its slot (#299)")
	r.Equal("configChanges", keptOnly.Resolve(pkg.keyPool))
	r.Empty(droppedOnly.Resolve(pkg.keyPool), "a key only a dropped spec references must be blanked")
	r.Empty(unreferenced.Resolve(pkg.keyPool), "a key nothing references must be blanked")
}

func TestCheckStripAssumptions(t *testing.T) {
	t.Run("android-35 shape", func(t *testing.T) {
		require.NoError(t, checkStripAssumptions(stripFixture()))
	})

	t.Run("type pool reordered", func(t *testing.T) {
		pkg := stripFixture()
		pkg.typePool.strings[1], pkg.typePool.strings[2] = pkg.typePool.strings[2], pkg.typePool.strings[1]
		require.ErrorContains(t, checkStripAssumptions(pkg), "type pool no longer starts with")
	})

	t.Run("type pool too short", func(t *testing.T) {
		pkg := stripFixture()
		pkg.typePool.strings = pkg.typePool.strings[:3]
		require.ErrorContains(t, checkStripAssumptions(pkg), "type pool no longer starts with")
	})

	t.Run("too few specs", func(t *testing.T) {
		pkg := stripFixture()
		pkg.specs = pkg.specs[:3]
		require.ErrorContains(t, checkStripAssumptions(pkg), "type specs")
	})

	// The stripping reaches its types through pkg.specs[3] and pkg.specs[:4],
	// so a package whose specs are not in type-id order would be stripped of
	// the wrong types even though its type pool looks right.
	t.Run("specs out of type-id order", func(t *testing.T) {
		pkg := stripFixture()
		pkg.specs[2], pkg.specs[3] = pkg.specs[3], pkg.specs[2]
		require.ErrorContains(t, checkStripAssumptions(pkg), `want 3 ("style")`)
	})
}
