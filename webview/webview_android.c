//go:build android

// JNI bridge for widget.NativeViewHost on Android (#63).
//
// The Java half lives in GoNativeActivity.java, because a WebView needs real
// Java subclasses and a @JavascriptInterface object, none of which can be
// created from JNI. This file is only the two-way plumbing: down-calls invoke
// the static methods there, up-calls implement the native methods it declares.
//
// The native methods are bound by JNI's name-based resolution across the whole
// process image, so defining them here — in the optional webview package rather
// than in the driver — works, and an app that never imports this package simply
// leaves GoNativeActivity's native declarations unbound. They are only ever
// called from code paths that a created web view reaches, so nothing calls an
// unbound one.

#include <jni.h>
#include <stdint.h>
#include <stdbool.h>
#include <stdlib.h>
#include <string.h>

#include "_cgo_export.h"

// The activity class and its webview entry points, cached once at attach.
// Method IDs stay valid for the life of the class, and the class reference is a
// global one so it survives the JNI frame it was looked up in.
static jclass webviewClass;
static jmethodID webviewCreateMethod;
static jmethodID webviewDestroyMethod;
static jmethodID webviewSetGeometryMethod;
static jmethodID webviewSetVisibleMethod;
static jmethodID webviewLoadURLMethod;
static jmethodID webviewLoadHTMLMethod;
static jmethodID webviewEvalMethod;
static jmethodID webviewNavMethod;
static jmethodID webviewHasFocusMethod;
static jmethodID webviewSetFocusMethod;

// belluiWebViewInit caches the activity class and its methods. ctx is the
// activity object handed out by RunNative, so its class is looked up directly
// rather than through FindClass — which on a JNI thread would search the wrong
// class loader.
//
// It returns false when the class does not carry the webview methods, which is
// what an app built against an older embedded dex looks like.
bool belluiWebViewInit(uintptr_t jniEnv, uintptr_t ctx) {
    JNIEnv *env = (JNIEnv *)jniEnv;
    if (webviewClass != NULL) {
        return true;
    }

    jclass local = (*env)->GetObjectClass(env, (jobject)ctx);
    if (local == NULL) {
        (*env)->ExceptionClear(env);
        return false;
    }

    jmethodID create = (*env)->GetStaticMethodID(env, local, "webviewCreate",
                                                 "(JLjava/lang/String;)I");
    if (create == NULL) {
        (*env)->ExceptionClear(env);
        (*env)->DeleteLocalRef(env, local);
        return false;
    }

    webviewClass = (jclass)(*env)->NewGlobalRef(env, local);
    (*env)->DeleteLocalRef(env, local);

    webviewCreateMethod = create;
    webviewDestroyMethod = (*env)->GetStaticMethodID(env, webviewClass, "webviewDestroy", "(I)V");
    webviewSetGeometryMethod =
        (*env)->GetStaticMethodID(env, webviewClass, "webviewSetGeometry", "(IIIIIIIII)V");
    webviewSetVisibleMethod =
        (*env)->GetStaticMethodID(env, webviewClass, "webviewSetVisible", "(IZ)V");
    webviewLoadURLMethod =
        (*env)->GetStaticMethodID(env, webviewClass, "webviewLoadURL", "(ILjava/lang/String;)V");
    webviewLoadHTMLMethod = (*env)->GetStaticMethodID(
        env, webviewClass, "webviewLoadHTML", "(ILjava/lang/String;Ljava/lang/String;)V");
    webviewEvalMethod =
        (*env)->GetStaticMethodID(env, webviewClass, "webviewEval", "(ILjava/lang/String;JZ)V");
    webviewNavMethod = (*env)->GetStaticMethodID(env, webviewClass, "webviewNav", "(II)Z");
    webviewHasFocusMethod =
        (*env)->GetStaticMethodID(env, webviewClass, "webviewHasFocus", "(I)Z");
    webviewSetFocusMethod =
        (*env)->GetStaticMethodID(env, webviewClass, "webviewSetFocus", "(IZ)Z");
    (*env)->ExceptionClear(env);
    return true;
}

int belluiWebViewCreate(uintptr_t jniEnv, uint64_t token, const char *script) {
    JNIEnv *env = (JNIEnv *)jniEnv;
    if (webviewClass == NULL || webviewCreateMethod == NULL) {
        return 0;
    }
    jstring jscript = (*env)->NewStringUTF(env, script ? script : "");
    jint id = (*env)->CallStaticIntMethod(env, webviewClass, webviewCreateMethod, (jlong)token,
                                          jscript);
    (*env)->DeleteLocalRef(env, jscript);
    return (int)id;
}

void belluiWebViewDestroy(uintptr_t jniEnv, int id) {
    JNIEnv *env = (JNIEnv *)jniEnv;
    if (webviewClass == NULL || webviewDestroyMethod == NULL) {
        return;
    }
    (*env)->CallStaticVoidMethod(env, webviewClass, webviewDestroyMethod, (jint)id);
}

void belluiWebViewSetGeometry(uintptr_t jniEnv, int id, int clipX, int clipY, int clipW, int clipH,
                              int viewX, int viewY, int viewW, int viewH) {
    JNIEnv *env = (JNIEnv *)jniEnv;
    if (webviewClass == NULL || webviewSetGeometryMethod == NULL) {
        return;
    }
    (*env)->CallStaticVoidMethod(env, webviewClass, webviewSetGeometryMethod, (jint)id,
                                 (jint)clipX, (jint)clipY, (jint)clipW, (jint)clipH, (jint)viewX,
                                 (jint)viewY, (jint)viewW, (jint)viewH);
}

void belluiWebViewSetVisible(uintptr_t jniEnv, int id, bool visible) {
    JNIEnv *env = (JNIEnv *)jniEnv;
    if (webviewClass == NULL || webviewSetVisibleMethod == NULL) {
        return;
    }
    (*env)->CallStaticVoidMethod(env, webviewClass, webviewSetVisibleMethod, (jint)id,
                                 visible ? JNI_TRUE : JNI_FALSE);
}

void belluiWebViewLoadURL(uintptr_t jniEnv, int id, const char *url) {
    JNIEnv *env = (JNIEnv *)jniEnv;
    if (webviewClass == NULL || webviewLoadURLMethod == NULL) {
        return;
    }
    jstring jurl = (*env)->NewStringUTF(env, url ? url : "");
    (*env)->CallStaticVoidMethod(env, webviewClass, webviewLoadURLMethod, (jint)id, jurl);
    (*env)->DeleteLocalRef(env, jurl);
}

void belluiWebViewLoadHTML(uintptr_t jniEnv, int id, const char *html, const char *baseURL) {
    JNIEnv *env = (JNIEnv *)jniEnv;
    if (webviewClass == NULL || webviewLoadHTMLMethod == NULL) {
        return;
    }
    jstring jhtml = (*env)->NewStringUTF(env, html ? html : "");
    jstring jbase = (*env)->NewStringUTF(env, baseURL ? baseURL : "");
    (*env)->CallStaticVoidMethod(env, webviewClass, webviewLoadHTMLMethod, (jint)id, jhtml, jbase);
    (*env)->DeleteLocalRef(env, jhtml);
    (*env)->DeleteLocalRef(env, jbase);
}

void belluiWebViewEval(uintptr_t jniEnv, int id, const char *js, uint64_t callID, bool wantResult) {
    JNIEnv *env = (JNIEnv *)jniEnv;
    if (webviewClass == NULL || webviewEvalMethod == NULL) {
        return;
    }
    jstring jjs = (*env)->NewStringUTF(env, js ? js : "");
    (*env)->CallStaticVoidMethod(env, webviewClass, webviewEvalMethod, (jint)id, jjs,
                                 (jlong)callID, wantResult ? JNI_TRUE : JNI_FALSE);
    (*env)->DeleteLocalRef(env, jjs);
}

bool belluiWebViewNav(uintptr_t jniEnv, int id, int op) {
    JNIEnv *env = (JNIEnv *)jniEnv;
    if (webviewClass == NULL || webviewNavMethod == NULL) {
        return false;
    }
    return (*env)->CallStaticBooleanMethod(env, webviewClass, webviewNavMethod, (jint)id,
                                           (jint)op) == JNI_TRUE;
}

bool belluiWebViewHasFocus(uintptr_t jniEnv, int id) {
    JNIEnv *env = (JNIEnv *)jniEnv;
    if (webviewClass == NULL || webviewHasFocusMethod == NULL) {
        return false;
    }
    return (*env)->CallStaticBooleanMethod(env, webviewClass, webviewHasFocusMethod, (jint)id) ==
           JNI_TRUE;
}

bool belluiWebViewSetFocus(uintptr_t jniEnv, int id, bool take) {
    JNIEnv *env = (JNIEnv *)jniEnv;
    if (webviewClass == NULL || webviewSetFocusMethod == NULL) {
        return false;
    }
    return (*env)->CallStaticBooleanMethod(env, webviewClass, webviewSetFocusMethod, (jint)id,
                                           take ? JNI_TRUE : JNI_FALSE) == JNI_TRUE;
}

// ---- up-calls: the native methods GoNativeActivity declares ----

// goStr copies a jstring into a NUL-terminated C string the caller frees. A
// NULL jstring becomes the empty string, so the Go side never has to
// distinguish "absent" from "empty".
static char *goStr(JNIEnv *env, jstring s) {
    if (s == NULL) {
        return strdup("");
    }
    const char *utf = (*env)->GetStringUTFChars(env, s, NULL);
    char *out = strdup(utf ? utf : "");
    (*env)->ReleaseStringUTFChars(env, s, utf);
    return out;
}

JNIEXPORT void JNICALL Java_org_golang_app_GoNativeActivity_webviewMessage(
    JNIEnv *env, jclass clazz, jlong token, jstring origin, jstring body) {
    char *corigin = goStr(env, origin);
    char *cbody = goStr(env, body);
    belluiWebViewMessageGo((uint64_t)token, corigin, cbody);
    free(corigin);
    free(cbody);
}

JNIEXPORT void JNICALL Java_org_golang_app_GoNativeActivity_webviewEvalDone(
    JNIEnv *env, jclass clazz, jlong token, jlong callID, jstring result, jstring err) {
    char *cresult = goStr(env, result);
    char *cerr = goStr(env, err);
    belluiWebViewEvalDoneGo((uint64_t)token, (uint64_t)callID, cresult, cerr);
    free(cresult);
    free(cerr);
}

JNIEXPORT void JNICALL Java_org_golang_app_GoNativeActivity_webviewLoad(
    JNIEnv *env, jclass clazz, jlong token, jint state, jstring url, jstring err) {
    char *curl = goStr(env, url);
    char *cerr = goStr(env, err);
    belluiWebViewLoadGo((uint64_t)token, (int)state, curl, cerr);
    free(curl);
    free(cerr);
}

JNIEXPORT jboolean JNICALL Java_org_golang_app_GoNativeActivity_webviewNavigate(
    JNIEnv *env, jclass clazz, jlong token, jstring url, jint kind, jboolean mainFrame,
    jboolean newWindow) {
    char *curl = goStr(env, url);
    // WebViewClient is blocked on this answer, so the Go side hops to the main
    // goroutine and waits there — which is why OnNavigate is documented as
    // having to return promptly.
    char allow = belluiWebViewNavigateGo((uint64_t)token, curl, (int)kind,
                                         mainFrame == JNI_TRUE, newWindow == JNI_TRUE);
    free(curl);
    return allow ? JNI_TRUE : JNI_FALSE;
}
