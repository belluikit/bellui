// CEF C API shim for the #60 phase 0 probe. See cef.h for why this exists.

// Pin the API version. Leaving this undefined selects CEF_API_VERSION_EXPERIMENTAL,
// which is explicitly *not* forward or backward compatible with other CEF builds —
// so an unversioned binding would have to be re-vendored on every payload bump,
// which is exactly the treadmill cost belluikit/bellui#98 is about. An explicit
// version is compatible across a range (MIN 13300 .. LAST 15101 in this build),
// and the OSR surface this shim uses carries no CEF_API_ADDED guards at all.
#define CEF_API_VERSION 15101

#include "cef.h"

#include <dlfcn.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "include/capi/cef_app_capi.h"
#include "include/capi/cef_browser_capi.h"
#include "include/capi/cef_client_capi.h"
#include "include/capi/cef_life_span_handler_capi.h"
#include "include/capi/cef_render_handler_capi.h"

#define MAX_SWITCHES 32

#define STR_(x) #x
#define STR(x) STR_(x)

// ---------------------------------------------------------------------------
// libcef.so, resolved at runtime.

static void *g_lib;
static char g_err[512];

static const char *(*p_api_hash)(int, int);
static int (*p_api_version)(void);
static int (*p_initialize)(const cef_main_args_t *, const struct _cef_settings_t *,
                           cef_app_t *, void *);
static void (*p_shutdown)(void);
static void (*p_do_message_loop_work)(void);
static int (*p_create_browser)(const cef_window_info_t *, struct _cef_client_t *,
                               const cef_string_t *,
                               const struct _cef_browser_settings_t *,
                               struct _cef_dictionary_value_t *,
                               struct _cef_request_context_t *);
static int (*p_utf8_to_utf16)(const char *, size_t, cef_string_t *);

static void fail(const char *fmt, const char *a, const char *b) {
  snprintf(g_err, sizeof(g_err), fmt, a, b ? b : "");
}

const char *belluiCefError(void) { return g_err; }

// resolve fills one function pointer, reporting the first symbol that is absent.
static int resolve(void **slot, const char *name) {
  *slot = dlsym(g_lib, name);
  if (*slot == NULL) {
    fail("libcef.so has no symbol %s (%s)", name, dlerror());
    return -1;
  }
  return 0;
}

// setstr copies a UTF-8 C string into a cef_string_t, which owns UTF-16.
static void setstr(cef_string_t *dst, const char *src) {
  if (src != NULL && src[0] != '\0') {
    p_utf8_to_utf16(src, strlen(src), dst);
  }
}

// ---------------------------------------------------------------------------
// Reference counting.
//
// Every handler below is a file-scope singleton that outlives CEF, so the
// counts are inert: nothing is ever freed and the object is always live. A
// binding with dynamic handler lifetimes (belluikit/bellui#103) needs real
// counts here — the whole C API is refcounted through this one struct.

static void CEF_CALLBACK ref_add(cef_base_ref_counted_t *self) { (void)self; }
static int CEF_CALLBACK ref_release(cef_base_ref_counted_t *self) {
  (void)self;
  return 0;
}
static int CEF_CALLBACK ref_has_one(cef_base_ref_counted_t *self) {
  (void)self;
  return 1;
}
static int CEF_CALLBACK ref_has_any(cef_base_ref_counted_t *self) {
  (void)self;
  return 1;
}

// init_base sets the size field CEF uses to tell API versions apart, plus the
// four refcount slots. Getting size wrong is the classic C-API failure: CEF
// reads it to decide which members exist, so a stale sizeof silently truncates
// the struct.
static void init_base(cef_base_ref_counted_t *base, size_t size) {
  memset(base, 0, size);
  base->size = size;
  base->add_ref = ref_add;
  base->release = ref_release;
  base->has_one_ref = ref_has_one;
  base->has_at_least_one_ref = ref_has_any;
}

// ---------------------------------------------------------------------------
// State.

static cef_browser_t *g_browser;
static int g_width = 800;
static int g_height = 600;

// ---------------------------------------------------------------------------
// cef_render_handler_t — the offscreen surface.

static cef_render_handler_t g_render_handler;

static void CEF_CALLBACK rh_get_view_rect(struct _cef_render_handler_t *self,
                                          struct _cef_browser_t *browser,
                                          cef_rect_t *rect) {
  (void)self;
  (void)browser;
  rect->x = 0;
  rect->y = 0;
  rect->width = g_width;
  rect->height = g_height;
}

static void CEF_CALLBACK rh_on_paint(struct _cef_render_handler_t *self,
                                     struct _cef_browser_t *browser,
                                     cef_paint_element_type_t type,
                                     size_t dirtyRectsCount,
                                     cef_rect_t const *dirtyRects,
                                     const void *buffer, int width, int height) {
  (void)self;
  (void)browser;
  (void)dirtyRectsCount;
  (void)dirtyRects;
  belluiCefPaint((void *)buffer, width, height, type == PET_POPUP);
}

static void CEF_CALLBACK rh_on_accelerated_paint(
    struct _cef_render_handler_t *self, struct _cef_browser_t *browser,
    cef_paint_element_type_t type, size_t dirtyRectsCount,
    cef_rect_t const *dirtyRects, const cef_accelerated_paint_info_t *info) {
  (void)self;
  (void)browser;
  (void)type;
  (void)dirtyRectsCount;
  (void)dirtyRects;
  if (info == NULL || info->plane_count < 1) {
    belluiCefAccelPaint(0, -1, 0, 0, 0);
    return;
  }
  belluiCefAccelPaint(info->plane_count, info->planes[0].fd,
                      info->planes[0].stride, info->modifier, info->format);
}

// ---------------------------------------------------------------------------
// cef_life_span_handler_t — where the browser handle arrives.

static cef_life_span_handler_t g_life_span_handler;

static void CEF_CALLBACK ls_on_after_created(struct _cef_life_span_handler_t *self,
                                             struct _cef_browser_t *browser) {
  (void)self;
  g_browser = browser;
  browser->base.add_ref(&browser->base);

  // A windowless browser has no window whose visibility CEF can observe, so it
  // has to be told — and until it is, whether a first frame ever arrives is a
  // race. Without this the probe produced frames on roughly two runs in three,
  // with identical logs either way, which is the worst possible failure shape.
  //
  // The same call is what a real backend drives from Show/Hide and from
  // scrolling out of a clipping ancestor (belluikit/bellui#105); set_focus is
  // the seam the #59 focus bridge lands on. invalidate asks for the first
  // paint rather than waiting for the page to dirty itself.
  cef_browser_host_t *host = browser->get_host(browser);
  if (host != NULL) {
    host->was_hidden(host, 0);
    host->set_focus(host, 1);
    host->invalidate(host, PET_VIEW);
    host->base.release(&host->base);
  }
}

static void CEF_CALLBACK ls_on_before_close(struct _cef_life_span_handler_t *self,
                                            struct _cef_browser_t *browser) {
  (void)self;
  (void)browser;
  g_browser = NULL;
}

// ---------------------------------------------------------------------------
// cef_client_t — hands CEF the two handlers above.

static cef_client_t g_client;

static struct _cef_render_handler_t *CEF_CALLBACK cl_get_render_handler(
    struct _cef_client_t *self) {
  (void)self;
  return &g_render_handler;
}

static struct _cef_life_span_handler_t *CEF_CALLBACK cl_get_life_span_handler(
    struct _cef_client_t *self) {
  (void)self;
  return &g_life_span_handler;
}

// ---------------------------------------------------------------------------

static void init_handlers(void) {
  init_base(&g_render_handler.base, sizeof(g_render_handler));
  g_render_handler.get_view_rect = rh_get_view_rect;
  g_render_handler.on_paint = rh_on_paint;
  g_render_handler.on_accelerated_paint = rh_on_accelerated_paint;

  init_base(&g_life_span_handler.base, sizeof(g_life_span_handler));
  g_life_span_handler.on_after_created = ls_on_after_created;
  g_life_span_handler.on_before_close = ls_on_before_close;

  init_base(&g_client.base, sizeof(g_client));
  g_client.get_render_handler = cl_get_render_handler;
  g_client.get_life_span_handler = cl_get_life_span_handler;
}

int belluiCefInit(const char *cefDir, const char *helperPath, const char *switches) {
  char path[1024];
  snprintf(path, sizeof(path), "%s/libcef.so", cefDir);

  // RTLD_GLOBAL: the subprocesses and CEF's own internals expect the symbols to
  // be visible process-wide.
  g_lib = dlopen(path, RTLD_NOW | RTLD_GLOBAL);
  if (g_lib == NULL) {
    fail("cannot load %s: %s", path, dlerror());
    return -1;
  }

  if (resolve((void **)&p_api_hash, "cef_api_hash") != 0 ||
      resolve((void **)&p_api_version, "cef_api_version") != 0 ||
      resolve((void **)&p_initialize, "cef_initialize") != 0 ||
      resolve((void **)&p_shutdown, "cef_shutdown") != 0 ||
      resolve((void **)&p_do_message_loop_work, "cef_do_message_loop_work") != 0 ||
      resolve((void **)&p_create_browser, "cef_browser_host_create_browser") != 0 ||
      resolve((void **)&p_utf8_to_utf16, "cef_string_utf8_to_utf16") != 0) {
    return -1;
  }

  // THE VERSION HANDSHAKE, and it must come before every other CEF call.
  //
  // cef_api_hash reads as a query, but its doc comment says it "configures the
  // CEF API version ... any changes to this value will be ignored after the
  // first call". Skip it and libcef has no configured version, so
  // cef_api_version() reports -1 and the C→C++ layer rejects every struct
  // handed to it — fatally, and naming whichever type it happened to be
  // validating: "CefClient_0_CToCpp called with invalid version -1", which
  // says nothing about the actual mistake.
  //
  // This is the single most important thing belluikit/bellui#103 has to get
  // right, and it is why the binding pins CEF_API_VERSION rather than letting
  // it default to CEF_API_VERSION_EXPERIMENTAL.
  p_api_hash(CEF_API_VERSION, 0);
  if (p_api_version() != CEF_API_VERSION) {
    static char got[64];
    snprintf(got, sizeof(got), "%d", p_api_version());
    fail("libcef refused API version " STR(CEF_API_VERSION) ", reports %s%s", got, "");
    return -1;
  }

  init_handlers();

  // Chromium reads switches from argv, and propagates them to the subprocesses
  // it launches — so appending them here is enough, and no
  // on_before_command_line_processing handler is needed.
  static char *argv[MAX_SWITCHES + 1];
  static char sw[1024];
  int argc = 0;
  argv[argc++] = (char *)"cefprobe";
  if (switches != NULL && switches[0] != '\0') {
    snprintf(sw, sizeof(sw), "%s", switches);
    char *save = NULL;
    for (char *tok = strtok_r(sw, " ", &save); tok != NULL && argc < MAX_SWITCHES;
         tok = strtok_r(NULL, " ", &save)) {
      argv[argc++] = tok;
    }
  }
  argv[argc] = NULL;

  cef_main_args_t args;
  args.argc = argc;
  args.argv = argv;

  cef_settings_t settings;
  memset(&settings, 0, sizeof(settings));
  settings.size = sizeof(settings);
  settings.no_sandbox = 1;
  settings.windowless_rendering_enabled = 1;
  settings.log_severity = LOGSEVERITY_WARNING;
  // Opaque white, so a page with no background of its own is distinguishable
  // from an untouched buffer.
  settings.background_color = 0xFFFFFFFFu;
  setstr(&settings.browser_subprocess_path, helperPath);

  // Resources sit flat beside libcef.so — see setup.sh for why a nested
  // Resources/ directory fails in ICU rather than anywhere informative.
  setstr(&settings.resources_dir_path, cefDir);
  snprintf(path, sizeof(path), "%s/locales", cefDir);
  setstr(&settings.locales_dir_path, path);
  snprintf(path, sizeof(path), "%s/cache", cefDir);
  setstr(&settings.root_cache_path, path);

  if (p_initialize(&args, &settings, NULL, NULL) != 1) {
    fail("cef_initialize failed%s%s", "", "");
    return -1;
  }
  return 0;
}

int belluiCefCreateBrowser(const char *url, int w, int h, int sharedTexture) {
  g_width = w;
  g_height = h;

  cef_window_info_t wi;
  memset(&wi, 0, sizeof(wi));
  wi.size = sizeof(wi);
  wi.windowless_rendering_enabled = 1;
  wi.shared_texture_enabled = sharedTexture ? 1 : 0;
  wi.bounds.width = w;
  wi.bounds.height = h;

  cef_browser_settings_t bs;
  memset(&bs, 0, sizeof(bs));
  bs.size = sizeof(bs);
  // 60 rather than CEF's default of 30: bellui repaints at display rate and a
  // half-rate web view is visible as stutter.
  bs.windowless_frame_rate = 60;

  cef_string_t curl;
  memset(&curl, 0, sizeof(curl));
  setstr(&curl, url);

  if (p_create_browser(&wi, &g_client, &curl, &bs, NULL, NULL) != 1) {
    fail("cef_browser_host_create_browser failed%s%s", "", "");
    return -1;
  }
  return 0;
}

void belluiCefWork(void) {
  if (p_do_message_loop_work != NULL) {
    p_do_message_loop_work();
  }
}

void belluiCefResize(int w, int h) {
  g_width = w;
  g_height = h;
  if (g_browser != NULL) {
    cef_browser_host_t *host = g_browser->get_host(g_browser);
    if (host != NULL) {
      host->was_resized(host);
      host->base.release(&host->base);
    }
  }
}

void belluiCefShutdown(void) {
  if (g_browser != NULL) {
    cef_browser_host_t *host = g_browser->get_host(g_browser);
    if (host != NULL) {
      host->close_browser(host, 1);
      host->base.release(&host->base);
    }
    // Let CEF run the close through; on_before_close clears g_browser.
    for (int i = 0; i < 100 && g_browser != NULL; i++) {
      p_do_message_loop_work();
    }
  }
  if (p_shutdown != NULL) {
    p_shutdown();
  }
}
