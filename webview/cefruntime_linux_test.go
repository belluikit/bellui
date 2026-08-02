//go:build linux && !android && cgo

package webview

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The #107 sandbox decision: enable only behind a verified root-owned setuid
// helper, never guess — an enabled sandbox with no usable helper is a
// Chromium process abort, not an error.

type fakeFileInfo struct {
	mode os.FileMode
	sys  any
}

func (f fakeFileInfo) Name() string       { return "chrome-sandbox" }
func (f fakeFileInfo) Size() int64        { return 1 }
func (f fakeFileInfo) Mode() os.FileMode  { return f.mode }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return false }
func (f fakeFileInfo) Sys() any           { return f.sys }

func TestSandboxHelperUsable(t *testing.T) {
	rootSetuid := fakeFileInfo{mode: 0o755 | os.ModeSetuid, sys: &syscall.Stat_t{Uid: 0}}
	assert.True(t, sandboxHelperUsable(rootSetuid), "root-owned setuid helper is usable")

	rootPlain := fakeFileInfo{mode: 0o755, sys: &syscall.Stat_t{Uid: 0}}
	assert.False(t, sandboxHelperUsable(rootPlain), "root-owned but not setuid")

	userSetuid := fakeFileInfo{mode: 0o755 | os.ModeSetuid, sys: &syscall.Stat_t{Uid: 1000}}
	assert.False(t, sandboxHelperUsable(userSetuid), "setuid but not root-owned")

	noStat := fakeFileInfo{mode: 0o755 | os.ModeSetuid, sys: nil}
	assert.False(t, sandboxHelperUsable(noStat), "no stat data reads as unusable, not a panic")
}

// The #139 platform decision: ozone-wayland caps the page's devicePixelRatio
// at the compositor's own output scale regardless of what GetScreenInfo
// answers, so CEF runs on ozone-x11 whenever an X display is reachable — and
// an explicit operator choice always wins.
func TestCEFOzonePlatformSwitches(t *testing.T) {
	assert.Equal(t, []string{"--ozone-platform=x11"},
		cefOzonePlatformSwitches(nil, ":0"),
		"a reachable X display pins ozone-x11")

	assert.Equal(t, []string{"--disable-gpu", "--ozone-platform=x11"},
		cefOzonePlatformSwitches([]string{"--disable-gpu"}, ":0"),
		"unrelated switches are kept, x11 appended")

	assert.Nil(t, cefOzonePlatformSwitches(nil, ""),
		"no X display: Chromium's own auto-selection stands")

	wayland := []string{"--ozone-platform=wayland"}
	assert.Equal(t, wayland, cefOzonePlatformSwitches(wayland, ":0"),
		"an explicit platform choice is never overridden")

	headless := []string{"--ozone-platform=headless"}
	assert.Equal(t, headless, cefOzonePlatformSwitches(headless, ""),
		"the probe's headless choice passes through untouched")
}

func TestResolveCEFSandbox_DisabledWithoutUsableHelper(t *testing.T) {
	dir := t.TempDir()
	assert.Empty(t, resolveCEFSandbox(dir), "no helper at all")

	// A helper owned by the (non-root) test user: present but not usable.
	// CI never runs these tests as root; guard rather than silently pass.
	sb := filepath.Join(dir, "chrome-sandbox")
	require.NoError(t, os.WriteFile(sb, []byte("#!/bin/true\n"), 0o755))
	if os.Getuid() == 0 {
		t.Skip("running as root: the user-owned case cannot be represented")
	}
	assert.Empty(t, resolveCEFSandbox(dir), "user-owned helper must not enable the sandbox")
}
