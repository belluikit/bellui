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

	"golang.org/x/sys/windows"
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

// TestInstallKeepsAnInstalledCopy pins #577: a process installing a DLL that
// another process installed first keeps that copy. Replacing it holds the file
// exclusively, and a process checking it at that moment cannot open it.
func TestInstallKeepsAnInstalledCopy(t *testing.T) {
	dll := thisMachineDLL(t)
	dir := t.TempDir()
	if err := installDLL(dir, "libegl.dll", dll); err != nil {
		t.Fatal(err)
	}
	first := fileIndex(t, filepath.Join(dir, "libegl.dll"))

	if err := installDLL(dir, "libegl.dll", dll); err != nil {
		t.Fatal(err)
	}
	if fileIndex(t, filepath.Join(dir, "libegl.dll")) != first {
		t.Error("the second install replaced the first copy")
	}
	assertNoTempFiles(t, dir)
}

// TestInstallReplacesAnIncompleteFile: a file left from before that is not a
// complete DLL for this machine is replaced.
func TestInstallReplacesAnIncompleteFile(t *testing.T) {
	dll := thisMachineDLL(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "libegl.dll"), dll[:100], 0o644); err != nil {
		t.Fatal(err)
	}
	if err := checkDLL(dir, "libegl.dll"); err == nil {
		t.Fatal("a truncated file passes the check")
	}

	if err := installDLL(dir, "libegl.dll", dll); err != nil {
		t.Fatal(err)
	}
	if err := checkDLL(dir, "libegl.dll"); err != nil {
		t.Errorf("not replaced: %v", err)
	}
	assertNoTempFiles(t, dir)
}

// thisMachineDLL returns the test binary, a PE file for this machine, to stand
// in for a DLL.
func thisMachineDLL(t *testing.T) []byte {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// fileIndex returns the NTFS file index of path now. os.SameFile cannot tell:
// on Windows a FileInfo from os.Stat looks the index up by path when compared,
// so two of them always see the file that has the name at that time.
func fileIndex(t *testing.T, path string) uint64 {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(windows.Handle(f.Fd()), &info); err != nil {
		t.Fatal(err)
	}
	return uint64(info.FileIndexHigh)<<32 | uint64(info.FileIndexLow)
}

func assertNoTempFiles(t *testing.T, dir string) {
	t.Helper()
	tmps, err := filepath.Glob(filepath.Join(dir, "*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(tmps) != 0 {
		t.Errorf("temporary files left: %v", tmps)
	}
}
