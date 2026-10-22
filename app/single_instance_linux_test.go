//go:build linux

package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// A file whose name is not UTF-8 (Latin-1 "café.txt") can be opened by the
// running copy from the path a second launch forwarded (#661). Linux file
// systems take any bytes in a name, and so this runs only there.
func TestSingleInstance_ForwardedNonUTF8PathOpens(t *testing.T) {
	file := filepath.Join(t.TempDir(), "caf\xe9.txt")
	require.NoError(t, os.WriteFile(file, []byte("contents"), 0o600))

	id := "com.example.si-latin1-" + t.Name()
	path := singleInstanceSocketPath(id)
	t.Cleanup(func() { os.Remove(path); os.Remove(path + ".lock") })

	got := make(chan []string, 1)
	primary, closer, err := singleInstanceBind(id, func(args []string) { got <- args })
	require.NoError(t, err)
	require.True(t, primary)
	t.Cleanup(closer)

	require.NoError(t, forwardTo(path, []string{file}))
	select {
	case args := <-got:
		require.Len(t, args, 1)
		data, err := os.ReadFile(args[0])
		require.NoError(t, err, "open the forwarded path %q", args[0])
		require.Equal(t, "contents", string(data))
	case <-time.After(3 * time.Second):
		t.Fatal("arguments not delivered")
	}
}
