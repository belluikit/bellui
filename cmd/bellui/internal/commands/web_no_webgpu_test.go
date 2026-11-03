package commands

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/cmd/bellui/internal/templates"
)

const noWebGPUMessage = "This browser does not support WebGPU"

func TestUsesWebGPU(t *testing.T) {
	assert.True(t, usesWebGPU(""))
	assert.True(t, usesWebGPU("debug"))
	assert.True(t, usesWebGPU("debug,glx"))
	assert.False(t, usesWebGPU("gl"))
	assert.False(t, usesWebGPU("debug,gl"))
}

func renderIndex(t *testing.T, webgpu bool) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, templates.IndexHTML.Execute(&buf, webData{AppName: "Notes", AppVersion: "1.0.0", WasmFile: "Notes.wasm", WebGPU: webgpu}))
	return buf.String()
}

// A WebGPU build checks for an adapter before loading; a WebGL build keeps
// its WebGL check (#384).
func TestIndexChecksTheRendererTheBuildNeeds(t *testing.T) {
	page := renderIndex(t, true)
	assert.Contains(t, page, "webgpu_support().then(")
	assert.Contains(t, page, noWebGPUMessage)
	// A WebGPU build must not also demand WebGL (#339).
	assert.NotContains(t, page, "if (webgl_support()) {")

	page = renderIndex(t, false)
	assert.NotContains(t, page, "webgpu_support().then(")
	assert.Contains(t, page, "if (webgl_support()) {")
}

// Stubs that make the browser lack WebGPU the two ways the issue saw, set
// before the page's own scripts run. Chrome's --disable-features=WebGPU alone
// is not dependable: on CI's macOS machines navigator.gpu stayed and the
// adapter request never settled.
const (
	stubNoWebGPU  = `Object.defineProperty(Navigator.prototype, "gpu", {get() { return undefined; }, configurable: true});`
	stubNoAdapter = `Object.defineProperty(Navigator.prototype, "gpu", {get() { return {requestAdapter: async () => null}; }, configurable: true});`
)

// In a browser with no WebGPU, the page of a WebGPU build says so instead of
// staying blank (#384). It runs the real page in headless Chrome; the message
// appears before the page would fetch anything, so no wasm is needed.
func TestIndexReportsMissingWebGPUInChrome(t *testing.T) {
	chrome := findChrome()
	if chrome == "" {
		t.Skip("needs Chrome")
	}
	// The first browser start on CI's macOS machines took 70-80 s, whichever
	// case ran first, while every later start took under a second (locally
	// 5 s, then 0.4 s). Starting Chrome without a page (--version) is not
	// enough. Pay that cost once on a blank page, with a generous limit, and
	// record it.
	warmDir := t.TempDir()
	warm, cancelWarm := context.WithTimeout(context.Background(), 4*time.Minute)
	start := time.Now()
	_ = exec.CommandContext(warm, chrome, append(chromeFlags(warmDir), "--dump-dom", "about:blank")...).Run()
	cancelWarm()
	t.Logf("Chrome warm-up (blank page) took %v", time.Since(start).Round(time.Millisecond))
	for _, tc := range []struct {
		name   string
		webgpu bool
		stub   string
		want   bool
	}{
		{"WebGPU build, no navigator.gpu", true, stubNoWebGPU, true},
		{"WebGPU build, gpu but no adapter", true, stubNoAdapter, true},
		{"WebGL build", false, stubNoWebGPU, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			page := strings.Replace(renderIndex(t, tc.webgpu), "<head>", "<head><script>"+tc.stub+"</script>", 1)
			index := filepath.Join(dir, "index.html")
			require.NoError(t, os.WriteFile(index, []byte(page), 0o644))

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, chrome,
				append(chromeFlags(dir), "--virtual-time-budget=3000", "--dump-dom", "file://"+index)...)
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			out, err := cmd.Output()
			if err != nil && chromeTeardownTimedOut(err, out, stderr.String()) {
				// The page was rendered and dumped; only Chrome's own shutdown
				// hung, which its watchdog ends with exit status 2 (#518).
				t.Logf("Chrome hung shutting down after dumping the page; its watchdog ended it")
				err = nil
			}
			// Chrome says why it failed only on stderr (#518).
			require.NoError(t, err, "Chrome's stderr:\n%s", stderr.String())
			require.Contains(t, string(out), `id="action"`, "Chrome rendered the page")
			assert.Equal(t, tc.want, bytes.Contains(out, []byte(noWebGPUMessage)))
		})
	}
}

// chromeTeardownTimedOut reports whether Chrome failed only in shutting down:
// it dumped the whole page, then its teardown watchdog ended it with exit
// status 2. Seen intermittently on CI's macOS runners (#518).
func chromeTeardownTimedOut(err error, out []byte, stderr string) bool {
	var exit *exec.ExitError
	return errors.As(err, &exit) && exit.ExitCode() == 2 &&
		strings.Contains(stderr, "Teardown watchdog expired") &&
		bytes.Contains(out, []byte("</html>"))
}

// chromeFlags runs Chrome headless with a fresh profile in dir, skipping
// first-run work and the login keychain, which a test page needs neither of.
func chromeFlags(dir string) []string {
	return []string{
		"--headless=new", "--disable-gpu", "--no-first-run", "--no-default-browser-check",
		"--use-mock-keychain", "--password-store=basic", "--disable-component-update",
		"--user-data-dir=" + filepath.Join(dir, "profile"),
	}
}

func findChrome() string {
	candidates := []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser"}
	if runtime.GOOS == "darwin" {
		candidates = append([]string{"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"}, candidates...)
	}
	for _, c := range candidates {
		if p, err := exec.LookPath(c); err == nil {
			return p
		}
	}
	return ""
}

// Only an exit 2 from the teardown watchdog after a whole page passes; a
// cut-off page, another exit code or another message still fails (#518).
func TestChromeTeardownTimedOut(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("needs sh to make an exit status")
	}
	exitWith := func(code string) error {
		return exec.Command("sh", "-c", "exit "+code).Run()
	}
	page := []byte(`<html><body id="action"></body></html>`)
	watchdog := "Teardown watchdog expired; recording dump and terminating."
	assert.True(t, chromeTeardownTimedOut(exitWith("2"), page, watchdog))
	assert.False(t, chromeTeardownTimedOut(exitWith("2"), page[:20], watchdog), "page cut off")
	assert.False(t, chromeTeardownTimedOut(exitWith("1"), page, watchdog), "other exit code")
	assert.False(t, chromeTeardownTimedOut(exitWith("2"), page, "something else"), "other failure")
	assert.False(t, chromeTeardownTimedOut(context.DeadlineExceeded, page, watchdog), "not an exit")
}
