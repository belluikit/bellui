// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package binres

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// platformsFixture points ANDROID_HOME at a temp SDK whose platforms/ holds
// exactly the named directories.
func platformsFixture(t *testing.T, names ...string) {
	t.Helper()
	sdkdir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(sdkdir, "platforms"), 0o755))
	for _, name := range names {
		require.NoError(t, os.MkdirAll(filepath.Join(sdkdir, "platforms", name), 0o755))
	}
	t.Setenv("ANDROID_HOME", sdkdir)
}

// TestSDKPlatformPinConsistency keeps the three SDKPlatform* constants moving
// together: the version must be the API level SDKPlatform names.
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

	t.Run("preview wins only when nothing numbered is installed", func(t *testing.T) {
		platformsFixture(t, "android-Baklava", "android-VanillaIceCream")
		require.Equal(t, "android-VanillaIceCream", latest(t))
	})

	t.Run("stray entries are not platforms", func(t *testing.T) {
		platformsFixture(t, "android-34", "tools", ".DS_Store")
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
