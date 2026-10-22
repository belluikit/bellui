package mobile

import (
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInfoPlistUsageDescriptions(t *testing.T) {
	buf := &bytes.Buffer{}
	err := infoplistTmpl.Execute(buf, infoplistTmplData{
		BundleID: "com.example.testing",
		Name:     "Testing",
		Version:  "1.0",
		Build:    1,
		UsageDescriptions: sortedPlistEntries(map[string]string{
			"NSMicrophoneUsageDescription": "Record voice notes.",
			"NSCameraUsageDescription":     "Scan documents.",
		}),
	})
	assert.NoError(t, err)
	out := buf.String()

	assert.Contains(t, out, "<key>NSCameraUsageDescription</key>")
	assert.Contains(t, out, "<string>Scan documents.</string>")
	assert.Contains(t, out, "<key>NSMicrophoneUsageDescription</key>")
	assert.Contains(t, out, "<string>Record voice notes.</string>")

	// Deterministic: keys are emitted in sorted order (Camera before Microphone),
	// so repeated packaging produces a byte-identical Info.plist.
	assert.Less(t, strings.Index(out, "NSCameraUsageDescription"),
		strings.Index(out, "NSMicrophoneUsageDescription"))

	// The injected keys live inside the plist dict, before its close.
	assert.Less(t, strings.Index(out, "NSCameraUsageDescription"),
		strings.LastIndex(out, "</dict>"))
}

func TestInfoPlistNoUsageDescriptions(t *testing.T) {
	buf := &bytes.Buffer{}
	err := infoplistTmpl.Execute(buf, infoplistTmplData{
		BundleID: "com.example.testing",
		Name:     "Testing",
		Version:  "1.0",
		Build:    1,
	})
	assert.NoError(t, err)
	out := buf.String()

	// No usage-description keys when none are declared, and the plist is still valid.
	assert.NotContains(t, out, "UsageDescription")
	assert.True(t, strings.HasSuffix(strings.TrimSpace(out), "</plist>"))
}

// Values from the manifest are user text, so the generated Info.plist must
// escape them: a raw & or < makes the plist unparseable and the build fails
// in xcodebuild with an unhelpful error.
func TestInfoPlistEscapesValues(t *testing.T) {
	buf := &bytes.Buffer{}
	err := infoplistTmpl.Execute(buf, infoplistTmplData{
		BundleID: "com.example.testing",
		Name:     "Tom & Jerry",
		Version:  "1.0",
		Build:    1,
		UsageDescriptions: sortedPlistEntries(map[string]string{
			"NSCameraUsageDescription": "Scan <documents> & receipts.",
		}),
	})
	assert.NoError(t, err)
	assert.NoError(t, xmlWellFormed(buf.Bytes()), buf.String())
	assert.Contains(t, buf.String(), "<string>Scan &lt;documents&gt; &amp; receipts.</string>")
	assert.Contains(t, buf.String(), "<string>Tom &amp; Jerry</string>")
}

func xmlWellFormed(b []byte) error {
	d := xml.NewDecoder(bytes.NewReader(b))
	for {
		if _, err := d.Token(); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}
