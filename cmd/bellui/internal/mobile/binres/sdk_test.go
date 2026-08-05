// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package binres

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
