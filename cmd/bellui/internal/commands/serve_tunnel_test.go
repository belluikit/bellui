package commands

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeCloudflared puts a cloudflared on the PATH that records its arguments
// in the returned file and then runs script.
func fakeCloudflared(t *testing.T, script string) (argsFile string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake cloudflared is a shell script")
	}
	dir := t.TempDir()
	argsFile = filepath.Join(dir, "args")
	body := "#!/bin/sh\necho \"$@\" > '" + argsFile + "'\n" + script
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cloudflared"), []byte(body), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return argsFile
}

// What a quick tunnel prints, trimmed from cloudflared 2026.10.7.
const quickTunnelLog = `echo '2026-11-21T03:00:00Z INF Requesting new quick Tunnel on trycloudflare.com...' >&2
echo '2026-11-21T03:00:01Z INF +--------------------------------------------------------------------------------------------+' >&2
echo '2026-11-21T03:00:01Z INF |  Your quick Tunnel has been created! Visit it at (it may take some time to be reachable):  |' >&2
echo '2026-11-21T03:00:01Z INF |  https://quiet-river-1234-example.trycloudflare.com                                        |' >&2
`

// serve --tunnel starts cloudflared at serve's own port, prints the address
// it gives, and stops it with serve (#1359).
func TestServeTunnelPrintsTheTunnelAddress(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "pid")
	argsFile := fakeCloudflared(t, "echo $$ > '"+pidFile+"'\n"+quickTunnelLog+"exec sleep 60\n")
	s := &Server{appData: &appData{}, tunnel: true}
	port, out, stop, _ := startRun(t, s, nil)

	require.Eventually(t, func() bool { return strings.Contains(out.String(), "Public tunnel:") }, 10*time.Second, 20*time.Millisecond)
	assert.Contains(t, out.String(), "Public tunnel: https://quiet-river-1234-example.trycloudflare.com\n")
	args, err := os.ReadFile(argsFile)
	require.NoError(t, err)
	assert.Equal(t, "tunnel --no-autoupdate --url http://127.0.0.1:"+strconv.Itoa(port)+"\n", string(args))
	assert.NoError(t, stop(), "ending serve ends the tunnel without an error")

	pidText, err := os.ReadFile(pidFile)
	require.NoError(t, err)
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidText)))
	require.NoError(t, err)
	// A cloudflared left running would keep the app public after serve ends.
	assert.Eventually(t, func() bool { return !processAlive(pid) }, 5*time.Second, 20*time.Millisecond, "cloudflared stops with serve")
}

// With --https the tunnel's origin is serve's self-signed page, which
// cloudflared must be told not to verify.
func TestServeTunnelToHTTPSOrigin(t *testing.T) {
	argsFile := fakeCloudflared(t, quickTunnelLog+"exec sleep 60\n")
	s := &Server{appData: &appData{}, tunnel: true, https: true}
	port, out, stop, _ := startRun(t, s, nil)

	require.Eventually(t, func() bool { return strings.Contains(out.String(), "Public tunnel:") }, 10*time.Second, 20*time.Millisecond)
	args, err := os.ReadFile(argsFile)
	require.NoError(t, err)
	assert.Equal(t, "tunnel --no-autoupdate --url https://127.0.0.1:"+strconv.Itoa(port)+" --no-tls-verify\n", string(args))
	assert.NoError(t, stop())
}

// A cloudflared that fails before giving an address fails serve, with what it
// said; one that dies later closes serve too, instead of leaving a dead link.
func TestServeTunnelFailures(t *testing.T) {
	t.Run("exits before an address", func(t *testing.T) {
		fakeCloudflared(t, "echo 'ERR failed to request quick Tunnel: 429 Too Many Requests' >&2\nexit 1\n")
		_, _, _, wait := startRun(t, &Server{appData: &appData{}, tunnel: true}, nil)
		err := wait()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cloudflared exited before opening a tunnel")
		assert.Contains(t, err.Error(), "429 Too Many Requests")
	})
	t.Run("service address before the tunnel", func(t *testing.T) {
		fakeCloudflared(t, "echo 'ERR Error unmarshaling QuickTunnel response: Post \"https://api.trycloudflare.com/tunnel\": EOF' >&2\nexit 1\n")
		_, _, _, wait := startRun(t, &Server{appData: &appData{}, tunnel: true}, nil)
		err := wait()
		require.Error(t, err, "api.trycloudflare.com is not the tunnel")
		assert.Contains(t, err.Error(), "cloudflared exited before opening a tunnel")
	})
	t.Run("address on stdout", func(t *testing.T) {
		fakeCloudflared(t, strings.ReplaceAll(quickTunnelLog, " >&2", "")+"exec sleep 60\n")
		_, out, stop, _ := startRun(t, &Server{appData: &appData{}, tunnel: true}, nil)
		require.Eventually(t, func() bool { return strings.Contains(out.String(), "Public tunnel:") }, 10*time.Second, 20*time.Millisecond)
		assert.NoError(t, stop())
	})
	t.Run("exits after the address", func(t *testing.T) {
		fakeCloudflared(t, quickTunnelLog+"sleep 0.3\nexit 1\n")
		s := &Server{appData: &appData{}, tunnel: true}
		_, out, _, wait := startRun(t, s, nil)
		require.Eventually(t, func() bool { return strings.Contains(out.String(), "Public tunnel:") }, 10*time.Second, 20*time.Millisecond)
		err := wait()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "the tunnel closed")
	})
	t.Run("no address in time", func(t *testing.T) {
		prev := tunnelStartTimeout
		tunnelStartTimeout = 300 * time.Millisecond
		t.Cleanup(func() { tunnelStartTimeout = prev })
		fakeCloudflared(t, "exec sleep 60\n")
		_, _, _, wait := startRun(t, &Server{appData: &appData{}, tunnel: true}, nil)
		err := wait()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "did not open a tunnel")
	})
	t.Run("no cloudflared", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		_, _, _, wait := startRun(t, &Server{appData: &appData{}, tunnel: true}, nil)
		err := wait()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--tunnel needs cloudflared")
	})
}
