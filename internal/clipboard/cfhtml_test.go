package clipboard

import (
	"bytes"
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeCFHTML_RoundTrip(t *testing.T) {
	for _, fragment := range []string{
		"<b>hello</b>",
		"",
		"<p>multi\r\nline</p>",
		"<span>päivää — 你好, мир 🙂</span>", // multibyte: offsets are bytes, not runes
	} {
		payload := EncodeCFHTML(fragment)
		got, ok := DecodeCFHTML(payload)
		require.True(t, ok, "fragment %q", fragment)
		assert.Equal(t, fragment, got)
	}
}

func TestEncodeCFHTML_OffsetsAreByteAccurate(t *testing.T) {
	fragment := "<i>héllo…</i>"
	payload := EncodeCFHTML(fragment)

	offset := func(key string) int {
		idx := bytes.Index(payload, []byte(key))
		require.GreaterOrEqual(t, idx, 0, key)
		num := payload[idx+len(key):]
		end := bytes.IndexAny(num, "\r\n")
		require.GreaterOrEqual(t, end, 0, key)
		val, err := strconv.Atoi(string(num[:end]))
		require.NoError(t, err, key)
		return val
	}

	startFragment := offset("StartFragment:")
	endFragment := offset("EndFragment:")
	assert.Equal(t, fragment, string(payload[startFragment:endFragment]))

	startHTML := offset("StartHTML:")
	endHTML := offset("EndHTML:")
	assert.Equal(t, byte('<'), payload[startHTML])
	assert.Equal(t, len(payload), endHTML)
}

// A trimmed-down capture of what Chromium places on the clipboard, including
// the SourceURL line.
func TestDecodeCFHTML_ChromiumPayload(t *testing.T) {
	fragment := `<b style="font-weight:bold;">copied</b>`
	doc := "<html>\r\n<body>\r\n<!--StartFragment-->" + fragment + "<!--EndFragment-->\r\n</body>\r\n</html>"
	header := "Version:0.9\r\nStartHTML:%010d\r\nEndHTML:%010d\r\nStartFragment:%010d\r\nEndFragment:%010d\r\nSourceURL:https://example.com/page\r\n"
	headerLen := len(fmt.Sprintf(header, 0, 0, 0, 0))
	prefixLen := len("<html>\r\n<body>\r\n<!--StartFragment-->")
	payload := fmt.Sprintf(header,
		headerLen,
		headerLen+len(doc),
		headerLen+prefixLen,
		headerLen+prefixLen+len(fragment)) + doc

	got, ok := DecodeCFHTML([]byte(payload))
	require.True(t, ok)
	assert.Equal(t, fragment, got)
}

// Some producers only write the comment markers with placeholder offsets.
func TestDecodeCFHTML_MarkerFallback(t *testing.T) {
	fragment := "<u>markers only</u>"
	payload := "Version:1.0\nStartHTML:-1\nEndHTML:-1\nStartFragment:-1\nEndFragment:-1\n" +
		"<html><body><!--StartFragment-->" + fragment + "<!--EndFragment--></body></html>"

	got, ok := DecodeCFHTML([]byte(payload))
	require.True(t, ok)
	assert.Equal(t, fragment, got)
}

func TestDecodeCFHTML_Invalid(t *testing.T) {
	for name, payload := range map[string]string{
		"empty":         "",
		"no header":     "<html><body>hi</body></html>",
		"out of bounds": "Version:0.9\r\nStartFragment:0000000100\r\nEndFragment:0000000200\r\nshort",
	} {
		_, ok := DecodeCFHTML([]byte(payload))
		assert.False(t, ok, name)
	}
}
