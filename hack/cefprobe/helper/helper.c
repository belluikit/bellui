// CEF subprocess helper (#60 phase 0, belluikit/bellui#102).
//
// CEF is multi-process: the renderer, GPU and utility processes are launches of
// an executable whose entry point calls cef_execute_process. That executable is
// deliberately **not** the Go binary. The Go runtime starts threads before main
// runs, and Chromium's zygote forks — so re-executing a Go program as a
// Chromium subprocess is a fight, and every Go/CEF integration that works uses
// a separate helper. CefSettings.browser_subprocess_path points here.
//
// It stays tiny and dependency-free: libcef.so is found beside this binary via
// /proc/self/exe, so the helper needs no rpath, no link-time CEF, and no
// environment.
//
// It lives in its own directory on purpose: cgo compiles every .c file in a
// package directory, so a second main() beside main.go fails the link.
//
// Build (see setup.sh):
//   cc -o cefprobe_helper helper/helper.c -ldl

#define CEF_API_VERSION 15101

#include <dlfcn.h>
#include <libgen.h>
#include <stdio.h>
#include <string.h>
#include <unistd.h>

#include "include/capi/cef_app_capi.h"

int main(int argc, char **argv) {
  char exe[1024];
  ssize_t n = readlink("/proc/self/exe", exe, sizeof(exe) - 1);
  if (n < 0) {
    fprintf(stderr, "cefprobe_helper: cannot read /proc/self/exe\n");
    return 1;
  }
  exe[n] = '\0';

  char lib[1100];
  snprintf(lib, sizeof(lib), "%s/libcef.so", dirname(exe));

  void *handle = dlopen(lib, RTLD_NOW | RTLD_GLOBAL);
  if (handle == NULL) {
    fprintf(stderr, "cefprobe_helper: cannot load %s: %s\n", lib, dlerror());
    return 1;
  }

  // The version handshake, before any other CEF call — see cef.c for what
  // skipping it looks like. The subprocess crosses the same C→C++ boundary as
  // the browser process, so it needs the same configuration.
  const char *(*api_hash)(int, int) = dlsym(handle, "cef_api_hash");
  if (api_hash == NULL) {
    fprintf(stderr, "cefprobe_helper: libcef.so has no cef_api_hash\n");
    return 1;
  }
  api_hash(CEF_API_VERSION, 0);

  int (*execute_process)(const cef_main_args_t *, cef_app_t *, void *) =
      dlsym(handle, "cef_execute_process");
  if (execute_process == NULL) {
    fprintf(stderr, "cefprobe_helper: libcef.so has no cef_execute_process\n");
    return 1;
  }

  cef_main_args_t args;
  args.argc = argc;
  args.argv = argv;

  // Blocks until the subprocess should exit, then reports its exit code.
  return execute_process(&args, NULL, NULL);
}
