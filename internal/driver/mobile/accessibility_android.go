//go:build accessibility && android

package mobile

/*
#include <stdlib.h>
#include <stdint.h>

int belluiA11yEnabled(uintptr_t jni_env, uintptr_t ctx);
void belluiA11ySetup(uintptr_t jni_env, uintptr_t ctx);
void belluiA11yClearNodes(uintptr_t jni_env, uintptr_t ctx);
void belluiA11yAddNode(uintptr_t jni_env, uintptr_t ctx, int id, int role, int flags,
	const char *label, const char *value, const char *hint,
	int x, int y, int width, int height,
	int parent, int row, int col, int rowCount, int colCount,
	const char *text, int selStart, int selEnd,
	const char *customActs, const char *textLayout);
void belluiA11yCommitNodes(uintptr_t jni_env, uintptr_t ctx);
void belluiA11yAnnounce(uintptr_t jni_env, uintptr_t ctx, const char *text, int assertive);
*/
import "C"

import (
	"unsafe"

	"github.com/belluikit/bellui/internal/accessibility"
	"github.com/belluikit/bellui/internal/driver/mobile/app"
)

// mobileA11yActionStateChanged is the pseudo action kind the Java side sends
// (with id 0) when the system accessibility state flips on, so a tree that
// was being skipped gets rebuilt. Keep in sync with GoNativeActivity.java.
const mobileA11yActionStateChanged = 5

func init() {
	// TalkBack actions arrive on the Android UI thread via the JNI up-call;
	// route them into the shared dispatcher, which queues onto the main
	// goroutine.
	app.SetAccessibilityActionHandler(func(id, kind int32) {
		if kind == mobileA11yActionStateChanged {
			// Fired on both enable and disable. Clear any in-scene focus ring
			// (pre-rebrand tracker #759) before the repaint — on disable no _FOCUS_CLEARED reaches
			// the torn-down proxies, so the ring would otherwise stay painted;
			// on enable the focus is empty, so this is a no-op.
			a11yFocusedHandle.Store(0)
			mobileA11yRefreshContent()
			return
		}
		mobileA11yDispatch(uintptr(id), int(kind))
	})

	// An accessibility service asking a virtualized grid proxy to bring a
	// (row, col) cell into view (AccessibilityAction.ACTION_SCROLL_TO_POSITION,
	// pre-rebrand tracker #817 — TalkBack has no point-addressed cell read, so scroll-into-view is
	// Android's realize analog: the cell then enters the walked window as a
	// normal element). Arrives on the Android UI thread; the widget call is
	// queued to the main goroutine inside mobileA11yScrollCellIntoView.
	app.SetAccessibilityScrollToHandler(func(id, row, col int32) {
		mobileA11yScrollCellIntoView(uintptr(id), int(row), int(col))
	})

	// Editable-text node actions (pre-rebrand tracker #974 option D): the Java protocol layer
	// resolves granularity segments and UTF-16 offsets itself, applies the
	// result optimistically, and sends the edit here as rune offsets. Arrives
	// on the Android UI thread; the dispatch queues onto the main goroutine
	// and lands on the editor via the internal/textinput primitives.
	app.SetAccessibilityTextActionHandler(func(id, kind, a, b int32, text string) {
		mobileA11yTextDispatch(uintptr(id), int(kind), int(a), int(b), text)
	})

	// TalkBack accessibility-focus changes drive the in-scene focus ring (pre-rebrand tracker #759):
	// the OS ring cannot reach the flat GPU surface, so Bellui paints its own over
	// the focused element. The callback arrives on the Android UI thread;
	// mobileA11ySetFocus stores the handle atomically and queues the repaint.
	app.SetAccessibilityFocusHandler(func(id int32, focused bool) {
		mobileA11ySetFocus(uintptr(id), focused)
	})

	// Live-region announcements (pre-rebrand tracker #525): deliver accessibility.Announce calls
	// via View.announceForAccessibility so TalkBack speaks transient status
	// (Toast, Snackbar) without moving its focus.
	accessibility.AddAnnouncementListener(func(text string, assertive bool) {
		ctext := C.CString(text)
		defer C.free(unsafe.Pointer(ctext))
		var flag C.int
		if assertive {
			flag = 1
		}
		app.RunOnJVM(func(_, jniEnv, ctx uintptr) error { //nolint:errcheck
			C.belluiA11yAnnounce(C.uintptr_t(jniEnv), C.uintptr_t(ctx), ctext, flag)
			return nil
		})
	})
}

// updateAccessibility publishes the current semantic tree to TalkBack as a
// flat overlay of touch-transparent views (one per element, positioned over
// the widget it describes) hosted by GoNativeActivity. The Java side skips
// the rebuild when nothing changed. Runs on the main goroutine from the
// paint loop.
func (w *window) updateAccessibility() {
	enabled := false
	app.RunOnJVM(func(_, jniEnv, ctx uintptr) error { //nolint:errcheck
		enabled = C.belluiA11yEnabled(C.uintptr_t(jniEnv), C.uintptr_t(ctx)) != 0
		return nil
	})
	if !enabled {
		// No assistive technology running; skip the gather entirely. Also drop
		// any stale in-scene focus ring (pre-rebrand tracker #759) so it can't outlive the screen
		// reader if the state-change event was ever missed — cleared here, gone
		// on the next repaint.
		a11yFocusedHandle.Store(0)
		return
	}

	nodes, _ := w.mobileA11yGather() // Java rebuilds wholesale; no per-element teardown

	app.RunOnJVM(func(_, jniEnv, ctx uintptr) error { //nolint:errcheck
		jenv := C.uintptr_t(jniEnv)
		jctx := C.uintptr_t(ctx)

		C.belluiA11yClearNodes(jenv, jctx)
		for i := range nodes {
			n := &nodes[i]
			label := C.CString(n.label)
			value := C.CString(n.value)
			hint := C.CString(n.hint)
			text := C.CString(n.text)
			customActs := C.CString(a11yEncodeCustomActs(n.customActs))
			textLayout := C.CString(n.textLayout)
			C.belluiA11yAddNode(jenv, jctx,
				C.int(n.handle), C.int(n.role), C.int(n.flags),
				label, value, hint,
				C.int(n.x), C.int(n.y), C.int(n.w), C.int(n.h),
				C.int(n.gridHandle), C.int(n.cellRow), C.int(n.cellCol),
				C.int(n.rowCount), C.int(n.colCount),
				text, C.int(n.selStart), C.int(n.selEnd),
				customActs, textLayout)
			C.free(unsafe.Pointer(label))
			C.free(unsafe.Pointer(value))
			C.free(unsafe.Pointer(hint))
			C.free(unsafe.Pointer(text))
			C.free(unsafe.Pointer(customActs))
			C.free(unsafe.Pointer(textLayout))
		}
		C.belluiA11yCommitNodes(jenv, jctx)
		return nil
	})
}

func (w *window) initAccessibilityForWindow() {
	app.RunOnJVM(func(_, jniEnv, ctx uintptr) error { //nolint:errcheck
		C.belluiA11ySetup(C.uintptr_t(jniEnv), C.uintptr_t(ctx))
		return nil
	})
}

func (w *window) cleanupAccessibilityForWindow() {
	mobileA11yForgetWindow(w)
	app.RunOnJVM(func(_, jniEnv, ctx uintptr) error { //nolint:errcheck
		jenv := C.uintptr_t(jniEnv)
		jctx := C.uintptr_t(ctx)
		C.belluiA11yClearNodes(jenv, jctx)
		C.belluiA11yCommitNodes(jenv, jctx)
		return nil
	})
}
