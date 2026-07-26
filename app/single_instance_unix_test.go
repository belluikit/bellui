//go:build (linux || darwin || freebsd || openbsd || netbsd) && !android && !ios && !wasm

package app

import (
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// forwardTo dials an existing primary socket and sends args, mimicking what a
// secondary launch does (forwardArgs uses os.Args[1:], the tests need arbitrary args).
func forwardTo(path string, args []string) error {
	conn, err := net.DialTimeout("unix", path, 2*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	return writeArgs(conn, args)
}

func TestSingleInstance_PrimaryThenForward(t *testing.T) {
	id := "com.example.si-test-" + t.Name()
	path := singleInstanceSocketPath(id)
	t.Cleanup(func() { os.Remove(path); os.Remove(path + ".lock") })

	got := make(chan []string, 1)
	primary, closer, err := singleInstanceBind(id, func(args []string) { got <- args })
	require.NoError(t, err)
	require.True(t, primary, "first bind should be primary")
	t.Cleanup(closer)

	// A second bind of the same ID must detect the primary and not become one.
	secondary, closer2, err := singleInstanceBind(id, nil)
	require.NoError(t, err)
	assert.False(t, secondary, "second bind should not be primary")
	assert.Nil(t, closer2, "secondary has nothing to close")

	// The secondary's forwarded os.Args[1:] should reach the primary's callback.
	select {
	case args := <-got:
		assert.Equal(t, os.Args[1:], args)
	case <-time.After(3 * time.Second):
		t.Fatal("primary never received forwarded arguments")
	}
}

func TestSingleInstance_ForwardsExactArgs(t *testing.T) {
	id := "com.example.si-args-" + t.Name()
	path := singleInstanceSocketPath(id)
	t.Cleanup(func() { os.Remove(path); os.Remove(path + ".lock") })

	got := make(chan []string, 1)
	primary, closer, err := singleInstanceBind(id, func(args []string) { got <- args })
	require.NoError(t, err)
	require.True(t, primary)
	t.Cleanup(closer)

	want := []string{"file:///tmp/doc.txt", "--flag", "value with spaces", "unicode-π"}
	require.NoError(t, forwardTo(path, want))

	select {
	case args := <-got:
		assert.Equal(t, want, args)
	case <-time.After(3 * time.Second):
		t.Fatal("arguments not delivered")
	}
}

func TestSingleInstance_StaleSocketRecovery(t *testing.T) {
	id := "com.example.si-stale-" + t.Name()
	path := singleInstanceSocketPath(id)
	t.Cleanup(func() { os.Remove(path); os.Remove(path + ".lock") })

	// Simulate a crashed primary: a genuine socket file lingers on disk with no
	// process listening. Disable unlink-on-close so the stale inode survives.
	ln, err := net.Listen("unix", path)
	require.NoError(t, err)
	ln.(*net.UnixListener).SetUnlinkOnClose(false)
	require.NoError(t, ln.Close())
	_, statErr := os.Stat(path)
	require.False(t, os.IsNotExist(statErr), "stale socket file should remain for the test")

	primary, closer, err := singleInstanceBind(id, nil)
	require.NoError(t, err)
	assert.True(t, primary, "should recover from a stale socket and become primary")
	if closer != nil {
		t.Cleanup(closer)
	}
}

func TestSingleInstance_RaceConvergence(t *testing.T) {
	id := "com.example.si-race-" + t.Name()
	path := singleInstanceSocketPath(id)
	t.Cleanup(func() { os.Remove(path); os.Remove(path + ".lock") })

	const n = 8
	var (
		mu       sync.Mutex
		primary  int
		closers  []func()
		start    = make(chan struct{})
		wg       sync.WaitGroup
		received = make(chan []string, n)
	)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			isPrimary, closer, err := singleInstanceBind(id, func(args []string) { received <- args })
			if err != nil {
				return
			}
			mu.Lock()
			if isPrimary {
				primary++
				closers = append(closers, closer)
			}
			mu.Unlock()
		}()
	}

	close(start)
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 1, primary, "exactly one racer should become primary")
	for _, c := range closers {
		if c != nil {
			c()
		}
	}
}

func TestSingleInstance_SocketPathPerUserAndStable(t *testing.T) {
	id := "com.example.si-path"
	p1 := singleInstanceSocketPath(id)
	p2 := singleInstanceSocketPath(id)
	assert.Equal(t, p1, p2, "path must be stable across calls for the same ID")
	assert.NotEqual(t, p1, singleInstanceSocketPath("com.example.other"))
	assert.Contains(t, p1, ".sock")
}
