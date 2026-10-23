//go:build android

package main

/*
#include <jni.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

// callBgAudio calls the static String method name(Context) on
// bellui.probe.BgAudio, loaded through the context's class loader (FindClass
// on this thread would use the system loader, which does not see app
// classes). The caller frees the result.
static char* callBgAudio(uintptr_t envp, uintptr_t ctxp, const char* name) {
	JNIEnv* env = (JNIEnv*)envp;
	jobject ctx = (jobject)ctxp;

	jclass ctxCls = (*env)->GetObjectClass(env, ctx);
	jmethodID getCL = (*env)->GetMethodID(env, ctxCls, "getClassLoader", "()Ljava/lang/ClassLoader;");
	jobject loader = (*env)->CallObjectMethod(env, ctx, getCL);
	jclass loaderCls = (*env)->FindClass(env, "java/lang/ClassLoader");
	jmethodID loadClass = (*env)->GetMethodID(env, loaderCls, "loadClass", "(Ljava/lang/String;)Ljava/lang/Class;");
	jstring cls = (*env)->NewStringUTF(env, "bellui.probe.BgAudio");
	jclass bg = (jclass)(*env)->CallObjectMethod(env, loader, loadClass, cls);
	if ((*env)->ExceptionCheck(env)) {
		(*env)->ExceptionDescribe(env);
		(*env)->ExceptionClear(env);
		return strdup("error: bellui.probe.BgAudio not found (see logcat)");
	}
	jmethodID m = (*env)->GetStaticMethodID(env, bg, name, "(Landroid/content/Context;)Ljava/lang/String;");
	jstring res = (jstring)(*env)->CallStaticObjectMethod(env, bg, m, ctx);
	if ((*env)->ExceptionCheck(env)) {
		(*env)->ExceptionDescribe(env);
		(*env)->ExceptionClear(env);
		return strdup("error: BgAudio threw (see logcat)");
	}
	const char* s = (*env)->GetStringUTFChars(env, res, NULL);
	char* out = strdup(s);
	(*env)->ReleaseStringUTFChars(env, res, s);
	return out;
}
*/
import "C"

import (
	"errors"
	"fmt"
	"unsafe"

	"github.com/belluikit/bellui/driver"
)

func callBgAudio(method string) (string, error) {
	var out string
	err := driver.RunNative(func(ctx any) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok {
			return errors.New("no AndroidContext")
		}
		name := C.CString(method)
		defer C.free(unsafe.Pointer(name))
		cs := C.callBgAudio(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx), name)
		defer C.free(unsafe.Pointer(cs))
		out = C.GoString(cs)
		return nil
	})
	return out, err
}

func startAudio() error {
	msg, err := callBgAudio("start")
	if err != nil {
		return err
	}
	if msg != "" {
		return errors.New(msg)
	}
	return nil
}

// audioStats maps BgAudio.stats onto the shared log line: state is the
// process importance Android reports (foreground, visible, cached, …).
func audioStats() stats {
	var s stats
	out, err := callBgAudio("stats")
	if err != nil {
		s.state = "error:" + err.Error()
		return s
	}
	if _, err := fmt.Sscanf(out, "%s %d %t", &s.state, &s.frames, &s.running); err != nil {
		s.state = "unparsed:" + out
	}
	return s
}
