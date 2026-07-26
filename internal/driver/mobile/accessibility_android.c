//go:build accessibility && android

// JNI down-calls for the TalkBack bridge (#488): thin shims from the Go side
// (via app.RunOnJVM) to the static accessibility methods on GoNativeActivity.
// Method IDs are cached on first use; a missing method (an APK built with an
// older embedded dex) degrades to a warning instead of crashing.

#include <android/log.h>
#include <jni.h>
#include <stdint.h>
#include <stdlib.h>

#define LOG_WARN(...) __android_log_print(ANDROID_LOG_WARN, "Bellui", __VA_ARGS__)

static jclass a11y_activity_class = NULL;
static jmethodID a11y_enabled_method = 0;
static jmethodID a11y_setup_method = 0;
static jmethodID a11y_clear_method = 0;
static jmethodID a11y_add_method = 0;
static jmethodID a11y_addv2_method = 0;
static jmethodID a11y_addv3_method = 0;
static jmethodID a11y_addv4_method = 0;
static jmethodID a11y_addv5_method = 0;
static jmethodID a11y_commit_method = 0;
static jmethodID a11y_announce_method = 0;

// Populate the method-ID cache. ctx is the GoNativeActivity jobject, so
// GetObjectClass resolves against the application class loader.
static void belluiA11yEnsureCached(JNIEnv *env, jobject ctx) {
	if (a11y_activity_class != NULL) {
		return;
	}

	jclass clazz = (*env)->GetObjectClass(env, ctx);
	if (clazz == NULL) {
		LOG_WARN("accessibility: cannot get activity class");
		return;
	}
	a11y_activity_class = (jclass)(*env)->NewGlobalRef(env, clazz);

	struct {
		jmethodID *id;
		const char *name;
		const char *sig;
	} methods[] = {
		{&a11y_enabled_method, "isAccessibilityEnabled", "()Z"},
		{&a11y_setup_method, "setupAccessibility", "()V"},
		{&a11y_clear_method, "clearAccessibilityNodes", "()V"},
		{&a11y_add_method, "addAccessibilityNode",
			"(IIILjava/lang/String;Ljava/lang/String;Ljava/lang/String;IIII)V"},
		// V2 adds the virtualized-grid association (#817): parent grid node id,
		// cell display (row, col) and — for the grid itself — the collection
		// extent. Missing on an APK built with an older embedded dex; the add
		// call falls back to the original method then.
		{&a11y_addv2_method, "addAccessibilityNodeV2",
			"(IIILjava/lang/String;Ljava/lang/String;Ljava/lang/String;IIIIIIIII)V"},
		// V3 adds the editable-text document (#974 option D): the live text, and
		// the half-open selection as rune offsets (-1/-1 when the node carries no
		// document). Missing on an older embedded dex; falls back to V2 then V1.
		{&a11y_addv3_method, "addAccessibilityNodeV3",
			"(IIILjava/lang/String;Ljava/lang/String;Ljava/lang/String;IIIIIIIIILjava/lang/String;II)V"},
		// V4 adds the custom accessibility actions (#1025): the encoded
		// (registry id, label) list a node offers beyond the standard
		// semantics (undo/redo, ...). Missing on an older embedded dex; falls
		// back to V3, V2 then V1.
		{&a11y_addv4_method, "addAccessibilityNodeV4",
			"(IIILjava/lang/String;Ljava/lang/String;Ljava/lang/String;IIIIIIIIILjava/lang/String;IILjava/lang/String;)V"},
		// V5 adds the focused editor's visual-layout sidecar (#995): the
		// encoded line-boundary window + per-character rectangles behind
		// legacy-TalkBack LINE granularity and the character-location extra.
		// Missing on an older embedded dex; falls back to V4..V1.
		{&a11y_addv5_method, "addAccessibilityNodeV5",
			"(IIILjava/lang/String;Ljava/lang/String;Ljava/lang/String;IIIIIIIIILjava/lang/String;IILjava/lang/String;Ljava/lang/String;)V"},
		{&a11y_commit_method, "commitAccessibilityNodes", "()V"},
		{&a11y_announce_method, "announceAccessibility", "(Ljava/lang/String;Z)V"},
	};
	for (size_t i = 0; i < sizeof(methods) / sizeof(methods[0]); i++) {
		*methods[i].id = (*env)->GetStaticMethodID(env, a11y_activity_class,
			methods[i].name, methods[i].sig);
		if (*methods[i].id == 0) {
			LOG_WARN("accessibility: %s not found", methods[i].name);
			(*env)->ExceptionClear(env);
		}
	}
}

static void belluiA11yClearException(JNIEnv *env) {
	if ((*env)->ExceptionOccurred(env)) {
		(*env)->ExceptionClear(env);
	}
}

int belluiA11yEnabled(uintptr_t jni_env, uintptr_t ctx) {
	JNIEnv *env = (JNIEnv *)jni_env;
	belluiA11yEnsureCached(env, (jobject)ctx);
	if (a11y_enabled_method == 0) {
		return 0;
	}
	jboolean on = (*env)->CallStaticBooleanMethod(env, a11y_activity_class, a11y_enabled_method);
	belluiA11yClearException(env);
	return on ? 1 : 0;
}

void belluiA11ySetup(uintptr_t jni_env, uintptr_t ctx) {
	JNIEnv *env = (JNIEnv *)jni_env;
	belluiA11yEnsureCached(env, (jobject)ctx);
	if (a11y_setup_method == 0) {
		return;
	}
	(*env)->CallStaticVoidMethod(env, a11y_activity_class, a11y_setup_method);
	belluiA11yClearException(env);
}

void belluiA11yClearNodes(uintptr_t jni_env, uintptr_t ctx) {
	JNIEnv *env = (JNIEnv *)jni_env;
	belluiA11yEnsureCached(env, (jobject)ctx);
	if (a11y_clear_method == 0) {
		return;
	}
	(*env)->CallStaticVoidMethod(env, a11y_activity_class, a11y_clear_method);
	belluiA11yClearException(env);
}

void belluiA11yAddNode(uintptr_t jni_env, uintptr_t ctx, int id, int role, int flags,
	const char *label, const char *value, const char *hint,
	int x, int y, int width, int height,
	int parent, int row, int col, int rowCount, int colCount,
	const char *text, int selStart, int selEnd,
	const char *customActs, const char *textLayout) {
	JNIEnv *env = (JNIEnv *)jni_env;
	belluiA11yEnsureCached(env, (jobject)ctx);
	if (a11y_add_method == 0 && a11y_addv2_method == 0 && a11y_addv3_method == 0
			&& a11y_addv4_method == 0 && a11y_addv5_method == 0) {
		return;
	}
	jstring jlabel = (*env)->NewStringUTF(env, label ? label : "");
	jstring jvalue = (*env)->NewStringUTF(env, value ? value : "");
	jstring jhint = (*env)->NewStringUTF(env, hint ? hint : "");
	if (a11y_addv5_method != 0) {
		jstring jtext = (*env)->NewStringUTF(env, text ? text : "");
		jstring jacts = (*env)->NewStringUTF(env, customActs ? customActs : "");
		jstring jlayout = (*env)->NewStringUTF(env, textLayout ? textLayout : "");
		(*env)->CallStaticVoidMethod(env, a11y_activity_class, a11y_addv5_method,
			(jint)id, (jint)role, (jint)flags, jlabel, jvalue, jhint,
			(jint)x, (jint)y, (jint)width, (jint)height,
			(jint)parent, (jint)row, (jint)col, (jint)rowCount, (jint)colCount,
			jtext, (jint)selStart, (jint)selEnd, jacts, jlayout);
		(*env)->DeleteLocalRef(env, jtext);
		(*env)->DeleteLocalRef(env, jacts);
		(*env)->DeleteLocalRef(env, jlayout);
	} else if (a11y_addv4_method != 0) {
		jstring jtext = (*env)->NewStringUTF(env, text ? text : "");
		jstring jacts = (*env)->NewStringUTF(env, customActs ? customActs : "");
		(*env)->CallStaticVoidMethod(env, a11y_activity_class, a11y_addv4_method,
			(jint)id, (jint)role, (jint)flags, jlabel, jvalue, jhint,
			(jint)x, (jint)y, (jint)width, (jint)height,
			(jint)parent, (jint)row, (jint)col, (jint)rowCount, (jint)colCount,
			jtext, (jint)selStart, (jint)selEnd, jacts);
		(*env)->DeleteLocalRef(env, jtext);
		(*env)->DeleteLocalRef(env, jacts);
	} else if (a11y_addv3_method != 0) {
		// Older embedded dex: deliver the node without its custom actions.
		jstring jtext = (*env)->NewStringUTF(env, text ? text : "");
		(*env)->CallStaticVoidMethod(env, a11y_activity_class, a11y_addv3_method,
			(jint)id, (jint)role, (jint)flags, jlabel, jvalue, jhint,
			(jint)x, (jint)y, (jint)width, (jint)height,
			(jint)parent, (jint)row, (jint)col, (jint)rowCount, (jint)colCount,
			jtext, (jint)selStart, (jint)selEnd);
		(*env)->DeleteLocalRef(env, jtext);
	} else if (a11y_addv2_method != 0) {
		// Older embedded dex: deliver the node without its editable document.
		(*env)->CallStaticVoidMethod(env, a11y_activity_class, a11y_addv2_method,
			(jint)id, (jint)role, (jint)flags, jlabel, jvalue, jhint,
			(jint)x, (jint)y, (jint)width, (jint)height,
			(jint)parent, (jint)row, (jint)col, (jint)rowCount, (jint)colCount);
	} else {
		// Even older: no grid association either.
		(*env)->CallStaticVoidMethod(env, a11y_activity_class, a11y_add_method,
			(jint)id, (jint)role, (jint)flags, jlabel, jvalue, jhint,
			(jint)x, (jint)y, (jint)width, (jint)height);
	}
	(*env)->DeleteLocalRef(env, jlabel);
	(*env)->DeleteLocalRef(env, jvalue);
	(*env)->DeleteLocalRef(env, jhint);
	belluiA11yClearException(env);
}

void belluiA11yCommitNodes(uintptr_t jni_env, uintptr_t ctx) {
	JNIEnv *env = (JNIEnv *)jni_env;
	belluiA11yEnsureCached(env, (jobject)ctx);
	if (a11y_commit_method == 0) {
		return;
	}
	(*env)->CallStaticVoidMethod(env, a11y_activity_class, a11y_commit_method);
	belluiA11yClearException(env);
}

void belluiA11yAnnounce(uintptr_t jni_env, uintptr_t ctx, const char *text, int assertive) {
	JNIEnv *env = (JNIEnv *)jni_env;
	belluiA11yEnsureCached(env, (jobject)ctx);
	if (a11y_announce_method == 0) {
		return;
	}
	jstring jtext = (*env)->NewStringUTF(env, text ? text : "");
	(*env)->CallStaticVoidMethod(env, a11y_activity_class, a11y_announce_method,
		jtext, (jboolean)(assertive != 0));
	(*env)->DeleteLocalRef(env, jtext);
	belluiA11yClearException(env);
}
