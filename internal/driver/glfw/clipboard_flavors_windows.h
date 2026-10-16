//go:build windows && !wasm && !test_web_driver

// Win32 clipboard bridge for the clipboard flavor interfaces (pre-rebrand tracker #513).
// Ownership contract: every pointer returned to Go is a malloc'd copy that
// Go frees; pointers passed in are borrowed and copied before returning.
//
// Status codes let Go retry only genuine open failures (the classic
// "Access Denied" while another process holds the clipboard, PR#1679)
// instead of treating an absent flavor as an error.
#define BELLUI_CB_OK 0
#define BELLUI_CB_NO_DATA 1  // clipboard opened fine, flavor absent
#define BELLUI_CB_ERR_OPEN 2 // OpenClipboard failed - worth retrying
#define BELLUI_CB_ERR_OTHER 3

// belluiWinClipboardGetImage copies the clipboard image bytes: the registered
// "PNG" format when available (*format = 1), otherwise CF_DIB (*format = 2).
int belluiWinClipboardGetImage(void **data, int *length, int *format);

// belluiWinClipboardSetImage replaces the clipboard content with the image in
// both the registered "PNG" format and CF_DIB in one transaction.
int belluiWinClipboardSetImage(const void *png, int pngLen, const void *dib, int dibLen);

// belluiWinClipboardGetFiles copies the CF_HDROP file list as one malloc'd
// buffer of NUL-separated, double-NUL-terminated UTF-8 paths.
int belluiWinClipboardGetFiles(char **paths, int *count, int *length);

// belluiWinClipboardSetFiles replaces the clipboard content with a CF_HDROP
// list built from UTF-8 paths.
int belluiWinClipboardSetFiles(const char **utf8paths, int count);

// belluiWinClipboardGetHTML copies the raw registered "HTML Format" payload
// (CF_HTML header block included) as a malloc'd NUL-terminated string.
int belluiWinClipboardGetHTML(char **payload);

// belluiWinClipboardSetHTML replaces the clipboard content with the prepared
// CF_HTML payload plus a CF_UNICODETEXT companion in one transaction.
int belluiWinClipboardSetHTML(const char *cfhtml, const char *plainUtf8);

// belluiWinClipboardSetFlavors replaces the clipboard content with every
// present flavor in one Empty+SetClipboardData transaction: CF_UNICODETEXT,
// the registered "HTML Format" (prepared CF_HTML payload), "PNG" + CF_DIB
// and a CF_HDROP list. NULL/empty arguments mark a flavor absent; an
// all-absent call just clears the clipboard.
int belluiWinClipboardSetFlavors(const char *plainUtf8, const char *cfhtml,
                               const void *png, int pngLen,
                               const void *dib, int dibLen,
                               const char **utf8paths, int fileCount);
