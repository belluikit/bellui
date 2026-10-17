package commands

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

	page = renderIndex(t, false)
	assert.NotContains(t, page, "webgpu_support().then(")
	assert.Contains(t, page, "if (webgl_support()) {")
}

// In a browser with no WebGPU, the page of a WebGPU build says so instead of
// staying blank (#384). It runs the real page in headless Chrome with WebGPU
// disabled; the message appears before the page would fetch anything, so no
// wasm is needed.
func TestIndexReportsMissingWebGPUInChrome(t *testing.T) {
	chrome := findChrome()
	if chrome == "" {
		t.Skip("needs Chrome")
	}
	for _, tc := range []struct {
		name   string
		webgpu bool
		want   bool
	}{
		{"WebGPU build", true, true},
		{"WebGL build", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			index := filepath.Join(dir, "index.html")
			require.NoError(t, os.WriteFile(index, []byte(renderIndex(t, tc.webgpu)), 0o644))

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			out, err := exec.CommandContext(ctx, chrome, "--headless=new", "--disable-gpu",
				"--disable-features=WebGPU", "--user-data-dir="+filepath.Join(dir, "profile"),
				"--virtual-time-budget=5000", "--dump-dom", "file://"+index).Output()
			require.NoError(t, err)
			require.Contains(t, string(out), `id="action"`, "Chrome rendered the page")
			assert.Equal(t, tc.want, bytes.Contains(out, []byte(noWebGPUMessage)))
		})
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
