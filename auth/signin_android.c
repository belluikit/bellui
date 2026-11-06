//go:build android

// JNI plumbing for sign-in on Android (#1042). The Java half lives in
// GoNativeActivity.java, which reaches Credential Manager by reflection. This
// file calls its static entry point and implements the native method it
// reports back through. JNI binds that method by name across the process
// image, so defining it here, in the optional auth package, leaves it unbound
// in an app that never imports auth, which also never starts a sign-in.

#include <jni.h>
#include <stdbool.h>
#include <stdint.h>

#include "_cgo_export.h"

static jclass authClass;
static jmethodID authGoogleSignInMethod;

// belluiAuthInit caches the activity class and its sign-in entry point. ctx is
// the activity, so its class is looked up from it rather than through
// FindClass, which on a JNI thread searches the wrong class loader. It returns
// false when the class has no entry point: an app built against an older
// embedded dex.
bool belluiAuthInit(uintptr_t jniEnv, uintptr_t ctx) {
    JNIEnv *env = (JNIEnv *)jniEnv;
    if (authClass != NULL) {
        return true;
    }
    jclass local = (*env)->GetObjectClass(env, (jobject)ctx);
    if (local == NULL) {
        (*env)->ExceptionClear(env);
        return false;
    }
    jmethodID m = (*env)->GetStaticMethodID(env, local, "authGoogleSignIn",
                                            "(JLjava/lang/String;Ljava/lang/String;)V");
    if (m == NULL) {
        (*env)->ExceptionClear(env);
        (*env)->DeleteLocalRef(env, local);
        return false;
    }
    authClass = (jclass)(*env)->NewGlobalRef(env, local);
    (*env)->DeleteLocalRef(env, local);
    authGoogleSignInMethod = m;
    return true;
}

void belluiAuthGoogleSignIn(uintptr_t jniEnv, uint64_t token, const char *serverClientID,
                            const char *nonce) {
    JNIEnv *env = (JNIEnv *)jniEnv;
    jstring id = (*env)->NewStringUTF(env, serverClientID);
    jstring n = (*env)->NewStringUTF(env, nonce);
    (*env)->CallStaticVoidMethod(env, authClass, authGoogleSignInMethod, (jlong)token, id, n);
    (*env)->ExceptionClear(env);
    (*env)->DeleteLocalRef(env, id);
    (*env)->DeleteLocalRef(env, n);
}

static const char *chars(JNIEnv *env, jstring s) {
    return s ? (*env)->GetStringUTFChars(env, s, NULL) : "";
}

static void release(JNIEnv *env, jstring s, const char *c) {
    if (s) {
        (*env)->ReleaseStringUTFChars(env, s, c);
    }
}

JNIEXPORT void JNICALL Java_org_golang_app_GoNativeActivity_authGoogleDone(
    JNIEnv *env, jclass clazz, jlong token, jint status, jstring idToken, jstring email,
    jstring name, jstring err) {
    const char *t = chars(env, idToken), *e = chars(env, email), *nm = chars(env, name),
               *m = chars(env, err);
    belluiAuthGoogleDone((uint64_t)token, status, (char *)t, (char *)e, (char *)nm, (char *)m);
    release(env, idToken, t);
    release(env, email, e);
    release(env, name, nm);
    release(env, err, m);
}
