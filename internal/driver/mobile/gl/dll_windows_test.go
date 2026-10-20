//go:build windows

package gl

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

const installHelperEnv = "BELLUI_ANGLE_INSTALL_HELPER"

// TestConcurrentInstall pins #438: test binaries started together on a fresh
// machine all miss ANGLE, all download it, and all install it into the same
// directory. Each process here gets an empty LOCALAPPDATA, Program Files,
// GOPATH and temp directory, so every lookup misses and each one downloads and
// installs; every one of them must then load the DLLs.
func TestConcurrentInstall(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	tmp := filepath.Join(dir, "tmp")
	if err := os.Mkdir(tmp, 0o755); err != nil {
		t.Fatal(err)
	}
	env := append(
		os.Environ(),
		installHelperEnv+"=1",
		"LOCALAPPDATA="+dir,
		"ProgramFiles(x86)="+filepath.Join(dir, "none"),
		"GOPATH="+filepath.Join(dir, "none"),
		"TMP="+tmp, "TEMP="+tmp,
	)

	const n = 6
	errs := make([]string, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := exec.Command(exe, "-test.run=^TestInstallHelper$", "-test.count=1")
			cmd.Env = env
			if out, err := cmd.CombinedOutput(); err != nil {
				errs[i] = fmt.Sprintf("%v: %s", err, out)
			}
		}()
	}
	wg.Wait()

	var failed []string
	offline := true
	for _, e := range errs {
		if e == "" {
			offline = false
			continue
		}
		failed = append(failed, e)
		if !strings.Contains(e, "error fetching") && !strings.Contains(e, "dial tcp") && !strings.Contains(e, "no such host") {
			offline = false
		}
	}
	if len(failed) == n && offline {
		t.Skipf("ANGLE could not be downloaded: %s", failed[0])
	}
	for _, e := range failed {
		t.Error(e)
	}
	// A positive marker that the installs ran: the DLLs are in the directory
	// all the processes installed into.
	if !containsDLLs(filepath.Join(dir, "GoGL", runtime.GOARCH)) {
		t.Errorf("no complete ANGLE install in %s", filepath.Join(dir, "GoGL"))
	}
}

// TestInstallHelper is the process TestConcurrentInstall starts several of.
func TestInstallHelper(t *testing.T) {
	if os.Getenv(installHelperEnv) == "" {
		t.Skip("run by TestConcurrentInstall")
	}
	if err := findDLLs(); err != nil {
		t.Fatal(err)
	}
}
