//go:build ignore

// #60 spike, second attempt: the same question — DMA-BUF (zero-copy) or SHM? —
// asked through the WPE Platform API that WPE WebKit 2.44+ development moved
// to, rather than the legacy wpe_view_backend_exportable_fdo path.
//
// The legacy path produced no frames at all on 2.48/2.52, so this asks the
// engine the way the engine now expects to be asked. A WPEView receives frames
// through its class's render_buffer vfunc, so the probe installs its own in
// front of the real one and reports what type of buffer arrives.
#include <glib.h>
#include <glib-object.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <wpe/webkit.h>
#include <wpe/wpe-platform.h>
#include <wpe/headless/wpe-headless.h>

static int dmabufFrames = 0, shmFrames = 0, otherFrames = 0;
static GMainLoop *loop = NULL;
static gboolean (*origRenderBuffer)(WPEView *, WPEBuffer *, const WPERectangle *, guint, GError **) = NULL;

static gboolean onRenderBuffer(WPEView *view, WPEBuffer *buffer,
                               const WPERectangle *damage, guint nDamage, GError **error) {
    if (WPE_IS_BUFFER_DMA_BUF(buffer)) {
        WPEBufferDMABuf *dma = WPE_BUFFER_DMA_BUF(buffer);
        if (dmabufFrames == 0) {
            printf("[frame] DMA-BUF  %dx%d  fourcc=0x%08x planes=%u\n",
                   wpe_buffer_get_width(buffer), wpe_buffer_get_height(buffer),
                   wpe_buffer_dma_buf_get_format(dma), wpe_buffer_dma_buf_get_n_planes(dma));
        }
        dmabufFrames++;
    } else if (WPE_IS_BUFFER_SHM(buffer)) {
        if (shmFrames == 0) {
            printf("[frame] SHM      %dx%d  stride=%u\n",
                   wpe_buffer_get_width(buffer), wpe_buffer_get_height(buffer),
                   wpe_buffer_shm_get_stride(WPE_BUFFER_SHM(buffer)));
        }
        shmFrames++;
    } else {
        otherFrames++;
        printf("[frame] other buffer type: %s\n", G_OBJECT_TYPE_NAME(buffer));
    }
    fflush(stdout);

    if (dmabufFrames + shmFrames + otherFrames >= 3 && loop) {
        g_main_loop_quit(loop);
    }
    if (origRenderBuffer) {
        return origRenderBuffer(view, buffer, damage, nDamage, error);
    }
    wpe_view_buffer_rendered(view, buffer);
    return TRUE;
}

static gboolean onTimeout(gpointer _) {
    printf("[timeout] no more frames\n");
    g_main_loop_quit(loop);
    return G_SOURCE_REMOVE;
}

// dumpProps lists the construct properties, so a wrong guess at the display
// property name is self-diagnosing rather than a silent failure.
static void dumpProps(GType type) {
    GObjectClass *klass = g_type_class_ref(type);
    guint n = 0;
    GParamSpec **props = g_object_class_list_properties(klass, &n);
    printf("[diag] %s construct properties:\n", g_type_name(type));
    for (guint i = 0; i < n; i++) {
        if (props[i]->flags & G_PARAM_CONSTRUCT_ONLY) {
            printf("        %-24s %s\n", props[i]->name, g_type_name(props[i]->value_type));
        }
    }
    g_free(props);
    g_type_class_unref(klass);
}

int main(int argc, char **argv) {
    printf("=== WPE Platform API probe: DMA-BUF or SHM? ===\n");

    WPEDisplay *display = wpe_display_headless_new();
    if (!display) { printf("FAIL: no headless WPEDisplay\n"); return 1; }
    printf("[wpe] headless display: %s\n", G_OBJECT_TYPE_NAME(display));

    GError *err = NULL;
    if (!wpe_display_connect(display, &err)) {
        printf("[wpe] connect: %s\n", err ? err->message : "failed");
        g_clear_error(&err);
    }

    // Whether zero-copy is even possible comes down to this: a display that
    // exposes no DRM device has no way to allocate dma-bufs, so it can only
    // ever hand over SHM — independently of the GPU underneath.
    WPEDRMDevice *drm = wpe_display_get_drm_device(display);
    if (drm) {
        printf("[wpe] DRM device: primary=%s render=%s  => dma-buf is possible\n",
               wpe_drm_device_get_primary_node(drm), wpe_drm_device_get_render_node(drm));
    } else {
        printf("[wpe] DRM device: NONE  => this display can only produce SHM\n");
    }

    // A headless display bound to a specific render node, which is the form
    // that could plausibly allocate dma-bufs.
    const char *node = getenv("WPE_PROBE_RENDER_NODE");
    if (node) {
        GError *e2 = NULL;
        WPEDisplay *d2 = wpe_display_headless_new_for_device(node, &e2);
        if (d2) {
            WPEDRMDevice *drm2 = wpe_display_get_drm_device(d2);
            printf("[wpe] headless_for_device(%s): DRM device %s\n", node,
                   drm2 ? "PRESENT" : "still none");
            if (drm2) {
                display = d2; // prefer the one that can allocate dma-bufs
                wpe_display_connect(display, NULL);
            }
        } else {
            printf("[wpe] headless_for_device(%s): %s\n", node, e2 ? e2->message : "failed");
            g_clear_error(&e2);
        }
    }

    if (getenv("WPE_PROBE_DUMP_PROPS")) {
        dumpProps(WEBKIT_TYPE_WEB_VIEW);
    }

    WebKitWebView *webView = g_object_new(WEBKIT_TYPE_WEB_VIEW, "display", display, NULL);
    if (!webView) { printf("FAIL: no WebKitWebView\n"); return 1; }

    WPEView *view = webkit_web_view_get_wpe_view(webView);
    if (!view) { printf("FAIL: web view has no WPEView\n"); return 1; }
    printf("[wpe] view: %s\n", G_OBJECT_TYPE_NAME(view));

    // Take the frames as the engine delivers them: render_buffer is the vfunc a
    // view implementation supplies, so chaining in front of it observes exactly
    // what WebKit hands over, with no second copy or conversion in between.
    WPEViewClass *viewClass = WPE_VIEW_GET_CLASS(view);
    origRenderBuffer = viewClass->render_buffer;
    viewClass->render_buffer = onRenderBuffer;

    wpe_view_map(view);
    wpe_view_resized(view, 800, 600);

    webkit_web_view_load_html(webView,
        "<html><body style='margin:0;background:#ff00aa'>"
        "<h1 style='color:#fff;font:48px sans-serif'>WPE platform</h1>"
        "</body></html>", NULL);

    loop = g_main_loop_new(NULL, FALSE);
    g_timeout_add_seconds(45, onTimeout, NULL);
    g_main_loop_run(loop);

    printf("\n[result] dmabuf=%d shm=%d other=%d\n", dmabufFrames, shmFrames, otherFrames);
    if (dmabufFrames > 0) {
        printf("[result] ZERO-COPY REACHABLE: frames arrive as dma-bufs.\n");
        return 0;
    }
    if (shmFrames > 0) {
        printf("[result] SHM only: no zero-copy path on this stack.\n");
        return 0;
    }
    printf("[result] FAIL: no frames produced\n");
    return 2;
}
