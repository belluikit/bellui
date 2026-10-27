// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build android

/*
Android Apps are built with -buildmode=c-shared. They are loaded by a
running Java process.

Before any entry point is reached, a global constructor initializes the
Go runtime, calling all Go init functions. All cgo calls will block
until this is complete. Next JNI_OnLoad is called. When that is
complete, one of two entry points is called.

All-Go apps built using NativeActivity enter at ANativeActivity_onCreate.

Go libraries (for example, those built with gomobile bind) do not use
the app package initialization.
*/

package app

/*
#cgo LDFLAGS: -landroid -llog -lEGL -lGLESv2

#include <android/configuration.h>
#include <android/input.h>
#include <android/keycodes.h>
#include <android/looper.h>
#include <android/native_activity.h>
#include <android/native_window.h>
#include <EGL/egl.h>
#include <jni.h>
#include <pthread.h>
#include <stdlib.h>
#include <stdbool.h>

extern EGLDisplay display;
extern EGLSurface surface;

char* createEGLSurface(ANativeWindow* window);
char* destroyEGLSurface();
int32_t getKeyRune(JNIEnv* env, AInputEvent* e);

void showKeyboard(JNIEnv* env, int keyboardType);
void hideKeyboard(JNIEnv* env);
void setKeyboardState(JNIEnv* env, char* text, int selStart, int selEnd);
void keyboardEditorChanged(JNIEnv* env);
void showFileOpen(JNIEnv* env, char* mimes);
void showFileOpenMulti(JNIEnv* env, char* mimes);
void showFileSave(JNIEnv* env, char* mimes, char* filename);
void finish(JNIEnv* env, jobject ctx);

void Java_org_golang_app_GoNativeActivity_filePickerReturned(JNIEnv *env, jclass clazz, jstring str);
void Java_org_golang_app_GoNativeActivity_filePickerMultiReturned(JNIEnv *env, jclass clazz, jstring str);
*/
import "C"

import (
	"fmt"
	"log"
	"mime"
	"os"
	"strings"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/belluikit/bellui/internal/async"
	"github.com/belluikit/bellui/internal/driver/mobile/app/callfn"
	"github.com/belluikit/bellui/internal/driver/mobile/event/config"
	"github.com/belluikit/bellui/internal/driver/mobile/event/key"
	"github.com/belluikit/bellui/internal/driver/mobile/event/lifecycle"
	"github.com/belluikit/bellui/internal/driver/mobile/event/paint"
	"github.com/belluikit/bellui/internal/driver/mobile/event/size"
	"github.com/belluikit/bellui/internal/driver/mobile/event/touch"
	"github.com/belluikit/bellui/internal/driver/mobile/mobileinit"
)

// mimeMap contains standard mime entries that are missing on Android
var mimeMap = map[string]string{
	".txt": "text/plain",
}

// GoBack asks the OS to go to the previous app / activity
func GoBack() {
	err := RunOnJVM(func(_, jniEnv, ctx uintptr) error {
		env := (*C.JNIEnv)(unsafe.Pointer(jniEnv))
		C.finish(env, C.jobject(ctx))
		return nil
	})
	if err != nil {
		log.Fatalf("app: %v", err)
	}
}

// RunOnJVM runs fn on a new goroutine locked to an OS thread with a JNIEnv.
//
// RunOnJVM blocks until the call to fn is complete. Any Java
// exception or failure to attach to the JVM is returned as an error.
//
// The function fn takes vm, the current JavaVM*,
// env, the current JNIEnv*, and
// ctx, a jobject representing the global android.context.Context.
func RunOnJVM(fn func(vm, jniEnv, ctx uintptr) error) error {
	return mobileinit.RunOnJVM(fn)
}

//export setCurrentContext
func setCurrentContext(vm *C.JavaVM, ctx C.jobject) {
	mobileinit.SetCurrentContext(unsafe.Pointer(vm), uintptr(ctx))
}

//export callMain
func callMain(mainPC uintptr) {
	for _, name := range []string{"FILESDIR", "TMPDIR", "PATH", "LD_LIBRARY_PATH"} {
		n := C.CString(name)
		os.Setenv(name, C.GoString(C.getenv(n)))
		C.free(unsafe.Pointer(n))
	}

	// Set timezone.
	//
	// Note that Android zoneinfo is stored in /system/usr/share/zoneinfo,
	// but it is in some kind of packed TZiff file that we do not support
	// yet. As a stopgap, we build a fixed zone using the tm_zone name.
	var curtime C.time_t
	var curtm C.struct_tm
	C.time(&curtime)
	C.localtime_r(&curtime, &curtm)
	tzOffset := int(curtm.tm_gmtoff)
	tz := C.GoString(curtm.tm_zone)
	time.Local = time.FixedZone(tz, tzOffset)

	go callfn.CallFn(mainPC)
}

//export onSaveInstanceState
func onSaveInstanceState(activity *C.ANativeActivity, outSize *C.size_t) unsafe.Pointer {
	return nil
}

//export onBackPressed
func onBackPressed() {
	k := key.Event{
		Code:      key.CodeBackButton,
		Direction: key.DirPress,
	}
	theApp.events.In() <- k

	k.Direction = key.DirRelease
	theApp.events.In() <- k
}

//export onCreate
func onCreate(activity *C.ANativeActivity) {
	// Set the initial configuration.
	//
	// Note we use unbuffered channels to talk to the activity loop, and
	// NativeActivity calls these callbacks sequentially, so configuration
	// will be set before <-windowRedrawNeeded is processed.
	windowConfigChange <- windowConfigRead(activity)
}

//export onDestroy
func onDestroy(activity *C.ANativeActivity) {
	activityDestroyed <- struct{}{}
}

//export activityPaused
func activityPaused() { activityLifecycle <- activityPause }

//export activityStopped
func activityStopped() { activityLifecycle <- activityStop }

//export activityResumed
func activityResumed() { activityLifecycle <- activityResume }

//export onWindowFocusChanged
func onWindowFocusChanged(activity *C.ANativeActivity, hasFocus C.int) {
}

//export onNativeWindowCreated
func onNativeWindowCreated(activity *C.ANativeActivity, window *C.ANativeWindow) {
	windowCreated <- window
}

//export onNativeWindowRedrawNeeded
func onNativeWindowRedrawNeeded(activity *C.ANativeActivity, window *C.ANativeWindow) {
	// Called on orientation change and window resize.
	// Send a request for redraw, and block this function
	// until a complete draw and buffer swap is completed.
	// This is required by the redraw documentation to
	// avoid bad draws.
	windowRedrawNeeded <- window
	<-windowRedrawDone
}

//export onNativeWindowDestroyed
func onNativeWindowDestroyed(activity *C.ANativeActivity, window *C.ANativeWindow) {
	windowDestroyed <- window
}

//export onInputQueueCreated
func onInputQueueCreated(activity *C.ANativeActivity, q *C.AInputQueue) {
	inputQueue <- q
	<-inputQueueDone
}

//export onInputQueueDestroyed
func onInputQueueDestroyed(activity *C.ANativeActivity, q *C.AInputQueue) {
	inputQueue <- nil
	<-inputQueueDone
}

//export onContentRectChanged
func onContentRectChanged(activity *C.ANativeActivity, rect *C.ARect) {
}

//export setDarkMode
func setDarkMode(dark C.bool) {
	darkMode = bool(dark)
}

type windowConfig struct {
	orientation size.Orientation
	pixelsPerPt float32
}

// density.go mirrors these NDK constants so that the density mapping builds
// without cgo; a drift would be silent, so tie them together at compile time
// (each pair must be 0 from both sides — a negative constant fails to convert).
const (
	_ uint = densityDefault - C.ACONFIGURATION_DENSITY_DEFAULT
	_ uint = C.ACONFIGURATION_DENSITY_DEFAULT - densityDefault
	_ uint = densityMedium - C.ACONFIGURATION_DENSITY_MEDIUM
	_ uint = C.ACONFIGURATION_DENSITY_MEDIUM - densityMedium
	_ uint = densityAny - C.ACONFIGURATION_DENSITY_ANY
	_ uint = C.ACONFIGURATION_DENSITY_ANY - densityAny
	_ uint = densityNone - C.ACONFIGURATION_DENSITY_NONE
	_ uint = C.ACONFIGURATION_DENSITY_NONE - densityNone
)

func windowConfigRead(activity *C.ANativeActivity) windowConfig {
	aconfig := C.AConfiguration_new()
	C.AConfiguration_fromAssetManager(aconfig, activity.assetManager)
	orient := C.AConfiguration_getOrientation(aconfig)
	density := C.AConfiguration_getDensity(aconfig)
	C.AConfiguration_delete(aconfig)

	// The density → dpi decision lives in density.go so it can be tested off
	// device; see androidDPIForDensity for why an off-bucket value is taken
	// as-is and what the sentinels mean (#272).
	dpi, usable := androidDPIForDensity(int32(density))
	if !usable {
		log.Printf("android device reports no usable screen density: %d", density)
	}

	o := size.OrientationUnknown
	switch orient {
	case C.ACONFIGURATION_ORIENTATION_PORT:
		o = size.OrientationPortrait
	case C.ACONFIGURATION_ORIENTATION_LAND:
		o = size.OrientationLandscape
	}

	return windowConfig{
		orientation: o,
		pixelsPerPt: float32(dpi) / 72,
	}
}

//export onConfigurationChanged
func onConfigurationChanged(activity *C.ANativeActivity) {
	// A rotation event first triggers onConfigurationChanged, then
	// calls onNativeWindowRedrawNeeded. We extract the orientation
	// here and save it for the redraw event.
	windowConfigChange <- windowConfigRead(activity)

	// A configuration change the manifest declares (locale|layoutDirection,
	// #287) arrives here in place — no recreation, no lifecycle crossing the
	// driver could hook. Forward it as an event so the driver re-resolves the
	// locale on its own goroutine, not this Java main thread (#288). The send
	// above already serialized with mainUI, and events is unbounded, so this
	// never blocks.
	theApp.events.In() <- config.Event{}
}

//export onLowMemory
func onLowMemory(activity *C.ANativeActivity) {
	cleanCaches()
}

var (
	inputQueue         = make(chan *C.AInputQueue)
	inputQueueDone     = make(chan struct{})
	windowCreated      = make(chan *C.ANativeWindow)
	windowDestroyed    = make(chan *C.ANativeWindow)
	windowRedrawNeeded = make(chan *C.ANativeWindow)
	windowRedrawDone   = make(chan struct{})
	windowConfigChange = make(chan windowConfig)
	activityDestroyed  = make(chan struct{})
	activityLifecycle  = make(chan activityEvent)

	currentSize size.Event
	darkMode    bool

	// nativeWindowLive tracks whether the ANativeWindow the driver last saw is
	// still alive. mainUI is the sole writer: false as soon as it receives
	// onNativeWindowDestroyed, true again on onNativeWindowCreated /
	// onNativeWindowRedrawNeeded. The WebGPU driver reads it (NativeWindowLive)
	// to refuse painting or presenting into a destroyed window: during an
	// in-process activity recreation (a per-app locale override is a guaranteed
	// trigger on Android 13+) the destroy→create gap overlaps queued paint
	// ticks, and a Vulkan present into the dead window wedges the emulator's
	// GPU channel so hard that the next surface call never returns — the #283
	// ANR, which then poisons every later launch until the emulator host
	// process restarts. The GL path needs no flag: mainUI destroys the EGL
	// surface synchronously in the windowDestroyed case and Publish checks
	// C.surface.
	nativeWindowLive atomic.Bool
)

// NativeWindowLive reports whether the Android native window is currently
// alive — false from onNativeWindowDestroyed until the next
// onNativeWindowCreated or onNativeWindowRedrawNeeded. The WebGPU driver
// backend gates painting, surface rebuilds and presenting on it (#283).
func NativeWindowLive() bool { return nativeWindowLive.Load() }

func init() {
	theApp.registerGLViewportFilter()
}

func main(f func(App)) {
	mainUserFn = f
	// TODO: merge the runInputQueue and mainUI functions?
	go func() {
		if err := mobileinit.RunOnJVM(runInputQueue); err != nil {
			log.Fatalf("app: %v", err)
		}
	}()
	// Preserve this OS thread for:
	//	1. the attached JNI thread
	//	2. the GL context
	if err := mobileinit.RunOnJVM(mainUI); err != nil {
		log.Fatalf("app: %v", err)
	}
}

// keyboardOps serializes every soft-keyboard JVM call — show, hide and the
// pre-rebrand tracker #572 state mirror — through one background worker goroutine. FIFO order is
// preserved (a focus switch's hide-then-show lands in sequence, and a state
// push never overtakes the show that precedes it), but the caller returns
// immediately. The previous blocking RunOnJVM round-trip from the main
// goroutine deadlocked against the Android UI thread when a TalkBack-dispatched
// focus switch re-entered the show/hide path while the keyboard was already up
// (pre-rebrand tracker #973); keyboard calls carry no result, so nothing needs the round-trip.
var keyboardOps = async.NewUnboundedChan[func()]()

func init() {
	go func() {
		for op := range keyboardOps.Out() {
			op()
		}
	}()
}

// shownKeyboard is the soft keyboard last asked for, read by the native key
// path (processKey) on the input goroutine (#824).
var shownKeyboard atomic.Int32

// driverShowVirtualKeyboard requests the driver to show a virtual keyboard for text input
func driverShowVirtualKeyboard(keyboard KeyboardType) {
	shownKeyboard.Store(int32(keyboard))
	keyboardOps.In() <- func() {
		err := mobileinit.RunOnJVM(func(vm, jniEnv, ctx uintptr) error {
			env := (*C.JNIEnv)(unsafe.Pointer(jniEnv)) // not a Go heap pointer
			C.showKeyboard(env, C.int(int32(keyboard)))
			return nil
		})
		if err != nil {
			log.Fatalf("app: %v", err)
		}
	}
}

// SetKeyboardState mirrors the focused bellui editor's flat text + selection (rune
// offsets) into the bellui-owned IME buffer (pre-rebrand tracker #1006 Phase 4). It is the push half of
// the bellui-driven flat transport: the buffer is what the soft keyboard's
// InputConnection reads, and this push keeps it in step with bellui's document
// whenever the document changes off the IME path (SetText, a tap/undo/TalkBack
// edit). The Java side (doSetKeyboardState) clears the composing region on every
// push — so the IME can never recompose already-committed text (pre-rebrand tracker #966) — and skips
// the push while an IME composition is active. Because the IME's own edits already
// update the buffer through BelluiInputConnection, an echoing push of an
// IME-originated change is a strict no-op there.
//
// Routed through keyboardOps so it stays ordered behind the show that precedes it
// and runs off the main goroutine (the RunOnJVM round-trip from the main goroutine
// deadlocked against the Android UI thread under a TalkBack-dispatched focus
// switch, pre-rebrand tracker #973).
func SetKeyboardState(text string, selStart, selEnd int) {
	keyboardOps.In() <- func() {
		err := mobileinit.RunOnJVM(func(vm, jniEnv, ctx uintptr) error {
			env := (*C.JNIEnv)(unsafe.Pointer(jniEnv)) // not a Go heap pointer
			ctext := C.CString(text)
			C.setKeyboardState(env, ctext, C.int(int32(selStart)), C.int(int32(selEnd)))
			C.free(unsafe.Pointer(ctext))
			return nil
		})
		if err != nil {
			log.Fatalf("app: %v", err)
		}
	}
}

// KeyboardEditorChanged tells the Java side the bound bellui editor's identity
// changed (pre-rebrand tracker #966): the IME session state it holds — an in-flight composing word,
// the echo/recent-state filters — belongs to the PREVIOUS field, so it must be
// dropped and the next setKeyboardState seed must reseed + restartInput
// unconditionally, never be swallowed as an echo. Routed through keyboardOps so
// it stays ordered between the show that precedes it and the seed push that
// follows (the same ordering contract as SetKeyboardState).
func KeyboardEditorChanged() {
	keyboardOps.In() <- func() {
		err := mobileinit.RunOnJVM(func(vm, jniEnv, ctx uintptr) error {
			env := (*C.JNIEnv)(unsafe.Pointer(jniEnv)) // not a Go heap pointer
			C.keyboardEditorChanged(env)
			return nil
		})
		if err != nil {
			log.Fatalf("app: %v", err)
		}
	}
}

// SetKeyboardUndoState is a no-op on Android (pre-rebrand tracker #1134): Android soft keyboards
// have no undo-manager-driven UI; TalkBack undo/redo ride the pre-rebrand tracker #1025 custom
// accessibility actions instead.
func SetKeyboardUndoState(canUndo, canRedo bool) {}

// driverHideVirtualKeyboard requests the driver to hide any visible virtual keyboard
func driverHideVirtualKeyboard() {
	keyboardOps.In() <- func() {
		if err := mobileinit.RunOnJVM(hideSoftInput); err != nil {
			log.Fatalf("app: %v", err)
		}
	}
}

func hideSoftInput(vm, jniEnv, ctx uintptr) error {
	env := (*C.JNIEnv)(unsafe.Pointer(jniEnv)) // not a Go heap pointer
	C.hideKeyboard(env)
	return nil
}

var fileCallback func(string, func())

//export filePickerReturned
func filePickerReturned(str *C.char) {
	if fileCallback == nil {
		return
	}

	fileCallback(C.GoString(str), nil)
	fileCallback = nil
}

var fileMultiCallback func([]string, []func())

//export filePickerMultiReturned
func filePickerMultiReturned(str *C.char) {
	if fileMultiCallback == nil {
		return
	}
	callback := fileMultiCallback
	fileMultiCallback = nil

	// The Java side joins the picked content URIs with newlines (never valid
	// inside a URI); an empty payload is a cancelled picker. Android needs no
	// per-URI closer — SAF read grants last for the activity's lifetime, there
	// is no security scope to end (the single-select path passes nil too).
	joined := C.GoString(str)
	if joined == "" {
		callback(nil, nil)
		return
	}
	callback(strings.Split(joined, "\n"), nil)
}

//export insetsChanged
func insetsChanged(top, bottom, left, right int) {
	currentSize.InsetTopPx = top
	currentSize.InsetBottomPx = bottom
	currentSize.InsetLeftPx = left
	currentSize.InsetRightPx = right

	theApp.events.In() <- currentSize
}

func mimeStringFromFilter(filter *FileFilter) string {
	mimes := "*/*"
	if filter.MimeTypes != nil {
		mimes = strings.Join(filter.MimeTypes, "|")
	} else if filter.Extensions != nil {
		var mimeTypes []string
		for _, ext := range filter.Extensions {
			if mimeEntry, ok := mimeMap[ext]; ok {
				mimeTypes = append(mimeTypes, mimeEntry)

				continue
			}

			mimeType := mime.TypeByExtension(ext)
			if mimeType == "" {
				log.Println("Could not find mime for extension " + ext + ", allowing all")
				return "*/*" // could not find one, so allow all
			}

			mimeTypes = append(mimeTypes, mimeType)
		}
		mimes = strings.Join(mimeTypes, "|")
	}
	return mimes
}

func driverShowFileOpenPicker(callback func(string, func()), filter *FileFilter) {
	fileCallback = callback

	mimes := mimeStringFromFilter(filter)
	mimeStr := C.CString(mimes)
	defer C.free(unsafe.Pointer(mimeStr))

	open := func(vm, jniEnv, ctx uintptr) error {
		// TODO pass in filter...
		env := (*C.JNIEnv)(unsafe.Pointer(jniEnv)) // not a Go heap pointer
		C.showFileOpen(env, mimeStr)
		return nil
	}

	if err := mobileinit.RunOnJVM(open); err != nil {
		log.Fatalf("app: %v", err)
	}
}

func driverShowFileOpenMultiplePicker(callback func([]string, []func()), filter *FileFilter) {
	fileMultiCallback = callback

	mimes := mimeStringFromFilter(filter)
	mimeStr := C.CString(mimes)
	defer C.free(unsafe.Pointer(mimeStr))

	open := func(vm, jniEnv, ctx uintptr) error {
		env := (*C.JNIEnv)(unsafe.Pointer(jniEnv)) // not a Go heap pointer
		C.showFileOpenMulti(env, mimeStr)
		return nil
	}

	if err := mobileinit.RunOnJVM(open); err != nil {
		log.Fatalf("app: %v", err)
	}
}

func driverShowFileSavePicker(callback func(string, func()), filter *FileFilter, filename string) {
	fileCallback = callback

	mimes := mimeStringFromFilter(filter)
	mimeStr := C.CString(mimes)
	defer C.free(unsafe.Pointer(mimeStr))
	filenameStr := C.CString(filename)
	defer C.free(unsafe.Pointer(filenameStr))

	save := func(vm, jniEnv, ctx uintptr) error {
		env := (*C.JNIEnv)(unsafe.Pointer(jniEnv)) // not a Go heap pointer
		C.showFileSave(env, mimeStr, filenameStr)
		return nil
	}

	if err := mobileinit.RunOnJVM(save); err != nil {
		log.Fatalf("app: %v", err)
	}
}

var mainUserFn func(App)

var DisplayMetrics struct {
	WidthPx  int
	HeightPx int
}

func mainUI(vm, jniEnv, ctx uintptr) error {
	workAvailable := theApp.worker.WorkAvailable()

	donec := make(chan struct{})
	go func() {
		mainUserFn(theApp)
		close(donec)
	}()

	var pixelsPerPt float32
	var surfaceInitialized, wasDestroyed bool
	// windowReady is true while the window has been drawn into and not
	// destroyed since: the state in which a resume can raise the app straight
	// back to focused (#673).
	var windowReady bool

	for {
		select {
		case <-donec:
			return nil
		case cfg := <-windowConfigChange:
			pixelsPerPt = cfg.pixelsPerPt
		case w := <-windowCreated:
			nativeWindowLive.Store(true)
			if surfaceInitialized && !wasDestroyed && !webgpuBackend {
				if errStr := C.destroyEGLSurface(); errStr != nil {
					return fmt.Errorf("%s (%s)", C.GoString(errStr), eglGetError())
				}
				if errStr := C.createEGLSurface(w); errStr != nil {
					return fmt.Errorf("%s (%s)", C.GoString(errStr), eglGetError())
				}
			}
			if surfaceInitialized && webgpuBackend {
				// WebGPU (Vulkan) owns the ANativeWindow directly, so there is no EGL
				// surface to recreate here. Android delivers the fresh window via
				// onNativeWindowCreated on resume; forward the new handle to the painter
				// now so the surface is rebuilt as soon as the window exists, instead of
				// waiting for the next hardware onNativeWindowRedrawNeeded to carry it.
				// When that redraw lags (seconds, on some devices) the painter would
				// otherwise present to the destroyed surface — a black canvas (pre-rebrand tracker #312).
				// handlePaint recreates the surface only when the handle actually
				// changed, so a redraw arriving in the same frame is a no-op.
				theApp.events.In() <- paint.Event{External: true, Window: uintptr(unsafe.Pointer(w))}
			}
		case w := <-windowRedrawNeeded:
			nativeWindowLive.Store(true)
			// Under -tags webgpu the ANativeWindow is owned by wgpu (Vulkan), so
			// no EGL surface is created; surfaceInitialized gates the one-time
			// metrics setup instead of C.surface.
			needSurface := C.surface == nil
			if webgpuBackend {
				needSurface = !surfaceInitialized
			}
			if needSurface {
				if !webgpuBackend {
					if errStr := C.createEGLSurface(w); errStr != nil {
						return fmt.Errorf("%s (%s)", C.GoString(errStr), eglGetError())
					}
				}
				surfaceInitialized = true
				DisplayMetrics.WidthPx = int(C.ANativeWindow_getWidth(w))
				DisplayMetrics.HeightPx = int(C.ANativeWindow_getHeight(w))
			}
			windowReady = true
			theApp.sendLifecycle(lifecycle.StageFocused)
			widthPx := int(C.ANativeWindow_getWidth(w))
			heightPx := int(C.ANativeWindow_getHeight(w))
			currentSize = size.Event{
				WidthPx:       widthPx,
				HeightPx:      heightPx,
				WidthPt:       float32(widthPx) / pixelsPerPt,
				HeightPt:      float32(heightPx) / pixelsPerPt,
				InsetTopPx:    currentSize.InsetTopPx,
				InsetBottomPx: currentSize.InsetBottomPx,
				InsetLeftPx:   currentSize.InsetLeftPx,
				InsetRightPx:  currentSize.InsetRightPx,
				PixelsPerPt:   pixelsPerPt,
				Orientation:   screenOrientation(widthPx, heightPx), // we are guessing orientation here as it was not always working
				DarkMode:      darkMode,
			}
			theApp.events.In() <- currentSize
			theApp.events.In() <- paint.Event{External: true, Window: uintptr(unsafe.Pointer(w))}
		case <-windowDestroyed:
			// Flip liveness before anything else: the onNativeWindowDestroyed
			// callback returns (and Android destroys the window) as soon as its
			// channel send is received, so this store is the driver's only
			// near-synchronous signal to stop presenting (#283).
			nativeWindowLive.Store(false)
			if C.surface != nil {
				if errStr := C.destroyEGLSurface(); errStr != nil {
					return fmt.Errorf("%s (%s)", C.GoString(errStr), eglGetError())
				}
			}
			C.surface = nil
			wasDestroyed = true
			windowReady = false
			theApp.sendLifecycle(lifecycle.StageAlive)
		case e := <-activityLifecycle:
			if to, ok := activityStage(e, theApp.lifecycleStage, windowReady); ok {
				theApp.sendLifecycle(to)
			}
		case <-activityDestroyed:
			theApp.sendLifecycle(lifecycle.StageDead)
		case <-workAvailable:
			theApp.worker.DoWork()
		case <-theApp.publish:
			// TODO: compare a generation number to redrawGen for stale paints?
			if C.surface != nil {
				// eglSwapBuffers blocks until vsync.
				if C.eglSwapBuffers(C.display, C.surface) == C.EGL_FALSE {
					log.Printf("app: failed to swap buffers (%s)", eglGetError())
				}
			}
			select {
			case windowRedrawDone <- struct{}{}:
			default:
			}
			theApp.publishResult <- PublishResult{}
		}
	}
}

func runInputQueue(vm, jniEnv, ctx uintptr) error {
	env := (*C.JNIEnv)(unsafe.Pointer(jniEnv)) // not a Go heap pointer

	// Android loopers select on OS file descriptors, not Go channels, so we
	// translate the inputQueue channel to an ALooper_wake call.
	l := C.ALooper_prepare(C.ALOOPER_PREPARE_ALLOW_NON_CALLBACKS)
	pending := make(chan *C.AInputQueue, 1)
	go func() {
		for q := range inputQueue {
			pending <- q
			C.ALooper_wake(l)
		}
	}()

	var q *C.AInputQueue
	for {
		if C.ALooper_pollOnce(-1, nil, nil, nil) == C.ALOOPER_POLL_WAKE {
			select {
			default:
			case p := <-pending:
				if q != nil {
					processEvents(env, q)
					C.AInputQueue_detachLooper(q)
				}
				q = p
				if q != nil {
					C.AInputQueue_attachLooper(q, l, 0, nil, nil)
				}
				inputQueueDone <- struct{}{}
			}
		}
		if q != nil {
			processEvents(env, q)
		}
	}
}

func processEvents(env *C.JNIEnv, q *C.AInputQueue) {
	var e *C.AInputEvent
	for C.AInputQueue_getEvent(q, &e) >= 0 {
		if C.AInputQueue_preDispatchEvent(q, e) != 0 {
			continue
		}
		processEvent(env, e)
		C.AInputQueue_finishEvent(q, e, 0)
	}
}

func processEvent(env *C.JNIEnv, e *C.AInputEvent) {
	switch C.AInputEvent_getType(e) {
	case C.AINPUT_EVENT_TYPE_KEY:
		processKey(env, e)
	case C.AINPUT_EVENT_TYPE_MOTION:
		// At most one of the events in this batch is an up or down event; get its index and change.
		upDownIndex := C.size_t(C.AMotionEvent_getAction(e)&C.AMOTION_EVENT_ACTION_POINTER_INDEX_MASK) >> C.AMOTION_EVENT_ACTION_POINTER_INDEX_SHIFT
		upDownType := touch.TypeMove
		cancelled := false
		switch C.AMotionEvent_getAction(e) & C.AMOTION_EVENT_ACTION_MASK {
		case C.AMOTION_EVENT_ACTION_DOWN, C.AMOTION_EVENT_ACTION_POINTER_DOWN:
			upDownType = touch.TypeBegin
		case C.AMOTION_EVENT_ACTION_UP, C.AMOTION_EVENT_ACTION_POINTER_UP:
			upDownType = touch.TypeEnd
		case C.AMOTION_EVENT_ACTION_CANCEL:
			// The system took the whole gesture (another window opened on
			// top, focus was taken): every pointer in it ends without
			// completing. It
			// used to fall through to a move, so the release was never seen
			// (#392).
			cancelled = true
		}

		for i, n := C.size_t(0), C.AMotionEvent_getPointerCount(e); i < n; i++ {
			t := touch.TypeMove
			if cancelled {
				t = touch.TypeCancel
			} else if i == upDownIndex {
				t = upDownType
			}
			theApp.events.In() <- touch.Event{
				X:        float32(C.AMotionEvent_getX(e, i)),
				Y:        float32(C.AMotionEvent_getY(e, i)),
				Sequence: touch.Sequence(C.AMotionEvent_getPointerId(e, i)),
				Type:     t,
			}
		}
	default:
		log.Printf("unknown input event, type=%d", C.AInputEvent_getType(e))
	}
}

func processKey(env *C.JNIEnv, e *C.AInputEvent) {
	deviceID := C.AInputEvent_getDeviceId(e)
	if deviceID == 0 {
		// Software keyboard input, leaving for scribe/IME.
		return
	}

	k := key.Event{
		Rune:      rune(C.getKeyRune(env, e)),
		Code:      convAndroidKeyCode(int32(C.AKeyEvent_getKeyCode(e))),
		Modifiers: androidKeyModifiers(int32(C.AKeyEvent_getMetaState(e))),
		Native:    true, // raw input queue — hardware/injected, never an IC edit
	}
	if nativeDigitIsEcho(k.Rune, KeyboardType(shownKeyboard.Load())) {
		return // the text keyboard delivers this digit as text
	}
	switch C.AKeyEvent_getAction(e) {
	case C.AKEY_STATE_DOWN:
		k.Direction = key.DirPress
	case C.AKEY_STATE_UP:
		k.Direction = key.DirRelease
	default:
		k.Direction = key.DirNone
	}
	theApp.events.In() <- k
}

// keyboardShortcutKey routes a hardware-keyboard shortcut combination (a key
// pressed with Ctrl/Alt/Meta held) from the IME's InputConnection.sendKeyEvent
// into bellui's key path, carrying the Android meta-state so the driver can build
// the matching bellui.Shortcut (pre-rebrand tracker #1077). Gboard hands these to the InputConnection
// with getUnicodeChar()==0, so they never reach the native input queue; without
// this they went to the inert host EditText and were lost. Delivered as a
// NON-native key event, so the one-writer reroute's native-echo suppression
// (pre-rebrand tracker #1061) never touches it. Lives here (android-tagged) beside convAndroidKeyCode
// / androidKeyModifiers, unlike the cross-platform keyboard.go up-calls.
//
// keyboardArrowKey routes an IME-delivered arrow key (soft editing-panel or a
// hardware DPAD forwarded through sendKeyEvent) into bellui's key path. Arrow keys
// never reach the native input queue (`convAndroidKeyCode` leaves DPAD
// unmapped), so this is their sole delivery — it now carries the meta-state so
// an editor extends its selection on Shift+Arrow (pre-rebrand tracker #1083). Android-tagged beside
// androidKeyModifiers, like keyboardShortcutKey.
//
//export keyboardArrowKey
func keyboardArrowKey(code, metaState C.int) {
	var kc key.Code
	switch int(code) {
	case 19: // AKEYCODE_DPAD_UP
		kc = key.CodeUpArrow
	case 20: // AKEYCODE_DPAD_DOWN
		kc = key.CodeDownArrow
	case 21: // AKEYCODE_DPAD_LEFT
		kc = key.CodeLeftArrow
	case 22: // AKEYCODE_DPAD_RIGHT
		kc = key.CodeRightArrow
	default:
		return
	}
	mod := androidKeyModifiers(int32(metaState))
	theApp.events.In() <- key.Event{Code: kc, Modifiers: mod, Direction: key.DirPress}
	theApp.events.In() <- key.Event{Code: kc, Modifiers: mod, Direction: key.DirRelease}
}

//export keyboardShortcutKey
func keyboardShortcutKey(keyCode, metaState C.int) {
	code := convAndroidKeyCode(int32(keyCode))
	if code == key.CodeUnknown {
		return
	}
	mod := androidKeyModifiers(int32(metaState))
	theApp.events.In() <- key.Event{Code: code, Modifiers: mod, Direction: key.DirPress}
	theApp.events.In() <- key.Event{Code: code, Modifiers: mod, Direction: key.DirRelease}
}

// androidKeyModifiers maps an Android AMETA_* meta-state bitmask to gomobile's
// key.Modifiers. Populated on hardware-key events so the driver can tell a bare
// keystroke from a shortcut combination (Ctrl/Alt/Super) — the latter is never
// suppressed by the one-writer IME reroute (pre-rebrand tracker #1061).
func androidKeyModifiers(meta int32) key.Modifiers {
	var m key.Modifiers
	if meta&C.AMETA_SHIFT_ON != 0 {
		m |= key.ModShift
	}
	if meta&C.AMETA_CTRL_ON != 0 {
		m |= key.ModControl
	}
	if meta&C.AMETA_ALT_ON != 0 {
		m |= key.ModAlt
	}
	if meta&C.AMETA_META_ON != 0 {
		m |= key.ModMeta
	}
	return m
}

func eglGetError() string {
	switch errNum := C.eglGetError(); errNum {
	case C.EGL_SUCCESS:
		return "EGL_SUCCESS"
	case C.EGL_NOT_INITIALIZED:
		return "EGL_NOT_INITIALIZED"
	case C.EGL_BAD_ACCESS:
		return "EGL_BAD_ACCESS"
	case C.EGL_BAD_ALLOC:
		return "EGL_BAD_ALLOC"
	case C.EGL_BAD_ATTRIBUTE:
		return "EGL_BAD_ATTRIBUTE"
	case C.EGL_BAD_CONTEXT:
		return "EGL_BAD_CONTEXT"
	case C.EGL_BAD_CONFIG:
		return "EGL_BAD_CONFIG"
	case C.EGL_BAD_CURRENT_SURFACE:
		return "EGL_BAD_CURRENT_SURFACE"
	case C.EGL_BAD_DISPLAY:
		return "EGL_BAD_DISPLAY"
	case C.EGL_BAD_SURFACE:
		return "EGL_BAD_SURFACE"
	case C.EGL_BAD_MATCH:
		return "EGL_BAD_MATCH"
	case C.EGL_BAD_PARAMETER:
		return "EGL_BAD_PARAMETER"
	case C.EGL_BAD_NATIVE_PIXMAP:
		return "EGL_BAD_NATIVE_PIXMAP"
	case C.EGL_BAD_NATIVE_WINDOW:
		return "EGL_BAD_NATIVE_WINDOW"
	case C.EGL_CONTEXT_LOST:
		return "EGL_CONTEXT_LOST"
	default:
		return fmt.Sprintf("Unknown EGL err: %d", errNum)
	}
}

var androidKeycoe = map[int32]key.Code{
	C.AKEYCODE_HOME:            key.CodeHome,
	C.AKEYCODE_0:               key.Code0,
	C.AKEYCODE_1:               key.Code1,
	C.AKEYCODE_2:               key.Code2,
	C.AKEYCODE_3:               key.Code3,
	C.AKEYCODE_4:               key.Code4,
	C.AKEYCODE_5:               key.Code5,
	C.AKEYCODE_6:               key.Code6,
	C.AKEYCODE_7:               key.Code7,
	C.AKEYCODE_8:               key.Code8,
	C.AKEYCODE_9:               key.Code9,
	C.AKEYCODE_VOLUME_UP:       key.CodeVolumeUp,
	C.AKEYCODE_VOLUME_DOWN:     key.CodeVolumeDown,
	C.AKEYCODE_A:               key.CodeA,
	C.AKEYCODE_B:               key.CodeB,
	C.AKEYCODE_C:               key.CodeC,
	C.AKEYCODE_D:               key.CodeD,
	C.AKEYCODE_E:               key.CodeE,
	C.AKEYCODE_F:               key.CodeF,
	C.AKEYCODE_G:               key.CodeG,
	C.AKEYCODE_H:               key.CodeH,
	C.AKEYCODE_I:               key.CodeI,
	C.AKEYCODE_J:               key.CodeJ,
	C.AKEYCODE_K:               key.CodeK,
	C.AKEYCODE_L:               key.CodeL,
	C.AKEYCODE_M:               key.CodeM,
	C.AKEYCODE_N:               key.CodeN,
	C.AKEYCODE_O:               key.CodeO,
	C.AKEYCODE_P:               key.CodeP,
	C.AKEYCODE_Q:               key.CodeQ,
	C.AKEYCODE_R:               key.CodeR,
	C.AKEYCODE_S:               key.CodeS,
	C.AKEYCODE_T:               key.CodeT,
	C.AKEYCODE_U:               key.CodeU,
	C.AKEYCODE_V:               key.CodeV,
	C.AKEYCODE_W:               key.CodeW,
	C.AKEYCODE_X:               key.CodeX,
	C.AKEYCODE_Y:               key.CodeY,
	C.AKEYCODE_Z:               key.CodeZ,
	C.AKEYCODE_COMMA:           key.CodeComma,
	C.AKEYCODE_PERIOD:          key.CodeFullStop,
	C.AKEYCODE_ALT_LEFT:        key.CodeLeftAlt,
	C.AKEYCODE_ALT_RIGHT:       key.CodeRightAlt,
	C.AKEYCODE_SHIFT_LEFT:      key.CodeLeftShift,
	C.AKEYCODE_SHIFT_RIGHT:     key.CodeRightShift,
	C.AKEYCODE_TAB:             key.CodeTab,
	C.AKEYCODE_SPACE:           key.CodeSpacebar,
	C.AKEYCODE_ENTER:           key.CodeReturnEnter,
	C.AKEYCODE_DEL:             key.CodeDeleteBackspace,
	C.AKEYCODE_GRAVE:           key.CodeGraveAccent,
	C.AKEYCODE_MINUS:           key.CodeHyphenMinus,
	C.AKEYCODE_EQUALS:          key.CodeEqualSign,
	C.AKEYCODE_LEFT_BRACKET:    key.CodeLeftSquareBracket,
	C.AKEYCODE_RIGHT_BRACKET:   key.CodeRightSquareBracket,
	C.AKEYCODE_BACKSLASH:       key.CodeBackslash,
	C.AKEYCODE_SEMICOLON:       key.CodeSemicolon,
	C.AKEYCODE_APOSTROPHE:      key.CodeApostrophe,
	C.AKEYCODE_SLASH:           key.CodeSlash,
	C.AKEYCODE_PAGE_UP:         key.CodePageUp,
	C.AKEYCODE_PAGE_DOWN:       key.CodePageDown,
	C.AKEYCODE_ESCAPE:          key.CodeEscape,
	C.AKEYCODE_FORWARD_DEL:     key.CodeDeleteForward,
	C.AKEYCODE_CTRL_LEFT:       key.CodeLeftControl,
	C.AKEYCODE_CTRL_RIGHT:      key.CodeRightControl,
	C.AKEYCODE_CAPS_LOCK:       key.CodeCapsLock,
	C.AKEYCODE_META_LEFT:       key.CodeLeftGUI,
	C.AKEYCODE_META_RIGHT:      key.CodeRightGUI,
	C.AKEYCODE_INSERT:          key.CodeInsert,
	C.AKEYCODE_F1:              key.CodeF1,
	C.AKEYCODE_F2:              key.CodeF2,
	C.AKEYCODE_F3:              key.CodeF3,
	C.AKEYCODE_F4:              key.CodeF4,
	C.AKEYCODE_F5:              key.CodeF5,
	C.AKEYCODE_F6:              key.CodeF6,
	C.AKEYCODE_F7:              key.CodeF7,
	C.AKEYCODE_F8:              key.CodeF8,
	C.AKEYCODE_F9:              key.CodeF9,
	C.AKEYCODE_F10:             key.CodeF10,
	C.AKEYCODE_F11:             key.CodeF11,
	C.AKEYCODE_F12:             key.CodeF12,
	C.AKEYCODE_NUM_LOCK:        key.CodeKeypadNumLock,
	C.AKEYCODE_NUMPAD_0:        key.CodeKeypad0,
	C.AKEYCODE_NUMPAD_1:        key.CodeKeypad1,
	C.AKEYCODE_NUMPAD_2:        key.CodeKeypad2,
	C.AKEYCODE_NUMPAD_3:        key.CodeKeypad3,
	C.AKEYCODE_NUMPAD_4:        key.CodeKeypad4,
	C.AKEYCODE_NUMPAD_5:        key.CodeKeypad5,
	C.AKEYCODE_NUMPAD_6:        key.CodeKeypad6,
	C.AKEYCODE_NUMPAD_7:        key.CodeKeypad7,
	C.AKEYCODE_NUMPAD_8:        key.CodeKeypad8,
	C.AKEYCODE_NUMPAD_9:        key.CodeKeypad9,
	C.AKEYCODE_NUMPAD_DIVIDE:   key.CodeKeypadSlash,
	C.AKEYCODE_NUMPAD_MULTIPLY: key.CodeKeypadAsterisk,
	C.AKEYCODE_NUMPAD_SUBTRACT: key.CodeKeypadHyphenMinus,
	C.AKEYCODE_NUMPAD_ADD:      key.CodeKeypadPlusSign,
	C.AKEYCODE_NUMPAD_DOT:      key.CodeKeypadFullStop,
	C.AKEYCODE_NUMPAD_ENTER:    key.CodeKeypadEnter,
	C.AKEYCODE_NUMPAD_EQUALS:   key.CodeKeypadEqualSign,
	C.AKEYCODE_VOLUME_MUTE:     key.CodeMute,
}

func convAndroidKeyCode(aKeyCode int32) key.Code {
	if code, ok := androidKeycoe[aKeyCode]; ok {
		return code
	}
	return key.CodeUnknown
}

/*
	Many Android key codes do not map into USB HID codes.
	For those, key.CodeUnknown is returned. This switch has all
	cases, even the unknown ones, to serve as a documentation
	and search aid.
	C.AKEYCODE_UNKNOWN
	C.AKEYCODE_SOFT_LEFT
	C.AKEYCODE_SOFT_RIGHT
	C.AKEYCODE_BACK
	C.AKEYCODE_CALL
	C.AKEYCODE_ENDCALL
	C.AKEYCODE_STAR
	C.AKEYCODE_POUND
	C.AKEYCODE_DPAD_UP
	C.AKEYCODE_DPAD_DOWN
	C.AKEYCODE_DPAD_LEFT
	C.AKEYCODE_DPAD_RIGHT
	C.AKEYCODE_DPAD_CENTER
	C.AKEYCODE_POWER
	C.AKEYCODE_CAMERA
	C.AKEYCODE_CLEAR
	C.AKEYCODE_SYM
	C.AKEYCODE_EXPLORER
	C.AKEYCODE_ENVELOPE
	C.AKEYCODE_AT
	C.AKEYCODE_NUM
	C.AKEYCODE_HEADSETHOOK
	C.AKEYCODE_FOCUS
	C.AKEYCODE_PLUS
	C.AKEYCODE_MENU
	C.AKEYCODE_NOTIFICATION
	C.AKEYCODE_SEARCH
	C.AKEYCODE_MEDIA_PLAY_PAUSE
	C.AKEYCODE_MEDIA_STOP
	C.AKEYCODE_MEDIA_NEXT
	C.AKEYCODE_MEDIA_PREVIOUS
	C.AKEYCODE_MEDIA_REWIND
	C.AKEYCODE_MEDIA_FAST_FORWARD
	C.AKEYCODE_MUTE
	C.AKEYCODE_PICTSYMBOLS
	C.AKEYCODE_SWITCH_CHARSET
	C.AKEYCODE_BUTTON_A
	C.AKEYCODE_BUTTON_B
	C.AKEYCODE_BUTTON_C
	C.AKEYCODE_BUTTON_X
	C.AKEYCODE_BUTTON_Y
	C.AKEYCODE_BUTTON_Z
	C.AKEYCODE_BUTTON_L1
	C.AKEYCODE_BUTTON_R1
	C.AKEYCODE_BUTTON_L2
	C.AKEYCODE_BUTTON_R2
	C.AKEYCODE_BUTTON_THUMBL
	C.AKEYCODE_BUTTON_THUMBR
	C.AKEYCODE_BUTTON_START
	C.AKEYCODE_BUTTON_SELECT
	C.AKEYCODE_BUTTON_MODE
	C.AKEYCODE_SCROLL_LOCK
	C.AKEYCODE_FUNCTION
	C.AKEYCODE_SYSRQ
	C.AKEYCODE_BREAK
	C.AKEYCODE_MOVE_HOME
	C.AKEYCODE_MOVE_END
	C.AKEYCODE_FORWARD
	C.AKEYCODE_MEDIA_PLAY
	C.AKEYCODE_MEDIA_PAUSE
	C.AKEYCODE_MEDIA_CLOSE
	C.AKEYCODE_MEDIA_EJECT
	C.AKEYCODE_MEDIA_RECORD
	C.AKEYCODE_NUMPAD_COMMA
	C.AKEYCODE_NUMPAD_LEFT_PAREN
	C.AKEYCODE_NUMPAD_RIGHT_PAREN
	C.AKEYCODE_INFO
	C.AKEYCODE_CHANNEL_UP
	C.AKEYCODE_CHANNEL_DOWN
	C.AKEYCODE_ZOOM_IN
	C.AKEYCODE_ZOOM_OUT
	C.AKEYCODE_TV
	C.AKEYCODE_WINDOW
	C.AKEYCODE_GUIDE
	C.AKEYCODE_DVR
	C.AKEYCODE_BOOKMARK
	C.AKEYCODE_CAPTIONS
	C.AKEYCODE_SETTINGS
	C.AKEYCODE_TV_POWER
	C.AKEYCODE_TV_INPUT
	C.AKEYCODE_STB_POWER
	C.AKEYCODE_STB_INPUT
	C.AKEYCODE_AVR_POWER
	C.AKEYCODE_AVR_INPUT
	C.AKEYCODE_PROG_RED
	C.AKEYCODE_PROG_GREEN
	C.AKEYCODE_PROG_YELLOW
	C.AKEYCODE_PROG_BLUE
	C.AKEYCODE_APP_SWITCH
	C.AKEYCODE_BUTTON_1
	C.AKEYCODE_BUTTON_2
	C.AKEYCODE_BUTTON_3
	C.AKEYCODE_BUTTON_4
	C.AKEYCODE_BUTTON_5
	C.AKEYCODE_BUTTON_6
	C.AKEYCODE_BUTTON_7
	C.AKEYCODE_BUTTON_8
	C.AKEYCODE_BUTTON_9
	C.AKEYCODE_BUTTON_10
	C.AKEYCODE_BUTTON_11
	C.AKEYCODE_BUTTON_12
	C.AKEYCODE_BUTTON_13
	C.AKEYCODE_BUTTON_14
	C.AKEYCODE_BUTTON_15
	C.AKEYCODE_BUTTON_16
	C.AKEYCODE_LANGUAGE_SWITCH
	C.AKEYCODE_MANNER_MODE
	C.AKEYCODE_3D_MODE
	C.AKEYCODE_CONTACTS
	C.AKEYCODE_CALENDAR
	C.AKEYCODE_MUSIC
	C.AKEYCODE_CALCULATOR

	Defined in an NDK API version beyond what we use today:
	C.AKEYCODE_ASSIST
	C.AKEYCODE_BRIGHTNESS_DOWN
	C.AKEYCODE_BRIGHTNESS_UP
	C.AKEYCODE_RO
	C.AKEYCODE_YEN
	C.AKEYCODE_ZENKAKU_HANKAKU
*/
