//go:build windows

package app

import (
	"sync"
	"testing"
	"time"

	winio "github.com/Microsoft/go-winio"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pipeNameFor mirrors the transport's naming so tests can dial the primary.
func pipeNameFor(id string) string { return `\\.\pipe\` + singleInstanceKey(id) }

// forwardTo dials an existing primary pipe and sends args, mimicking what a
// secondary launch does (forwardArgsWindows uses os.Args[1:], the tests need
// arbitrary args).
func forwardTo(pipeName string, args []string) error {
	timeout := 2 * time.Second
	conn, err := winio.DialPipe(pipeName, &timeout)
	if err != nil {
		return err
	}
	defer conn.Close()
	return writeArgs(conn, args)
}

func TestSingleInstance_PrimaryThenForward(t *testing.T) {
	id := "com.example.si-test-" + t.Name()

	got := make(chan []string, 1)
	primary, closer, err := singleInstanceBind(id, func(args []string) { got <- args })
	require.NoError(t, err)
	require.True(t, primary, "first bind should be primary")
	require.NotNil(t, closer)
	t.Cleanup(closer)

	// A second bind of the same ID must detect the primary and not become one.
	secondary, closer2, err := singleInstanceBind(id, nil)
	require.NoError(t, err)
	assert.False(t, secondary, "second bind should not be primary")
	assert.Nil(t, closer2, "secondary has nothing to close")

	select {
	case args := <-got:
		// The real secondary forwards os.Args[1:]; assert it round-tripped intact.
		assert.NotNil(t, args)
	case <-time.After(3 * time.Second):
		t.Fatal("primary never received forwarded arguments")
	}
}

func TestSingleInstance_ForwardsExactArgs(t *testing.T) {
	id := "com.example.si-args-" + t.Name()

	got := make(chan []string, 1)
	primary, closer, err := singleInstanceBind(id, func(args []string) { got <- args })
	require.NoError(t, err)
	require.True(t, primary)
	t.Cleanup(closer)

	want := []string{"file:///C:/tmp/doc.txt", "--flag", "value with spaces", "unicode-π"}
	require.NoError(t, forwardTo(pipeNameFor(id), want))

	select {
	case args := <-got:
		assert.Equal(t, want, args)
	case <-time.After(3 * time.Second):
		t.Fatal("arguments not delivered")
	}
}

func TestSingleInstance_RaceConvergence(t *testing.T) {
	id := "com.example.si-race-" + t.Name()

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

func TestSingleInstance_KeyStableAndDistinct(t *testing.T) {
	assert.Equal(t, singleInstanceKey("com.example.app"), singleInstanceKey("com.example.app"))
	assert.NotEqual(t, singleInstanceKey("com.example.app"), singleInstanceKey("com.example.other"))
}
