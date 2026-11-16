//go:build !wasm && !windows

package tintcheck

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// fatalRecorder stands in for the test runChrome is given, and keeps the
// message it fails with instead of failing the real test.
type fatalRecorder struct {
	testing.TB
	msg string
}

func (f *fatalRecorder) Fatalf(format string, args ...any) {
	f.msg = fmt.Sprintf(format, args...)
	runtime.Goexit()
}

// A Chrome that never reports and keeps writing output must fail with the
// timeout message once the output has been copied, not race the copy (#1271;
// run under -race).
func TestRunChrome_TimeoutReadsOutputAfterExit(t *testing.T) {
	chrome := filepath.Join(t.TempDir(), "chrome")
	script := "#!/bin/sh\nwhile :; do echo still starting; done\n"
	if err := os.WriteFile(chrome, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	defer func(d time.Duration) { chromeTimeout = d }(chromeTimeout)
	chromeTimeout = 300 * time.Millisecond

	f := &fatalRecorder{TB: t}
	done := make(chan struct{})
	go func() {
		defer close(done)
		runChrome(f, chrome, nil)
	}()
	<-done

	if !strings.Contains(f.msg, "no report from "+chrome+" within the timeout") {
		t.Fatalf("runChrome failed with %q, want the timeout message", f.msg)
	}
	if !strings.Contains(f.msg, "still starting") {
		t.Errorf("the timeout message does not carry the stand-in's output: %q", f.msg[:min(len(f.msg), 200)])
	}
}
