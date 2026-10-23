//go:build !wasm

package tintcheck

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

// shaderGlob is the set of files the painter embeds (see the go:embed in
// internal/painter/webgpu/shaders.go), relative to this package.
const shaderGlob = "../shaders/*.wgsl"

// canaryName and canarySource are a shader Tint must reject, in the shape of
// the v1.2.0 bug: an implicit-derivative textureSample inside a branch on a
// per-fragment value. If the browser accepts it, the check is not measuring
// Tint's validation (a stub adapter, a flag that turned validation off), and
// a green result would mean nothing.
const (
	canaryName   = "canary-nonuniform-textureSample"
	canarySource = `@group(0) @binding(0) var t: texture_2d<f32>;
@group(0) @binding(1) var s: sampler;
@vertex fn vs_main(@builtin(vertex_index) i: u32) -> @builtin(position) vec4<f32> {
    return vec4<f32>(f32(i), 0.0, 0.0, 1.0);
}
@fragment fn fs_main(@builtin(position) p: vec4<f32>) -> @location(0) vec4<f32> {
    if (p.x > 0.5) {
        return textureSample(t, s, p.xy);
    }
    return vec4<f32>(0.0);
}
`
	canaryWant = "uniform control flow"
)

type shaderIn struct {
	Name string `json:"name"`
	Code string `json:"code"`
}

type compileMessage struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Line    int    `json:"line"`
	Col     int    `json:"col"`
}

type shaderOut struct {
	Name       string           `json:"name"`
	Messages   []compileMessage `json:"messages"`
	ScopeError string           `json:"scopeError"`
}

// errors returns the compile errors for one shader: getCompilationInfo
// messages of type "error", plus a validation error captured by an error
// scope around createShaderModule.
func (s shaderOut) errors() []string {
	var out []string
	for _, m := range s.Messages {
		if m.Type == "error" {
			out = append(out, fmt.Sprintf("%d:%d: %s", m.Line, m.Col, m.Message))
		}
	}
	if s.ScopeError != "" {
		out = append(out, "validation: "+s.ScopeError)
	}
	return out
}

type report struct {
	UserAgent string      `json:"userAgent"`
	Adapter   string      `json:"adapter"`
	Fallback  bool        `json:"fallback"`
	Error     string      `json:"error"`
	Shaders   []shaderOut `json:"shaders"`
}

// The page asks for an adapter (falling back to forceFallbackAdapter, which
// is SwiftShader in Chrome), compiles each shader, and posts what Tint said.
const page = `<!doctype html><meta charset="utf-8"><title>tint</title><script>
async function run() {
  const out = {userAgent: navigator.userAgent, adapter: "", fallback: false, error: "", shaders: []};
  try {
    if (!navigator.gpu) throw new Error("navigator.gpu is undefined: WebGPU is not enabled in this browser");
    let adapter = await navigator.gpu.requestAdapter();
    if (!adapter) {
      adapter = await navigator.gpu.requestAdapter({forceFallbackAdapter: true});
      out.fallback = true;
    }
    if (!adapter) throw new Error("requestAdapter returned null (also with forceFallbackAdapter)");
    const i = adapter.info || {};
    out.adapter = [i.vendor, i.architecture, i.device, i.description].filter(Boolean).join(" / ") || "(no adapter info)";
    const device = await adapter.requestDevice();
    const shaders = await (await fetch("/shaders")).json();
    for (const s of shaders) {
      device.pushErrorScope("validation");
      const mod = device.createShaderModule({label: s.name, code: s.code});
      const info = await mod.getCompilationInfo();
      const scope = await device.popErrorScope();
      out.shaders.push({
        name: s.name,
        messages: info.messages.map(m => ({type: m.type, message: m.message, line: m.lineNum, col: m.linePos})),
        scopeError: scope ? scope.message : "",
      });
    }
  } catch (e) {
    out.error = String(e && e.stack || e);
  }
  await fetch("/result", {method: "POST", body: JSON.stringify(out)});
}
run();
</script>`

// findChrome returns a Chrome or Chromium binary, or "" when there is none.
func findChrome() string {
	if p := os.Getenv("BELLUI_CHROME"); p != "" {
		return p
	}
	for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "chrome"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	if runtime.GOOS == "darwin" {
		for _, p := range []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		} {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return ""
}

// unavailable skips the test, or fails it when BELLUI_REQUIRE_TINT=1 says the
// environment promised a browser with WebGPU. A CI job must never read a skip
// as a pass.
func unavailable(t *testing.T, format string, args ...any) {
	t.Helper()
	msg := fmt.Sprintf(format, args...)
	if os.Getenv("BELLUI_REQUIRE_TINT") == "1" {
		t.Fatalf("BELLUI_REQUIRE_TINT=1 but the Tint check cannot run: %s", msg)
	}
	t.Skipf("skipping the Tint check (set BELLUI_REQUIRE_TINT=1 to make this a failure): %s", msg)
}

func loadShaders(t *testing.T) []shaderIn {
	t.Helper()
	files, err := filepath.Glob(shaderGlob)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatalf("no shaders match %s: the check would measure nothing", shaderGlob)
	}
	sort.Strings(files)
	var out []shaderIn
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, shaderIn{Name: filepath.Base(f), Code: string(b)})
	}
	return out
}

// runChrome serves the page, runs headless Chrome on it, and returns the
// report the page posts back.
func runChrome(t *testing.T, chrome string, shaders []shaderIn) report {
	t.Helper()
	got := make(chan report, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, page)
	})
	mux.HandleFunc("/shaders", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(shaders)
	})
	mux.HandleFunc("/result", func(w http.ResponseWriter, r *http.Request) {
		var rep report
		if err := json.NewDecoder(r.Body).Decode(&rep); err != nil {
			rep.Error = "decoding the page's report: " + err.Error()
		}
		select {
		case got <- rep:
		default:
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	// http://127.0.0.1 is a secure context, so navigator.gpu is exposed.
	// --no-sandbox: Ubuntu 24.04's AppArmor policy blocks the user namespaces
	// Chrome's sandbox needs (see web_tests.yml); the page is our own.
	// SwiftShader is the software adapter, so no GPU is needed.
	cmd := exec.CommandContext(
		ctx, chrome,
		"--headless=new",
		"--no-sandbox",
		"--no-first-run",
		"--no-default-browser-check",
		"--user-data-dir="+t.TempDir(),
		"--enable-unsafe-webgpu",
		"--enable-features=Vulkan",
		"--use-angle=swiftshader",
		"--use-webgpu-adapter=swiftshader",
		srv.URL+"/",
	)
	var stderr bytes.Buffer
	cmd.Stdout = &stderr
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		unavailable(t, "starting %s: %v", chrome, err)
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	defer func() {
		cancel()
		<-exited
	}()

	select {
	case rep := <-got:
		return rep
	case err := <-exited:
		t.Fatalf("%s exited before the page reported (%v); its output:\n%s", chrome, err, stderr.String())
	case <-ctx.Done():
		t.Fatalf("no report from %s within the timeout; its output:\n%s", chrome, stderr.String())
	}
	return report{}
}

// TestShadersCompileWithTint compiles every painter shader with Chrome's
// WebGPU compiler and fails on any error Tint reports (#685).
func TestShadersCompileWithTint(t *testing.T) {
	chrome := findChrome()
	if chrome == "" {
		unavailable(t, "no Chrome or Chromium found (PATH, BELLUI_CHROME, or the macOS app path)")
	}
	shaders := loadShaders(t)
	all := append([]shaderIn{{Name: canaryName, Code: canarySource}}, shaders...)

	rep := runChrome(t, chrome, all)
	if rep.Error != "" {
		unavailable(t, "the WebGPU page failed in %s (%s): %s", chrome, rep.UserAgent, rep.Error)
	}
	if len(rep.Shaders) != len(all) {
		t.Fatalf("the page compiled %d modules, want %d", len(rep.Shaders), len(all))
	}

	canary := rep.Shaders[0]
	if canary.Name != canaryName {
		t.Fatalf("first result is %q, want the canary", canary.Name)
	}
	if errs := canary.errors(); len(errs) == 0 || !strings.Contains(strings.Join(errs, "\n"), canaryWant) {
		t.Fatalf("Tint accepted the canary shader, which it must reject with %q (got %q): "+
			"this browser is not validating WGSL the way Chrome does, so a pass would mean nothing",
			canaryWant, errs)
	}

	for _, s := range rep.Shaders[1:] {
		for _, m := range s.Messages {
			if m.Type != "error" {
				t.Logf("%s:%d:%d: %s: %s", s.Name, m.Line, m.Col, m.Type, m.Message)
			}
		}
		if errs := s.errors(); len(errs) > 0 {
			t.Errorf("Chrome's WebGPU compiler (Tint) rejects %s:\n  %s", s.Name, strings.Join(errs, "\n  "))
		}
	}
	// Positive marker: the check ran, on what, and against which compiler.
	t.Logf("compiled %d shaders via Tint (canary rejected as expected), adapter=%q fallback=%v, browser=%q",
		len(shaders), rep.Adapter, rep.Fallback, rep.UserAgent)
}
