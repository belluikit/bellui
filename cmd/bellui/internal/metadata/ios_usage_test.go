package metadata

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIOSUsageDescriptionsRoundTrip verifies the AppDetails.IOSUsageDescriptions
// schema addition serializes to the manifest.pxf format and parses back without
// loss (the map that drives Info.plist usage-string injection).
func TestIOSUsageDescriptionsRoundTrip(t *testing.T) {
	in := &BelluiApp{
		Details: AppDetails{
			Name: "Testing",
			ID:   "com.example.testing",
			IOSUsageDescriptions: map[string]string{
				"NSCameraUsageDescription":            "Scan documents with the camera.",
				"NSMicrophoneUsageDescription":        "Record voice notes.",
				"NSLocationWhenInUseUsageDescription": "Tag notes with your location.",
			},
		},
	}

	buf := &bytes.Buffer{}
	assert.NoError(t, Save(in, buf))

	out, err := Load(bytes.NewReader(buf.Bytes()))
	assert.NoError(t, err)

	assert.Equal(t, in.Details.IOSUsageDescriptions, out.Details.IOSUsageDescriptions)
}
