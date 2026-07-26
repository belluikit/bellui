package webgpu

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestShaderSourceLookup verifies every advertised shader module is embedded,
// non-empty, and exposes the expected entry points; and that unknown names
// return nil.
func TestShaderSourceLookup(t *testing.T) {
	for _, name := range Names() {
		src := ShaderSource(name)
		if len(src) == 0 {
			t.Errorf("shader %q: missing or empty source", name)
			continue
		}
		text := string(src)
		if !strings.Contains(text, "fn "+VertexEntry) {
			t.Errorf("shader %q: missing vertex entry %q", name, VertexEntry)
		}
		if name == "blur" {
			for _, entry := range []string{BlurFragmentEntryH, BlurFragmentEntryV} {
				if !strings.Contains(text, "fn "+entry) {
					t.Errorf("shader %q: missing fragment entry %q", name, entry)
				}
			}
		} else if !strings.Contains(text, "fn "+FragmentEntry) {
			t.Errorf("shader %q: missing fragment entry %q", name, FragmentEntry)
		}
	}

	if ShaderSource("does_not_exist") != nil {
		t.Error("ShaderSource for unknown name should return nil")
	}
}

// TestShadersValidateWithNaga runs each WGSL module through the naga validator
// when it is available, giving real syntax + type checking without a GPU. It is
// skipped (not failed) when naga is not installed, so the suite stays green in
// environments without the Rust toolchain. Install with: cargo install naga-cli.
func TestShadersValidateWithNaga(t *testing.T) {
	naga, err := exec.LookPath("naga")
	if err != nil {
		if home, herr := os.UserHomeDir(); herr == nil {
			cand := filepath.Join(home, ".cargo", "bin", "naga")
			if _, serr := os.Stat(cand); serr == nil {
				naga = cand
			}
		}
	}
	if naga == "" {
		t.Skip("naga not found in PATH or ~/.cargo/bin; skipping WGSL validation")
	}

	for _, name := range Names() {
		t.Run(name, func(t *testing.T) {
			src := ShaderSource(name)
			tmp := filepath.Join(t.TempDir(), name+".wgsl")
			if err := os.WriteFile(tmp, src, 0o600); err != nil {
				t.Fatalf("write temp shader: %v", err)
			}
			cmd := exec.Command(naga, tmp)
			var out bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &out
			if err := cmd.Run(); err != nil {
				t.Fatalf("naga validation failed for %q: %v\n%s", name, err, out.String())
			}
		})
	}
}
