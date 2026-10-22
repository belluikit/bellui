package app

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
)

func TestSetSingleInstance_RequiresID(t *testing.T) {
	a := &belluiApp{} // no uniqueID, and default metadata has an empty ID
	primary, err := a.SetSingleInstance(nil)
	assert.Error(t, err)
	assert.False(t, primary)
	assert.False(t, a.siEnabled)
}

func TestSetSingleInstance_PrimaryAndIdempotent(t *testing.T) {
	a := NewWithID("io.bellui.test.single-instance").(*belluiApp)
	t.Cleanup(func() {
		if a.siCloser != nil {
			a.siCloser()
		}
	})

	si, ok := bellui.App(a).(bellui.SingleInstanceApp)
	require.True(t, ok, "app should implement bellui.SingleInstanceApp")

	primary, err := si.SetSingleInstance(nil)
	require.NoError(t, err)
	assert.True(t, primary, "sole running instance is the primary")
	assert.True(t, a.siEnabled)

	// A second call is a no-op that reports the same role without rebinding.
	primaryAgain, err := si.SetSingleInstance(func([]string) {})
	require.NoError(t, err)
	assert.Equal(t, primary, primaryAgain)
}

// Forwarded arguments are PXF, and a running copy still reads the JSON array a
// version before #523 sends (#523).
func TestForwardedArgs_Format(t *testing.T) {
	args := []string{"--open", "file with spaces.txt", `quote"and\backslash`, "ünïcode", ""}

	var buf bytes.Buffer
	require.NoError(t, writeArgs(&buf, args))
	payload := buf.Bytes()[4:]
	assert.NotEqual(t, byte('['), payload[0], "written as PXF, not a JSON array")
	got, err := readArgs(&buf)
	require.NoError(t, err)
	assert.Equal(t, args, got)

	legacy, err := json.Marshal(args)
	require.NoError(t, err)
	buf.Reset()
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(legacy)))
	buf.Write(hdr[:])
	buf.Write(legacy)
	got, err = readArgs(&buf)
	require.NoError(t, err)
	assert.Equal(t, args, got)

	buf.Reset()
	require.NoError(t, writeArgs(&buf, nil))
	got, err = readArgs(&buf)
	require.NoError(t, err)
	assert.Empty(t, got, "a launch with no arguments")
}

// TestForwardedArgs_GoldenDecode guards the forwarded-arguments message (#523):
// a running copy must read what a later version's launch sends, so it decodes
// a committed testdata/forwarded_args.pxf payload. Do not regenerate it to make
// the test pass; only when the format changes on purpose:
//
//	BELLUI_REGEN_GOLDEN=1 go test -run TestForwardedArgs_GoldenDecode ./app/
func TestForwardedArgs_GoldenDecode(t *testing.T) {
	path := filepath.Join("testdata", "forwarded_args.pxf")
	golden := []string{"--open", "/tmp/a file.txt"}
	if os.Getenv("BELLUI_REGEN_GOLDEN") != "" {
		var buf bytes.Buffer
		require.NoError(t, writeArgs(&buf, golden))
		require.NoError(t, os.WriteFile(path, buf.Bytes()[4:], 0o600))
	}

	payload, err := os.ReadFile(path)
	require.NoError(t, err)
	var buf bytes.Buffer
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(payload)))
	buf.Write(hdr[:])
	buf.Write(payload)
	got, err := readArgs(&buf)
	require.NoError(t, err)
	assert.Equal(t, golden, got)
}

// An argument that is not valid UTF-8, such as a Latin-1 file name on Linux,
// reaches the running copy byte for byte, with the other arguments, so the
// file it names can be opened (#661). The JSON message before #523, and PXF
// before this escape, delivered "caf\uFFFD.txt" instead, a file that does not
// exist.
func TestForwardedArgs_InvalidUTF8(t *testing.T) {
	args := []string{"--open", "/home/u/caf\xe9.txt", "/home/u/\xff\xfe", "café.txt"}

	var buf bytes.Buffer
	require.NoError(t, writeArgs(&buf, args))
	got, err := readArgs(&buf)
	require.NoError(t, err)
	assert.Equal(t, args, got)
	assert.Equal(t, []byte("/home/u/caf\xe9.txt"), []byte(got[1]), "the exact bytes")
}

// The escape is part of the wire format, so a running copy and a launch of
// another version agree on it: these encodings must never change (#661).
func TestForwardedArgs_EscapeForm(t *testing.T) {
	for in, want := range map[string]string{
		"/tmp/café.txt":     "/tmp/café.txt",               // valid UTF-8 is kept
		"/tmp/caf\xe9.txt":  "/tmp/caf\uefe9.txt",          // a Latin-1 byte
		"/tmp/\xc3":         "/tmp/\uefc3",                 // a cut-off sequence
		"/tmp/\xed\xa0\x80": "/tmp/\uefed\uefa0\uef80",     // an encoded surrogate
		"/tmp/\uef9a.txt":   "/tmp/\uefee\uefbe\uef9a.txt", // a real escape-range code point
		"":                  "",
	} {
		e := escapeArg(in)
		assert.Equal(t, want, e, "escapeArg(%q)", in)
		assert.True(t, utf8.ValidString(e), "escapeArg(%q) is valid UTF-8", in)
		assert.Equal(t, in, unescapeArg(e), "unescapeArg(escapeArg(%q))", in)
	}
}

// Any byte string survives the escape, weighted towards the bytes that start
// or continue a sequence in the escape range.
func TestForwardedArgs_EscapeRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewPCG(661, 523))
	tricky := []byte{0xEE, 0xBE, 0xBF, 0xC3, 0xED, 0x80}
	for range 200_000 {
		b := make([]byte, rng.IntN(24))
		for j := range b {
			if rng.IntN(3) == 0 {
				b[j] = tricky[rng.IntN(len(tricky))]
			} else {
				b[j] = byte(rng.IntN(256))
			}
		}
		e := escapeArg(string(b))
		if !utf8.ValidString(e) || unescapeArg(e) != string(b) {
			t.Fatalf("%q escapes to %q, which reads back as %q", b, e, unescapeArg(e))
		}
	}
}
