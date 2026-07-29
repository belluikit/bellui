//go:build linux

#include "wpe.h"

#include <EGL/egl.h>
#include <EGL/eglext.h>
#include <glib.h>
#include <pthread.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <wayland-server.h>
#include <wpe/fdo.h>
#include <wpe/fdo-egl.h>
#include <wpe/webkit.h>

static pthread_t thread;
static pthread_mutex_t mu = PTHREAD_MUTEX_INITIALIZER;

static unsigned char *frameBuf = NULL;
static int frameLen = 0, frameW = 0, frameH = 0, frameStride = 0;
static uint64_t frameSerial = 0;
static int exportKind = 0;

static GMainLoop *loop = NULL;
static struct wpe_view_backend_exportable_fdo *exportable = NULL;
static int viewW = 0, viewH = 0;
static char *pageHTML = NULL;

static void onShmBuffer(void *data, struct wpe_fdo_shm_exported_buffer *buffer) {
    struct wl_shm_buffer *shm = wpe_fdo_shm_exported_buffer_get_shm_buffer(buffer);
    int32_t w = wl_shm_buffer_get_width(shm);
    int32_t h = wl_shm_buffer_get_height(shm);
    int32_t stride = wl_shm_buffer_get_stride(shm);

    wl_shm_buffer_begin_access(shm);
    const unsigned char *px = wl_shm_buffer_get_data(shm);

    pthread_mutex_lock(&mu);
    exportKind = 1;
    int need = stride * h;
    if (need > frameLen) {
        frameBuf = realloc(frameBuf, need);
        frameLen = need;
    }
    if (frameBuf) {
        memcpy(frameBuf, px, need);
        frameW = w; frameH = h; frameStride = stride;
        frameSerial++;
    }
    pthread_mutex_unlock(&mu);

    wl_shm_buffer_end_access(shm);
    wpe_view_backend_exportable_fdo_egl_dispatch_release_shm_exported_buffer(exportable, buffer);
    wpe_view_backend_exportable_fdo_dispatch_frame_complete(exportable);
}

// onEglImage records that the engine took the zero-copy path. Nothing is read
// back here: an EGLImage is a GPU handle, and turning it into pixels is the
// part of the spike this stack never reached.
static void onEglImage(void *data, struct wpe_fdo_egl_exported_image *image) {
    pthread_mutex_lock(&mu);
    exportKind = 2;
    frameW = wpe_fdo_egl_exported_image_get_width(image);
    frameH = wpe_fdo_egl_exported_image_get_height(image);
    frameSerial++;
    pthread_mutex_unlock(&mu);

    wpe_view_backend_exportable_fdo_egl_dispatch_release_exported_image(exportable, image);
    wpe_view_backend_exportable_fdo_dispatch_frame_complete(exportable);
}

static EGLDisplay headlessDisplay(void) {
    const char *exts = eglQueryString(EGL_NO_DISPLAY, EGL_EXTENSIONS);
    if (exts && strstr(exts, "EGL_EXT_platform_device")) {
        PFNEGLQUERYDEVICESEXTPROC queryDevices =
            (PFNEGLQUERYDEVICESEXTPROC)eglGetProcAddress("eglQueryDevicesEXT");
        PFNEGLGETPLATFORMDISPLAYEXTPROC getPlatformDisplay =
            (PFNEGLGETPLATFORMDISPLAYEXTPROC)eglGetProcAddress("eglGetPlatformDisplayEXT");
        if (queryDevices && getPlatformDisplay) {
            EGLDeviceEXT devices[8]; EGLint n = 0;
            if (queryDevices(8, devices, &n) && n > 0) {
                EGLDisplay d = getPlatformDisplay(EGL_PLATFORM_DEVICE_EXT, devices[0], NULL);
                if (d != EGL_NO_DISPLAY) return d;
            }
        }
    }
    return eglGetDisplay(EGL_DEFAULT_DISPLAY);
}

static void *run(void *arg) {
    EGLDisplay dpy = headlessDisplay();
    if (dpy == EGL_NO_DISPLAY) { fprintf(stderr, "wpe: no EGLDisplay\n"); return NULL; }
    EGLint major = 0, minor = 0;
    if (!eglInitialize(dpy, &major, &minor)) { fprintf(stderr, "wpe: eglInitialize failed\n"); return NULL; }

    wpe_fdo_initialize_for_egl_display(dpy);

    static const struct wpe_view_backend_exportable_fdo_egl_client client = {
        .export_egl_image = NULL,
        .export_fdo_egl_image = onEglImage,
        .export_shm_buffer = onShmBuffer,
    };
    exportable = wpe_view_backend_exportable_fdo_egl_create(&client, NULL, viewW, viewH);
    if (!exportable) { fprintf(stderr, "wpe: no exportable backend\n"); return NULL; }

    WebKitWebViewBackend *backend = webkit_web_view_backend_new(
        wpe_view_backend_exportable_fdo_get_view_backend(exportable), NULL, NULL);
    WebKitWebView *view = webkit_web_view_new(backend);
    webkit_web_view_load_html(view, pageHTML, NULL);

    loop = g_main_loop_new(NULL, FALSE);
    g_main_loop_run(loop);
    return NULL;
}

int belluiWpeStart(int width, int height, const char *html) {
    viewW = width; viewH = height;
    pageHTML = strdup(html ? html : "<html></html>");
    // libwpe otherwise looks for libWPEBackend-default.so, which Ubuntu does
    // not ship; fdo is the backend this whole approach is built on.
    if (!getenv("WPE_BACKEND_LIBRARY")) {
        setenv("WPE_BACKEND_LIBRARY", "libWPEBackend-fdo-1.0.so.1", 1);
    }
    return pthread_create(&thread, NULL, run, NULL);
}

uint64_t belluiWpeFrame(unsigned char *out, int outLen, int *w, int *h, int *stride) {
    pthread_mutex_lock(&mu);
    uint64_t serial = frameSerial;
    if (serial && frameBuf && out) {
        int need = frameStride * frameH;
        if (need > outLen) need = outLen;
        memcpy(out, frameBuf, need);
        *w = frameW; *h = frameH; *stride = frameStride;
    }
    pthread_mutex_unlock(&mu);
    return serial;
}

int belluiWpeExportKind(void) {
    pthread_mutex_lock(&mu);
    int k = exportKind;
    pthread_mutex_unlock(&mu);
    return k;
}

void belluiWpeStop(void) {
    if (loop) g_main_loop_quit(loop);
}
