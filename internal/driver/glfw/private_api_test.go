//go:build !wasm && !mobile

package glfw

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// undeclaredSelectors are AppKit selectors the macOS SDK does not declare. A
// call to one puts the selector string in every macOS binary, which App Review
// can reject as private API (guideline 2.5.1, #934).
var undeclaredSelectors = []string{"setAppleMenu"}

// glfwVersionsWithSetAppleMenu are the releases of the GLFW fork that still
// call setAppleMenu: (belluikit/glfw#6). Pinning one of them skips the fork
// half of the check with a message; any other version must be clean.
var glfwVersionsWithSetAppleMenu = map[string]bool{"v1.10.0": true}

func TestNoUndeclaredAppKitSelectors_Bellui(t *testing.T) {
	assertNoUndeclaredSelectors(t, "menu_darwin.m")
}

func TestNoUndeclaredAppKitSelectors_GLFWFork(t *testing.T) {
	out, err := exec.Command("go", "list", "-m", "-json", "github.com/belluikit/glfw").Output()
	if err != nil {
		t.Fatalf("go list -m github.com/belluikit/glfw: %v", err)
	}
	var mod struct {
		Version, Dir string
		Replace      *struct{ Version, Dir string }
	}
	if err := json.Unmarshal(out, &mod); err != nil {
		t.Fatal(err)
	}
	if mod.Replace != nil {
		mod.Version, mod.Dir = mod.Replace.Version, mod.Replace.Dir
	}
	if mod.Dir == "" {
		t.Fatalf("github.com/belluikit/glfw %s is not in the module cache", mod.Version)
	}
	if glfwVersionsWithSetAppleMenu[mod.Version] {
		t.Skipf("github.com/belluikit/glfw %s still calls setAppleMenu: (belluikit/glfw#6); "+
			"move the pin to a release without it (#934)", mod.Version)
	}
	assertNoUndeclaredSelectors(t, filepath.Join(mod.Dir, "glfw", "src", "cocoa_init.m"))
}

func assertNoUndeclaredSelectors(t *testing.T, path string) {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "NSApp") {
		t.Fatalf("%s does not look like the AppKit source it should be", path)
	}
	for _, sel := range undeclaredSelectors {
		for i, line := range strings.Split(string(src), "\n") {
			code, _, _ := strings.Cut(line, "//")
			if strings.Contains(code, sel) {
				t.Errorf("%s:%d calls the undeclared selector %s: (#934): %s", path, i+1, sel, strings.TrimSpace(line))
			}
		}
	}
}
