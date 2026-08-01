package org.golang.app;

import android.app.Activity;
import android.app.NativeActivity;
import android.content.Context;
import android.content.Intent;
import android.content.pm.ActivityInfo;
import android.content.pm.PackageManager;
import android.content.res.Configuration;
import android.graphics.Rect;
import android.net.Uri;
import android.os.Build;
import android.os.Bundle;
import android.text.Editable;
import android.text.InputType;
import android.text.Selection;
import android.text.SpannableStringBuilder;
import android.text.Spanned;
import android.text.TextWatcher;
import android.text.method.DigitsKeyListener;
import android.util.Log;
import android.view.Gravity;
import android.view.KeyCharacterMap;
import android.view.MotionEvent;
import android.view.View;
import android.view.ViewGroup;
import android.view.ViewParent;
import android.view.WindowInsets;
import android.view.accessibility.AccessibilityEvent;
import android.view.accessibility.AccessibilityManager;
import android.view.accessibility.AccessibilityNodeInfo;
import android.view.accessibility.AccessibilityNodeProvider;
import android.view.inputmethod.BaseInputConnection;
import android.view.inputmethod.EditorInfo;
import android.view.inputmethod.ExtractedText;
import android.view.inputmethod.ExtractedTextRequest;
import android.view.inputmethod.InputConnection;
import android.view.inputmethod.InputMethodManager;
import android.view.KeyEvent;
import android.webkit.WebResourceError;
import android.webkit.WebResourceRequest;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.widget.EditText;
import android.widget.FrameLayout;
import android.widget.TextView;
import android.widget.TextView.OnEditorActionListener;

public class GoNativeActivity extends NativeActivity {
	private static GoNativeActivity goNativeActivity;
	private static final int FILE_OPEN_CODE = 1;
	private static final int FILE_SAVE_CODE = 2;

	// The shadow EditText must advertise a real text class: an inputType with
	// no TYPE_MASK_CLASS bits reads as TYPE_NULL to the IME, and IMEs disable
	// inline composition against null-class editors (Gboard pinyin composed in
	// its own bar and never called setComposingText — found on the #945 device
	// pass). NO_SUGGESTIONS is intentionally NOT set (#1006 Phase 4): it was a
	// #945 band-aid that suppressed the latin autocorrect strip because, in the
	// pre-Phase-4 architecture, autocorrect's recompose of already-committed
	// words drove the #966 replication. Phase 4 owns the composing region and
	// clears it on every bellui-side selection change (setKeyboardState), so
	// autocorrect / inline suggestions are now safe and are wanted (the Phase 4
	// gate). CJK composition is unaffected either way.
	// AUTO_CORRECT + CAP_SENTENCES mark this as a normal prose text field, so the
	// IME offers autocorrect and suggestions at all (#1007). Jetpack Compose sets
	// exactly these from its own (also custom, non-EditText) input connection —
	// EditorInfo.update() ORs TYPE_TEXT_FLAG_AUTO_CORRECT whenever autocorrect is
	// on, its default, plus the capitalization flag — and a working stock field
	// (OnePlus Notes, measured: inputType=0x2c0a1) carries the identical flag set.
	// Not applied to password/number fields (autocorrect there is wrong).
	private static final int TEXT_SUGGEST_FLAGS = InputType.TYPE_TEXT_FLAG_AUTO_CORRECT
			| InputType.TYPE_TEXT_FLAG_CAP_SENTENCES;
	private static final int DEFAULT_INPUT_TYPE = InputType.TYPE_CLASS_TEXT
			| InputType.TYPE_TEXT_FLAG_MULTI_LINE
			| TEXT_SUGGEST_FLAGS;

	// -1: no keyboard applied. Distinct from every real code, which start at 0
	// and must mirror mobile.KeyboardType in driver/mobile/keyboard.go.
	private static final int NO_KEYBOARD_CODE = -1;
	private static final int DEFAULT_KEYBOARD_CODE = 0;
	private static final int SINGLELINE_KEYBOARD_CODE = 1;
	private static final int NUMBER_KEYBOARD_CODE = 2;
	private static final int PASSWORD_KEYBOARD_CODE = 3;

    private native void filePickerReturned(String str);
    private native void insetsChanged(int top, int bottom, int left, int right);
    private native void keyboardTyped(String str);
    private native void keyboardReplace(int start, int end, String str);
    private native void keyboardComposing(String str, int cursor);
    private native void keyboardComposeCommit(String str);
    private native void keyboardDelete();
    private native void keyboardArrow(int code, int metaState);
    private native void keyboardShortcut(int keyCode, int metaState);
    private native void keyboardSetSelection(int start, int end);
    private native void backPressed();
    private native void setDarkMode(boolean dark);
    private native void accessibilityAction(int id, int kind);
    private native void accessibilityActionScrollTo(int id, int row, int col);
    private native void accessibilityTextAction(int id, int kind, int a, int b, String text);
    private native void accessibilityFocusChanged(int id, boolean focused);
    private native void setTouchExploration(boolean enabled);
    private native void openIntent(String uri);
    private native void notificationActivated(long token, int action);

	private EditText mTextEdit;
	private boolean keyboardUp = false;
	// The keyboard state actually APPLIED to mTextEdit, owned by (and only ever
	// touched on) the UI thread — unlike keyboardUp, which the static
	// showKeyboard/hideKeyboard entry points set eagerly on the caller thread
	// before the UI-thread runnable has run, so it cannot answer "is the IME
	// already up for this type?" (#1016). NO_KEYBOARD_CODE = nothing applied.
	private int mAppliedKeyboardType = NO_KEYBOARD_CODE;
	private int mAppliedInputType = 0;
	private int mAppliedImeOptions = 0;
	// Set (on the UI thread) by keyboardEditorChanged when the bound bellui editor's
	// IDENTITY changes (#966), consumed by the next doSetKeyboardState: that push
	// is the NEW field's seed and must reseed + restartInput unconditionally —
	// never be dropped by the composing-echo or recent-state filters (whose state
	// belongs to the previous field) and never take the sameText short-circuit
	// (two empty fields still need the restart, or the IME session — and its
	// composing word — silently spans both fields).
	private boolean mEditorChanged = false;
	// #1006 Phase 4 — bellui-driven flat IME transport (the Flutter
	// InputConnectionAdaptor model). mTextEdit is only the focusable host that
	// triggers the soft keyboard and vends onCreateInputConnection; its own
	// TextView document is NOT the source of truth. The IME edits a bellui-owned
	// flat buffer through BelluiInputConnection, and bellui pushes its document into
	// that buffer via setKeyboardState. The bellui Entry/RichEntry document remains
	// the single source of truth; nothing is read back out of a native view.
	//
	// mBelluiBuffer is the flat mirror the IME's InputConnection operates on
	// (BaseInputConnection maintains the composing span + selection here). bellui
	// drives its content/selection; edits made through it are forwarded to bellui
	// as rune-offset ops. A bellui-side push notifies the IMM against mTextEdit
	// (updateSelection / restartInput).
	private Editable mBelluiBuffer;
	// IME composition state. TWO DISTINCT STATES share the buffer's composing
	// span, and only one of them is this flag (#1021):
	//
	//   composing == true — a BELLUI-SIDE PREEDIT: the soft keyboard is building
	//   inline composition text (a CJK candidate) that bellui shows display-only
	//   and keeps OUT of its document until commit. Reported via
	//   keyboardComposing/keyboardComposeCommit; composingText holds the latest
	//   preedit so finishComposingText (accept-as-typed) can commit it. Only in
	//   this state do bellui's document and the buffer diverge, so only this state
	//   suppresses the bellui-side push (it would clobber the preedit).
	//
	//   composing == false with a buffer composing SPAN — a REGION RECOMPOSE:
	//   the IME marked already-COMMITTED text (Gboard does this to the word at
	//   the caret on every connect). The text stays committed on both sides —
	//   bellui == buffer — the span is a marker only (the platform/Flutter model),
	//   and pushes keep flowing; the span rides along in updateSelection so the
	//   IME drops it itself when the caret leaves it. Treating this state as
	//   "composing" is what deadlocked TalkBack caret moves (#1021).
	private boolean composing = false;
	private String composingText = "";
	// Extracted-text dirty region (#1009): the union of buffer regions edited
	// since the last extracted-text report to the IME, maintained by a
	// TextWatcher span on mBelluiBuffer (the TextView editor's ChangeWatcher
	// model) so every mutation path — IC ops, the DEL key's manual delete,
	// bellui re-seeds — is covered without per-site bookkeeping. Start offsets
	// are stable across later edits (the prefix above them is untouched); the
	// end is tracked in CURRENT-text coordinates alongside the accumulated
	// length delta, so the OLD-text end the ExtractedText partial contract
	// wants (partialEndOffset) is extractDirtyEndNew - extractDirtyDelta.
	// extractDirtyStart < 0 = clean (nothing to report). Serving any
	// extracted-text report resets the region — the IME's cache is then
	// current again.
	private int extractDirtyStart = -1;
	private int extractDirtyEndNew = -1;
	private int extractDirtyDelta = 0;

	// noteExtractDirty folds one buffer change — replace of [start,
	// start+before) with `after` chars — into the dirty-region union (#1009).
	private void noteExtractDirty(int start, int before, int after) {
		if (extractDirtyStart < 0) {
			extractDirtyStart = start;
			extractDirtyEndNew = start + after;
		} else {
			extractDirtyStart = Math.min(extractDirtyStart, start);
			if (start + before <= extractDirtyEndNew) {
				// The change lies at or before the tracked end: it shifts.
				extractDirtyEndNew += after - before;
			} else {
				// The change extends past the tracked end: the new end is the
				// change's own end (text beyond it is untouched).
				extractDirtyEndNew = start + after;
			}
			if (extractDirtyEndNew < extractDirtyStart) {
				extractDirtyEndNew = extractDirtyStart;
			}
		}
		extractDirtyDelta += after - before;
	}

	private void clearExtractDirty() {
		extractDirtyStart = -1;
		extractDirtyEndNew = -1;
		extractDirtyDelta = 0;
	}

	// snapExtractBoundary moves a window boundary off the middle of a
	// surrogate pair (the framework contract never splits one).
	private static int snapExtractBoundary(CharSequence s, int off) {
		if (off > 0 && off < s.length() && Character.isLowSurrogate(s.charAt(off))
				&& Character.isHighSurrogate(s.charAt(off - 1))) {
			return off - 1;
		}
		return off;
	}
	// Push reconciliation state (#1007). mBelluiBuffer and bellui's document can
	// disagree for three very different reasons, and only one of them may answer
	// with imm.restartInput() — a restart tears down the IME's session, and an IME
	// consults the editor state it cached at onStartInput before offering an
	// autocorrect or replacing a word, so restarting every keystroke breaks those
	// features outright (Gboard logged "startInput restarting=true" mid-word):
	//   1. bellui is LAGGING behind the IME's own edit. The IC mutates the buffer
	//      synchronously but forwards to bellui asynchronously (event queue), so a
	//      push — especially the per-frame one — can still carry pre-edit text. The
	//      buffer is already correct: drop the push, never restart. Detected by the
	//      push matching a RECENT PAST buffer state (recentStates).
	//   2. The IME's own edit round-tripped: bellui's text changed and now equals the
	//      buffer. Nothing to do but sync the selection. Never restart.
	//   3. bellui genuinely holds a different document — a programmatic SetText, undo,
	//      a TalkBack edit, or (the case a naive echo filter gets fatally wrong) a
	//      FOCUS SWITCH, where the push carries a wholly different editor's text.
	//      The buffer must be re-seeded and the IME must re-read: restartInput.
	// recentStates is a small ring of hash+length keys of past buffer states, and is
	// what separates case 1 from case 3: a lag echo is always a document the buffer
	// HAS held, while a focus switch never is. (Compose's updateState() compares its
	// own oldValue to newValue for the same purpose; it can, because the framework
	// is the single writer — here the buffer runs ahead of bellui, so past buffer
	// states are the equivalent signal.) Cheap keys rather than retained strings so
	// a large document is not copied per edit; a hash collision would drop one push,
	// which the next push corrects.
	//
	// Entries EXPIRE (#1023): "the buffer has held this document" is only evidence
	// of lag for a moment — the bellui echo of an IME edit lands within a frame
	// (measured 7 ms; bounded by the event-queue round-trip). An UNDO also returns
	// bellui to a past buffer state, but seconds later, and dropping that push
	// desyncs permanently: the per-repaint Sync is redundant-push-filtered, so no
	// retry ever comes, the buffer keeps the pre-undo document, and the IME's next
	// edit lands at bogus offsets (measured: select-all + type + undo×2 + type →
	// the character at position 0). An expired match falls through to case 3
	// (re-seed + restartInput) — exactly what an undo needs.
	// UI thread only (the IC callbacks and doSetKeyboardState both run there).
	private static final long RECENT_STATE_TTL_MS = 1000;
	private final long[] recentStates = new long[8];
	private final long[] recentStatesAtMs = new long[8];
	private int recentStatesAt = 0;

	private static long bufferStateKey(String t) {
		return ((long) t.hashCode() << 32) | (t.length() & 0xffffffffL);
	}

	// noteBufferState records the buffer's current content as a recent state, so a
	// later push carrying it is recognised as bellui lagging rather than as an
	// external change. Called after every mutation of mBelluiBuffer.
	private void noteBufferState() {
		if (mBelluiBuffer == null) {
			return;
		}
		recentStates[recentStatesAt] = bufferStateKey(mBelluiBuffer.toString());
		recentStatesAtMs[recentStatesAt] = android.os.SystemClock.uptimeMillis();
		recentStatesAt = (recentStatesAt + 1) % recentStates.length;
	}

	private boolean isRecentBufferState(String want) {
		long k = bufferStateKey(want);
		long now = android.os.SystemClock.uptimeMillis();
		for (int i = 0; i < recentStates.length; i++) {
			if (recentStates[i] == k && now - recentStatesAtMs[i] <= RECENT_STATE_TTL_MS) {
				return true;
			}
		}
		return false;
	}

	// clearRecentBufferStates drops the echo ring wholesale on an editor change
	// (#966): its entries describe the PREVIOUS field's buffer, and the new
	// field's seed push must never match one and be dropped as "bellui lagging"
	// (e.g. both fields empty within the TTL window).
	private void clearRecentBufferStates() {
		for (int i = 0; i < recentStates.length; i++) {
			recentStates[i] = 0;
			recentStatesAtMs[i] = 0;
		}
	}


	public GoNativeActivity() {
		super();
		goNativeActivity = this;
	}

	String getTmpdir() {
		return getCacheDir().getAbsolutePath();
	}

	void updateLayout() {
	    try {
            WindowInsets insets = getWindow().getDecorView().getRootWindowInsets();
            if (insets == null) {
                return;
            }

            insetsChanged(insets.getSystemWindowInsetTop(), insets.getSystemWindowInsetBottom(),
                insets.getSystemWindowInsetLeft(), insets.getSystemWindowInsetRight());
        } catch (java.lang.NoSuchMethodError e) {
    	    Rect insets = new Rect();
            getWindow().getDecorView().getWindowVisibleDisplayFrame(insets);

            View view = findViewById(android.R.id.content).getRootView();
            insetsChanged(insets.top, view.getHeight() - insets.height() - insets.top,
                insets.left, view.getWidth() - insets.width() - insets.left);
        }
    }

    static void showKeyboard(int keyboardType) {
        goNativeActivity.doShowKeyboard(keyboardType);
        goNativeActivity.keyboardUp = true;
    }

    void doShowKeyboard(final int keyboardType) {
        runOnUiThread(new Runnable() {
            @Override
            public void run() {
                int imeOptions = EditorInfo.IME_FLAG_NO_ENTER_ACTION;
                int inputType = DEFAULT_INPUT_TYPE;
                String keys = "";
                switch (keyboardType) {
                    case DEFAULT_KEYBOARD_CODE:
                        imeOptions = EditorInfo.IME_FLAG_NO_ENTER_ACTION;
                        break;
                    case SINGLELINE_KEYBOARD_CODE:
                        imeOptions = EditorInfo.IME_ACTION_DONE;
                        // NO_SUGGESTIONS intentionally dropped in #1006 Phase 4
                        // (see DEFAULT_INPUT_TYPE): autocorrect is now safe. The
                        // suggest flags mark it a prose field (#1007).
                        inputType = InputType.TYPE_CLASS_TEXT | TEXT_SUGGEST_FLAGS;
                        break;
                    case NUMBER_KEYBOARD_CODE:
                        imeOptions = EditorInfo.IME_ACTION_DONE;
                        // A single class only: OR-ing class bits onto a text
                        // class would spell a different class entirely.
                        inputType = InputType.TYPE_CLASS_NUMBER | InputType.TYPE_NUMBER_VARIATION_NORMAL;
                        keys = "0123456789.,-' "; // work around android bug where some number keys are blocked
                        break;
                    case PASSWORD_KEYBOARD_CODE:
                        imeOptions = EditorInfo.IME_ACTION_DONE;
                        inputType = InputType.TYPE_CLASS_TEXT
                                | InputType.TYPE_TEXT_VARIATION_PASSWORD;
                        break;
                    default:
                        Log.e("Bellui", "unknown keyboard type, use default");
                }
                imeOptions |= EditorInfo.IME_FLAG_NO_FULLSCREEN;

                // Idempotence gate (#1016, #1006 Phase 5). doShowKeyboard is
                // called on EVERY focus or overlay change — canvas.go's
                // `OnFocus: d.handleKeyboard` and `overlayChanged()` — so a
                // selection-handles overlay appearing, or a re-focus of the
                // already-focused field, used to re-run the whole body below.
                // That is not free: setInputType on a focused View forces a
                // restartInput by itself and showSoftInput triggers
                // ATTACH_NEW_INPUT, so the IME session was restarted on any UI
                // churn (48 GoogleInputMethodService.onStartInput in one short
                // session, including mid-typing). An IME consults the editor
                // state it cached at onStartInput before offering an autocorrect
                // or replacing a word, so that churn throws away exactly the
                // cache #1007 depends on, and racing it against the a11y
                // reconcile's layout traversals (which make ViewRootImpl call
                // imm.checkFocus()) is the suspected source of the intermittent
                // mServedView=null under TalkBack — keyboard up, no served
                // editor, keystrokes discarded (#1005 Bug 1).
                //
                // So: when the IME is already up for this exact configuration
                // and the connection is HEALTHY, there is nothing to do —
                // return without touching the IME. Correctness rests on this
                // being the APPLIED state (mAppliedKeyboardType is written below
                // on the UI thread, and cleared by doHideKeyboard), never the
                // eagerly-set keyboardUp: a back-press dismissal runs
                // doHideKeyboard, so a later show still re-raises the keyboard.
                // A focus switch between two fields of the same type is
                // deliberately skipped too — the shadow is a document-less sink
                // (#1006 Phase 3), nothing about it is per-field except the
                // input type, and the document swap is handled by
                // setKeyboardState's re-seed + restartInput (#1007).
                //
                // isActive(mTextEdit) is the load-bearing half of "healthy": it
                // reports whether mTextEdit is the view the IME is actually
                // SERVING (IMM matches it against its served view), which is
                // exactly the state #1005 Bug 1 observes going wrong —
                // mServedView=null while the shadow is focused and mInputShown
                // is true, so the keyboard is up but keystrokes are discarded.
                // Gating on focus alone would skip the show in precisely that
                // state and make a dropped connection PERMANENT; gating on
                // isActive() instead makes a re-show the repair path for it, so
                // any focus/overlay change reconnects a lost IME rather than
                // restarting a healthy one.
                InputMethodManager m = (InputMethodManager) getSystemService(Context.INPUT_METHOD_SERVICE);
                if (mAppliedKeyboardType == keyboardType
                        && mAppliedInputType == inputType
                        && mAppliedImeOptions == imeOptions
                        && mTextEdit.getVisibility() == View.VISIBLE
                        && mTextEdit.isFocused()
                        && m.isActive(mTextEdit)) {
                    return;
                }

                // Clear any stale composition state from a previous field: an IME
                // that abandoned composition without finishComposingText (e.g. on
                // a focus switch) would otherwise leave composing==true, wedging
                // the setKeyboardState push (which skips while composing) for the
                // new field. Deliberately below the gate: a redundant show must
                // not wipe an IN-FLIGHT composition (an overlay appearing
                // mid-CJK-composition would otherwise wedge every later push).
                composing = false;
                composingText = "";

                if (mAppliedImeOptions != imeOptions) {
                    mTextEdit.setImeOptions(imeOptions);
                }
                // setInputType on a focused View forces a restartInput, so only
                // ever call it when the value actually changes (#1016). It also
                // resets the key listener, hence the re-apply below it.
                if (mAppliedInputType != inputType) {
                    mTextEdit.setInputType(inputType);
                    if (!keys.isEmpty()) {
                        mTextEdit.setKeyListener(DigitsKeyListener.getInstance(keys));
                    }
                }

                mTextEdit.setOnEditorActionListener(new OnEditorActionListener() {
                    @Override
                    public boolean onEditorAction(TextView v, int actionId, KeyEvent event) {
                        if (actionId == EditorInfo.IME_ACTION_DONE) {
                            keyboardTyped("\n");
                        }
                        return false;
                    }
                });

                // The shadow EditText is a document-less IME sink: it does not
                // mirror the focused Bellui Entry (#1006 Phase 3 removed the #572
                // state push). bellui drives its buffer through setKeyboardState
                // and the IME edits it through BelluiInputConnection (Phase 4).
                mTextEdit.setVisibility(View.VISIBLE);
                mTextEdit.bringToFront();
                mTextEdit.requestFocus();

                m.showSoftInput(mTextEdit, 0);

                mAppliedKeyboardType = keyboardType;
                mAppliedInputType = inputType;
                mAppliedImeOptions = imeOptions;
            }
        });
    }

    static void hideKeyboard() {
        goNativeActivity.doHideKeyboard();
        goNativeActivity.keyboardUp = false;
    }

    void doHideKeyboard() {
        // InputMethodManager is driven from the UI thread only: calling
        // hideSoftInputFromWindow from the JNI-attached caller thread entered
        // IMM's internal locking off-thread, one edge of the field-switch
        // deadlock under a TalkBack activate dispatch (fyne#973).
        runOnUiThread(new Runnable() {
            @Override
            public void run() {
                // While a webview panel holds the keyboard, this hide is not
                // ours to make: the driver decides for bellui's own entries
                // only, and stomping the web view's IME made the keyboard
                // vanish mid-typing (#76). The panel gives the keyboard back
                // through the focus bridge, which flips it non-focusable.
                if (webviewAnyPanelFocused()) {
                    return;
                }
                composing = false;
                composingText = "";
                // The applied-state cache is what doShowKeyboard's idempotence
                // gate tests (#1016); clearing it here is what makes a later
                // show re-raise the keyboard rather than skip as redundant —
                // including the back-press dismissal, which hides the keyboard
                // while the bellui field stays focused, so the next show carries
                // an unchanged keyboard type. mAppliedInputType/ImeOptions are
                // deliberately NOT cleared: they track what is actually set on
                // the View, which hiding does not change, so the next show can
                // still skip a needless setInputType (= a needless restartInput).
                mAppliedKeyboardType = NO_KEYBOARD_CODE;
                InputMethodManager imm = (InputMethodManager) getSystemService(Context.INPUT_METHOD_SERVICE);
                View view = findViewById(android.R.id.content).getRootView();
                imm.hideSoftInputFromWindow(view.getWindowToken(), 0);
                mTextEdit.setVisibility(View.GONE);
            }
        });
    }

    // setKeyboardState is called from Go (#1006 Phase 4) with the focused bellui
    // editor's flat text and selection (rune offsets). It mirrors that state into
    // the bellui-owned IME buffer and notifies the IMM, so the soft keyboard's
    // InputConnection always reads bellui's real document + caret. Skipped only
    // while a BELLUI-SIDE PREEDIT is active (composing == true) — bellui's committed
    // text excludes the in-flight preedit and would clobber it. A region-recompose
    // span over committed text does NOT block the push (#1021): bellui == buffer
    // then, and the selection report is exactly what lets the IME notice the
    // caret left its span and finish composing on its own.
    //
    // Three-way, in strict order of how disruptive the answer is (#1007) — see the
    // recentStates comment for the full reasoning:
    //   1. the push carries a RECENT PAST buffer state → bellui is lagging behind an
    //      IME edit already in the buffer: drop it, never restart;
    //   2. the push matches the buffer → at most an updateSelection, never a
    //      restart, so the IME keeps its session and any composing region;
    //   3. otherwise bellui genuinely holds a different document (SetText, undo,
    //      TalkBack, or a FOCUS SWITCH to another field): re-seed the buffer, remove
    //      the composing span (so the IME can never spontaneously recompose
    //      committed text — the #966 replication) and restartInput so it re-reads.
    // Case 3 used to swallow case 1, restarting the IME every keystroke; a naive
    // echo filter instead swallows case 3, leaving the buffer on the previous
    // field's document so every edit lands at a bogus offset.
    //
    // A push arriving with mEditorChanged armed (keyboardEditorChanged, #966) is
    // the NEW field's seed and bypasses all three: filters describing the
    // previous field must not judge it, and it must restartInput even when the
    // text coincidentally matches (two empty fields) so the IME session — and
    // any word it was composing in the previous field — ends at the boundary.
    static void setKeyboardState(String text, int selStart, int selEnd) {
        if (goNativeActivity == null) {
            return;
        }
        goNativeActivity.doSetKeyboardState(text, selStart, selEnd);
    }

    // keyboardEditorChanged is called from Go (via the ordered keyboard-op queue,
    // between the show and the seed push) when the BOUND EDITOR'S IDENTITY
    // changes — focus moved to a different bellui field (#966). Every piece of IME
    // session state held here belongs to the previous field: the bellui-side
    // composing mirror (Gboard composes ordinary Latin words, so composing is
    // routinely true mid-word — and doShowKeyboard's idempotence gate skips
    // same-type focus switches WITHOUT clearing it), and the recent-buffer-state
    // echo ring, which would let the new field's seed be dropped as "bellui
    // lagging". Reset both and arm mEditorChanged so the seed push restarts the
    // IME session unconditionally. Without this, one Gboard editing session
    // spanned fields: its composing word followed the focus into the next field,
    // recompose cycles replayed the concatenated buffer at misaligned offsets,
    // and a password's mask and plaintext sprayed into sibling fields.
    static void keyboardEditorChanged() {
        if (goNativeActivity == null) {
            return;
        }
        goNativeActivity.doEditorChanged();
    }

    void doEditorChanged() {
        runOnUiThread(new Runnable() {
            @Override
            public void run() {
                // While a webview panel holds the keyboard, this hide is not
                // ours to make: the driver decides for bellui's own entries
                // only, and stomping the web view's IME made the keyboard
                // vanish mid-typing (#76). The panel gives the keyboard back
                // through the focus bridge, which flips it non-focusable.
                if (webviewAnyPanelFocused()) {
                    return;
                }
                composing = false;
                composingText = "";
                clearRecentBufferStates();
                mEditorChanged = true;
            }
        });
    }

    void doSetKeyboardState(final String text, final int selStart, final int selEnd) {
        runOnUiThread(new Runnable() {
            @Override
            public void run() {
                if (mBelluiBuffer == null) {
                    return;
                }
                // An armed editor change (#966) makes this push the NEW field's
                // seed: skip every filter below (their state was reset with the
                // previous field's session) and reseed + restartInput even when
                // text and selection look unchanged — the IME session itself
                // must end so nothing composed in the previous field can ever
                // be recomposed into this one.
                boolean editorChanged = mEditorChanged;
                mEditorChanged = false;
                if (editorChanged) {
                    // doEditorChanged already cleared the composing mirror, but
                    // the OLD session's IME can interleave a setComposingText on
                    // the UI thread between that runnable and this one — clear
                    // again so the flag can never describe the previous field
                    // past its restart.
                    composing = false;
                    composingText = "";
                }
                String want = text != null ? text : "";
                if (!editorChanged && composing) {
                    // During an active bellui preedit the buffer holds the
                    // preedit but bellui's document does not: the echo of each
                    // composing keystroke arrives as a document equal to the
                    // buffer MINUS the composing span. Skip those echoes (the
                    // old blanket skip), but let a genuinely different
                    // document through — an out-of-band change like undo/redo
                    // from the accessibility actions menu (#1034). Falling
                    // through converges: the composing mirror is cleared and
                    // the 3-way reseeds + restartInput, which makes the IME
                    // abandon its composition and re-read.
                    int cs = BaseInputConnection.getComposingSpanStart(mBelluiBuffer);
                    int ce = BaseInputConnection.getComposingSpanEnd(mBelluiBuffer);
                    String bufText = mBelluiBuffer.toString();
                    String sansPreedit = bufText;
                    if (cs >= 0 && ce >= 0 && cs <= ce && ce <= bufText.length()) {
                        sansPreedit = bufText.substring(0, cs) + bufText.substring(ce);
                    }
                    if (want.equals(sansPreedit) || want.equals(bufText)) {
                        return; // composing echo — never disturb the session
                    }
                    composing = false;
                    composingText = "";
                }
                // Selection arrives as rune (code-point) offsets; the buffer is
                // UTF-16.
                int s16 = a11yUtf16FromRune(want, Math.max(0, selStart));
                int e16 = a11yUtf16FromRune(want, Math.max(0, selEnd));
                if (s16 > e16) { int t = s16; s16 = e16; e16 = t; }
                boolean sameText = mBelluiBuffer.toString().equals(want);
                boolean sameSel = Selection.getSelectionStart(mBelluiBuffer) == s16
                        && Selection.getSelectionEnd(mBelluiBuffer) == e16;

                // Case 1: bellui is lagging — this push is a document the buffer has
                // already moved past. Never clobber or restart on it.
                if (!editorChanged && !sameText && isRecentBufferState(want)) {
                    return;
                }

                if (!editorChanged && sameText) {
                    if (sameSel) {
                        return; // strict no-op — no spurious IMM notification
                    }
                    // Selection-only move: report it WITHOUT restarting input, so
                    // the IME keeps its session (and any composing region).
                    Selection.setSelection(mBelluiBuffer, s16, e16);
                    InputMethodManager immSel = (InputMethodManager)
                            getSystemService(Context.INPUT_METHOD_SERVICE);
                    if (immSel != null) {
                        immSel.updateSelection(mTextEdit, s16, e16,
                                BaseInputConnection.getComposingSpanStart(mBelluiBuffer),
                                BaseInputConnection.getComposingSpanEnd(mBelluiBuffer));
                    }
                    return;
                }

                // Case 3: a genuinely different document — SetText, undo, a
                // TalkBack edit, or a focus switch to another field. Re-seed.
                mBelluiBuffer.replace(0, mBelluiBuffer.length(), want);
                // Drop any stale composing span before repositioning the caret so
                // the IME resyncs against committed text with no preedit (#966).
                BaseInputConnection.removeComposingSpans(mBelluiBuffer);
                Selection.setSelection(mBelluiBuffer, s16, e16);

                noteBufferState();
                InputMethodManager imm = (InputMethodManager)
                        getSystemService(Context.INPUT_METHOD_SERVICE);
                if (imm != null) {
                    imm.restartInput(mTextEdit);
                }
            }
        });
    }

    static void showFileOpen(String mimes) {
        goNativeActivity.doShowFileOpen(mimes);
    }

    void doShowFileOpen(String mimes) {
        Intent intent = new Intent(Intent.ACTION_OPEN_DOCUMENT);
        if ("application/x-directory".equals(mimes) && Build.VERSION.SDK_INT >= Build.VERSION_CODES.LOLLIPOP) {
            intent = new Intent(Intent.ACTION_OPEN_DOCUMENT_TREE); // ask for a directory picker if OS supports it
            intent.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION);
        } else if (mimes.contains("|") && Build.VERSION.SDK_INT >= Build.VERSION_CODES.KITKAT) {
            intent.setType("*/*");
            intent.putExtra(Intent.EXTRA_MIME_TYPES, mimes.split("\\|"));
            intent.addCategory(Intent.CATEGORY_OPENABLE);
        } else {
            intent.setType(mimes);
            intent.addCategory(Intent.CATEGORY_OPENABLE);
        }
        startActivityForResult(Intent.createChooser(intent, "Open File"), FILE_OPEN_CODE);
    }

    static void showFileSave(String mimes, String filename) {
        goNativeActivity.doShowFileSave(mimes, filename);
    }

    void doShowFileSave(String mimes, String filename) {
        Intent intent = new Intent(Intent.ACTION_CREATE_DOCUMENT);
        if (mimes.contains("|") && Build.VERSION.SDK_INT >= Build.VERSION_CODES.KITKAT) {
            intent.setType("*/*");
            intent.putExtra(Intent.EXTRA_MIME_TYPES, mimes.split("\\|"));
        } else {
            intent.setType(mimes);
        }
        intent.putExtra(Intent.EXTRA_TITLE, filename);
        intent.addCategory(Intent.CATEGORY_OPENABLE);
        startActivityForResult(Intent.createChooser(intent, "Save File"), FILE_SAVE_CODE);
    }

    // doShareFile hands a local file to the Android share sheet (ACTION_SEND) as
    // a content:// URI vended by BelluiFileProvider, granting the receiving app
    // temporary read access (fyne#711). Called from the app JNI (app package) via
    // the ctx object's class; Go has already copied the file into the provider's
    // share directory (cacheDir/belluishare), so path is that copy.
    void doShareFile(String path, String mime) {
        try {
            java.io.File file = new java.io.File(path);
            String authority = getPackageName() + ".fileprovider";
            Uri uri = BelluiFileProvider.uriForFile(authority, file);

            Intent intent = new Intent(Intent.ACTION_SEND);
            intent.setType(mime == null || mime.isEmpty() ? "application/octet-stream" : mime);
            intent.putExtra(Intent.EXTRA_STREAM, uri);
            intent.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION);
            startActivity(Intent.createChooser(intent, null));
        } catch (Exception e) {
            Log.e("Bellui", "shareFile failed", e);
        }
    }

    // doSetClipboardImage places the PNG at path on the system clipboard as an
    // image/png content:// URI, vended through BelluiFileProvider (fyne#580). The
    // ClipDescription MIME is set explicitly so pasting apps recognise it as an
    // image (the provider's getType is generic).
    void doSetClipboardImage(String path) {
        try {
            java.io.File file = new java.io.File(path);
            String authority = getPackageName() + ".fileprovider";
            Uri uri = BelluiFileProvider.uriForFile(authority, file);

            android.content.ClipData clip = new android.content.ClipData(
                new android.content.ClipDescription("image", new String[]{"image/png"}),
                new android.content.ClipData.Item(uri));
            android.content.ClipboardManager cm =
                (android.content.ClipboardManager) getSystemService(android.content.Context.CLIPBOARD_SERVICE);
            cm.setPrimaryClip(clip);
        } catch (Exception e) {
            Log.e("Bellui", "setClipboardImage failed", e);
        }
    }

    // doSetClipboardFlavors places every present flavor on the system clipboard
    // in one transaction (fyne#1142): a single ClipData.Item natively carries
    // text + HTML + an image content:// URI (vended through BelluiFileProvider
    // like doSetClipboardImage), and the ClipDescription lists each present
    // MIME. Null arguments mark a flavor absent; an all-absent call clears.
    void doSetClipboardFlavors(String text, String html, String imagePath) {
        try {
            android.content.ClipboardManager cm =
                (android.content.ClipboardManager) getSystemService(android.content.Context.CLIPBOARD_SERVICE);
            if (cm == null) {
                return;
            }
            if (text == null && html == null && imagePath == null) {
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
                    cm.clearPrimaryClip();
                } else {
                    cm.setPrimaryClip(android.content.ClipData.newPlainText("", ""));
                }
                return;
            }

            java.util.ArrayList<String> mimes = new java.util.ArrayList<String>();
            if (text != null) {
                mimes.add(android.content.ClipDescription.MIMETYPE_TEXT_PLAIN);
            }
            if (html != null) {
                mimes.add(android.content.ClipDescription.MIMETYPE_TEXT_HTML);
            }
            Uri uri = null;
            if (imagePath != null) {
                java.io.File file = new java.io.File(imagePath);
                String authority = getPackageName() + ".fileprovider";
                uri = BelluiFileProvider.uriForFile(authority, file);
                mimes.add("image/png");
            }

            // An HTML item structurally requires the text slot; an empty string
            // stands in when no text flavor was set.
            android.content.ClipData.Item item = new android.content.ClipData.Item(
                text != null ? text : "", html, null, uri);
            android.content.ClipData clip = new android.content.ClipData(
                new android.content.ClipDescription("bellui", mimes.toArray(new String[0])),
                item);
            cm.setPrimaryClip(clip);
        } catch (Exception e) {
            Log.e("Bellui", "setClipboardFlavors failed", e);
        }
    }

    // doGetClipboardImage returns the path to a PNG temp file re-encoded from the
    // clipboard's image URI, or null when the clipboard holds no image (fyne#580).
    // The image MIME is checked against the ClipDescription (not the provider's
    // getType) so it works for both our own and other apps' image clips.
    String doGetClipboardImage() {
        try {
            android.content.ClipboardManager cm =
                (android.content.ClipboardManager) getSystemService(android.content.Context.CLIPBOARD_SERVICE);
            if (cm == null) {
                return null;
            }
            android.content.ClipData clip = cm.getPrimaryClip();
            if (clip == null || clip.getItemCount() == 0) {
                return null;
            }
            if (clip.getDescription() == null || !clip.getDescription().hasMimeType("image/*")) {
                return null;
            }
            Uri uri = clip.getItemAt(0).getUri();
            if (uri == null) {
                return null;
            }
            java.io.InputStream in = getContentResolver().openInputStream(uri);
            if (in == null) {
                return null;
            }
            android.graphics.Bitmap bmp = android.graphics.BitmapFactory.decodeStream(in);
            in.close();
            if (bmp == null) {
                return null;
            }
            java.io.File out = new java.io.File(getCacheDir(), "bellui-clip-in.png");
            java.io.FileOutputStream fos = new java.io.FileOutputStream(out);
            bmp.compress(android.graphics.Bitmap.CompressFormat.PNG, 100, fos);
            fos.close();
            return out.getAbsolutePath();
        } catch (Exception e) {
            Log.e("Bellui", "getClipboardImage failed", e);
            return null;
        }
    }

    // handleIntent inspects a launch or new intent for an "open with" payload —
    // a VIEW/EDIT data Uri (file association or custom URL scheme / deep link) —
    // and forwards it to Go via openIntent (fyne#622). Plain launcher (MAIN)
    // intents carry no data and are ignored.
    private void handleIntent(Intent intent) {
        if (intent == null) {
            return;
        }
        String action = intent.getAction();
        if (!Intent.ACTION_VIEW.equals(action) && !Intent.ACTION_EDIT.equals(action)) {
            return;
        }
        Uri data = intent.getData();
        if (data == null) {
            return;
        }
        openIntent(data.toString());
    }

    // handleNotifyIntent inspects a launch or new intent for a rich-notification
    // activation payload — the token + action index a bellui notification's
    // PendingIntent carries (fyne#723) — and forwards it to Go. A body tap uses
    // action -1; an action button uses its 0-based index. Non-notification
    // intents carry no bellui_notify_token extra and are ignored.
    private void handleNotifyIntent(Intent intent) {
        if (intent == null || !intent.hasExtra("bellui_notify_token")) {
            return;
        }
        long token = intent.getLongExtra("bellui_notify_token", 0);
        int action = intent.getIntExtra("bellui_notify_action", -1);
        notificationActivated(token, action);
    }
	static int getRune(int deviceId, int keyCode, int metaState) {
		try {
			int rune = KeyCharacterMap.load(deviceId).get(keyCode, metaState);
			if (rune == 0) {
				return -1;
			}
			return rune;
		} catch (KeyCharacterMap.UnavailableException e) {
			return -1;
		} catch (Exception e) {
			Log.e("Bellui", "exception reading KeyCharacterMap", e);
			return -1;
		}
	}

	private void load() {
		// Interestingly, NativeActivity uses a different method
		// to find native code to execute, avoiding
		// System.loadLibrary. The result is Java methods
		// implemented in C with JNIEXPORT (and JNI_OnLoad) are not
		// available unless an explicit call to System.loadLibrary
		// is done. So we do it here, borrowing the name of the
		// library from the same AndroidManifest.xml metadata used
		// by NativeActivity.
		try {
			ActivityInfo ai = getPackageManager().getActivityInfo(
					getIntent().getComponent(), PackageManager.GET_META_DATA);
			if (ai.metaData == null) {
				Log.e("Bellui", "loadLibrary: no manifest metadata found");
				return;
			}
			String libName = ai.metaData.getString("android.app.lib_name");
			System.loadLibrary(libName);
		} catch (Exception e) {
			Log.e("Bellui", "loadLibrary failed", e);
		}
	}

	@Override
	public void onCreate(Bundle savedInstanceState) {
		load();
		super.onCreate(savedInstanceState);
		setupEntry();
		updateTheme(getResources().getConfiguration());

		View view = findViewById(android.R.id.content).getRootView();
		view.addOnLayoutChangeListener(new View.OnLayoutChangeListener() {
			public void onLayoutChange (View v, int left, int top, int right, int bottom,
			                            int oldLeft, int oldTop, int oldRight, int oldBottom) {
				GoNativeActivity.this.updateLayout();
			}
		});

		// Deliver a launch "open with" payload (file association / deep link).
		// The native library is loaded above, so openIntent is safe to call; the
		// Go side buffers it until the app registers a handler (fyne#622).
		handleIntent(getIntent());
		// A launch from tapping a rich notification (fyne#723).
		handleNotifyIntent(getIntent());
    }

    @Override
    protected void onNewIntent(Intent intent) {
        super.onNewIntent(intent);
        // An already-running app was handed a new "open with" payload or a
        // rich-notification tap/action (fyne#622, fyne#723).
        setIntent(intent);
        handleIntent(intent);
        handleNotifyIntent(intent);
    }

    private void setupEntry() {
        runOnUiThread(new Runnable() {
            @Override
            public void run() {
                // The bellui-owned flat buffer the IME edits through (#1006
                // Phase 4). bellui drives its content/selection; it is never read
                // back out as document truth.
                mBelluiBuffer = new SpannableStringBuilder();
                Selection.setSelection(mBelluiBuffer, 0);
                // Dirty-region tracking for partial extracted-text updates
                // (#1009): a full-range INCLUSIVE_INCLUSIVE TextWatcher span
                // fires for every text mutation of the buffer, whatever path
                // made it — the same mechanism TextView's ChangeWatcher uses.
                mBelluiBuffer.setSpan(new TextWatcher() {
                    @Override
                    public void beforeTextChanged(CharSequence s, int start, int count, int after) {
                    }

                    @Override
                    public void onTextChanged(CharSequence s, int start, int before, int count) {
                        noteExtractDirty(start, before, count);
                    }

                    @Override
                    public void afterTextChanged(Editable s) {
                    }
                }, 0, mBelluiBuffer.length(), Spanned.SPAN_INCLUSIVE_INCLUSIVE);
                mTextEdit = new EditText(goNativeActivity) {
                    @Override
                    public boolean onKeyPreIme(int keyCode, KeyEvent event) {
                        // Intercept hardware-keyboard shortcut combinations
                        // (Ctrl/Alt/Meta + key) BEFORE the IME sees them and route
                        // them straight to bellui's shortcut path (#1077). This is the
                        // pre-IME hook — the point Flutter routes hardware keys
                        // through its own framework rather than letting the IME claim
                        // them. Gboard, handed a Ctrl+Z, runs its OWN undo through
                        // the InputConnection (a bouncing dance of setSelection +
                        // empty commitText "ghost" ops, and its transaction log grows
                        // while the field sits idle) instead of forwarding the key —
                        // so bellui's undo never fired cleanly and a single undo took
                        // many caret-bouncing presses (#1098). Consuming the combo
                        // here keeps Gboard from ever seeing it, so each Ctrl+Z is
                        // one clean ShortcutUndo. getUnicodeChar()==0 is the shortcut
                        // signature (Ctrl+Z/A/C/V/X, …); a printable combo such as
                        // AltGr+e (é) carries a non-zero char and must still reach
                        // the IME, so it is left alone. Bare modifier keys pass
                        // through; both directions of a real shortcut are consumed so
                        // no stray key-up leaks to the IME.
                        int meta = event.getMetaState();
                        if ((meta & (KeyEvent.META_CTRL_ON | KeyEvent.META_ALT_ON | KeyEvent.META_META_ON)) != 0
                                && !KeyEvent.isModifierKey(keyCode)
                                && event.getUnicodeChar() == 0) {
                            if (event.getAction() == KeyEvent.ACTION_DOWN) {
                                keyboardShortcut(keyCode, meta);
                            }
                            return true;
                        }
                        return super.onKeyPreIme(keyCode, event);
                    }
                    @Override
                    public InputConnection onCreateInputConnection(EditorInfo outAttrs) {
                        // Let EditText fill outAttrs (inputType / imeOptions / IME
                        // action) so the soft keyboard is the right type, but
                        // discard its EditableInputConnection: the IME must edit
                        // the bellui-owned buffer, not this View's TextView document
                        // (#1006 Phase 4). Seed the initial selection from the
                        // buffer so the IME opens on bellui's caret.
                        super.onCreateInputConnection(outAttrs);
                        int s = Math.max(0, Selection.getSelectionStart(mBelluiBuffer));
                        int e = Math.max(0, Selection.getSelectionEnd(mBelluiBuffer));
                        outAttrs.initialSelStart = Math.min(s, e);
                        outAttrs.initialSelEnd = Math.max(s, e);
                        // Seed the surrounding-text context the IME reads at
                        // connect (Compose does this via
                        // EditorInfoCompat.setInitialSurroundingText). Gboard uses
                        // it as a compose-mode trust signal (#1007). API 30+.
                        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
                            outAttrs.setInitialSurroundingText(mBelluiBuffer);
                        }
                        return new BelluiInputConnection();
                    }
                };
                mTextEdit.setVisibility(View.GONE);
                mTextEdit.setInputType(DEFAULT_INPUT_TYPE);
                // The shadow EditText is the IME input sink only; the proxy-View
                // overlay is the sole accessibility surface. Left accessibility-
                // important, this visible+input-focused EditText makes TalkBack
                // couple accessibility focus to input focus — a11y focus snaps
                // back to the focused field's proxy after the user explores
                // another one (re-announcing it), so a double-tap never lands a
                // stable ACTION_CLICK on the newly explored field and the caret
                // cannot move (fyne#945 device pass). Hide it from a11y.
                mTextEdit.setImportantForAccessibility(View.IMPORTANT_FOR_ACCESSIBILITY_NO);
                // Importance gates the TREE, not events: the shadow still
                // emits its own per-keystroke TYPE_VIEW_TEXT_CHANGED /
                // selection events (the visibility gate is isShown()), which
                // would race the proxy node's synthesized ones — the editable
                // text protocol (#974 option D) makes the proxy the single
                // event source, so swallow the shadow's entirely.
                mTextEdit.setAccessibilityDelegate(new View.AccessibilityDelegate() {
                    @Override
                    public void sendAccessibilityEvent(View host, int eventType) {
                        // swallowed: the proxy node is the event source
                    }
                    @Override
                    public void sendAccessibilityEventUnchecked(View host, AccessibilityEvent event) {
                        // swallowed: the proxy node is the event source
                    }
                });

                FrameLayout.LayoutParams mEditTextLayoutParams = new FrameLayout.LayoutParams(
                    FrameLayout.LayoutParams.WRAP_CONTENT, FrameLayout.LayoutParams.WRAP_CONTENT);
                mTextEdit.setLayoutParams(mEditTextLayoutParams);
                addContentView(mTextEdit, mEditTextLayoutParams);

                // The shadow's TextWatcher raw-delta echo (afterTextChanged ->
                // keyboardTyped/keyboardDelete, the #966 echo path) was removed
                // in #1006 Phase 3. The shadow no longer holds or reports the
                // bellui document: TalkBack editing flows through the option-D
                // proxy-node actions, and soft-keyboard typing is knowingly
                // stubbed until #1006 Phase 4 rebuilds a bellui-driven flat IME
                // transport that owns the composing region.
            }
        });
	}

	// composeCursor converts InputConnection.setComposingText's newCursorPosition
	// (which is RELATIVE: >0 is measured from the end of the composing text, <=0
	// from the start) into an absolute caret offset — in code points, matching
	// Bellui's rune-based cursor — within the composing text. TalkBack/Gboard send
	// the whole growing word as one composition with newCursorPosition=1 every
	// time; passing that 1 straight through pinned the caret at column 1 as the
	// preedit grew (the fyne#572 stuck-caret bug). newCursorPosition=1 means "just
	// after the composing text", i.e. its end.
	private static int composeCursor(String text, int newCursorPosition) {
	    int len = text.codePointCount(0, text.length());
	    int cursor = newCursorPosition > 0 ? len + newCursorPosition - 1 : newCursorPosition;
	    if (cursor < 0) cursor = 0;
	    if (cursor > len) cursor = len;
	    return cursor;
	}

	// BelluiInputConnection is the bellui-driven flat IME transport (#1006 Phase 4),
	// modelled on Flutter's InputConnectionAdaptor. It is a BaseInputConnection
	// over the bellui-owned mBelluiBuffer (getEditable) rather than the host EditText's
	// TextView document, so:
	//   * the IME's context queries (getTextBeforeCursor / getExtractedText /
	//     getCursorCapsMode) and its composing-span / selection bookkeeping run
	//     against a buffer whose content and caret bellui drives (via
	//     setKeyboardState); and
	//   * every edit the IME makes is forwarded to bellui as a rune-offset op
	//     (keyboardReplace / keyboardComposing / keyboardComposeCommit /
	//     keyboardDelete) applied at the bellui caret through internal/textinput,
	//     then super mutates the buffer so its state stays coherent for the next
	//     IME query.
	// bellui owns the document and the composing region; nothing is ever read back
	// out of a native view as truth. The flat buffer serves Entry and RichEntry
	// identically (RichEntry projects a flat rune-offset view via AccessibleTexter).
	private final class BelluiInputConnection extends BaseInputConnection {
	    // Batch-edit nesting depth: selection is reported to the IME only when it
	    // returns to 0, so a multi-op batch (e.g. autocorrect's delete+commit)
	    // yields one settled updateSelection, not a mid-batch flicker.
	    private int batchDepth = 0;
	    // Batch-edit coalescing (#1080, the correct redo of the reverted #1015).
	    // A TalkBack/IME batch that replaces text (Select-all then type → delete-all
	    // + commit inside one beginBatchEdit/endBatchEdit pair; autocorrect
	    // delete+commit; a suggestion-chip replaceText) must reach bellui as ONE
	    // gated, undo-recorded edit — one keyboardReplace, one undo step — not two
	    // steps with an empty document between them. batchBaseText snapshots the
	    // committed buffer at the outermost beginBatchEdit; while it is non-null
	    // (deferCommitted()) the per-op committed forwards are SKIPPED (super.*
	    // still mutates mBelluiBuffer), and endBatchEdit emits a single surrogate-safe
	    // diff of base→final. This is why the #1067 attempt doubled ordinary typing:
	    // Gboard wraps even a single-char commit in a batch, so forwarding per-op
	    // AND at the batch boundary fired twice — deferring the per-op forward is
	    // the fix. Composition is excluded: a fresh preedit mid-batch flushes the
	    // pending committed diff first (the iOS GoInputView model), then forwards
	    // compose events immediately (batchBaseText is cleared by the flush).
	    private String batchBaseText = null;
	    // Re-entrancy guard (#1080): BaseInputConnection.commitText/replaceText/
	    // deleteSurroundingText/setComposingText etc. internally wrap their Editable
	    // mutation in beginBatchEdit()/endBatchEdit(). Those dispatch virtually to
	    // OUR overrides, so super.* would trigger a spurious snapshot+flush and
	    // double-forward every edit (the #1067 regression). superEdit is raised
	    // around every super.* edit call; while it is non-zero the batch overrides
	    // only track depth and skip the snapshot/flush — coalescing keys off the
	    // IME's OWN outer beginBatchEdit/endBatchEdit only.
	    private int superEdit = 0;
	    // Extracted-text monitoring: when the IME requests getExtractedText with
	    // the MONITOR flag we remember its token and push updateExtractedText after
	    // every settled edit. Providing this is the trust signal that makes Gboard
	    // run predictive text — composing the current word (so a suggestion-strip
	    // pick REPLACES it) instead of committing char-by-char (where a chip pick
	    // could only insert). A real EditText's TextView does exactly this; our
	    // getEditable() is the bellui-owned buffer, not the View's text, so we drive
	    // the monitor ourselves.
	    private boolean monitorExtract = false;
	    private int extractToken = 0;
	    // The IME's own declared response cap (ExtractedTextRequest.hintMaxChars,
	    // 0 = unlimited), remembered from the monitor request so oversized
	    // monitor pushes degrade to a selection window instead of shipping a
	    // whole large document (#1009).
	    private int extractHint = 0;

	    BelluiInputConnection() {
	        super(mTextEdit, true);
	    }

	    @Override
	    public Editable getEditable() {
	        return mBelluiBuffer;
	    }

	    // deferCommitted reports whether committed-text forwarding should be held
	    // back and coalesced into the single endBatchEdit diff (#1080). True only
	    // while an outermost batch is open AND no composition has broken the
	    // accumulation (see flushBatchDiff / setComposingText).
	    private boolean deferCommitted() {
	        return batchBaseText != null;
	    }

	    // flushBatchDiff forwards the net change the batched super.* calls made to
	    // mBelluiBuffer since batchBaseText was snapshotted, as ONE keyboardReplace,
	    // then stops deferring. Called at the outermost endBatchEdit and before any
	    // composition/immediate op that must not be swallowed into the batch. The
	    // range is expressed against batchBaseText because bellui's committed document
	    // still equals it — every committed forward in the batch was deferred — so a
	    // single replace of base→final brings bellui to the final text in one undo
	    // step. Common prefix/suffix is computed in UTF-16 units with the same
	    // surrogate-pair guards the per-op paths use, so an astral edit is not split.
	    private void flushBatchDiff() {
	        String before = batchBaseText;
	        batchBaseText = null; // stop deferring regardless of the outcome below
	        if (before == null || mBelluiBuffer == null) {
	            return;
	        }
	        String after = mBelluiBuffer.toString();
	        if (before.equals(after)) {
	            return; // batch touched selection/composition only, no committed edit
	        }
	        int bLen = before.length(), aLen = after.length();
	        int p = 0, min = Math.min(bLen, aLen);
	        while (p < min && before.charAt(p) == after.charAt(p)) {
	            p++;
	        }
	        // Never split a surrogate pair at the prefix boundary.
	        if (p > 0 && p < bLen && p < aLen
	                && Character.isHighSurrogate(before.charAt(p - 1))
	                && (Character.isLowSurrogate(before.charAt(p))
	                        || Character.isLowSurrogate(after.charAt(p)))) {
	            p--;
	        }
	        int be = bLen, ae = aLen;
	        while (be > p && ae > p && before.charAt(be - 1) == after.charAt(ae - 1)) {
	            be--;
	            ae--;
	        }
	        // ...and not at the suffix boundary either.
	        if (be > p && be < bLen
	                && Character.isLowSurrogate(before.charAt(be))
	                && Character.isHighSurrogate(before.charAt(be - 1))) {
	            be++;
	            ae++;
	        }
	        keyboardReplace(a11yRuneFromUtf16(before, p), a11yRuneFromUtf16(before, be),
	                after.substring(p, ae));
	    }

	    // forwardReplace forwards a committed replace (rune offsets) to bellui,
	    // dropping a genuine no-op — an empty range AND empty text, which deletes
	    // nothing and inserts nothing. Gboard's hardware-Ctrl+Z undo does not reach
	    // the #1077 keyboardShortcut path: Gboard INTERCEPTS Ctrl+Z and drives its
	    // OWN undo through the InputConnection as setComposingRegion(caret,caret) +
	    // an empty commitText, which lands in commitText's region-recompose branch
	    // as keyboardReplace(caret,caret,"") once per press. Forwarding those
	    // recorded ghost undo steps in bellui (and bounced the caret) before the real
	    // deletions began (#1098). A real edit — non-empty range or non-empty text —
	    // always forwards, so nothing legitimate is dropped.
	    private void forwardReplace(int start, int end, String text) {
	        if (start == end && (text == null || text.isEmpty())) {
	            return;
	        }
	        keyboardReplace(start, end, text == null ? "" : text);
	    }

	    // afterEdit reports the buffer's post-edit selection + composing span to the
	    // IME (deferred until the outermost batch closes). Without this the IME's
	    // own editing-state cache drifts — it never receives updateSelection because
	    // our getEditable() is not the host EditText's TextView text, so the View's
	    // updateSelection machinery never runs — and a suggestion-strip pick then
	    // no-ops (it can't locate the word to replace). Flutter/Compose report the
	    // full editing state after every edit for the same reason.
	    private void afterEdit() {
	        // Record the post-edit buffer content so bellui's echo of the PRE-edit
	        // document is recognised as lag, not as an external change (#1007).
	        noteBufferState();
	        if (batchDepth == 0) {
	            updateImeSelection();
	        }
	    }

	    private void updateImeSelection() {
	        if (mBelluiBuffer == null) {
	            return;
	        }
	        InputMethodManager imm = (InputMethodManager)
	                getSystemService(Context.INPUT_METHOD_SERVICE);
	        if (imm == null) {
	            return;
	        }
	        int selS = Selection.getSelectionStart(mBelluiBuffer);
	        int selE = Selection.getSelectionEnd(mBelluiBuffer);
	        int compS = getComposingSpanStart(mBelluiBuffer);
	        int compE = getComposingSpanEnd(mBelluiBuffer);
	        imm.updateSelection(mTextEdit, selS, selE, compS, compE);
	        // Push the extracted-text delta only when the TEXT changed (#1009):
	        // a caret-only move already travelled in updateSelection above, and
	        // TextView's editor likewise reports extracts only on content change.
	        if (monitorExtract && extractDirtyStart >= 0) {
	            imm.updateExtractedText(mTextEdit, extractToken,
	                    buildExtractedText(extractHint, true));
	        }
	    }


	    // buildExtractedText serves the IME's editing-state cache (#1009).
	    // allowPartial (monitor pushes) reports only the dirty region as a
	    // partial update — partialStartOffset/partialEndOffset are the replaced
	    // range in the text the IME already holds, et.text the replacement — so
	    // a keystroke costs O(edit), not O(document), in both allocation and
	    // IPC (Entry/RichEntry back onto the textbuf rope and reach MBs; a
	    // whole-document push per key was a real per-key cost, and a binder
	    // transaction that size would fail outright). A direct request caps the
	    // response at the IME's own declared hintMaxChars with a window around
	    // the selection (et.startOffset carries the window origin, selection
	    // offsets are window-relative per TextView.setExtractedText's
	    // application of the struct); an oversized dirty region — a re-seed of
	    // a large document — degrades to that windowed form too rather than
	    // shipping the whole text. Serving any form resets the dirty region:
	    // the IME's cache is current again.
	    private ExtractedText buildExtractedText(int hintMaxChars, boolean allowPartial) {
	        ExtractedText et = new ExtractedText();
	        int len = mBelluiBuffer.length();
	        int selS = Math.max(0, Selection.getSelectionStart(mBelluiBuffer));
	        int selE = Math.max(0, Selection.getSelectionEnd(mBelluiBuffer));
	        et.flags = 0;
	        et.startOffset = 0;
	        et.selectionStart = selS;
	        et.selectionEnd = selE;
	        int dirtySize = extractDirtyEndNew - extractDirtyStart;
	        if (allowPartial && extractDirtyStart >= 0
	                && (hintMaxChars <= 0 || dirtySize <= hintMaxChars)) {
	            int s = Math.max(0, Math.min(extractDirtyStart, len));
	            int eNew = Math.max(s, Math.min(extractDirtyEndNew, len));
	            et.text = mBelluiBuffer.subSequence(s, eNew).toString();
	            et.partialStartOffset = s;
	            // The replaced range ends in the IME's OLD text: undo the net
	            // length change the dirty edits applied.
	            et.partialEndOffset = Math.max(s, eNew - extractDirtyDelta);
	        } else if (hintMaxChars > 0 && len > hintMaxChars) {
	            int selMin = Math.min(selS, selE), selMax = Math.max(selS, selE);
	            int wStart = selMin - Math.max(0, hintMaxChars - (selMax - selMin)) / 2;
	            wStart = Math.max(0, Math.min(wStart, len - hintMaxChars));
	            int wEnd = Math.min(len, wStart + hintMaxChars);
	            wStart = snapExtractBoundary(mBelluiBuffer, wStart);
	            wEnd = snapExtractBoundary(mBelluiBuffer, wEnd);
	            et.text = mBelluiBuffer.subSequence(wStart, wEnd).toString();
	            et.startOffset = wStart;
	            et.selectionStart = Math.max(0, Math.min(selS - wStart, wEnd - wStart));
	            et.selectionEnd = Math.max(0, Math.min(selE - wStart, wEnd - wStart));
	            et.partialStartOffset = -1;
	            et.partialEndOffset = -1;
	        } else {
	            et.text = mBelluiBuffer.toString();
	            et.partialStartOffset = -1; // whole-text update
	            et.partialEndOffset = -1;
	        }
	        clearExtractDirty();
	        return et;
	    }

	    @Override
	    public boolean setComposingText(CharSequence text, int newCursorPosition) {
	        if (composing) {
	            // An active bellui-side preedit: update it.
	            composingText = text.toString();
	            keyboardComposing(composingText, composeCursor(composingText, newCursorPosition));
	            superEdit++;
	            boolean r = super.setComposingText(text, newCursorPosition);
	            superEdit--;
	            afterEdit();
	            return r;
	        }
	        int cs = getComposingSpanStart(mBelluiBuffer);
	        int ce = getComposingSpanEnd(mBelluiBuffer);
	        if (cs >= 0 && ce >= 0) {
	            // Region recompose (see setComposingRegion): the marked text is
	            // COMMITTED in bellui, so the IME's replacement of it is a committed
	            // edit too — the explicit-range replaceText shape (#1007), the
	            // range being the span super is about to replace. bellui and the
	            // buffer stay in lockstep; no bellui preedit is opened, so composing
	            // stays false and state pushes keep flowing.
	            if (!deferCommitted()) {
	                String buf = mBelluiBuffer.toString();
	                int s16 = Math.min(Math.min(cs, ce), buf.length());
	                int e16 = Math.min(Math.max(cs, ce), buf.length());
	                keyboardReplace(a11yRuneFromUtf16(buf, s16),
	                        a11yRuneFromUtf16(buf, e16), text.toString());
	            }
	            superEdit++;
	            boolean r = super.setComposingText(text, newCursorPosition);
	            superEdit--;
	            afterEdit();
	            return r;
	        }
	        // Fresh preedit at the caret (CJK and other composing IMEs): bellui-side
	        // composition — display-only, outside the document until commit. A preedit
	        // opening mid-batch flushes the pending committed diff first (#1080) so the
	        // committed edits land as one step before composition starts forwarding
	        // immediately (the flush clears batchBaseText, ending deferral).
	        if (deferCommitted()) {
	            flushBatchDiff();
	        }
	        composing = true;
	        composingText = text.toString();
	        keyboardComposing(composingText, composeCursor(composingText, newCursorPosition));
	        superEdit++;
	        boolean r = super.setComposingText(text, newCursorPosition);
	        superEdit--;
	        afterEdit();
	        return r;
	    }

	    @Override
	    public boolean setComposingRegion(int start, int end) {
	        // The IME marks already-committed text (autocorrect recorrection,
	        // word-granular re-edit) as the composing region. Gboard does this to
	        // the word at the caret THE MOMENT a field is focused, so this path is
	        // hot on every latin session, not a CJK corner (#1021).
	        //
	        // Non-destructive, matching both the platform contract and Flutter's
	        // InputConnectionAdaptor (a pure super delegation): only the span
	        // markers move — the text stays committed, in the buffer AND in bellui.
	        // The IME's later edit of the region arrives via setComposingText/
	        // commitText, which forward it as a committed replace of the span
	        // range (above/below).
	        //
	        // This used to lift the region OUT of bellui's document
	        // (keyboardReplace(region, "") + keyboardComposing(region)) to reuse
	        // the bellui preedit machinery. That was doubly wrong (#1021, measured
	        // on-device): the splice was destructive — an abandoned or misaligned
	        // recompose permanently deleted document text — and it set
	        // composing=true for the whole session (even for an EMPTY region),
	        // which deadlocked the bellui→IME state push: composing → push skipped →
	        // no updateSelection → the IME never learns the caret moved (TalkBack
	        // granularity moves don't go through it) → it never finishes composing
	        // → composing never clears — so typing after a TalkBack caret move
	        // committed into the stale region, where the caret USED to be.
	        //
	        // A recompose arriving while a genuine bellui preedit is active would
	        // leave the buffer holding preedit text that bellui never committed
	        // (super only moves the markers); converge first by committing the
	        // preedit — the IME is re-marking, so it considers composition over.
	        if (composing) {
	            composing = false;
	            keyboardComposeCommit(composingText);
	            composingText = "";
	        }
	        superEdit++;
	        boolean r = super.setComposingRegion(start, end);
	        superEdit--;
	        afterEdit();
	        return r;
	    }

	    @Override
	    public boolean commitText(CharSequence text, int newCursorPosition) {
	        if (composing) {
	            // A composition finalised (candidate chosen / accepted): commit the
	            // preedit as the given text through bellui's compose-commit path.
	            composing = false;
	            composingText = "";
	            keyboardComposeCommit(text.toString());
	            superEdit++;
	            boolean rc = super.commitText(text, newCursorPosition);
	            superEdit--;
	            afterEdit();
	            return rc;
	        }
	        int cs = getComposingSpanStart(mBelluiBuffer);
	        int ce = getComposingSpanEnd(mBelluiBuffer);
	        if (cs >= 0 && ce >= 0) {
	            // Region recompose finalised (see setComposingRegion): super
	            // replaces the SPAN, not the selection, so forward exactly that —
	            // a committed replace of the span range.
	            if (!deferCommitted()) {
	                String bufC = mBelluiBuffer.toString();
	                int s16 = Math.min(Math.min(cs, ce), bufC.length());
	                int e16 = Math.min(Math.max(cs, ce), bufC.length());
	                forwardReplace(a11yRuneFromUtf16(bufC, s16),
	                        a11yRuneFromUtf16(bufC, e16), text.toString());
	            }
	            superEdit++;
	            boolean rc = super.commitText(text, newCursorPosition);
	            superEdit--;
	            afterEdit();
	            return rc;
	        }
	        // Plain commit — soft-keyboard tap, gesture typing, autocorrect
	        // acceptance. Replace the current selection (or insert at the caret when
	        // collapsed) as ONE gated, undo-recorded bellui edit, so typing over a
	        // selection replaces it in a single step (#748). Read the selection
	        // BEFORE super mutates the buffer.
	        if (!deferCommitted()) {
	            String buf = mBelluiBuffer.toString();
	            int s = Selection.getSelectionStart(mBelluiBuffer);
	            int e = Selection.getSelectionEnd(mBelluiBuffer);
	            if (s < 0) s = buf.length();
	            if (e < 0) e = buf.length();
	            int rs = a11yRuneFromUtf16(buf, Math.min(s, e));
	            int re = a11yRuneFromUtf16(buf, Math.max(s, e));
	            forwardReplace(rs, re, text.toString());
	        }
	        superEdit++;
	        boolean r = super.commitText(text, newCursorPosition);
	        superEdit--;
	        afterEdit();
	        return r;
	    }

	    // replaceText is how a modern IME replaces a word WITHOUT composing —
	    // Gboard's suggestion-chip pick and latin autocorrect travel through it
	    // (added in API 34). A latin/US Gboard never composes AS IT TYPES: measured
	    // on-device, it commits each character and shows no underline even in a
	    // stock EditText (the as-you-type underline is a CJK-mode thing) — though
	    // it DOES mark the committed word at the caret via setComposingRegion on
	    // every connect (#1021). So for latin the chip replace arrives HERE and
	    // nowhere else — this override is what makes the feature work at all.
	    //
	    // Without this override BaseInputConnection's default implementation edits
	    // getEditable() — our bellui-owned buffer — directly, and bellui is never told:
	    // the buffer gains the correction while bellui keeps the misspelling, they
	    // desync permanently, and every later push is either a spurious
	    // restartInput (killing the IME session) or a dropped echo (freezing the
	    // buffer's caret so taps are overridden). That was the real #1007 bug.
	    //
	    // Unlike commitText, the range is given EXPLICITLY by the IME (absolute
	    // UTF-16 offsets into the buffer), so map it straight to bellui's rune space
	    // and forward as one gated, undo-recorded edit; super then mutates the
	    // buffer so both stay in lockstep.
	    @Override
	    public boolean replaceText(int start, int end, CharSequence text,
	            int newCursorPosition, android.view.inputmethod.TextAttribute attr) {
	        // replaceText also ends any composition (super drops the composing
	        // span). Clear our mirror of that state too: a stale composing==true
	        // would make every later bellui push skip (doSetKeyboardState bails while
	        // composing) and wedge the buffer.
	        if (composing) {
	            composing = false;
	            composingText = "";
	        }
	        if (!deferCommitted()) {
	            String buf = mBelluiBuffer.toString();
	            int s16 = Math.max(0, Math.min(Math.min(start, end), buf.length()));
	            int e16 = Math.max(0, Math.min(Math.max(start, end), buf.length()));
	            forwardReplace(a11yRuneFromUtf16(buf, s16), a11yRuneFromUtf16(buf, e16),
	                    text == null ? "" : text.toString());
	        }
	        superEdit++;
	        boolean r = super.replaceText(start, end, text, newCursorPosition, attr);
	        superEdit--;
	        afterEdit();
	        return r;
	    }

	    @Override
	    public boolean finishComposingText() {
	        if (composing) {
	            // The composition was accepted as typed (no explicit commit).
	            composing = false;
	            keyboardComposeCommit(composingText);
	            composingText = "";
	        }
	        // A region-recompose span over committed text (composing == false)
	        // needs no bellui edit: super only removes the span markers, the text
	        // stays — and bellui already holds it.
	        superEdit++;
	        boolean r = super.finishComposingText();
	        superEdit--;
	        afterEdit();
	        return r;
	    }

	    @Override
	    public boolean deleteSurroundingText(int beforeLength, int afterLength) {
	        // Per the InputConnection contract this deletes beforeLength UTF-16 units
	        // BEFORE the selection and afterLength units AFTER it — the selected text
	        // itself is never deleted here (an IME replacing a selection commits over
	        // it instead), and BaseInputConnection.super does exactly that to the
	        // buffer: content.delete(a - beforeLength, a) and content.delete(b, b +
	        // afterLength). Deleting one span ACROSS the selection, as this used to,
	        // both diverged bellui from the buffer (super deleted only the two outer
	        // ranges) and turned a select-all + keypress into delete-all + insert —
	        // two undo steps with an empty document between them (#1007).
	        //
	        // Snap the boundaries so a surrogate pair is never split (the framework
	        // contract deletes the whole pair). The trailing range goes first so
	        // deleting it leaves the leading range's offsets unshifted. Both lengths
	        // non-zero is two bellui edits (two undo steps); IMEs send one side at a
	        // time in practice (backspace is before=1, after=0).
	        String buf = mBelluiBuffer.toString();
	        int s = Math.max(0, Selection.getSelectionStart(mBelluiBuffer));
	        int e = Math.max(0, Selection.getSelectionEnd(mBelluiBuffer));
	        int selMin = Math.min(s, e), selMax = Math.max(s, e);

	        if (afterLength > 0 && selMax < buf.length()) {
	            int end = Math.min(buf.length(), selMax + afterLength);
	            if (end > 0 && end < buf.length()
	                    && Character.isLowSurrogate(buf.charAt(end))
	                    && Character.isHighSurrogate(buf.charAt(end - 1))) {
	                end++;
	            }
	            if (selMax < end && !deferCommitted()) {
	                keyboardReplace(a11yRuneFromUtf16(buf, selMax),
	                        a11yRuneFromUtf16(buf, end), "");
	            }
	        }
	        if (beforeLength > 0 && selMin > 0) {
	            int start = Math.max(0, selMin - beforeLength);
	            if (start > 0 && start < buf.length()
	                    && Character.isLowSurrogate(buf.charAt(start))
	                    && Character.isHighSurrogate(buf.charAt(start - 1))) {
	                start--;
	            }
	            if (start < selMin && !deferCommitted()) {
	                keyboardReplace(a11yRuneFromUtf16(buf, start),
	                        a11yRuneFromUtf16(buf, selMin), "");
	            }
	        }
	        superEdit++;
	        boolean r = super.deleteSurroundingText(beforeLength, afterLength);
	        superEdit--;
	        afterEdit();
	        return r;
	    }

	    // The code-point-counted variant of deleteSurroundingText (API 24+). It is
	    // the SAME bug class as replaceText (#1007): BaseInputConnection's default
	    // implementation would edit getEditable() — the bellui-owned buffer — with
	    // nothing forwarded to bellui, desyncing the two permanently. Convert the
	    // code-point counts to UTF-16 units against the buffer and reuse the
	    // handled path above.
	    @Override
	    public boolean deleteSurroundingTextInCodePoints(int beforeLength, int afterLength) {
	        String buf = mBelluiBuffer.toString();
	        int s = Math.max(0, Selection.getSelectionStart(mBelluiBuffer));
	        int e = Math.max(0, Selection.getSelectionEnd(mBelluiBuffer));
	        int selMin = Math.min(s, e), selMax = Math.max(s, e);
	        int before16 = selMin;
	        if (beforeLength > 0) {
	            int cps = Math.min(beforeLength, buf.codePointCount(0, selMin));
	            before16 = selMin - buf.offsetByCodePoints(selMin, -cps);
	        } else {
	            before16 = 0;
	        }
	        int after16 = 0;
	        if (afterLength > 0) {
	            int cps = Math.min(afterLength, buf.codePointCount(selMax, buf.length()));
	            after16 = buf.offsetByCodePoints(selMax, cps) - selMax;
	        }
	        return deleteSurroundingText(before16, after16);
	    }

	    @Override
	    public boolean sendKeyEvent(KeyEvent event) {
	        // The IME's editing-panel arrow keys arrive here, not through the
	        // native input queue; delegating them to super would deliver them to
	        // the focused IME-host EditText, whose empty document swallows them —
	        // bellui's caret never moved (#1029). Route the DOWN into bellui's key
	        // path (the caret move returns via the state push) and consume both
	        // directions so the shadow never sees a stray UP.
	        int arrowCode = event.getKeyCode();
	        if (arrowCode == KeyEvent.KEYCODE_DPAD_UP || arrowCode == KeyEvent.KEYCODE_DPAD_DOWN
	                || arrowCode == KeyEvent.KEYCODE_DPAD_LEFT || arrowCode == KeyEvent.KEYCODE_DPAD_RIGHT) {
	            if (event.getAction() == KeyEvent.ACTION_DOWN) {
	                // Forward the meta-state so Shift+Arrow extends the selection
	                // (#1083); arrow keys never reach the native input queue.
	                keyboardArrow(arrowCode, event.getMetaState());
	            }
	            return true;
	        }
	        if (event.getAction() == KeyEvent.ACTION_DOWN) {
	            int code = event.getKeyCode();
	            // Hardware-keyboard shortcut (Ctrl/Alt/Meta + key, e.g. Ctrl+A/
	            // C/V/X): Gboard routes these here with getUnicodeChar()==0, not
	            // through the native input queue, so delegating to super would
	            // drop them on the inert host EditText. Forward the keycode +
	            // meta-state to bellui's shortcut path and consume it (#1077). Bare
	            // modifier keys, and Shift-only combos (plain uppercase, which
	            // arrive via commitText), are excluded.
	            int meta = event.getMetaState();
	            if ((meta & (KeyEvent.META_CTRL_ON | KeyEvent.META_ALT_ON | KeyEvent.META_META_ON)) != 0
	                    && !KeyEvent.isModifierKey(code)) {
	                // A shortcut does not mutate the shadow buffer, so it cannot ride
	                // the batch diff — land any pending committed edit first, then
	                // forward it (#1080).
	                if (deferCommitted()) {
	                    flushBatchDiff();
	                }
	                keyboardShortcut(code, meta);
	                return true;
	            }
	            if (code == KeyEvent.KEYCODE_DEL) {
	                // Soft/hardware backspace: delete the selection, else one code
	                // point before the caret, through bellui's delete path. Maintain
	                // the buffer here rather than via super: with fullEditor=true,
	                // BaseInputConnection.sendKeyEvent dispatches to the host
	                // EditText's own (unused) text, not our buffer, so the buffer
	                // would go stale until the next push. Keep it coherent for an
	                // immediate IME re-query.
	                // Backspace mutates mBelluiBuffer below, so inside a batch the diff
	                // captures the deletion — defer the forward, don't double it (#1080).
	                if (!deferCommitted()) {
	                    keyboardDelete();
	                }
	                int s = Math.max(0, Selection.getSelectionStart(mBelluiBuffer));
	                int e = Math.max(0, Selection.getSelectionEnd(mBelluiBuffer));
	                if (s != e) {
	                    mBelluiBuffer.delete(Math.min(s, e), Math.max(s, e));
	                } else if (s > 0) {
	                    int prev = mBelluiBuffer.toString().offsetByCodePoints(s, -1);
	                    mBelluiBuffer.delete(prev, s);
	                }
	                afterEdit();
	                return true;
	            }
	            if (code == KeyEvent.KEYCODE_ENTER || code == KeyEvent.KEYCODE_NUMPAD_ENTER) {
	                // Route Enter to bellui (a newline in a multiline field, or submit
	                // on a single-line one). The following push reconciles the buffer
	                // to bellui's resulting document. Enter does not mutate the shadow
	                // buffer, so flush any pending batch diff first (#1080).
	                if (deferCommitted()) {
	                    flushBatchDiff();
	                }
	                keyboardTyped("\n");
	                return true;
	            }
	            if (code == KeyEvent.KEYCODE_FORWARD_DEL) {
	                // Forward-delete routes super to the host EditText, not mBelluiBuffer,
	                // so it cannot ride the batch diff — flush any pending edit first
	                // and forward immediately (#1080).
	                if (deferCommitted()) {
	                    flushBatchDiff();
	                }
	                String buf = mBelluiBuffer.toString();
	                int e16 = Math.max(0, Math.min(Selection.getSelectionEnd(mBelluiBuffer), buf.length()));
	                if (e16 < buf.length()) {
	                    int next = buf.offsetByCodePoints(e16, 1);
	                    keyboardReplace(a11yRuneFromUtf16(buf, e16),
	                            a11yRuneFromUtf16(buf, next), "");
	                }
	                superEdit++;
	                boolean r = super.sendKeyEvent(event);
	                superEdit--;
	                return r;
	            }
	            // Non-composing printable keys — the spacebar and the digit keys in
	            // a CJK input mode (Gboard's candidate-selection keys), and any key
	            // the IME chooses to deliver as a keystroke rather than compose —
	            // arrive HERE via sendKeyEvent carrying a unicode char, NOT via
	            // commitText, and a real EditText would insert them. Dropping them to
	            // super (which edits the unused host-EditText document, not
	            // mBelluiBuffer) silently ate the spacebar and digits in Pinyin mode
	            // (#1101). Insert the char through bellui's typed-rune path (which
	            // honours NumericEntry filtering) and let the following state push
	            // reconcile the shadow buffer — the same model the Enter key above
	            // uses, so no hand-mutation of mBelluiBuffer (which risks an
	            // out-of-bounds when its selection trails bellui's document). Only the
	            // DOWN inserts: an ordinary latin key's DOWN comes via commitText and
	            // only its UP echoes here, so acting on the DOWN never doubles normal
	            // typing. Restricted to non-password text fields — password/number
	            // fields don't compose, so Gboard passes their hardware keys through
	            // the native queue (handled there), and inserting from sendKeyEvent
	            // as well could double.
	            int uch = event.getUnicodeChar();
	            int it = mTextEdit.getInputType();
	            int variation = it & InputType.TYPE_MASK_VARIATION;
	            boolean prose = (it & InputType.TYPE_MASK_CLASS) == InputType.TYPE_CLASS_TEXT
	                    && variation != InputType.TYPE_TEXT_VARIATION_PASSWORD
	                    && variation != InputType.TYPE_TEXT_VARIATION_VISIBLE_PASSWORD
	                    && variation != InputType.TYPE_TEXT_VARIATION_WEB_PASSWORD;
	            if (uch != 0 && prose
	                    && (event.getMetaState() & (KeyEvent.META_CTRL_ON | KeyEvent.META_ALT_ON | KeyEvent.META_META_ON)) == 0) {
	                if (deferCommitted()) {
	                    flushBatchDiff();
	                }
	                keyboardTyped(new String(Character.toChars(uch)));
	                return true;
	            }
	        }
	        superEdit++;
	        boolean r = super.sendKeyEvent(event);
	        superEdit--;
	        return r;
	    }

	    @Override
	    public boolean setSelection(int start, int end) {
	        // An active bellui-side preedit is display-only — outside bellui's
	        // committed document — so buffer offsets and bellui offsets disagree
	        // by the preedit length and the forwarded selection would come up
	        // short by exactly that many characters (#1032). Converge first by
	        // committing it: the IME moving the selection treats the composition
	        // as settled (the same rule setComposingRegion applies), and the
	        // ordered event queue lands the commit before the selection, so the
	        // offsets below resolve against the converged document.
	        if (composing) {
	            composing = false;
	            keyboardComposeCommit(composingText);
	            composingText = "";
	        }
	        // The IME moving the caret or selecting a range (Select All,
	        // spacebar cursor control). super keeps the buffer coherent; the
	        // selection is also forwarded to bellui (#1030) — bellui owns the
	        // caret, so a buffer-only selection is invisible (no highlight, no
	        // Copy/Cut on the a11y node) and the next state push collapses it.
	        // Not flushed mid-batch on purpose (#1080): the canonical Select-all-
	        // then-type batch issues this BEFORE the commit, so flushing here (a
	        // no-op that would still clear batchBaseText) would end deferral early
	        // and un-coalesce the following edit. The immediate forward is safe —
	        // endBatchEdit's coalesced keyboardReplace carries explicit offsets and
	        // resets the caret, superseding this selection.
	        String buf = mBelluiBuffer.toString();
	        int s16 = Math.max(0, Math.min(Math.min(start, end), buf.length()));
	        int e16 = Math.max(0, Math.min(Math.max(start, end), buf.length()));
	        keyboardSetSelection(a11yRuneFromUtf16(buf, s16), a11yRuneFromUtf16(buf, e16));
	        superEdit++;
	        boolean r = super.setSelection(start, end);
	        superEdit--;
	        // ACK the change (InputConnection contract): the editor must report
	        // updateSelection after EVERY selection change, including ones the
	        // IME itself made. Without it Gboard keeps composing against a
	        // stale region, and typing after Select All extends that word
	        // instead of committing over the selection (#1030).
	        afterEdit();
	        return r;
	    }

	    @Override
	    public boolean beginBatchEdit() {
	        // Snapshot the committed buffer at the IME's OUTERMOST batch so endBatchEdit
	        // can coalesce every committed edit inside it into one diff (#1080). Skip
	        // batches super opens internally (superEdit>0) and composition (the buffer
	        // then holds the display-only preedit, which forwards immediately anyway).
	        if (superEdit == 0 && batchDepth == 0 && !composing && mBelluiBuffer != null) {
	            batchBaseText = mBelluiBuffer.toString();
	        }
	        batchDepth++;
	        return super.beginBatchEdit();
	    }

	    @Override
	    public boolean endBatchEdit() {
	        boolean stillNested = super.endBatchEdit();
	        if (batchDepth > 0) {
	            batchDepth--;
	        }
	        // Only the IME's own outer endBatchEdit flushes; a super-internal batch
	        // (superEdit>0) just unwinds depth, leaving batchBaseText for the outer one.
	        if (batchDepth == 0 && superEdit == 0) {
	            // Forward the coalesced committed edit (one keyboardReplace → one undo
	            // step) BEFORE pushing the settled selection/extracted text.
	            flushBatchDiff();
	            updateImeSelection();
	        }
	        return stillNested;
	    }

	    // The IME requests extracted-text monitoring to run predictive text; serve
	    // the bellui-owned buffer and (in updateImeSelection) push updateExtractedText
	    // after each settled edit so its view stays live.
	    @Override
	    public ExtractedText getExtractedText(ExtractedTextRequest request, int flags) {
	        if (request != null && (flags & GET_EXTRACTED_TEXT_MONITOR) != 0) {
	            monitorExtract = true;
	            extractToken = request.token;
	            extractHint = request.hintMaxChars;
	        }
	        // A direct request re-baselines the IME's cache: serve full text —
	        // windowed at the IME's own hintMaxChars for large documents
	        // (#1009) — never a partial (the IME may hold nothing yet).
	        return buildExtractedText(request != null ? request.hintMaxChars : 0, false);
	    }
	}

	@Override
    protected void onActivityResult(int requestCode, int resultCode, Intent data) {
        // unhandled request
        if (requestCode != FILE_OPEN_CODE && requestCode != FILE_SAVE_CODE) {
            return;
        }

        // dialog was cancelled
        if (resultCode != Activity.RESULT_OK) {
            filePickerReturned("");
            return;
        }

        Uri uri = data.getData();
        filePickerReturned(uri.toString());
    }

    @Override
    public void onBackPressed() {
        if (goNativeActivity.keyboardUp) {
            hideKeyboard();
            return;
        }

        // skip the default behaviour - we can call finishActivity if we want to go back
        backPressed();
    }

    public void finishActivity() {
        runOnUiThread(new Runnable() {
            @Override
            public void run() {
                GoNativeActivity.super.onBackPressed();
            }
        });
    }

    @Override
    public void onConfigurationChanged(Configuration config) {
        super.onConfigurationChanged(config);
        updateTheme(config);
    }

    protected void updateTheme(Configuration config) {
        boolean dark = (config.uiMode & Configuration.UI_MODE_NIGHT_MASK) == Configuration.UI_MODE_NIGHT_YES;
        setDarkMode(dark);
    }

    // -------------------------------------------------------------------------
    // Accessibility bridge (#488) — virtual-node provider approach (#1104).
    //
    // Go stages one entry per semantic element and commits a frame; the commit
    // reconciles a map of per-node state records (A11yProxy) behind ONE
    // full-screen, touch-transparent host View (A11yHostView) whose
    // AccessibilityNodeProvider serves the tree as virtual nodes keyed by the
    // stable Go-side handles. TalkBack resolves any node id on demand straight
    // from the map, so resolution never depends on a real child View staying
    // attached — which is what broke it under the former per-View overlay:
    // the per-frame reconcile's removeView/addView churn invalidated the node
    // ids TalkBack had cached, AccessibilityInteractionClient returned null,
    // and its two-finger scroll never found a node to send
    // ACTION_SCROLL_FORWARD to (#1104). The host intercepts framework hover
    // events to drive touch exploration over the virtual nodes and handles
    // accessibility focus itself (the ExploreByTouchHelper model, as Flutter
    // and Compose do). Real touches still fall through to the GL surface, and
    // actions are sent back to Go via the accessibilityAction native method.
    //
    // Role and flag values mirror the wire enums in the Go driver
    // (internal/driver/mobile/accessibility.go) — keep them in sync.
    // -------------------------------------------------------------------------

    static final int A11Y_ROLE_BUTTON     = 1;
    static final int A11Y_ROLE_TEXT       = 2;
    static final int A11Y_ROLE_LINK       = 3;
    static final int A11Y_ROLE_IMAGE      = 4;
    static final int A11Y_ROLE_CHECKBOX   = 5;
    static final int A11Y_ROLE_SWITCH     = 6;
    static final int A11Y_ROLE_RADIO      = 7;
    static final int A11Y_ROLE_TAB        = 8;
    static final int A11Y_ROLE_TEXTFIELD  = 9;
    static final int A11Y_ROLE_TEXTAREA   = 10;
    static final int A11Y_ROLE_PASSWORD   = 11;
    static final int A11Y_ROLE_SEARCH     = 12;
    static final int A11Y_ROLE_SLIDER     = 13;
    static final int A11Y_ROLE_PROGRESS   = 14;
    static final int A11Y_ROLE_SPIN       = 15;
    static final int A11Y_ROLE_HEADING    = 16;
    static final int A11Y_ROLE_LISTITEM   = 17;
    static final int A11Y_ROLE_COMBO      = 18;
    static final int A11Y_ROLE_MENUITEM   = 19;
    static final int A11Y_ROLE_TREEITEM   = 20;
    static final int A11Y_ROLE_CELL       = 21;
    // Virtualized grid/table container (#817): the one container role the flat
    // overlay keeps. Its proxy is a ViewGroup hosting the visible cell proxies,
    // so TalkBack associates the cells' CollectionItemInfo with its
    // CollectionInfo (full logical extent) and ACTION_SCROLL_TO_POSITION can
    // bring any off-viewport cell into view.
    static final int A11Y_ROLE_GRID       = 22;

    static final int A11Y_FLAG_DISABLED      = 1;
    static final int A11Y_FLAG_SELECTED      = 2;
    static final int A11Y_FLAG_CHECKED       = 4;
    static final int A11Y_FLAG_CHECKABLE     = 8;
    static final int A11Y_FLAG_PRESSABLE     = 16;
    static final int A11Y_FLAG_ADJUSTABLE    = 32;
    static final int A11Y_FLAG_DISMISSABLE   = 64;
    // Keep bit positions in sync with internal/driver/mobile/accessibility.go.
    static final int A11Y_FLAG_FOCUSED       = 128;
    static final int A11Y_FLAG_SCROLLABLE    = 256;
    static final int A11Y_FLAG_LONGPRESSABLE = 512;
    static final int A11Y_FLAG_TEXT_EDITABLE = 1024;

    static final int A11Y_ACTION_ACTIVATE        = 1;
    static final int A11Y_ACTION_INCREMENT       = 2;
    static final int A11Y_ACTION_DECREMENT       = 3;
    static final int A11Y_ACTION_DISMISS         = 4;
    static final int A11Y_ACTION_STATE_CHANGED   = 5;
    static final int A11Y_ACTION_SCROLL_FORWARD  = 6;
    static final int A11Y_ACTION_SCROLL_BACKWARD = 7;
    static final int A11Y_ACTION_LONG_PRESS      = 8;

    // Custom accessibility actions (#1025): a declared widget action with no
    // standard Android semantic (undo/redo on an editor, ...) arrives with a
    // small Go-side registry id. It is registered on the node as an
    // AccessibilityAction with id A11Y_CUSTOM_ACTION_BASE + registry id (a
    // block clear of the framework action ids and of android/app resource
    // ids), and dispatched back to Go as action kind A11Y_ACTION_CUSTOM +
    // registry id. Keep A11Y_ACTION_CUSTOM in sync with
    // internal/driver/mobile/accessibility.go's mobileA11yActionCustom.
    static final int A11Y_CUSTOM_ACTION_BASE = 0x0ff00000;
    static final int A11Y_ACTION_CUSTOM      = 0x1000;

    // Editable-text action kinds sent through accessibilityTextAction (#974
    // option D). Offsets are RUNE indices into the node's document — this side
    // converts from UTF-16 via codePointCount/offsetByCodePoints — and every
    // edit lands on the widget's gated, undo-recorded paths in Go. Keep in
    // sync with internal/driver/mobile/accessibility.go.
    static final int A11Y_TEXTACT_SET_SELECTION = 1;
    static final int A11Y_TEXTACT_REPLACE       = 2;
    static final int A11Y_TEXTACT_SET_TEXT      = 3;
    static final int A11Y_TEXTACT_INSERT        = 4;

    private static A11yHostView mA11yContainer;

    // Node ids are the Go-side handles (always >= 1); 0 doubles as "no node"
    // and as the wire's synthetic root parent id.
    private static final int A11Y_NO_NODE = 0;
    // The virtual node holding the screen reader's accessibility focus, and
    // the one under the touch-exploration hover pointer. With a node provider
    // the framework tracks neither for us (it only does for real Views), so
    // ACTION_ACCESSIBILITY_FOCUS/CLEAR and hover mapping are handled here.
    // UI thread only.
    private static int mA11yFocusedId = A11Y_NO_NODE;
    private static int mA11yHoveredId = A11Y_NO_NODE;

    // Reading order of the virtual tree, rebuilt on every commit: parent node
    // id (A11Y_NO_NODE = the host root) -> child node ids in wire order. The
    // provider serves children from these lists. UI thread only.
    private static final java.util.HashMap<Integer, java.util.ArrayList<Integer>> mA11yChildOrder =
            new java.util.HashMap<Integer, java.util.ArrayList<Integer>>();

    // Staging buffer — written from Go threads (via JNI), consumed on the UI
    // thread. Each node is int[]{id, role, flags, x, y, w, h, parent, row,
    // col, rowCount, colCount, selStart(rune), selEnd(rune)} plus
    // String[]{label, value, hint, text, customActs, textLayout}.
    private static final java.util.List<int[]> sA11yData = new java.util.ArrayList<int[]>();
    private static final java.util.List<String[]> sA11yStrings = new java.util.ArrayList<String[]>();
    // Signature of the last committed layout; skip UI work when nothing changed.
    private static String sA11ySignature = null;

    // Persistent per-node state records, keyed by nodeId, reconciled in place
    // across commits (see reconcileA11yNodes) so TalkBack's cached node ids
    // stay resolvable and its accessibility focus survives value and position
    // changes.
    private static final java.util.HashMap<Integer, A11yProxy> mA11yProxies =
            new java.util.HashMap<Integer, A11yProxy>();

    // Mutable per-node state behind one virtual node. The host View's
    // AccessibilityNodeProvider reads these fields live, so a reconcile
    // updates the published node in place.
    private static final class A11yProxy {
        final int nodeId;
        int role, flags, x, y, w, h;
        // Virtualized-grid association (#817): the grid proxy this node nests
        // under (0 = the root overlay), the node's display cell coordinates
        // (-1 = not a grid item) and, for a grid proxy, the collection extent.
        int parentId, row = -1, col = -1, rowCount, colCount;
        String label = "", value = "", hint = "";
        // Editable-text document (#974 option D): the live text of a
        // text-input node backed by the Go text seam, with its selection in
        // UTF-16 units (-1/-1 = the node carries no document). The selection
        // is stored RAW — selStart is the anchor and selEnd the free cursor,
        // so it may be reversed after extend-backward granularity moves; node
        // publication normalizes, traversal keeps the true cursor end.
        // Granularity moves and edits mutate these optimistically (the action
        // must answer synchronously; the Go dispatch is queued) and the
        // per-frame reconcile converges them onto widget truth.
        String text = "";
        int selStart16 = -1, selEnd16 = -1;
        // Custom accessibility actions (#1025): the encoded wire string and
        // its parsed (Go registry id, localized label) pairs; null when the
        // node offers none. Parsed once per change in the reconcile.
        String customActs = "";
        int[] customIds;
        String[] customLabels;
        // Visual-layout sidecar (#995), present only on the focused editor:
        // the encoded wire string and its parsed forms. lineBounds16 holds
        // UTF-16 boundary offsets of consecutive visual (soft-wrapped) lines
        // around the caret — line i of the window is [b[i], b[i+1]) — for
        // LINE-granularity traversal on TalkBack versions that don't
        // self-compute soft-wrap lines. charLocStart16/charLocRects hold
        // per-UTF-16-char rectangles (container px, 4 floats per char,
        // NaN = no geometry) around the selection, served through the
        // EXTRA_DATA_TEXT_CHARACTER_LOCATION_KEY request. Parsed once per
        // change in the reconcile.
        String textLayout = "";
        int[] lineBounds16;
        int charLocStart16 = -1;
        float[] charLocRects;

        A11yProxy(int nodeId) {
            this.nodeId = nodeId;
        }
    }

    // Called from Go (via JNI) before each frame's tree is staged.
    static boolean isAccessibilityEnabled() {
        AccessibilityManager am =
                (AccessibilityManager) goNativeActivity.getSystemService(Context.ACCESSIBILITY_SERVICE);
        return am != null && am.isEnabled();
    }

    // Called once from Go (via JNI) when the window shows.
    static void setupAccessibility() {
        goNativeActivity.doSetupAccessibility();
    }

    void doSetupAccessibility() {
        runOnUiThread(new Runnable() {
            @Override
            public void run() {
                if (mA11yContainer != null) {
                    return; // already set up
                }
                // Full-screen transparent host of the virtual accessibility
                // tree. It has no child Views and intercepts no touch events;
                // its AccessibilityNodeProvider publishes the semantic
                // elements.
                mA11yContainer = new A11yHostView(goNativeActivity);
                mA11yContainer.setClickable(false);
                mA11yContainer.setFocusable(false);
                mA11yContainer.setImportantForAccessibility(
                        View.IMPORTANT_FOR_ACCESSIBILITY_YES);

                ViewGroup decorView = (ViewGroup) getWindow().getDecorView();
                decorView.addView(mA11yContainer, new FrameLayout.LayoutParams(
                        FrameLayout.LayoutParams.MATCH_PARENT,
                        FrameLayout.LayoutParams.MATCH_PARENT));

                // Rebuild a tree Go was skipping when the screen reader is
                // switched on mid-session.
                AccessibilityManager am = (AccessibilityManager)
                        getSystemService(Context.ACCESSIBILITY_SERVICE);
                if (am != null) {
                    am.addAccessibilityStateChangeListener(
                            new AccessibilityManager.AccessibilityStateChangeListener() {
                        @Override
                        public void onAccessibilityStateChanged(boolean enabled) {
                            // Notify Go on both transitions: on enable, rebuild a
                            // tree Go was skipping; on disable, clear the in-scene
                            // focus ring (#759) — no _FOCUS_CLEARED reaches the
                            // proxies when the service is torn down, so the ring
                            // would otherwise stay painted.
                            accessibilityAction(0, A11Y_ACTION_STATE_CHANGED);
                        }
                    });

                    // Bellui consumes touch input through the native AInputQueue,
                    // which bypasses the framework's touch-exploration filter, so
                    // under TalkBack a single explore tap still reaches Bellui and
                    // activates the widget (both single- and double-tap "click").
                    // Track touch-exploration state so the driver suppresses raw
                    // tap activation while it is on — activation then comes only
                    // via the double-tap -> ACTION_CLICK path (#572 device pass).
                    am.addTouchExplorationStateChangeListener(
                            new AccessibilityManager.TouchExplorationStateChangeListener() {
                        @Override
                        public void onTouchExplorationStateChanged(boolean enabled) {
                            setTouchExploration(enabled);
                        }
                    });
                    setTouchExploration(am.isTouchExplorationEnabled());
                }

                // If Go committed nodes before the UI thread ran, apply them now.
                applyA11ySnapshot();
            }
        });
    }

    // Re-sync the touch-exploration flag whenever the window (re)gains focus.
    // The one-shot query + change-listener in doSetupAccessibility misses the
    // common case where the app *starts* while TalkBack is already active: the
    // startup isTouchExplorationEnabled() can race the service and latch false,
    // and no state-change event follows because nothing changed. Without the
    // flag the driver never suppresses raw taps, so every explore-touch focuses
    // and activates the widget instead of TalkBack's swipe-to-focus /
    // double-tap-to-activate two-stage model (fyne#945 device pass). Window
    // focus is regained on resume, TalkBack (re)connect and dialog dismiss —
    // exactly where a stale flag needs correcting.
    @Override
    public void onWindowFocusChanged(boolean hasFocus) {
        super.onWindowFocusChanged(hasFocus);
        if (!hasFocus) {
            return;
        }
        AccessibilityManager am = (AccessibilityManager)
                getSystemService(Context.ACCESSIBILITY_SERVICE);
        if (am != null) {
            setTouchExploration(am.isTouchExplorationEnabled());
        }
    }

    // Called from Go (via JNI) before rebuilding the accessibility tree.
    static synchronized void clearAccessibilityNodes() {
        sA11yData.clear();
        sA11yStrings.clear();
    }

    // Called from Go (via JNI) to register one accessible element. Kept for
    // APKs whose native library predates the V2 signature; nodes arrive
    // without their virtualized-grid association.
    static synchronized void addAccessibilityNode(int id, int role, int flags,
            String label, String value, String hint,
            int x, int y, int width, int height) {
        addAccessibilityNodeV2(id, role, flags, label, value, hint,
                x, y, width, height, 0, -1, -1, 0, 0);
    }

    // Called from Go (via JNI) to register one accessible element with its
    // virtualized-grid association (#817): the parent grid node id, the cell's
    // display (row, col) and — for a grid container — the collection extent.
    // Kept for APKs whose native library predates the V3 signature; nodes
    // arrive without their editable-text document.
    static synchronized void addAccessibilityNodeV2(int id, int role, int flags,
            String label, String value, String hint,
            int x, int y, int width, int height,
            int parent, int row, int col, int rowCount, int colCount) {
        addAccessibilityNodeV3(id, role, flags, label, value, hint,
                x, y, width, height, parent, row, col, rowCount, colCount,
                "", -1, -1);
    }

    // Called from Go (via JNI) to register one accessible element with its
    // editable-text document (#974 option D): the live text of a text-input
    // node and its half-open selection as RUNE offsets (-1/-1 = no document).
    // Kept for APKs whose native library predates the V4 signature; nodes
    // arrive without their custom accessibility actions.
    static synchronized void addAccessibilityNodeV3(int id, int role, int flags,
            String label, String value, String hint,
            int x, int y, int width, int height,
            int parent, int row, int col, int rowCount, int colCount,
            String text, int selStart, int selEnd) {
        addAccessibilityNodeV4(id, role, flags, label, value, hint,
                x, y, width, height, parent, row, col, rowCount, colCount,
                text, selStart, selEnd, "");
    }

    // Called from Go (via JNI) to register one accessible element with its
    // custom accessibility actions (#1025): declared widget actions with no
    // standard Android semantic (undo/redo, ...), encoded as
    // id U+001F label entries separated by U+001E ("" = none).
    static synchronized void addAccessibilityNodeV4(int id, int role, int flags,
            String label, String value, String hint,
            int x, int y, int width, int height,
            int parent, int row, int col, int rowCount, int colCount,
            String text, int selStart, int selEnd, String customActs) {
        addAccessibilityNodeV5(id, role, flags, label, value, hint,
                x, y, width, height, parent, row, col, rowCount, colCount,
                text, selStart, selEnd, customActs, "");
    }

    // Called from Go (via JNI) to register one accessible element with the
    // focused editor's visual-layout sidecar (#995): a window of visual
    // (soft-wrapped) line boundaries around the caret plus per-character
    // rectangles around the selection ("" = none). Consumed for legacy-
    // TalkBack LINE granularity and the character-location extra behind the
    // granular highlight/magnifier.
    static synchronized void addAccessibilityNodeV5(int id, int role, int flags,
            String label, String value, String hint,
            int x, int y, int width, int height,
            int parent, int row, int col, int rowCount, int colCount,
            String text, int selStart, int selEnd, String customActs,
            String textLayout) {
        sA11yData.add(new int[]{id, role, flags, x, y, width, height,
                parent, row, col, rowCount, colCount, selStart, selEnd});
        sA11yStrings.add(new String[]{
                label != null ? label : "",
                value != null ? value : "",
                hint != null ? hint : "",
                text != null ? text : "",
                customActs != null ? customActs : "",
                textLayout != null ? textLayout : ""});
    }

    // Called from Go (via JNI) after all nodes for this frame have been added.
    static synchronized void commitAccessibilityNodes() {
        // Cheap signature to avoid redundant UI work when nothing changed.
        StringBuilder sig = new StringBuilder();
        for (int i = 0; i < sA11yData.size(); i++) {
            int[] d = sA11yData.get(i);
            for (int v : d) {
                sig.append(v).append(',');
            }
            String[] s = sA11yStrings.get(i);
            sig.append(s[0]).append('\u0001').append(s[1]).append('\u0001')
               .append(s[2]).append('\u0001').append(s[3]).append('\u0001')
               .append(s[4]).append('\u0001').append(s[5]).append('|');
        }
        String newSig = sig.toString();
        if (newSig.equals(sA11ySignature)) {
            return;
        }
        sA11ySignature = newSig;

        // Snapshot for the UI thread; the staging lists are mutated from Go
        // threads.
        final java.util.List<int[]> snapData = new java.util.ArrayList<int[]>(sA11yData);
        final java.util.List<String[]> snapStrings = new java.util.ArrayList<String[]>(sA11yStrings);

        if (mA11yContainer == null) {
            return; // UI not ready yet; doSetupAccessibility applies the snapshot
        }
        mA11yContainer.post(new Runnable() {
            @Override
            public void run() {
                reconcileA11yNodes(snapData, snapStrings);
            }
        });
    }

    // Called from Go (via JNI) for screen-reader announcements (Toast,
    // Snackbar, bellui.AnnounceToScreenReader).
    static void announceAccessibility(final String text, boolean assertive) {
        goNativeActivity.runOnUiThread(new Runnable() {
            @Override
            public void run() {
                View target = mA11yContainer;
                if (target == null) {
                    target = goNativeActivity.getWindow().getDecorView();
                }
                target.announceForAccessibility(text);
            }
        });
    }

    // Applies the most recently committed snapshot; UI thread only.
    private static void applyA11ySnapshot() {
        final java.util.List<int[]> snapData;
        final java.util.List<String[]> snapStrings;
        synchronized (GoNativeActivity.class) {
            snapData = new java.util.ArrayList<int[]>(sA11yData);
            snapStrings = new java.util.ArrayList<String[]>(sA11yStrings);
        }
        reconcileA11yNodes(snapData, snapStrings);
    }

    private static String a11yClassName(int role) {
        switch (role) {
            case A11Y_ROLE_BUTTON:
            case A11Y_ROLE_LINK:
            case A11Y_ROLE_TAB:
            case A11Y_ROLE_SPIN:
            case A11Y_ROLE_MENUITEM:
                return "android.widget.Button";
            case A11Y_ROLE_IMAGE:
                return "android.widget.ImageView";
            case A11Y_ROLE_CHECKBOX:
                return "android.widget.CheckBox";
            case A11Y_ROLE_SWITCH:
                return "android.widget.Switch";
            case A11Y_ROLE_RADIO:
                return "android.widget.RadioButton";
            case A11Y_ROLE_TEXTFIELD:
            case A11Y_ROLE_TEXTAREA:
            case A11Y_ROLE_PASSWORD:
            case A11Y_ROLE_SEARCH:
                return "android.widget.EditText";
            case A11Y_ROLE_SLIDER:
                return "android.widget.SeekBar";
            case A11Y_ROLE_PROGRESS:
                return "android.widget.ProgressBar";
            case A11Y_ROLE_COMBO:
                return "android.widget.Spinner";
            default: // text, heading, list/tree items, cells
                return "android.widget.TextView";
        }
    }

    private static boolean a11yIsValueBearing(int role) {
        switch (role) {
            case A11Y_ROLE_TEXTFIELD:
            case A11Y_ROLE_TEXTAREA:
            case A11Y_ROLE_PASSWORD:
            case A11Y_ROLE_SEARCH:
            case A11Y_ROLE_SLIDER:
            case A11Y_ROLE_PROGRESS:
            case A11Y_ROLE_SPIN:
            case A11Y_ROLE_COMBO:
                return true;
        }
        return false;
    }

    private static boolean a11yIsTextual(int role) {
        switch (role) {
            case A11Y_ROLE_TEXT:
            case A11Y_ROLE_HEADING:
            case A11Y_ROLE_LISTITEM:
            case A11Y_ROLE_TREEITEM:
            case A11Y_ROLE_CELL:
                return true;
        }
        return false;
    }

    // Reconcile the per-node state records against the new snapshot IN PLACE,
    // keyed by nodeId, and re-derive the reading-order lists the provider
    // serves children from. No Views are created, moved or destroyed — the
    // provider answers TalkBack from these records on demand — so an update
    // can never invalidate the node ids TalkBack has cached (#1104), and its
    // accessibility focus survives value and position changes.
    // Must be called on the UI thread.
    private static void reconcileA11yNodes(java.util.List<int[]> data, java.util.List<String[]> strings) {
        if (mA11yContainer == null) {
            return;
        }
        java.util.HashSet<Integer> present = new java.util.HashSet<Integer>();
        mA11yChildOrder.clear();

        for (int i = 0; i < data.size(); i++) {
            int[] d = data.get(i);
            String[] s = strings.get(i);
            int nodeId = d[0];
            present.add(nodeId);

            A11yProxy p = mA11yProxies.get(nodeId);
            boolean isNew = p == null;
            if (isNew) {
                p = new A11yProxy(nodeId);
                mA11yProxies.put(nodeId, p);
            }

            boolean valueChanged = !isNew && (!p.label.equals(s[0]) || !p.value.equals(s[1]));
            boolean hadDoc = !isNew && a11yHasTextDoc(p);
            String oldText = p.text;
            int oldSel16 = p.selStart16, oldEnd16 = p.selEnd16;
            boolean wasFocused = !isNew && (p.flags & A11Y_FLAG_FOCUSED) != 0;
            p.role = d[1];
            p.flags = d[2];
            p.label = s[0];
            p.value = s[1];
            p.hint = s[2];
            String acts = s.length > 4 ? s[4] : "";
            if (!acts.equals(p.customActs)) {
                p.customActs = acts;
                a11yParseCustomActs(p);
            }
            String layout = s.length > 5 ? s[5] : "";
            if (!layout.equals(p.textLayout)) {
                p.textLayout = layout;
                a11yParseTextLayout(p);
            }

            p.x = d[3];
            p.y = d[4];
            p.w = d[5];
            p.h = d[6];
            p.parentId = d[7];
            p.row = d[8];
            p.col = d[9];
            p.rowCount = d[10];
            p.colCount = d[11];

            // Editable-text document sync (#974 option D): converge the
            // optimistically-applied Java state onto widget truth and
            // synthesize the events TalkBack consumes — a diff of the old vs
            // new published state, the Compose model. Selection arrives as
            // rune offsets and is converted to UTF-16 against the new text.
            a11yReconcileTextDoc(p, d, s, isNew, hadDoc,
                    oldText, oldSel16, oldEnd16, wasFocused);

            // Slot the node into the reading order under its parent. Cells of
            // a virtualized grid nest under the grid node (#817) so TalkBack
            // associates their CollectionItemInfo with its CollectionInfo;
            // everything else is a child of the host root. The grid precedes
            // its cells on the wire, so its record exists when they resolve.
            java.util.ArrayList<Integer> siblings = mA11yChildOrder.get(a11yParentKey(p));
            if (siblings == null) {
                siblings = new java.util.ArrayList<Integer>();
                mA11yChildOrder.put(a11yParentKey(p), siblings);
            }
            siblings.add(nodeId);

            // Re-announce a changed value on an adjustable node via a
            // TARGETED state-description change: TalkBack speaks the new value
            // of the focused control, without re-reading the window. A plain
            // TYPE_WINDOW_CONTENT_CHANGED (no change type) would make TalkBack
            // announce the window's first element instead, and our spoofed
            // SeekBar is not re-announced natively on adjust.
            if (valueChanged && (p.flags & A11Y_FLAG_ADJUSTABLE) != 0
                    && Build.VERSION.SDK_INT >= 30) {
                AccessibilityEvent ev = AccessibilityEvent.obtain(
                        AccessibilityEvent.TYPE_WINDOW_CONTENT_CHANGED);
                ev.setContentChangeTypes(
                        AccessibilityEvent.CONTENT_CHANGE_TYPE_STATE_DESCRIPTION);
                a11ySendEvent(p, ev);
            }
        }

        // Drop records whose nodes disappeared from the tree.
        java.util.Iterator<java.util.Map.Entry<Integer, A11yProxy>> it = mA11yProxies.entrySet().iterator();
        while (it.hasNext()) {
            java.util.Map.Entry<Integer, A11yProxy> e = it.next();
            if (!present.contains(e.getKey())) {
                int gone = e.getKey().intValue();
                it.remove();
                if (gone == mA11yHoveredId) {
                    mA11yHoveredId = A11Y_NO_NODE;
                }
                if (gone == mA11yFocusedId) {
                    // The focused node left the tree (scrolled out of the
                    // realized window, dialog dismissed, ...): report the
                    // clear so TalkBack re-anchors and the in-scene focus
                    // ring (#759) is dropped.
                    mA11yFocusedId = A11Y_NO_NODE;
                    a11ySendEventFor(gone, AccessibilityEvent.obtain(
                            AccessibilityEvent.TYPE_VIEW_ACCESSIBILITY_FOCUS_CLEARED));
                    goNativeActivity.accessibilityFocusChanged(gone, false);
                }
            }
        }

        // One structural invalidation per commit: with no real child Views
        // the framework no longer notifies the accessibility layer of tree
        // changes for us, and TalkBack would keep serving stale nodes from
        // its cache. Commits are already change-gated by sA11ySignature, so
        // this fires only on real changes.
        AccessibilityEvent ev = AccessibilityEvent.obtain(
                AccessibilityEvent.TYPE_WINDOW_CONTENT_CHANGED);
        ev.setContentChangeTypes(AccessibilityEvent.CONTENT_CHANGE_TYPE_SUBTREE);
        a11ySendEventFor(AccessibilityNodeProvider.HOST_VIEW_ID, ev);
    }

    // The provider parent of a node: the grid it nests under (#817), or
    // A11Y_NO_NODE for a child of the host root.
    private static int a11yParentKey(A11yProxy p) {
        if (p.parentId != A11Y_NO_NODE) {
            A11yProxy pp = mA11yProxies.get(p.parentId);
            if (pp != null && pp.role == A11Y_ROLE_GRID) {
                return p.parentId;
            }
        }
        return A11Y_NO_NODE;
    }

    // Routes an accessibility event for a virtual node up the view parent
    // chain. Events MUST travel this path (not View.sendAccessibilityEvent,
    // which would re-stamp the host as the source): ViewRootImpl inspects
    // them to track the accessibility-focused virtual node, which is what
    // makes the framework's findFocus round-trips resolve to our provider —
    // the ExploreByTouchHelper event model.
    private static void a11ySendEventFor(int nodeId, AccessibilityEvent ev) {
        A11yHostView host = mA11yContainer;
        if (host == null || !host.isAttachedToWindow()) {
            return;
        }
        ev.setPackageName(goNativeActivity.getPackageName());
        if (nodeId == AccessibilityNodeProvider.HOST_VIEW_ID) {
            ev.setSource(host);
        } else {
            ev.setSource(host, nodeId);
        }
        ViewParent parent = host.getParent();
        if (parent != null) {
            parent.requestSendAccessibilityEvent(host, ev);
        }
    }

    private static void a11ySendEvent(A11yProxy p, AccessibilityEvent ev) {
        a11ySendEventFor(p.nodeId, ev);
    }

    // -------------------------------------------------------------------------
    // Editable-text node protocol (#974 option D).
    //
    // A text-input node backed by the Go text seam publishes its live document
    // and selection and handles the platform editable-text actions itself:
    // granularity segments are resolved HERE (native UTF-16 +
    // java.text.BreakIterator = platform-exact semantics), the result is
    // applied to the proxy state optimistically and its events fired
    // immediately — performAccessibilityAction must answer synchronously while
    // the Go dispatch is queued (fyne#973) — and the edit is sent to Go as rune
    // offsets, where it lands on the widget's gated, undo-recorded paths. The
    // per-frame reconcile then converges the optimistic state onto widget
    // truth, firing corrective events only on a real disagreement. Flutter and
    // Compose drive TalkBack the same way.
    // -------------------------------------------------------------------------

    private static boolean a11yIsTextInput(int role) {
        switch (role) {
            case A11Y_ROLE_TEXTFIELD:
            case A11Y_ROLE_TEXTAREA:
            case A11Y_ROLE_PASSWORD:
            case A11Y_ROLE_SEARCH:
                return true;
        }
        return false;
    }

    // A node carries the editable-text document when it is a text-input role
    // whose Go object implements the text seam (the wire selection is >= 0).
    private static boolean a11yHasTextDoc(A11yProxy p) {
        return a11yIsTextInput(p.role) && p.selStart16 >= 0;
    }

    // Read-only and disabled editors keep the document (select/copy/traverse)
    // but not the mutating actions.
    private static boolean a11yIsEditableDoc(A11yProxy p) {
        return a11yHasTextDoc(p) && (p.flags & A11Y_FLAG_TEXT_EDITABLE) != 0;
    }

    // Rune (code point) <-> UTF-16 offset conversion against a document. The
    // Go wire speaks runes; everything Android-side is UTF-16.
    private static int a11yUtf16FromRune(String text, int runeOff) {
        if (runeOff < 0) {
            return -1;
        }
        int runes = text.codePointCount(0, text.length());
        if (runeOff > runes) {
            runeOff = runes;
        }
        return text.offsetByCodePoints(0, runeOff);
    }

    private static int a11yRuneFromUtf16(String text, int off16) {
        if (off16 < 0) {
            return -1;
        }
        if (off16 > text.length()) {
            off16 = text.length();
        }
        return text.codePointCount(0, off16);
    }

    // Converge a proxy's document state onto the freshly published wire node
    // and synthesize TalkBack's events from the diff. Called from the
    // reconcile with the pre-update state captured. UI thread only.
    private static void a11yReconcileTextDoc(A11yProxy p, int[] d, String[] s,
            boolean isNew, boolean hadDoc,
            String oldText, int oldSel16, int oldEnd16, boolean wasFocused) {
        int selStartRune = d.length > 13 ? d[12] : -1;
        int selEndRune = d.length > 13 ? d[13] : -1;
        String newText = s.length > 3 ? s[3] : "";

        if (selStartRune < 0 || !a11yIsTextInput(p.role)) {
            p.text = "";
            p.selStart16 = -1;
            p.selEnd16 = -1;
            return;
        }

        p.text = newText;
        int ns = a11yUtf16FromRune(newText, selStartRune);
        int ne = a11yUtf16FromRune(newText, selEndRune);
        boolean textChanged = hadDoc && !oldText.equals(newText);
        boolean selChanged = false;
        if (!hadDoc) {
            p.selStart16 = ns;
            p.selEnd16 = ne;
        } else if (!textChanged
                && Math.min(oldSel16, oldEnd16) == Math.min(ns, ne)
                && Math.max(oldSel16, oldEnd16) == Math.max(ns, ne)) {
            // The widget agrees modulo direction: keep the RAW (possibly
            // reversed) optimistic selection so the anchor/cursor split of an
            // extend-backward traversal survives the round-trip — the wire
            // normalizes start <= end.
            p.selStart16 = oldSel16;
            p.selEnd16 = oldEnd16;
        } else {
            p.selStart16 = ns;
            p.selEnd16 = ne;
            selChanged = Math.min(oldSel16, oldEnd16) != Math.min(ns, ne)
                    || Math.max(oldSel16, oldEnd16) != Math.max(ns, ne);
        }

        boolean focusedNow = (p.flags & A11Y_FLAG_FOCUSED) != 0;
        if (textChanged) {
            a11ySendTextChanged(p, oldText, newText);
        }
        if ((textChanged || selChanged) && focusedNow) {
            a11ySendSelectionChanged(p);
        }
        // Announce input focus so TalkBack couples its editing UX (cursor
        // control menu, typing echo) to the field the keyboard is on — the
        // shadow EditText that used to provide this is hidden from
        // accessibility (fyne#968).
        if (focusedNow && !wasFocused) {
            a11ySendEvent(p, AccessibilityEvent.obtain(AccessibilityEvent.TYPE_VIEW_FOCUSED));
        }
    }

    // TYPE_VIEW_TEXT_CHANGED from a surrogate-safe common prefix/suffix diff
    // of the old vs new document — TalkBack validates the arithmetic
    // (fromIndex + removed/added vs the texts) in UTF-16, so both strings and
    // the diff live entirely in Java.
    private static void a11ySendTextChanged(A11yProxy p, String oldText, String newText) {
        int oldLen = oldText.length(), newLen = newText.length();
        int max = Math.min(oldLen, newLen);
        int prefix = 0;
        while (prefix < max && oldText.charAt(prefix) == newText.charAt(prefix)) {
            prefix++;
        }
        // Never split a surrogate pair at the change boundary.
        if (prefix > 0 && prefix < max
                && Character.isHighSurrogate(oldText.charAt(prefix - 1))) {
            prefix--;
        }
        int suffix = 0;
        while (suffix < max - prefix
                && oldText.charAt(oldLen - 1 - suffix) == newText.charAt(newLen - 1 - suffix)) {
            suffix++;
        }
        if (suffix > 0 && suffix < max - prefix
                && Character.isLowSurrogate(oldText.charAt(oldLen - suffix))) {
            suffix--;
        }

        AccessibilityEvent ev = AccessibilityEvent.obtain(
                AccessibilityEvent.TYPE_VIEW_TEXT_CHANGED);
        ev.setBeforeText(oldText);
        ev.getText().add(newText);
        ev.setFromIndex(prefix);
        ev.setRemovedCount(oldLen - prefix - suffix);
        ev.setAddedCount(newLen - prefix - suffix);
        if (p.role == A11Y_ROLE_PASSWORD) {
            ev.setPassword(true);
        }
        a11ySendEvent(p, ev);
    }

    private static void a11ySendSelectionChanged(A11yProxy p) {
        AccessibilityEvent ev = AccessibilityEvent.obtain(
                AccessibilityEvent.TYPE_VIEW_TEXT_SELECTION_CHANGED);
        ev.getText().add(p.text);
        ev.setItemCount(p.text.length());
        ev.setFromIndex(Math.min(p.selStart16, p.selEnd16));
        ev.setToIndex(Math.max(p.selStart16, p.selEnd16));
        if (p.role == A11Y_ROLE_PASSWORD) {
            ev.setPassword(true);
        }
        a11ySendEvent(p, ev);
    }

    private static void a11ySendTraversed(A11yProxy p, boolean forward,
            int granularity, int segStart, int segEnd) {
        AccessibilityEvent ev = AccessibilityEvent.obtain(
                AccessibilityEvent.TYPE_VIEW_TEXT_TRAVERSED_AT_MOVEMENT_GRANULARITY);
        ev.getText().add(p.text);
        ev.setFromIndex(segStart);
        ev.setToIndex(segEnd);
        ev.setMovementGranularity(granularity);
        ev.setAction(forward
                ? AccessibilityNodeInfo.ACTION_NEXT_AT_MOVEMENT_GRANULARITY
                : AccessibilityNodeInfo.ACTION_PREVIOUS_AT_MOVEMENT_GRANULARITY);
        if (p.role == A11Y_ROLE_PASSWORD) {
            ev.setPassword(true);
        }
        a11ySendEvent(p, ev);
    }

    // Store the (raw, possibly reversed) selection optimistically and send it
    // to the widget as rune offsets; the widget clamps and normalizes.
    private static void a11yApplySelection(A11yProxy p, int s16, int e16) {
        p.selStart16 = s16;
        p.selEnd16 = e16;
        goNativeActivity.accessibilityTextAction(p.nodeId, A11Y_TEXTACT_SET_SELECTION,
                a11yRuneFromUtf16(p.text, s16), a11yRuneFromUtf16(p.text, e16), "");
    }

    // ACTION_SET_SELECTION: explicit offsets from the service (TalkBack's
    // cursor-control menu, and modern TalkBack's self-computed LINE
    // traversal). Absent/negative arguments clear the selection, collapsing
    // to the current cursor.
    private static boolean a11ySetSelection(A11yProxy p, Bundle args) {
        if (!a11yHasTextDoc(p)) {
            return false;
        }
        int s = -1, e = -1;
        if (args != null) {
            s = args.getInt(AccessibilityNodeInfo.ACTION_ARGUMENT_SELECTION_START_INT, -1);
            e = args.getInt(AccessibilityNodeInfo.ACTION_ARGUMENT_SELECTION_END_INT, -1);
        }
        int len = p.text.length();
        if (s < 0 && e < 0) {
            s = e = Math.max(0, Math.min(p.selEnd16, len));
        } else {
            if (e < 0) {
                e = s;
            }
            s = Math.max(0, Math.min(s, len));
            e = Math.max(0, Math.min(e, len));
        }
        a11yApplySelection(p, s, e);
        a11ySendSelectionChanged(p);
        return true;
    }

    // NEXT/PREVIOUS_AT_MOVEMENT_GRANULARITY: the View.java traversal model —
    // the free cursor is the selection end; forward moves it to the segment
    // end, backward to the segment start; extending keeps the anchor. Returns
    // false at the document edge so TalkBack knows it hit the boundary.
    private static boolean a11yTraverse(A11yProxy p, Bundle args, boolean forward) {
        if (!a11yHasTextDoc(p) || args == null) {
            return false;
        }
        int granularity = args.getInt(
                AccessibilityNodeInfo.ACTION_ARGUMENT_MOVEMENT_GRANULARITY_INT);
        boolean extend = args.getBoolean(
                AccessibilityNodeInfo.ACTION_ARGUMENT_EXTEND_SELECTION_BOOLEAN, false);
        String text = p.text;
        int current = p.selEnd16 < 0 ? (forward ? 0 : text.length())
                : Math.max(0, Math.min(p.selEnd16, text.length()));
        int[] seg = null;
        if (granularity == AccessibilityNodeInfo.MOVEMENT_GRANULARITY_LINE) {
            // Visual (soft-wrapped) lines from the focused editor's layout
            // sidecar (#995) — what legacy TalkBack (< ~14, no self-computed
            // soft-wrap LINE) needs for per-visual-line traversal. Outside
            // the sidecar's window this stays null and the logical
            // (paragraph) fallback below answers, as before.
            seg = a11yVisualLineSegment(p, current, forward);
        }
        if (seg == null) {
            seg = forward ? a11ySegmentAfter(text, current, granularity)
                    : a11ySegmentBefore(text, current, granularity);
        }
        if (seg == null) {
            return false;
        }
        int target = forward ? seg[1] : seg[0];
        int selStart, selEnd;
        if (extend) {
            selStart = p.selStart16 < 0 ? current : p.selStart16;
            selEnd = target;
        } else {
            selStart = selEnd = target;
        }
        a11yApplySelection(p, selStart, selEnd);
        // Compose's ordering: the selection change first, then the traversal
        // event TalkBack speaks the segment from.
        a11ySendSelectionChanged(p);
        a11ySendTraversed(p, forward, granularity, seg[0], seg[1]);
        return true;
    }

    // ACTION_SET_TEXT: whole-document replace; the caret lands at the end,
    // matching TextView.
    private static boolean a11ySetText(A11yProxy p, Bundle args) {
        if (!a11yIsEditableDoc(p)) {
            return false;
        }
        CharSequence cs = args != null ? args.getCharSequence(
                AccessibilityNodeInfo.ACTION_ARGUMENT_SET_TEXT_CHARSEQUENCE) : null;
        String newText = cs != null ? cs.toString() : "";
        String oldText = p.text;
        p.text = newText;
        p.selStart16 = newText.length();
        p.selEnd16 = newText.length();
        goNativeActivity.accessibilityTextAction(p.nodeId, A11Y_TEXTACT_SET_TEXT,
                0, 0, newText);
        a11ySendTextChanged(p, oldText, newText);
        a11ySendSelectionChanged(p);
        return true;
    }

    // ACTION_COPY / ACTION_CUT: the selected document range through the system
    // clipboard. Never offered on a password node (the document is the mask,
    // but the gesture must not pretend to copy the secret either).
    private static boolean a11yCopy(A11yProxy p, boolean cut) {
        if (!a11yHasTextDoc(p) || p.role == A11Y_ROLE_PASSWORD) {
            return false;
        }
        if (cut && !a11yIsEditableDoc(p)) {
            return false;
        }
        int len = p.text.length();
        int s = Math.max(0, Math.min(Math.min(p.selStart16, p.selEnd16), len));
        int e = Math.max(0, Math.min(Math.max(p.selStart16, p.selEnd16), len));
        if (s == e) {
            return false;
        }
        android.content.ClipboardManager cm = (android.content.ClipboardManager)
                goNativeActivity.getSystemService(Context.CLIPBOARD_SERVICE);
        if (cm == null) {
            return false;
        }
        cm.setPrimaryClip(android.content.ClipData.newPlainText("text", p.text.substring(s, e)));
        if (cut) {
            int rs = a11yRuneFromUtf16(p.text, s);
            int re = a11yRuneFromUtf16(p.text, e);
            String oldText = p.text;
            p.text = p.text.substring(0, s) + p.text.substring(e);
            p.selStart16 = s;
            p.selEnd16 = s;
            goNativeActivity.accessibilityTextAction(p.nodeId, A11Y_TEXTACT_REPLACE,
                    rs, re, "");
            a11ySendTextChanged(p, oldText, p.text);
            a11ySendSelectionChanged(p);
        }
        return true;
    }

    // ACTION_PASTE: replace the current selection (or insert at the caret)
    // with the clipboard text. The Go side's insert primitive replaces the
    // widget's own live selection, which the per-frame sync keeps equal to the
    // optimistic one here.
    private static boolean a11yPaste(A11yProxy p) {
        if (!a11yIsEditableDoc(p)) {
            return false;
        }
        android.content.ClipboardManager cm = (android.content.ClipboardManager)
                goNativeActivity.getSystemService(Context.CLIPBOARD_SERVICE);
        if (cm == null) {
            return false;
        }
        android.content.ClipData clip = cm.getPrimaryClip();
        if (clip == null || clip.getItemCount() == 0) {
            return false;
        }
        CharSequence cs = clip.getItemAt(0).coerceToText(goNativeActivity);
        if (cs == null) {
            return false;
        }
        String ins = cs.toString();
        int len = p.text.length();
        int s = Math.max(0, Math.min(Math.min(p.selStart16, p.selEnd16), len));
        int e = Math.max(0, Math.min(Math.max(p.selStart16, p.selEnd16), len));
        String oldText = p.text;
        p.text = p.text.substring(0, s) + ins + p.text.substring(e);
        p.selStart16 = s + ins.length();
        p.selEnd16 = p.selStart16;
        goNativeActivity.accessibilityTextAction(p.nodeId, A11Y_TEXTACT_INSERT,
                0, 0, ins);
        a11ySendTextChanged(p, oldText, p.text);
        a11ySendSelectionChanged(p);
        return true;
    }

    // Granularity segment iterators, ported from AOSP View.java's
    // AccessibilityIterators so segment semantics match a real TextView:
    // character and word via java.text.BreakIterator, paragraph on newline
    // boundaries. LINE means the LOGICAL (newline-delimited) line — the
    // contract's line semantics; modern TalkBack computes soft-wrap LINE
    // itself via node refresh + ACTION_SET_SELECTION — and PAGE has no layout
    // here, so both resolve as paragraph. Returns {start, end} in UTF-16
    // units, or null at the document edge / unsupported granularity.
    private static int[] a11ySegmentAfter(String text, int offset, int granularity) {
        switch (granularity) {
            case AccessibilityNodeInfo.MOVEMENT_GRANULARITY_CHARACTER:
                return a11yCharSegment(text, offset, true);
            case AccessibilityNodeInfo.MOVEMENT_GRANULARITY_WORD:
                return a11yWordSegment(text, offset, true);
            case AccessibilityNodeInfo.MOVEMENT_GRANULARITY_LINE:
            case AccessibilityNodeInfo.MOVEMENT_GRANULARITY_PARAGRAPH:
            case AccessibilityNodeInfo.MOVEMENT_GRANULARITY_PAGE:
                return a11yParagraphSegment(text, offset, true);
        }
        return null;
    }

    private static int[] a11ySegmentBefore(String text, int offset, int granularity) {
        switch (granularity) {
            case AccessibilityNodeInfo.MOVEMENT_GRANULARITY_CHARACTER:
                return a11yCharSegment(text, offset, false);
            case AccessibilityNodeInfo.MOVEMENT_GRANULARITY_WORD:
                return a11yWordSegment(text, offset, false);
            case AccessibilityNodeInfo.MOVEMENT_GRANULARITY_LINE:
            case AccessibilityNodeInfo.MOVEMENT_GRANULARITY_PARAGRAPH:
            case AccessibilityNodeInfo.MOVEMENT_GRANULARITY_PAGE:
                return a11yParagraphSegment(text, offset, false);
        }
        return null;
    }

    private static int[] a11yCharSegment(String text, int offset, boolean forward) {
        int len = text.length();
        if (len <= 0) {
            return null;
        }
        java.text.BreakIterator it = java.text.BreakIterator.getCharacterInstance();
        it.setText(text);
        if (forward) {
            if (offset >= len) {
                return null;
            }
            int start = Math.max(offset, 0);
            while (!it.isBoundary(start)) {
                start = it.following(start);
                if (start == java.text.BreakIterator.DONE) {
                    return null;
                }
            }
            int end = it.following(start);
            if (end == java.text.BreakIterator.DONE) {
                return null;
            }
            return new int[]{start, end};
        }
        if (offset <= 0) {
            return null;
        }
        int end = Math.min(offset, len);
        while (!it.isBoundary(end)) {
            end = it.preceding(end);
            if (end == java.text.BreakIterator.DONE) {
                return null;
            }
        }
        int start = it.preceding(end);
        if (start == java.text.BreakIterator.DONE) {
            return null;
        }
        return new int[]{start, end};
    }

    private static boolean a11yIsWordChar(String text, int index) {
        if (index < 0 || index >= text.length()) {
            return false;
        }
        return Character.isLetterOrDigit(text.codePointAt(index));
    }

    private static boolean a11yIsWordStart(String text, int index) {
        return a11yIsWordChar(text, index)
                && (index == 0 || !a11yIsWordChar(text, index - 1));
    }

    private static boolean a11yIsWordEnd(String text, int index) {
        return index > 0 && a11yIsWordChar(text, index - 1)
                && (index == text.length() || !a11yIsWordChar(text, index));
    }

    private static int[] a11yWordSegment(String text, int offset, boolean forward) {
        int len = text.length();
        if (len <= 0) {
            return null;
        }
        java.text.BreakIterator it = java.text.BreakIterator.getWordInstance();
        it.setText(text);
        if (forward) {
            if (offset >= len) {
                return null;
            }
            int start = Math.max(offset, 0);
            while (!a11yIsWordChar(text, start) && !a11yIsWordStart(text, start)) {
                start = it.following(start);
                if (start == java.text.BreakIterator.DONE) {
                    return null;
                }
            }
            int end = it.following(start);
            if (end == java.text.BreakIterator.DONE || !a11yIsWordEnd(text, end)) {
                return null;
            }
            return new int[]{start, end};
        }
        if (offset <= 0) {
            return null;
        }
        int end = Math.min(offset, len);
        while (end > 0 && !a11yIsWordChar(text, end - 1) && !a11yIsWordEnd(text, end)) {
            end = it.preceding(end);
            if (end == java.text.BreakIterator.DONE) {
                return null;
            }
        }
        int start = it.preceding(end);
        if (start == java.text.BreakIterator.DONE || !a11yIsWordStart(text, start)) {
            return null;
        }
        return new int[]{start, end};
    }

    private static boolean a11yIsParaStart(String text, int index) {
        return text.charAt(index) != '\n'
                && (index == 0 || text.charAt(index - 1) == '\n');
    }

    private static boolean a11yIsParaEnd(String text, int index) {
        return index > 0 && text.charAt(index - 1) != '\n'
                && (index == text.length() || text.charAt(index) == '\n');
    }

    private static int[] a11yParagraphSegment(String text, int offset, boolean forward) {
        int len = text.length();
        if (len <= 0) {
            return null;
        }
        if (forward) {
            if (offset >= len) {
                return null;
            }
            int start = Math.max(offset, 0);
            while (start < len && text.charAt(start) == '\n' && !a11yIsParaStart(text, start)) {
                start++;
            }
            if (start >= len) {
                return null;
            }
            int end = start + 1;
            while (end < len && !a11yIsParaEnd(text, end)) {
                end++;
            }
            return new int[]{start, end};
        }
        if (offset <= 0) {
            return null;
        }
        int end = Math.min(offset, len);
        while (end > 0 && text.charAt(end - 1) == '\n' && !a11yIsParaEnd(text, end)) {
            end--;
        }
        if (end <= 0) {
            return null;
        }
        int start = end - 1;
        while (start > 0 && !a11yIsParaStart(text, start)) {
            start--;
        }
        return new int[]{start, end};
    }

    // Parses a proxy's encoded custom-action wire string (#1025) — Go registry
    // id U+001F localized label, entries separated by U+001E — into the parsed
    // arrays the delegate registers from. Malformed entries are skipped; no
    // valid entries leaves the arrays null (no custom actions).
    private static void a11yParseCustomActs(A11yProxy p) {
        if (p.customActs.isEmpty()) {
            p.customIds = null;
            p.customLabels = null;
            return;
        }
        String[] entries = p.customActs.split(String.valueOf((char) 0x1e), -1);
        int[] ids = new int[entries.length];
        String[] labels = new String[entries.length];
        int n = 0;
        for (String e : entries) {
            int sep = e.indexOf((char) 0x1f);
            if (sep <= 0) {
                continue;
            }
            try {
                ids[n] = Integer.parseInt(e.substring(0, sep));
            } catch (NumberFormatException ex) {
                continue;
            }
            labels[n] = e.substring(sep + 1);
            n++;
        }
        if (n == 0) {
            p.customIds = null;
            p.customLabels = null;
            return;
        }
        p.customIds = java.util.Arrays.copyOf(ids, n);
        p.customLabels = java.util.Arrays.copyOf(labels, n);
    }

    // Parses the visual-layout sidecar (#995) into the proxy's consumable
    // forms. Wire format (see the Go encoder mobileA11yTextLayout):
    //   "L:b0,b1,..."               visual-line boundary offsets (UTF-16)
    //   "\nC:start16:x,y,w,h;-;..." per-char rects in container px, '-' = none
    private static void a11yParseTextLayout(A11yProxy p) {
        p.lineBounds16 = null;
        p.charLocStart16 = -1;
        p.charLocRects = null;
        if (p.textLayout.isEmpty()) {
            return;
        }
        for (String section : p.textLayout.split("\n", -1)) {
            try {
                if (section.startsWith("L:")) {
                    String[] parts = section.substring(2).split(",", -1);
                    int[] bounds = new int[parts.length];
                    for (int i = 0; i < parts.length; i++) {
                        bounds[i] = Integer.parseInt(parts[i]);
                    }
                    if (bounds.length >= 2) {
                        p.lineBounds16 = bounds;
                    }
                } else if (section.startsWith("C:")) {
                    int colon = section.indexOf(':', 2);
                    if (colon < 0) {
                        continue;
                    }
                    int start = Integer.parseInt(section.substring(2, colon));
                    String body = section.substring(colon + 1);
                    if (body.isEmpty()) {
                        continue;
                    }
                    String[] entries = body.split(";", -1);
                    float[] rects = new float[entries.length * 4];
                    for (int i = 0; i < entries.length; i++) {
                        String e = entries[i];
                        if (e.equals("-")) {
                            rects[i * 4] = Float.NaN;
                            continue;
                        }
                        String[] c = e.split(",", -1);
                        if (c.length != 4) {
                            rects[i * 4] = Float.NaN;
                            continue;
                        }
                        for (int j = 0; j < 4; j++) {
                            rects[i * 4 + j] = Integer.parseInt(c[j]);
                        }
                    }
                    p.charLocStart16 = start;
                    p.charLocRects = rects;
                }
            } catch (NumberFormatException ex) {
                // A malformed section is dropped; the consumers fall back.
            }
        }
    }

    // Visual (soft-wrapped) LINE segment from the sidecar's boundary window
    // (#995), mirroring AOSP LineTextSegmentIterator semantics over the
    // covered range: line i of the window is [b[i], b[i+1]). Returns null
    // when the node carries no sidecar, the offset walks outside the covered
    // window, or the walk leaves the window — the caller then falls back to
    // logical (paragraph) lines, the pre-#995 behavior; at a genuine
    // document edge that fallback reports the edge identically.
    private static int[] a11yVisualLineSegment(A11yProxy p, int offset, boolean forward) {
        int[] b = p.lineBounds16;
        if (b == null || b.length < 2) {
            return null;
        }
        int lines = b.length - 1;
        if (offset < b[0] || offset > b[lines]) {
            return null;
        }
        // The window line containing offset: the last i with b[i] <= offset.
        int line = 0;
        while (line + 1 < lines && b[line + 1] <= offset) {
            line++;
        }
        if (forward) {
            // The line containing offset, unless the offset already sits at
            // its end — then the next one (the View.java iterator model).
            if (offset >= b[line + 1]) {
                line++;
            }
            if (line >= lines) {
                return null;
            }
            return new int[]{b[line], b[line + 1]};
        }
        if (offset <= b[line]) {
            line--;
        }
        if (line < 0) {
            return null;
        }
        return new int[]{b[line], b[line + 1]};
    }

    // -------------------------------------------------------------------------
    // Virtual-node provider (#1104): builds AccessibilityNodeInfos, performs
    // actions and maps touch-exploration hovers for the records behind the
    // host View. All callbacks arrive on the UI thread.
    // -------------------------------------------------------------------------

    // Builds the node for one virtual id (or the host root). With no real
    // View behind a node, every framework default a View used to supply
    // (package, bounds, visibility, parent/child links, focus bookkeeping) is
    // set here alongside the role/state properties.
    private static AccessibilityNodeInfo a11yCreateNodeInfo(A11yHostView host, int virtualViewId) {
        if (virtualViewId == AccessibilityNodeProvider.HOST_VIEW_ID) {
            AccessibilityNodeInfo info = AccessibilityNodeInfo.obtain(host);
            host.onInitializeAccessibilityNodeInfo(info);
            java.util.ArrayList<Integer> roots = mA11yChildOrder.get(A11Y_NO_NODE);
            if (roots != null) {
                for (int i = 0; i < roots.size(); i++) {
                    info.addChild(host, roots.get(i).intValue());
                }
            }
            return info;
        }
        A11yProxy p = mA11yProxies.get(virtualViewId);
        if (p == null) {
            return null;
        }
        AccessibilityNodeInfo info = AccessibilityNodeInfo.obtain(host, virtualViewId);
        info.setPackageName(goNativeActivity.getPackageName());
        info.setVisibleToUser(true);
        if (Build.VERSION.SDK_INT >= 30) {
            // A real View inherited importance from the hierarchy; a virtual
            // node must claim it or TalkBack filters it out.
            info.setImportantForAccessibility(true);
        }
        int parentKey = a11yParentKey(p);
        if (parentKey == A11Y_NO_NODE) {
            info.setParent(host);
        } else {
            info.setParent(host, parentKey);
        }
        // The wire carries container-relative px; the node contract wants
        // screen coordinates.
        int[] loc = new int[2];
        host.getLocationOnScreen(loc);
        info.setBoundsInScreen(new Rect(loc[0] + p.x, loc[1] + p.y,
                loc[0] + p.x + p.w, loc[1] + p.y + p.h));
        if (p.role == A11Y_ROLE_GRID) {
            // The grid serves its realized cells as children (#817).
            java.util.ArrayList<Integer> cells = mA11yChildOrder.get(p.nodeId);
            if (cells != null) {
                for (int i = 0; i < cells.size(); i++) {
                    info.addChild(host, cells.get(i).intValue());
                }
            }
        }
        // Accessibility focus is tracked here, not by the framework (which
        // only tracks it for real Views).
        if (virtualViewId == mA11yFocusedId) {
            info.setAccessibilityFocused(true);
            info.addAction(AccessibilityNodeInfo.ACTION_CLEAR_ACCESSIBILITY_FOCUS);
        } else {
            info.setAccessibilityFocused(false);
            info.addAction(AccessibilityNodeInfo.ACTION_ACCESSIBILITY_FOCUS);
        }
        a11yPopulateNodeInfo(p, info);
        return info;
    }

    // Populates one virtual node's role/state/text/action properties from its
    // record — the former per-View delegate's onInitializeAccessibilityNodeInfo,
    // reading the same live fields.
    private static void a11yPopulateNodeInfo(A11yProxy p, AccessibilityNodeInfo info) {
        final int role = p.role, flags = p.flags;
        final String label = p.label, value = p.value, hint = p.hint;
        info.setClassName(a11yClassName(role));

        final boolean textDoc = a11yHasTextDoc(p);
        if (textDoc) {
            // Editable-text node (#974 option D): the document is
            // the node text — contentDescription would override it
            // on an EditText — and the placeholder label speaks as
            // hintText, the TextView convention.
            info.setText(p.text);
            String placeholder = !hint.isEmpty() ? hint : label;
            if (!placeholder.isEmpty() && Build.VERSION.SDK_INT >= 26) {
                info.setHintText(placeholder);
                info.setShowingHintText(p.text.isEmpty());
            }
        } else if (a11yIsTextual(role)) {
            // Text content speaks as text; contentDescription would
            // double-announce it.
            info.setText(label.isEmpty() ? value : label);
        } else if (a11yIsValueBearing(role)) {
            info.setText(value);
            info.setContentDescription(label);
        } else if (!value.isEmpty()) {
            info.setContentDescription(label.isEmpty() ? value : label + ", " + value);
        } else {
            info.setContentDescription(label);
        }

        info.setEnabled((flags & A11Y_FLAG_DISABLED) == 0);
        info.setSelected((flags & A11Y_FLAG_SELECTED) != 0);
        if ((flags & A11Y_FLAG_CHECKABLE) != 0) {
            boolean checked = (flags & A11Y_FLAG_CHECKED) != 0;
            info.setCheckable(true);
            info.setChecked(checked);
            if (Build.VERSION.SDK_INT >= 30) {
                // TalkBack reliably announces the checked state but
                // stays silent for unchecked on spoofed class names;
                // spell the state out explicitly.
                info.setStateDescription(checked ? "checked" : "not checked");
            }
        }
        if (role == A11Y_ROLE_PASSWORD) {
            info.setPassword(true);
        }
        if (role == A11Y_ROLE_HEADING && Build.VERSION.SDK_INT >= 28) {
            info.setHeading(true);
        }
        if (!textDoc && !hint.isEmpty() && Build.VERSION.SDK_INT >= 26) {
            info.setHintText(hint);
        }
        if (textDoc) {
            // Editable-text protocol (#974 option D): declare the
            // selection, every movement granularity — LINE included;
            // modern TalkBack resolves soft-wrap lines itself via
            // refresh + ACTION_SET_SELECTION, older falls back to
            // our paragraph handling — and the text actions gated
            // the way TextView gates them. TalkBack couples the
            // whole editing UX to isFocused(), which mirrors the
            // canvas keyboard focus.
            final int len = p.text.length();
            final int selS = Math.max(0,
                    Math.min(Math.min(p.selStart16, p.selEnd16), len));
            final int selE = Math.max(0,
                    Math.min(Math.max(p.selStart16, p.selEnd16), len));
            info.setFocusable(true);
            info.setFocused((flags & A11Y_FLAG_FOCUSED) != 0);
            info.setMultiLine(role == A11Y_ROLE_TEXTAREA);
            info.setTextSelection(selS, selE);
            // Advertise per-character geometry (#995): TalkBack's
            // granular highlight/magnifier requests it through
            // addExtraDataToAccessibilityNodeInfo below, served
            // from the sidecar the focused editor publishes.
            if (Build.VERSION.SDK_INT >= 26) {
                info.setAvailableExtraData(java.util.Collections.singletonList(
                        AccessibilityNodeInfo.EXTRA_DATA_TEXT_CHARACTER_LOCATION_KEY));
            }
            info.setMovementGranularities(
                    AccessibilityNodeInfo.MOVEMENT_GRANULARITY_CHARACTER
                    | AccessibilityNodeInfo.MOVEMENT_GRANULARITY_WORD
                    | AccessibilityNodeInfo.MOVEMENT_GRANULARITY_LINE
                    | AccessibilityNodeInfo.MOVEMENT_GRANULARITY_PARAGRAPH
                    | AccessibilityNodeInfo.MOVEMENT_GRANULARITY_PAGE);
            info.addAction(AccessibilityNodeInfo.ACTION_NEXT_AT_MOVEMENT_GRANULARITY);
            info.addAction(AccessibilityNodeInfo.ACTION_PREVIOUS_AT_MOVEMENT_GRANULARITY);
            info.addAction(AccessibilityNodeInfo.ACTION_SET_SELECTION);
            if (selS != selE && role != A11Y_ROLE_PASSWORD) {
                info.addAction(AccessibilityNodeInfo.ACTION_COPY);
            }
            if ((flags & A11Y_FLAG_TEXT_EDITABLE) != 0) {
                info.setEditable(true);
                info.addAction(AccessibilityNodeInfo.ACTION_SET_TEXT);
                info.addAction(AccessibilityNodeInfo.ACTION_PASTE);
                if (selS != selE && role != A11Y_ROLE_PASSWORD) {
                    info.addAction(AccessibilityNodeInfo.ACTION_CUT);
                }
            }
        }
        if (role == A11Y_ROLE_GRID) {
            // Virtualized grid container (#817): report the FULL
            // logical extent (the walk realizes only the visible
            // window), so TalkBack announces "row R of N" against
            // the real collection size, and accept
            // ACTION_SCROLL_TO_POSITION as the bring-into-view
            // path. The container itself is not a reading stop —
            // no text/description — TalkBack still reads its
            // CollectionInfo from the cells nested inside.
            info.setClassName("android.widget.GridView");
            info.setCollectionInfo(AccessibilityNodeInfo.CollectionInfo.obtain(
                    p.rowCount, p.colCount, false));
            info.setText(null);
            info.setContentDescription(null);
            if (Build.VERSION.SDK_INT >= 23) {
                info.addAction(AccessibilityNodeInfo.AccessibilityAction
                        .ACTION_SCROLL_TO_POSITION);
            }
        }
        if (p.row >= 0) {
            // A cell of a virtualized grid: its display coordinates
            // within the collection (#817).
            info.setCollectionItemInfo(AccessibilityNodeInfo.CollectionItemInfo.obtain(
                    p.row, 1, Math.max(p.col, 0), 1, false));
        }

        if ((flags & A11Y_FLAG_PRESSABLE) != 0) {
            info.setClickable(true);
            info.addAction(AccessibilityNodeInfo.ACTION_CLICK);
        }
        if ((flags & A11Y_FLAG_ADJUSTABLE) != 0) {
            // TalkBack treats a node as an adjustable "seek control"
            // (swipe up/down to increment/decrement) only when it
            // reports BOTH a SeekBar class name AND RangeInfo. A slider
            // role supplies the class name for free, but other
            // adjustable widgets (the TimePicker hour/minute fields,
            // reported as tabs) do not, so force it here; and no role
            // supplied RangeInfo, which is why every adjustable widget
            // — including plain Slider — was previously stuck (swipe
            // up/down fell through to the reading-control, e.g. speech
            // rate, instead of the widget). The bounds are a
            // placeholder: the spoken value comes from the
            // text/contentDescription set above and the real
            // increment/decrement runs in Go (ACTION_SCROLL_FORWARD/
            // BACKWARD -> A11Y_ACTION_INCREMENT/DECREMENT). (#796)
            info.setClassName("android.widget.SeekBar");
            info.addAction(AccessibilityNodeInfo.ACTION_SCROLL_FORWARD);
            info.addAction(AccessibilityNodeInfo.ACTION_SCROLL_BACKWARD);
            info.setRangeInfo(AccessibilityNodeInfo.RangeInfo.obtain(
                    AccessibilityNodeInfo.RangeInfo.RANGE_TYPE_PERCENT,
                    0f, 100f, 50f));
            // The RangeInfo above only exists to unlock the adjust
            // gesture; its bounds are a placeholder. Speak the widget's
            // real value through stateDescription — TalkBack announces
            // that instead of the placeholder "50 percent" — and clear
            // the text/contentDescription set earlier so the value is
            // not announced twice. (#796)
            if (Build.VERSION.SDK_INT >= 30) {
                info.setStateDescription(value.isEmpty() ? label : value);
                info.setText(null);
                info.setContentDescription(null);
            }
        } else if ((flags & A11Y_FLAG_SCROLLABLE) != 0) {
            // Inside a scroll area: expose the two-finger / scroll
            // gesture so TalkBack can page the scroller (#762).
            info.setScrollable(true);
            info.addAction(AccessibilityNodeInfo.ACTION_SCROLL_FORWARD);
            info.addAction(AccessibilityNodeInfo.ACTION_SCROLL_BACKWARD);
        }
        if ((flags & A11Y_FLAG_LONGPRESSABLE) != 0) {
            info.setLongClickable(true);
            info.addAction(AccessibilityNodeInfo.ACTION_LONG_CLICK);
        }
        if ((flags & A11Y_FLAG_DISMISSABLE) != 0) {
            info.addAction(AccessibilityNodeInfo.ACTION_DISMISS);
        }
        // Custom accessibility actions (#1025): declared widget
        // actions with no standard Android semantic (undo/redo on
        // an editor, ...), listed by TalkBack in its actions menu
        // with their localized labels.
        if (p.customIds != null && Build.VERSION.SDK_INT >= 21) {
            for (int i = 0; i < p.customIds.length; i++) {
                info.addAction(new AccessibilityNodeInfo.AccessibilityAction(
                        A11Y_CUSTOM_ACTION_BASE + p.customIds[i],
                        p.customLabels[i]));
            }
        }
    }

    // Serves EXTRA_DATA_TEXT_CHARACTER_LOCATION_KEY (#995): the
    // per-character screen rectangles TalkBack's granular
    // highlight/magnifier asks for. The request is synchronous on
    // the UI thread while the Go dispatch is queued, so it is
    // answered from the sidecar cache the focused editor pushes
    // per frame — chars outside the cached window (or scrolled
    // out of the viewport) answer null rects and TalkBack falls
    // back to the element bounds for them.
    private static void a11yAddExtraData(int virtualViewId, AccessibilityNodeInfo info,
            String extraDataKey, Bundle arguments) {
        A11yProxy p = mA11yProxies.get(virtualViewId);
        if (p == null || Build.VERSION.SDK_INT < 26
                || !AccessibilityNodeInfo.EXTRA_DATA_TEXT_CHARACTER_LOCATION_KEY
                        .equals(extraDataKey)
                || arguments == null) {
            return;
        }
        int start = arguments.getInt(AccessibilityNodeInfo
                .EXTRA_DATA_TEXT_CHARACTER_LOCATION_ARG_START_INDEX, -1);
        int length = arguments.getInt(AccessibilityNodeInfo
                .EXTRA_DATA_TEXT_CHARACTER_LOCATION_ARG_LENGTH, -1);
        if (start < 0 || length <= 0) {
            return;
        }
        android.graphics.RectF[] rects = new android.graphics.RectF[length];
        float[] cache = p.charLocRects;
        if (cache != null) {
            int[] loc = new int[2];
            if (mA11yContainer != null) {
                // Cache rects are container px; the extra-data
                // contract wants screen coordinates.
                mA11yContainer.getLocationOnScreen(loc);
            }
            int cached = cache.length / 4;
            for (int i = 0; i < length; i++) {
                int ci = start + i - p.charLocStart16;
                if (ci < 0 || ci >= cached || Float.isNaN(cache[ci * 4])) {
                    continue;
                }
                float x = cache[ci * 4] + loc[0];
                float y = cache[ci * 4 + 1] + loc[1];
                rects[i] = new android.graphics.RectF(
                        x, y, x + cache[ci * 4 + 2], y + cache[ci * 4 + 3]);
            }
        }
        info.getExtras().putParcelableArray(
                AccessibilityNodeInfo.EXTRA_DATA_TEXT_CHARACTER_LOCATION_KEY, rects);
    }

    // Action entry point for the provider. The accessibility-focus actions
    // are handled here — the framework only tracks accessibility focus for
    // real Views — and the focus change is reported to Go so Bellui can paint
    // its own in-scene focus ring (#759): TalkBack's green highlight cannot
    // render over the flat GPU surface. Everything else routes to the
    // per-node dispatch below.
    private static boolean a11yPerformAction(A11yHostView host, int virtualViewId,
            int action, Bundle args) {
        if (virtualViewId == AccessibilityNodeProvider.HOST_VIEW_ID) {
            return host.performAccessibilityAction(action, args);
        }
        A11yProxy p = mA11yProxies.get(virtualViewId);
        if (p == null) {
            return false;
        }
        switch (action) {
            case AccessibilityNodeInfo.ACTION_ACCESSIBILITY_FOCUS:
                if (virtualViewId == mA11yFocusedId) {
                    return false;
                }
                mA11yFocusedId = virtualViewId;
                a11ySendEvent(p, AccessibilityEvent.obtain(
                        AccessibilityEvent.TYPE_VIEW_ACCESSIBILITY_FOCUSED));
                goNativeActivity.accessibilityFocusChanged(virtualViewId, true);
                return true;
            case AccessibilityNodeInfo.ACTION_CLEAR_ACCESSIBILITY_FOCUS:
                if (virtualViewId != mA11yFocusedId) {
                    return false;
                }
                mA11yFocusedId = A11Y_NO_NODE;
                a11ySendEvent(p, AccessibilityEvent.obtain(
                        AccessibilityEvent.TYPE_VIEW_ACCESSIBILITY_FOCUS_CLEARED));
                goNativeActivity.accessibilityFocusChanged(virtualViewId, false);
                return true;
        }
        return a11yPerformNodeAction(p, action, args);
    }

    // Dispatches one node action — the former per-View delegate's
    // performAccessibilityAction. Dispatched by TalkBack only, never by real
    // touches, so handling these does not affect touch pass-through.
    private static boolean a11yPerformNodeAction(A11yProxy p, int action, Bundle args) {
        final int nodeId = p.nodeId, flags = p.flags;
        // Custom accessibility actions (#1025): map the Android
        // action id back to its Go registry id and dispatch it
        // through the standard action up-call as a custom kind.
        if (p.customIds != null && action >= A11Y_CUSTOM_ACTION_BASE) {
            for (int id : p.customIds) {
                if (action == A11Y_CUSTOM_ACTION_BASE + id) {
                    goNativeActivity.accessibilityAction(
                            nodeId, A11Y_ACTION_CUSTOM + id);
                    return true;
                }
            }
        }
        // Bring a (row, col) cell of a virtualized grid into view
        // (#817). Not a switch case: the action id is not a
        // compile-time constant.
        if (Build.VERSION.SDK_INT >= 23 && p.role == A11Y_ROLE_GRID
                && action == AccessibilityNodeInfo.AccessibilityAction
                        .ACTION_SCROLL_TO_POSITION.getId()) {
            int row = args != null
                    ? args.getInt(AccessibilityNodeInfo.ACTION_ARGUMENT_ROW_INT, -1) : -1;
            int col = args != null
                    ? args.getInt(AccessibilityNodeInfo.ACTION_ARGUMENT_COLUMN_INT, 0) : 0;
            if (row >= 0) {
                goNativeActivity.accessibilityActionScrollTo(nodeId, row, col);
                return true;
            }
            return false;
        }
        // Editable-text actions (#974 option D) are handled fully
        // here, against the published document (never a default View
        // traversal over the contentDescription). A false return
        // (document edge, empty selection, read-only) tells TalkBack
        // the action did not apply.
        if (a11yHasTextDoc(p)) {
            switch (action) {
                case AccessibilityNodeInfo.ACTION_SET_SELECTION:
                    return a11ySetSelection(p, args);
                case AccessibilityNodeInfo.ACTION_NEXT_AT_MOVEMENT_GRANULARITY:
                    return a11yTraverse(p, args, true);
                case AccessibilityNodeInfo.ACTION_PREVIOUS_AT_MOVEMENT_GRANULARITY:
                    return a11yTraverse(p, args, false);
                case AccessibilityNodeInfo.ACTION_SET_TEXT:
                    return a11ySetText(p, args);
                case AccessibilityNodeInfo.ACTION_COPY:
                    return a11yCopy(p, false);
                case AccessibilityNodeInfo.ACTION_CUT:
                    return a11yCopy(p, true);
                case AccessibilityNodeInfo.ACTION_PASTE:
                    return a11yPaste(p);
            }
        }
        switch (action) {
            case AccessibilityNodeInfo.ACTION_CLICK:
                if ((flags & A11Y_FLAG_PRESSABLE) != 0) {
                    goNativeActivity.accessibilityAction(nodeId, A11Y_ACTION_ACTIVATE);
                    return true;
                }
                break;
            case AccessibilityNodeInfo.ACTION_SCROLL_FORWARD:
                if ((flags & A11Y_FLAG_ADJUSTABLE) != 0) {
                    goNativeActivity.accessibilityAction(nodeId, A11Y_ACTION_INCREMENT);
                    return true;
                }
                if ((flags & A11Y_FLAG_SCROLLABLE) != 0) {
                    goNativeActivity.accessibilityAction(nodeId, A11Y_ACTION_SCROLL_FORWARD);
                    a11ySendScrolled(p);
                    return true;
                }
                break;
            case AccessibilityNodeInfo.ACTION_SCROLL_BACKWARD:
                if ((flags & A11Y_FLAG_ADJUSTABLE) != 0) {
                    goNativeActivity.accessibilityAction(nodeId, A11Y_ACTION_DECREMENT);
                    return true;
                }
                if ((flags & A11Y_FLAG_SCROLLABLE) != 0) {
                    goNativeActivity.accessibilityAction(nodeId, A11Y_ACTION_SCROLL_BACKWARD);
                    a11ySendScrolled(p);
                    return true;
                }
                break;
            case AccessibilityNodeInfo.ACTION_LONG_CLICK:
                if ((flags & A11Y_FLAG_LONGPRESSABLE) != 0) {
                    goNativeActivity.accessibilityAction(nodeId, A11Y_ACTION_LONG_PRESS);
                    return true;
                }
                break;
            case AccessibilityNodeInfo.ACTION_DISMISS:
                if ((flags & A11Y_FLAG_DISMISSABLE) != 0) {
                    goNativeActivity.accessibilityAction(nodeId, A11Y_ACTION_DISMISS);
                    return true;
                }
                break;
        }
        // Unhandled action: tell TalkBack it did not apply (there
        // is no real View to fall back to).
        return false;
    }

    // Report a performed AT scroll: TalkBack listens for TYPE_VIEW_SCROLLED
    // from its scroll target to consider the action delivered and to move
    // focus onto newly revealed content. The widget scroll runs queued on
    // the Go main goroutine, so this is sent optimistically, like the
    // editable-text actions; the next commit publishes the moved tree.
    private static void a11ySendScrolled(A11yProxy p) {
        a11ySendEvent(p, AccessibilityEvent.obtain(AccessibilityEvent.TYPE_VIEW_SCROLLED));
    }

    private static AccessibilityNodeInfo a11yFindFocus(A11yHostView host, int focusType) {
        if (focusType == AccessibilityNodeInfo.FOCUS_ACCESSIBILITY
                && mA11yFocusedId != A11Y_NO_NODE) {
            return a11yCreateNodeInfo(host, mA11yFocusedId);
        }
        if (focusType == AccessibilityNodeInfo.FOCUS_INPUT) {
            // The node mirroring the canvas keyboard focus.
            for (A11yProxy p : mA11yProxies.values()) {
                if ((p.flags & A11Y_FLAG_FOCUSED) != 0) {
                    return a11yCreateNodeInfo(host, p.nodeId);
                }
            }
        }
        return null;
    }

    // The virtual node under a container-relative point, for touch
    // exploration. Later wire entries stack on top (overlays are appended
    // last), so the walk is back-to-front; a grid container only matches
    // where none of its cells do.
    private static int a11yNodeAt(float x, float y) {
        java.util.ArrayList<Integer> roots = mA11yChildOrder.get(A11Y_NO_NODE);
        if (roots == null) {
            return A11Y_NO_NODE;
        }
        int gridHit = A11Y_NO_NODE;
        for (int i = roots.size() - 1; i >= 0; i--) {
            int id = roots.get(i).intValue();
            A11yProxy p = mA11yProxies.get(id);
            if (p == null || !a11yContains(p, x, y)) {
                continue;
            }
            if (p.role != A11Y_ROLE_GRID) {
                return id;
            }
            java.util.ArrayList<Integer> cells = mA11yChildOrder.get(id);
            if (cells != null) {
                for (int j = cells.size() - 1; j >= 0; j--) {
                    int cid = cells.get(j).intValue();
                    A11yProxy cp = mA11yProxies.get(cid);
                    if (cp != null && a11yContains(cp, x, y)) {
                        return cid;
                    }
                }
            }
            if (gridHit == A11Y_NO_NODE) {
                gridHit = id;
            }
        }
        return gridHit;
    }

    private static boolean a11yContains(A11yProxy p, float x, float y) {
        return x >= p.x && x < p.x + p.w && y >= p.y && y < p.y + p.h;
    }

    // Sends the hover enter/exit pair for a change of the explored node —
    // TalkBack moves its accessibility focus on TYPE_VIEW_HOVER_ENTER.
    private static void a11ySetHovered(int nodeId) {
        if (nodeId == mA11yHoveredId) {
            return;
        }
        int prev = mA11yHoveredId;
        mA11yHoveredId = nodeId;
        if (nodeId != A11Y_NO_NODE) {
            a11ySendEventFor(nodeId, AccessibilityEvent.obtain(
                    AccessibilityEvent.TYPE_VIEW_HOVER_ENTER));
        }
        if (prev != A11Y_NO_NODE) {
            a11ySendEventFor(prev, AccessibilityEvent.obtain(
                    AccessibilityEvent.TYPE_VIEW_HOVER_EXIT));
        }
    }

    // The single full-screen, touch-transparent host of the virtual
    // accessibility tree. It never has child Views; its provider publishes
    // the semantic elements, and framework hover events (touch exploration)
    // are mapped onto them. Real (non-hover) touches never arrive here — the
    // GL surface consumes them through the native input queue.
    private static final class A11yHostView extends FrameLayout {
        private final AccessibilityNodeProvider mProvider = new AccessibilityNodeProvider() {
            @Override
            public AccessibilityNodeInfo createAccessibilityNodeInfo(int virtualViewId) {
                return a11yCreateNodeInfo(A11yHostView.this, virtualViewId);
            }

            @Override
            public AccessibilityNodeInfo findFocus(int focusType) {
                return a11yFindFocus(A11yHostView.this, focusType);
            }

            @Override
            public boolean performAction(int virtualViewId, int action, Bundle args) {
                return a11yPerformAction(A11yHostView.this, virtualViewId, action, args);
            }

            @Override
            public void addExtraDataToAccessibilityNodeInfo(int virtualViewId,
                    AccessibilityNodeInfo info, String extraDataKey, Bundle arguments) {
                a11yAddExtraData(virtualViewId, info, extraDataKey, arguments);
            }
        };

        A11yHostView(Context context) {
            super(context);
        }

        @Override
        public AccessibilityNodeProvider getAccessibilityNodeProvider() {
            return mProvider;
        }

        // Touch exploration: the framework turns the explore finger into
        // hover events; map them onto the virtual node under the pointer and
        // send the enter/exit pair TalkBack moves its focus from (the
        // ExploreByTouchHelper model — with no real child Views nothing else
        // would produce these events).
        @Override
        public boolean dispatchHoverEvent(MotionEvent event) {
            switch (event.getActionMasked()) {
                case MotionEvent.ACTION_HOVER_ENTER:
                case MotionEvent.ACTION_HOVER_MOVE:
                    a11ySetHovered(a11yNodeAt(event.getX(), event.getY()));
                    return true;
                case MotionEvent.ACTION_HOVER_EXIT:
                    a11ySetHovered(A11Y_NO_NODE);
                    return true;
            }
            return super.dispatchHoverEvent(event);
        }
    }

    // ---- Native web view hosting (webview package, #63) ----
    //
    // android.webkit.WebView hosted over the GL surface for
    // widget.NativeViewHost. The Java half necessarily lives here, in the dex
    // every app ships, because a WebView needs real Java subclasses
    // (WebViewClient, WebChromeClient) and a @JavascriptInterface-annotated
    // object — none of which can be created from JNI. The Go half stays an
    // optional import, so an app that embeds no web content links no Go code
    // for it; these methods simply go unused, and the native up-calls they
    // would make are never bound.
    //
    // Threading: every entry point below is called from a JNI thread and posts
    // to the UI thread with runOnUiThread, which preserves order — so a create
    // posted before a load runs before it, and no operation needs to block.
    // Queries that must answer synchronously (canGoBack, hasFocus) read a
    // shadow copy kept up to date on the UI thread instead of round-tripping,
    // which is what keeps them off the deadlock path (#973).

    private static final java.util.HashMap<Integer, BelluiWebViewHost> mWebViews =
            new java.util.HashMap<Integer, BelluiWebViewHost>();
    private static int mWebViewNextId = 1;

    // Load states, matching webview.LoadState in navigation.go.
    private static final int WEBVIEW_LOAD_LOADING = 1;
    private static final int WEBVIEW_LOAD_COMPLETE = 2;
    private static final int WEBVIEW_LOAD_FAILED = 3;

    // Navigation kinds, matching webview.NavigationKind in navigation.go.
    private static final int WEBVIEW_NAV_OTHER = 0;
    private static final int WEBVIEW_NAV_LINK = 1;

    // Operations, matching webview.navOp in platform.go.
    private static final int WEBVIEW_OP_CAN_GO_BACK = 0;
    private static final int WEBVIEW_OP_GO_BACK = 1;
    private static final int WEBVIEW_OP_CAN_GO_FORWARD = 2;
    private static final int WEBVIEW_OP_GO_FORWARD = 3;
    private static final int WEBVIEW_OP_RELOAD = 4;
    private static final int WEBVIEW_OP_STOP_LOADING = 5;

    private static native void webviewMessage(long token, String origin, String body);

    private static native void webviewEvalDone(long token, long callID, String result, String err);

    private static native void webviewLoad(long token, int state, String url, String err);

    // webviewNavigate is answered synchronously because WebViewClient is: the
    // engine is blocked on the decision. The Go side hops to the main
    // goroutine and waits, so OnNavigate must return promptly — which is
    // already its documented contract.
    private static native boolean webviewNavigate(long token, String url, int kind,
            boolean mainFrame, boolean newWindow);

    // BelluiWebViewHost is the hole punch: a clipping FrameLayout carrying the
    // host's clip rectangle, with the WebView inside it carrying the host's own
    // rectangle. Without the clipping parent a web view inside a
    // container.Scroll would overflow the scroll's edge.
    private static final class BelluiWebViewHost {
        final long token;
        FrameLayout clip;
        WebView web;
        // The clip is hosted in its own panel WINDOW, not in the activity's
        // view hierarchy: a NativeActivity takes the window surface for GL,
        // so decor-view children are laid out and receive input but are never
        // drawn (#76 measured a WebView that answered taps and ran scripts
        // while completely invisible). A TYPE_APPLICATION_PANEL sub-window
        // has its own surface above the activity's; FLAG_NOT_TOUCH_MODAL
        // lets touches outside it fall through to bellui.
        android.view.WindowManager.LayoutParams lp;

        // Shadow state, written on the UI thread and read from JNI threads, so
        // a synchronous query never has to block on the UI thread.
        volatile boolean canGoBack;
        volatile boolean canGoForward;
        volatile boolean hasFocus;

        BelluiWebViewHost(long token) {
            this.token = token;
        }

        void syncHistory() {
            if (web != null) {
                canGoBack = web.canGoBack();
                canGoForward = web.canGoForward();
            }
        }
    }

    static int webviewCreate(final long token, final String bridgeScript) {
        final int id;
        synchronized (mWebViews) {
            id = mWebViewNextId++;
        }
        final GoNativeActivity activity = goNativeActivity;
        if (activity == null) {
            return 0;
        }
        activity.runOnUiThread(new Runnable() {
            @Override
            public void run() {
                activity.doWebviewCreate(id, token, bridgeScript);
            }
        });
        return id;
    }

    void doWebviewCreate(int id, long token, String bridgeScript) {
        final BelluiWebViewHost host = new BelluiWebViewHost(token);

        host.clip = new FrameLayout(this);
        host.clip.setClipChildren(true);
        host.clip.setClipToPadding(true);
        host.clip.setVisibility(View.GONE); // shown once the host reports geometry

        final WebView web = new WebView(this);
        host.web = web;
        web.getSettings().setJavaScriptEnabled(true);
        web.getSettings().setDomStorageEnabled(true);

        // The bridge is installed here or not at all: with no script the page
        // has neither window.bellui nor an object to post to, which is the
        // guarantee behind webview.WebView.EnableBridge being opt-in.
        final String shim = bridgeScript;
        if (shim != null && shim.length() > 0) {
            web.addJavascriptInterface(new BelluiWebViewBridge(token), "belluiHost");
        }

        web.setWebViewClient(new WebViewClient() {
            @Override
            public boolean shouldOverrideUrlLoading(WebView view, WebResourceRequest request) {
                int kind = request.isRedirect() ? WEBVIEW_NAV_OTHER : WEBVIEW_NAV_LINK;
                boolean allow = webviewNavigate(token, request.getUrl().toString(), kind,
                        request.isForMainFrame(), false);
                return !allow; // true means "handled by the app", i.e. cancelled
            }

            @Override
            public void onPageStarted(WebView view, String url, android.graphics.Bitmap favicon) {
                // Android offers no document-start user script, so the shim is
                // injected as early as a WebView allows. A page's own first
                // lines can therefore run before window.bellui exists — the
                // documented narrow race, also noted on the Go side.
                if (shim != null && shim.length() > 0) {
                    view.evaluateJavascript(shim, null);
                }
                webviewLoad(token, WEBVIEW_LOAD_LOADING, url, "");
            }

            @Override
            public void onPageFinished(WebView view, String url) {
                if (shim != null && shim.length() > 0) {
                    view.evaluateJavascript(shim, null); // idempotent; covers a missed start
                }
                BelluiWebViewHost h = webviewHost(mWebViewIdOf(token));
                if (h != null) {
                    h.syncHistory();
                }
                webviewLoad(token, WEBVIEW_LOAD_COMPLETE, url, "");
            }

            @Override
            public void onReceivedError(WebView view, WebResourceRequest request,
                    WebResourceError error) {
                if (!request.isForMainFrame()) {
                    return; // sub-resource failures are not the view's load state
                }
                webviewLoad(token, WEBVIEW_LOAD_FAILED, request.getUrl().toString(),
                        String.valueOf(error.getDescription()));
            }
        });

        web.setOnFocusChangeListener(new View.OnFocusChangeListener() {
            @Override
            public void onFocusChange(View v, boolean focused) {
                host.hasFocus = focused;
            }
        });

        host.clip.addView(web, new FrameLayout.LayoutParams(
                FrameLayout.LayoutParams.WRAP_CONTENT, FrameLayout.LayoutParams.WRAP_CONTENT));

        // Touching the web area is what asks for the keyboard: the panel
        // window starts NOT_FOCUSABLE so bellui keeps receiving keys, and a
        // touch inside flips it focusable before the WebView handles the
        // event — the overlay dance every view-over-NativeActivity does.
        host.clip.setOnTouchListener(new View.OnTouchListener() {
            @Override
            public boolean onTouch(View v, android.view.MotionEvent ev) {
                if (ev.getAction() == android.view.MotionEvent.ACTION_DOWN) {
                    webviewPanelFocusable(host, true);
                    // The same tap that flips the window focusable is the one
                    // the WebView uses to ask for the IME — and that request
                    // races the flag taking effect, so it is usually dropped
                    // (#76: "focus there does not bring the keyboard"). Nudge
                    // the IME once window focus has actually landed.
                    host.clip.postDelayed(new Runnable() {
                        @Override
                        public void run() { webviewNudgeIME(host); }
                    }, 150);
                    host.clip.postDelayed(new Runnable() {
                        @Override
                        public void run() { webviewNudgeIME(host); }
                    }, 500);
                }
                return false; // the WebView still handles the touch itself
            }
        });

        // Hosted in a panel sub-window of the activity (own surface, drawn
        // above the GL frame) — see the comment on BelluiWebViewHost.lp for
        // why the decor view cannot host a visible view here.
        host.lp = new android.view.WindowManager.LayoutParams(
                0, 0,
                android.view.WindowManager.LayoutParams.TYPE_APPLICATION_PANEL,
                android.view.WindowManager.LayoutParams.FLAG_NOT_FOCUSABLE
                        | android.view.WindowManager.LayoutParams.FLAG_NOT_TOUCH_MODAL
                        | android.view.WindowManager.LayoutParams.FLAG_LAYOUT_NO_LIMITS,
                android.graphics.PixelFormat.TRANSLUCENT);
        host.lp.gravity = Gravity.TOP | Gravity.LEFT;
        // The panel must not dodge the IME on its own: when the keyboard
        // opens for the web view's editor, the window manager would pan the
        // whole panel up while bellui's per-frame geometry sync keeps putting
        // it back — a visible tug-of-war (#76: "something keeps repositioning
        // the webview"). bellui owns this window's position, exclusively;
        // keyboard avoidance is the layout's job like for any other widget.
        host.lp.softInputMode =
                android.view.WindowManager.LayoutParams.SOFT_INPUT_ADJUST_NOTHING;
        host.clip.setVisibility(View.GONE); // shown once real geometry arrives
        // NOT added here: the first attach can run before the activity's
        // window token exists (BadTokenException — the same startup race the
        // iOS backend had). webviewEnsureHosted adopts the panel as soon as
        // the token appears, retried from the per-frame focus poll.
        webviewEnsureHosted(host);

        synchronized (mWebViews) {
            mWebViews.put(Integer.valueOf(id), host);
        }
    }

    // webviewEnsureHosted adds the panel window once the activity's window
    // token exists. UI thread only; safe to call repeatedly.
    static void webviewEnsureHosted(BelluiWebViewHost host) {
        GoNativeActivity activity = goNativeActivity;
        if (activity == null || host == null || host.clip == null || host.lp == null
                || host.clip.getParent() != null) {
            return;
        }
        android.os.IBinder token = activity.getWindow().getDecorView().getWindowToken();
        if (token == null) {
            return; // window not up yet; the focus poll retries
        }
        host.lp.token = token;
        activity.getWindowManager().addView(host.clip, host.lp);
    }

    // webviewNudgeIME asks the IME to show for the web view's focused editor,
    // used after the panel window turned focusable — the editor's own request
    // raced the flag and was dropped. A view with no editable focus makes
    // this a no-op.
    static void webviewNudgeIME(BelluiWebViewHost host) {
        GoNativeActivity activity = goNativeActivity;
        if (activity == null || host == null || host.web == null || !host.web.hasFocus()) {
            return;
        }
        android.view.inputmethod.InputMethodManager imm =
                (android.view.inputmethod.InputMethodManager)
                        activity.getSystemService(INPUT_METHOD_SERVICE);
        if (imm != null) {
            imm.showSoftInput(host.web, 0);
        }
    }

    // webviewAnyPanelFocused reports whether any webview panel window is
    // currently focusable — i.e. holds (or is about to hold) the keyboard.
    static boolean webviewAnyPanelFocused() {
        synchronized (mWebViews) {
            for (BelluiWebViewHost h : mWebViews.values()) {
                if (h.lp != null && (h.lp.flags
                        & android.view.WindowManager.LayoutParams.FLAG_NOT_FOCUSABLE) == 0) {
                    return true;
                }
            }
        }
        return false;
    }

    // webviewPanelFocusable moves real window focus between the panel and the
    // activity — Chromium's WebView refuses to start its IME machinery
    // without window focus, so no flag trickery substitutes (#76 measured
    // both: ALT_FOCUSABLE_IM summons nothing, and an in-place flag update on
    // this OEM never transfers focus, leaving the keyboard dead unless the
    // activity had it open already). Taking focus therefore RE-ADDS the
    // window with the focusable flags — window addition always re-evaluates
    // focus, and the topmost focusable window wins. Giving it back is the
    // easy direction: turning NOT_FOCUSABLE on drops focus to the activity
    // immediately. UI thread only.
    static void webviewPanelFocusable(BelluiWebViewHost host, boolean focusable) {
        GoNativeActivity activity = goNativeActivity;
        if (activity == null || host == null || host.lp == null || host.clip == null
                || host.clip.getParent() == null) {
            return;
        }
        boolean isFocusable =
                (host.lp.flags & android.view.WindowManager.LayoutParams.FLAG_NOT_FOCUSABLE) == 0;
        if (focusable == isFocusable) {
            return;
        }
        if (focusable) {
            host.lp.flags &= ~android.view.WindowManager.LayoutParams.FLAG_NOT_FOCUSABLE;
            activity.getWindowManager().removeView(host.clip);
            activity.getWindowManager().addView(host.clip, host.lp);
        } else {
            host.lp.flags |= android.view.WindowManager.LayoutParams.FLAG_NOT_FOCUSABLE;
            activity.getWindowManager().updateViewLayout(host.clip, host.lp);
        }
    }

    private static BelluiWebViewHost webviewHost(int id) {
        synchronized (mWebViews) {
            return mWebViews.get(Integer.valueOf(id));
        }
    }

    private static int mWebViewIdOf(long token) {
        synchronized (mWebViews) {
            for (java.util.Map.Entry<Integer, BelluiWebViewHost> e : mWebViews.entrySet()) {
                if (e.getValue().token == token) {
                    return e.getKey().intValue();
                }
            }
        }
        return 0;
    }

    // BelluiWebViewBridge is the page-side transport of the JS bridge. The shim
    // calls belluiHost.post(s); the origin is taken from the WebView's own
    // current URL rather than from anything the message claims, so the Go
    // side's OriginPolicy is deciding about the page that actually sent it.
    private static final class BelluiWebViewBridge {
        private final long token;

        BelluiWebViewBridge(long token) {
            this.token = token;
        }

        @android.webkit.JavascriptInterface
        public void post(String body, String origin) {
            webviewMessage(token, origin == null ? "null" : origin, body);
        }
    }

    static void webviewDestroy(final int id) {
        final GoNativeActivity activity = goNativeActivity;
        if (activity == null) {
            return;
        }
        activity.runOnUiThread(new Runnable() {
            @Override
            public void run() {
                BelluiWebViewHost host;
                synchronized (mWebViews) {
                    host = mWebViews.remove(Integer.valueOf(id));
                }
                if (host == null) {
                    return; // already destroyed
                }
                if (host.web != null) {
                    // Stop the engine and clear its clients before tearing the
                    // view down, so nothing already in flight reports through a
                    // token the Go side has just dropped.
                    host.web.setWebViewClient(new WebViewClient());
                    host.web.stopLoading();
                    host.web.loadUrl("about:blank");
                    host.clip.removeView(host.web);
                    host.web.destroy();
                    host.web = null;
                }
                if (host.clip.getParent() != null) {
                    activity.getWindowManager().removeView(host.clip);
                }
                host.clip = null;
                host.lp = null;
            }
        });
    }

    static void webviewSetGeometry(final int id,
            final int clipX, final int clipY, final int clipW, final int clipH,
            final int viewX, final int viewY, final int viewW, final int viewH) {
        final GoNativeActivity activity = goNativeActivity;
        if (activity == null) {
            return;
        }
        activity.runOnUiThread(new Runnable() {
            @Override
            public void run() {
                BelluiWebViewHost host = webviewHost(id);
                if (host == null || host.clip == null || host.lp == null) {
                    return;
                }
                webviewEnsureHosted(host);
                // The clip IS the panel window, so its rectangle is window
                // geometry rather than layout margins.
                host.lp.x = clipX;
                host.lp.y = clipY;
                host.lp.width = clipW;
                host.lp.height = clipH;
                if (host.clip.getParent() != null) {
                    activity.getWindowManager().updateViewLayout(host.clip, host.lp);
                }

                if (host.web != null) {
                    FrameLayout.LayoutParams webParams =
                            (FrameLayout.LayoutParams) host.web.getLayoutParams();
                    webParams.width = viewW;
                    webParams.height = viewH;
                    // Placed relative to the clip, so whatever falls outside it
                    // is masked by the parent rather than drawn.
                    webParams.leftMargin = viewX - clipX;
                    webParams.topMargin = viewY - clipY;
                    webParams.gravity = Gravity.TOP | Gravity.LEFT;
                    host.web.setLayoutParams(webParams);
                }
            }
        });
    }

    static void webviewSetVisible(final int id, final boolean visible) {
        final GoNativeActivity activity = goNativeActivity;
        if (activity == null) {
            return;
        }
        activity.runOnUiThread(new Runnable() {
            @Override
            public void run() {
                BelluiWebViewHost host = webviewHost(id);
                if (host != null && host.clip != null) {
                    host.clip.setVisibility(visible ? View.VISIBLE : View.GONE);
                }
            }
        });
    }

    static void webviewLoadURL(final int id, final String url) {
        final GoNativeActivity activity = goNativeActivity;
        if (activity == null) {
            return;
        }
        activity.runOnUiThread(new Runnable() {
            @Override
            public void run() {
                BelluiWebViewHost host = webviewHost(id);
                if (host != null && host.web != null) {
                    host.web.loadUrl(url);
                }
            }
        });
    }

    static void webviewLoadHTML(final int id, final String html, final String baseURL) {
        final GoNativeActivity activity = goNativeActivity;
        if (activity == null) {
            return;
        }
        activity.runOnUiThread(new Runnable() {
            @Override
            public void run() {
                BelluiWebViewHost host = webviewHost(id);
                if (host != null && host.web != null) {
                    host.web.loadDataWithBaseURL(
                            baseURL == null || baseURL.length() == 0 ? null : baseURL,
                            html, "text/html", "UTF-8", null);
                }
            }
        });
    }

    static void webviewEval(final int id, final String js, final long callID,
            final boolean wantResult) {
        final GoNativeActivity activity = goNativeActivity;
        if (activity == null) {
            return;
        }
        activity.runOnUiThread(new Runnable() {
            @Override
            public void run() {
                final BelluiWebViewHost host = webviewHost(id);
                if (host == null || host.web == null) {
                    if (wantResult) {
                        webviewEvalDone(0, callID, "", "webview: not attached to a window");
                    }
                    return;
                }
                if (!wantResult) {
                    host.web.evaluateJavascript(js, null);
                    return;
                }
                host.web.evaluateJavascript(js, new android.webkit.ValueCallback<String>() {
                    @Override
                    public void onReceiveValue(String value) {
                        // evaluateJavascript already hands back a JSON encoding
                        // of the value, which is exactly the eval contract; it
                        // has no channel for a thrown exception, so a script
                        // that throws reports null rather than an error.
                        webviewEvalDone(host.token, callID,
                                value == null ? "null" : value, "");
                    }
                });
            }
        });
    }

    // webviewNav answers the queries from shadow state and posts the commands,
    // so nothing here blocks a JNI thread on the UI thread.
    static boolean webviewNav(final int id, final int op) {
        final BelluiWebViewHost host = webviewHost(id);
        if (host == null) {
            return false;
        }
        switch (op) {
        case WEBVIEW_OP_CAN_GO_BACK:
            return host.canGoBack;
        case WEBVIEW_OP_CAN_GO_FORWARD:
            return host.canGoForward;
        case WEBVIEW_OP_GO_BACK:
            if (!host.canGoBack) {
                return false;
            }
            break;
        case WEBVIEW_OP_GO_FORWARD:
            if (!host.canGoForward) {
                return false;
            }
            break;
        default:
            break;
        }

        final GoNativeActivity activity = goNativeActivity;
        if (activity == null) {
            return false;
        }
        activity.runOnUiThread(new Runnable() {
            @Override
            public void run() {
                BelluiWebViewHost h = webviewHost(id);
                if (h == null || h.web == null) {
                    return;
                }
                switch (op) {
                case WEBVIEW_OP_GO_BACK:
                    h.web.goBack();
                    break;
                case WEBVIEW_OP_GO_FORWARD:
                    h.web.goForward();
                    break;
                case WEBVIEW_OP_RELOAD:
                    h.web.reload();
                    break;
                case WEBVIEW_OP_STOP_LOADING:
                    h.web.stopLoading();
                    break;
                }
                h.syncHistory();
            }
        });
        return true;
    }

    static boolean webviewHasFocus(final int id) {
        BelluiWebViewHost host = webviewHost(id);
        if (host == null) {
            return false;
        }
        if (host.clip != null && host.clip.getParent() == null) {
            // Polled every frame by the host's focus bridge, which makes this
            // the heartbeat that retries hosting until the window token
            // exists (see webviewEnsureHosted).
            final GoNativeActivity activity = goNativeActivity;
            if (activity != null) {
                activity.runOnUiThread(new Runnable() {
                    @Override
                    public void run() {
                        webviewEnsureHosted(webviewHost(id));
                    }
                });
            }
        }
        return host.hasFocus;
    }

    static boolean webviewSetFocus(final int id, final boolean take) {
        final GoNativeActivity activity = goNativeActivity;
        if (activity == null) {
            return false;
        }
        final BelluiWebViewHost host = webviewHost(id);
        if (host == null) {
            return false;
        }
        if (!take && !host.hasFocus) {
            return true; // nothing of ours to give back
        }
        activity.runOnUiThread(new Runnable() {
            @Override
            public void run() {
                BelluiWebViewHost h = webviewHost(id);
                if (h == null || h.web == null) {
                    return;
                }
                // The panel window's focusability decides where keys and the
                // IME go; the view focus follows inside it.
                webviewPanelFocusable(h, take);
                if (take) {
                    h.web.setFocusableInTouchMode(true);
                    h.web.requestFocus();
                } else {
                    h.web.clearFocus();
                    // Nothing is focused in its place: bellui's own keyboard
                    // input goes through the shadow EditText, which claims
                    // focus only while an entry is focused.
                }
            }
        });
        return true;
    }
}
