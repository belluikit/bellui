//go:build android

package main

/*
#include <jni.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

// callProbe loads bellui.probe.LibProbe through the context's class loader (a
// JNI FindClass on this thread would use the system loader, which does not
// see app classes) and returns probe(ctx), or an error string. The caller
// frees the result.
static char* callProbe(uintptr_t envp, uintptr_t ctxp) {
	JNIEnv* env = (JNIEnv*)envp;
	jobject ctx = (jobject)ctxp;

	jclass ctxCls = (*env)->GetObjectClass(env, ctx);
	jmethodID getCL = (*env)->GetMethodID(env, ctxCls, "getClassLoader", "()Ljava/lang/ClassLoader;");
	jobject loader = (*env)->CallObjectMethod(env, ctx, getCL);
	jclass loaderCls = (*env)->FindClass(env, "java/lang/ClassLoader");
	jmethodID loadClass = (*env)->GetMethodID(env, loaderCls, "loadClass", "(Ljava/lang/String;)Ljava/lang/Class;");
	jstring name = (*env)->NewStringUTF(env, "bellui.probe.LibProbe");
	jclass probe = (jclass)(*env)->CallObjectMethod(env, loader, loadClass, name);
	if ((*env)->ExceptionCheck(env)) {
		(*env)->ExceptionDescribe(env);
		(*env)->ExceptionClear(env);
		return strdup("error: bellui.probe.LibProbe not found (see logcat)");
	}
	jmethodID m = (*env)->GetStaticMethodID(env, probe, "probe", "(Landroid/content/Context;)Ljava/lang/String;");
	jstring res = (jstring)(*env)->CallStaticObjectMethod(env, probe, m, ctx);
	if ((*env)->ExceptionCheck(env)) {
		(*env)->ExceptionDescribe(env);
		(*env)->ExceptionClear(env);
		return strdup("error: LibProbe.probe threw (see logcat)");
	}
	const char* s = (*env)->GetStringUTFChars(env, res, NULL);
	char* out = strdup(s);
	(*env)->ReleaseStringUTFChars(env, res, s);
	return out;
}
*/
import "C"

import (
	"unsafe"

	"github.com/belluikit/bellui/driver"
)

func runProbe() string {
	result := "error: RunNative did not run"
	err := driver.RunNative(func(ctx any) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok {
			result = "error: no AndroidContext"
			return nil
		}
		cs := C.callProbe(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx))
		defer C.free(unsafe.Pointer(cs))
		result = C.GoString(cs)
		if len(result) < 6 || result[:6] != "error:" {
			result = "ok " + result
		}
		return nil
	})
	if err != nil {
		return "error: " + err.Error()
	}
	return result
}
