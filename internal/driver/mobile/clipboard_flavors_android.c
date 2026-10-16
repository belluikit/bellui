//go:build android

#include <jni.h>
#include <stdbool.h>
#include <stdlib.h>
#include <string.h>
#include <android/log.h>

// The clipboard-image content-URI plumbing (BelluiFileProvider + ClipData +
// ClipboardManager) needs app classes that FindClass can't reach on a
// JVM-attached native thread, so it lives in the Java GoNativeActivity
// (doSetClipboardImage / doGetClipboardImage). These shims resolve those methods
// from the ctx object's own class and call them (pre-rebrand tracker #580), mirroring shareFile.

void belluiClipboardSetImage(uintptr_t java_vm, uintptr_t jni_env, uintptr_t ctx, char *path) {
	JNIEnv *env = (JNIEnv*)jni_env;
	jstring pathStr = (*env)->NewStringUTF(env, path);

	jclass actClass = (*env)->GetObjectClass(env, (jobject)ctx);
	jmethodID m = (*env)->GetMethodID(env, actClass, "doSetClipboardImage", "(Ljava/lang/String;)V");
	if (m == 0) {
		(*env)->ExceptionClear(env);
		__android_log_print(ANDROID_LOG_ERROR, "Bellui", "doSetClipboardImage not found");
		return;
	}
	(*env)->CallVoidMethod(env, (jobject)ctx, m, pathStr);
}

// belluiClipboardSetFlavors forwards a multi-flavor clipboard write to the Java
// doSetClipboardFlavors (pre-rebrand tracker #1142). NULL C strings travel as null jstrings so
// the Java side can tell an absent flavor from an empty one.
void belluiClipboardSetFlavors(uintptr_t java_vm, uintptr_t jni_env, uintptr_t ctx,
                             char *text, char *html, char *imagePath) {
	JNIEnv *env = (JNIEnv*)jni_env;
	jstring textStr = text == NULL ? NULL : (*env)->NewStringUTF(env, text);
	jstring htmlStr = html == NULL ? NULL : (*env)->NewStringUTF(env, html);
	jstring pathStr = imagePath == NULL ? NULL : (*env)->NewStringUTF(env, imagePath);

	jclass actClass = (*env)->GetObjectClass(env, (jobject)ctx);
	jmethodID m = (*env)->GetMethodID(env, actClass, "doSetClipboardFlavors",
		"(Ljava/lang/String;Ljava/lang/String;Ljava/lang/String;)V");
	if (m == 0) {
		(*env)->ExceptionClear(env);
		__android_log_print(ANDROID_LOG_ERROR, "Bellui", "doSetClipboardFlavors not found");
		return;
	}
	(*env)->CallVoidMethod(env, (jobject)ctx, m, textStr, htmlStr, pathStr);
}

// belluiClipboardGetImage returns a malloc'd path to a PNG temp file the Java side
// wrote from the clipboard's image URI (NULL when the clipboard holds no image);
// the Go caller reads and frees it.
char *belluiClipboardGetImage(uintptr_t java_vm, uintptr_t jni_env, uintptr_t ctx) {
	JNIEnv *env = (JNIEnv*)jni_env;

	jclass actClass = (*env)->GetObjectClass(env, (jobject)ctx);
	jmethodID m = (*env)->GetMethodID(env, actClass, "doGetClipboardImage", "()Ljava/lang/String;");
	if (m == 0) {
		(*env)->ExceptionClear(env);
		return NULL;
	}
	jstring res = (jstring)(*env)->CallObjectMethod(env, (jobject)ctx, m);
	if (res == NULL) {
		return NULL;
	}
	const char *cstr = (*env)->GetStringUTFChars(env, res, NULL);
	char *out = strdup(cstr);
	(*env)->ReleaseStringUTFChars(env, res, cstr);
	return out;
}
