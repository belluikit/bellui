//go:build android

package main

/*
#cgo LDFLAGS: -landroid -lz
#include <jni.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <zlib.h>
#include <android/asset_manager.h>
#include <android/asset_manager_jni.h>

// javaRead loads bellui.probe.AssetProbe through the context's class loader
// (FindClass on this thread would use the system loader, which does not see
// app classes) and returns read(ctx, name), or an error string.
static char* javaRead(JNIEnv* env, jobject ctx, const char* asset) {
	jclass ctxCls = (*env)->GetObjectClass(env, ctx);
	jmethodID getCL = (*env)->GetMethodID(env, ctxCls, "getClassLoader", "()Ljava/lang/ClassLoader;");
	jobject loader = (*env)->CallObjectMethod(env, ctx, getCL);
	jclass loaderCls = (*env)->FindClass(env, "java/lang/ClassLoader");
	jmethodID loadClass = (*env)->GetMethodID(env, loaderCls, "loadClass", "(Ljava/lang/String;)Ljava/lang/Class;");
	jstring name = (*env)->NewStringUTF(env, "bellui.probe.AssetProbe");
	jclass probe = (jclass)(*env)->CallObjectMethod(env, loader, loadClass, name);
	if ((*env)->ExceptionCheck(env)) {
		(*env)->ExceptionDescribe(env);
		(*env)->ExceptionClear(env);
		return strdup("java=error:AssetProbe not found");
	}
	jmethodID m = (*env)->GetStaticMethodID(env, probe, "read", "(Landroid/content/Context;Ljava/lang/String;)Ljava/lang/String;");
	jstring res = (jstring)(*env)->CallStaticObjectMethod(env, probe, m, ctx, (*env)->NewStringUTF(env, asset));
	if ((*env)->ExceptionCheck(env)) {
		(*env)->ExceptionDescribe(env);
		(*env)->ExceptionClear(env);
		return strdup("java=error:AssetProbe.read threw");
	}
	const char* s = (*env)->GetStringUTFChars(env, res, NULL);
	char* out = strdup(s);
	(*env)->ReleaseStringUTFChars(env, res, s);
	return out;
}

// nativeRead reads the asset whole with AAsset_read, the NDK's path.
static char* nativeRead(JNIEnv* env, jobject ctx, const char* asset) {
	jclass ctxCls = (*env)->GetObjectClass(env, ctx);
	jmethodID getAssets = (*env)->GetMethodID(env, ctxCls, "getAssets", "()Landroid/content/res/AssetManager;");
	jobject jam = (*env)->CallObjectMethod(env, ctx, getAssets);
	AAssetManager* am = AAssetManager_fromJava(env, jam);
	AAsset* a = AAssetManager_open(am, asset, AASSET_MODE_STREAMING);
	if (a == NULL) {
		return strdup("native=error:open");
	}
	unsigned char buf[65536];
	uLong crc = crc32(0L, Z_NULL, 0);
	long long n = 0;
	int r;
	while ((r = AAsset_read(a, buf, sizeof buf)) > 0) {
		crc = crc32(crc, buf, r);
		n += r;
	}
	AAsset_close(a);
	char* out = malloc(64);
	if (r < 0) {
		snprintf(out, 64, "native=error:read");
	} else {
		snprintf(out, 64, "native=%lld/%08lx", n, (unsigned long)crc);
	}
	return out;
}

static char* probeAsset(uintptr_t envp, uintptr_t ctxp, const char* asset) {
	JNIEnv* env = (JNIEnv*)envp;
	jobject ctx = (jobject)ctxp;
	char* j = javaRead(env, ctx, asset);
	char* n = nativeRead(env, ctx, asset);
	size_t len = strlen(j) + strlen(n) + 2;
	char* out = malloc(len);
	snprintf(out, len, "%s %s", j, n);
	free(j);
	free(n);
	return out;
}
*/
import "C"

import (
	"strings"
	"unsafe"

	"github.com/belluikit/bellui/driver"
)

func runProbe() string {
	var parts []string
	err := driver.RunNative(func(ctx any) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok {
			parts = append(parts, "error: no AndroidContext")
			return nil
		}
		for _, a := range []struct{ label, name string }{
			{"notices", "THIRD_PARTY_NOTICES.txt"},
			{"tone", "media/tone.ogg"},
		} {
			name := C.CString(a.name)
			cs := C.probeAsset(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx), name)
			parts = append(parts, a.label+" "+C.GoString(cs))
			C.free(unsafe.Pointer(cs))
			C.free(unsafe.Pointer(name))
		}
		return nil
	})
	if err != nil {
		return "error: " + err.Error()
	}
	line := strings.Join(parts, " ")
	if strings.Contains(line, "error") {
		return line
	}
	return "ok " + line
}
