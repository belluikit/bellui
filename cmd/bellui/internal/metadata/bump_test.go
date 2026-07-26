package metadata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBumpBuildStandardPreservesFormatting verifies that bumping the build number
// on package is a surgical edit: only the build value changes, and the author's
// leading comment, key ordering and spacing round-trip byte-for-byte. A full
// re-serialize used to strip all of that on every `bellui package`.
func TestBumpBuildStandardPreservesFormatting(t *testing.T) {
	const src = `// AppViewer packaging metadata — keep this comment.
details {
  name = "appviewer"
  id = "com.trendvidia.appviewer"
  version = "1.0"
  build = 2
  ios_usage_descriptions = {
    NSMicrophoneUsageDescription: "The microphone is used for audio level metering."
    NSCameraUsageDescription: "The camera is used to capture photos."
  }
}
`
	dir := t.TempDir()
	path := filepath.Join(dir, ManifestFile)
	require.NoError(t, os.WriteFile(path, []byte(src), 0o600))

	require.NoError(t, BumpBuildStandard(dir))

	out, err := os.ReadFile(path)
	require.NoError(t, err)

	// Only the build number changed; everything else is byte-for-byte identical.
	want := strings.Replace(src, "build = 2", "build = 3", 1)
	assert.Equal(t, want, string(out))

	// Sanity: the comment survives and the non-canonical key order (mic before
	// camera) is untouched — both casualties of a full re-marshal.
	assert.Contains(t, string(out), "// AppViewer packaging metadata")
	assert.Less(t,
		strings.Index(string(out), "NSMicrophoneUsageDescription"),
		strings.Index(string(out), "NSCameraUsageDescription"))
}

// TestBumpBuildStandardNoManifest is a no-op (not an error) when the directory
// has no manifest to update, matching the previous packaging behaviour.
func TestBumpBuildStandardNoManifest(t *testing.T) {
	assert.NoError(t, BumpBuildStandard(t.TempDir()))
}
