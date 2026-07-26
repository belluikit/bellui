//go:build !tamago && !noos

package test

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resolveMasterFilename, on mobile, prefers a platform-specific master under
// <dir>/mobile/<base> when one exists, so widgets that render differently for
// touch (e.g. the always-wide scrollbar) don't diverge the shared desktop
// masters. Falls back to the shared master when no mobile variant is present.
func resolveMasterFilename(wd, name string) string {
	if !onMobileTestBuild() {
		return name
	}
	dir, base := filepath.Split(name)
	mobileName := filepath.Join(dir, "mobile", base)
	if _, err := os.Stat(filepath.Join(wd, "testdata", mobileName)); err == nil {
		return mobileName
	}
	return name
}

// AssertImageMatches asserts that the given image is the same as the one stored in the master file.
// The master filename is relative to the `testdata` directory which is relative to the test.
//
// Environment variables:
//   - UPDATE_TESTDATA=1 : auto-update master files instead of failing
//   - BELLUI_TEST_TOLERANCE=N : pixel delta tolerance (default 4)
//
// The test `t` fails if the given image is not equal to the loaded master image.
// In this case the given image is written into a file in `testdata/failed/<masterFilename>`.
func AssertImageMatches(t *testing.T, masterFilename string, img image.Image, msgAndArgs ...any) bool {
	wd, err := os.Getwd()
	require.NoError(t, err)
	masterFilename = resolveMasterFilename(wd, masterFilename)
	masterPath := filepath.Join(wd, "testdata", masterFilename)
	failedPath := filepath.Join(wd, "testdata/failed", masterFilename)

	// Auto-update mode: write the image as the new master.
	if os.Getenv("UPDATE_TESTDATA") != "" {
		require.NoError(t, writeImage(masterPath, img))
		t.Logf("Updated master: %s", masterPath)
		return true
	}

	_, err = os.Stat(masterPath)
	if os.IsNotExist(err) {
		require.NoError(t, writeImage(failedPath, img))
		t.Errorf("Master not found at %s. Image written to %s might be used as master.\n"+
			"Run with UPDATE_TESTDATA=1 to create it automatically.", masterPath, failedPath)
		return false
	}

	file, err := os.Open(masterPath)
	require.NoError(t, err)
	defer file.Close()
	raw, _, err := image.Decode(file)
	require.NoError(t, err)

	masterPix := pixelsForImage(t, raw)
	capturePix := pixelsForImage(t, img)

	// Use fuzzy comparison on all platforms — small rendering differences
	// across OS versions, GPU drivers, and font renderers are expected.
	if pixCloseEnough(masterPix, capturePix) {
		return true
	}

	// Fuzzy comparison failed — report the strict diff for debugging.
	var msg string
	if len(msgAndArgs) > 0 {
		msg = fmt.Sprintf(msgAndArgs[0].(string)+"\n", msgAndArgs[1:]...)
	}
	if !assert.Equal(t, masterPix, capturePix, "%sImage did not match master. Actual image written to file://%s.\n"+
		"Run with UPDATE_TESTDATA=1 to update the master.", msg, failedPath) {
		require.NoError(t, writeImage(failedPath, img))
		return false
	}
	return true
}

func pixelsForImage(t *testing.T, img image.Image) []uint8 {
	var pix []uint8
	if data, ok := img.(*image.RGBA); ok {
		pix = data.Pix
	} else if data, ok := img.(*image.NRGBA); ok {
		pix = data.Pix
	}
	if pix == nil {
		t.Error("Master image is unsupported type")
	}

	return pix
}
