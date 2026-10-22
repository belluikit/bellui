package app

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"testing"

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
