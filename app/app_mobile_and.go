//go:build !ci && android

package app

/*
#cgo LDFLAGS: -landroid -llog

#include <stdlib.h>

void openURL(uintptr_t java_vm, uintptr_t jni_env, uintptr_t ctx, char *url);
void sendNotification(uintptr_t java_vm, uintptr_t jni_env, uintptr_t ctx, char *title, char *content);
void sendNotificationWithActions(uintptr_t java_vm, uintptr_t jni_env, uintptr_t ctx, long long token, char *title, char *body, char *iconPath, char **labels, int labelCount);
void shareText(uintptr_t java_vm, uintptr_t jni_env, uintptr_t ctx, char *text);
void shareFile(uintptr_t java_vm, uintptr_t jni_env, uintptr_t ctx, char *path, char *mime);
*/
import "C"

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"unsafe"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal/driver/mobile/app"
	"github.com/belluikit/bellui/storage"
)

var _ bellui.SharerApp = (*belluiApp)(nil)

// ShareText hands text to the Android share sheet (ACTION_SEND chooser),
// implementing bellui.SharerApp (fyne#711).
func (a *belluiApp) ShareText(text string) error {
	textStr := C.CString(text)
	defer C.free(unsafe.Pointer(textStr))

	app.RunOnJVM(func(vm, env, ctx uintptr) error {
		C.shareText(C.uintptr_t(vm), C.uintptr_t(env), C.uintptr_t(ctx), textStr)
		return nil
	})
	return nil
}

// ShareFile hands a file to the Android share sheet (ACTION_SEND chooser),
// implementing bellui.SharerApp (fyne#711). The file's bytes are copied into the
// app-private share directory (cacheDir/belluishare) so BelluiFileProvider can vend
// them as a content:// URI with temporary read access; the source URI may be a
// local file:// or a content:// document.
func (a *belluiApp) ShareFile(uri bellui.URI) error {
	dst, err := copyToShareCache(uri)
	if err != nil {
		return err
	}

	pathStr := C.CString(dst)
	defer C.free(unsafe.Pointer(pathStr))
	mimeStr := C.CString(uri.MimeType())
	defer C.free(unsafe.Pointer(mimeStr))

	app.RunOnJVM(func(vm, env, ctx uintptr) error {
		C.shareFile(C.uintptr_t(vm), C.uintptr_t(env), C.uintptr_t(ctx), pathStr, mimeStr)
		return nil
	})
	return nil
}

// copyToShareCache copies the URI's contents into cacheDir/belluishare/<name> —
// the directory BelluiFileProvider serves — and returns the destination path.
// os.TempDir() is the Android cache dir (TMPDIR is set to getCacheDir), so this
// matches the provider's serve directory. Only the base name is used, so the
// staged file never escapes the share directory.
func copyToShareCache(uri bellui.URI) (string, error) {
	r, err := storage.Reader(uri)
	if err != nil {
		return "", fmt.Errorf("share: open source: %w", err)
	}
	defer r.Close()

	dir := filepath.Join(os.TempDir(), "belluishare")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("share: make cache dir: %w", err)
	}

	name := uri.Name()
	if name == "" {
		name = "shared"
	}
	dst := filepath.Join(dir, filepath.Base(name))

	f, err := os.Create(dst)
	if err != nil {
		return "", fmt.Errorf("share: create staged file: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return "", fmt.Errorf("share: copy: %w", err)
	}
	return dst, nil
}

func (a *belluiApp) OpenURL(url *url.URL) error {
	urlStr := C.CString(url.String())
	defer C.free(unsafe.Pointer(urlStr))

	app.RunOnJVM(func(vm, env, ctx uintptr) error {
		C.openURL(C.uintptr_t(vm), C.uintptr_t(env), C.uintptr_t(ctx), urlStr)
		return nil
	})
	return nil
}

func (a *belluiApp) SendNotification(n *bellui.Notification) {
	titleStr := C.CString(n.Title)
	defer C.free(unsafe.Pointer(titleStr))
	contentStr := C.CString(n.Content)
	defer C.free(unsafe.Pointer(contentStr))

	app.RunOnJVM(func(vm, env, ctx uintptr) error {
		C.sendNotification(C.uintptr_t(vm), C.uintptr_t(env), C.uintptr_t(ctx), titleStr, contentStr)
		return nil
	})
}

// SendNotificationWithOptions posts a rich notification (fyne#516/#723): body-tap
// and action-button callbacks are delivered via PendingIntents that route back
// through GoNativeActivity.onNewIntent -> notificationActivated -> the session
// registry (fireNotifyTapped/fireNotifyAction). A notification without callbacks
// or actions degrades to the plain path.
func (a *belluiApp) SendNotificationWithOptions(opts *bellui.NotificationOptions) {
	token := a.registerNotifyCallbacks(opts)
	if token == 0 {
		a.SendNotification(plainNotification(opts))
		return
	}

	var iconPath string
	if opts.Icon != nil {
		if p, err := writeTempNotificationIcon(opts.Icon, "bellui-notif-*.png"); err == nil {
			iconPath = p
		}
	}

	titleStr := C.CString(opts.Title)
	defer C.free(unsafe.Pointer(titleStr))
	bodyStr := C.CString(opts.Content)
	defer C.free(unsafe.Pointer(bodyStr))
	iconStr := C.CString(iconPath)
	defer C.free(unsafe.Pointer(iconStr))

	// Build a C array of action-label C strings (in action-index order). The
	// array is C-allocated so no Go pointer to Go pointers crosses into C.
	var labelsPtr **C.char
	if n := len(opts.Actions); n > 0 {
		arr := C.malloc(C.size_t(n) * C.size_t(unsafe.Sizeof(uintptr(0))))
		defer C.free(arr)
		slot := (*[1 << 16]*C.char)(arr)
		for i, act := range opts.Actions {
			cs := C.CString(act.Label)
			defer C.free(unsafe.Pointer(cs))
			slot[i] = cs
		}
		labelsPtr = (**C.char)(arr)
	}

	app.RunOnJVM(func(vm, env, ctx uintptr) error {
		C.sendNotificationWithActions(C.uintptr_t(vm), C.uintptr_t(env), C.uintptr_t(ctx),
			C.longlong(token), titleStr, bodyStr, iconStr, labelsPtr, C.int(len(opts.Actions)))
		return nil
	})
}
