//go:build darwin && !wasm && !test_web_driver

#import <stdbool.h>

// NSPasteboard bridge for the clipboard flavor interfaces (pre-rebrand tracker #513).
// Ownership contract: every pointer returned to Go is a malloc'd copy that
// Go frees; pointers passed in are borrowed and copied before returning.

// belluiClipboardGetImage returns malloc'd PNG bytes for the pasteboard image
// (converting a TIFF-only pasteboard), or NULL when no image is present.
void *belluiClipboardGetImage(int *length);

// belluiClipboardSetImage replaces the pasteboard content with the given PNG,
// writing a TIFF companion for older consumers.
bool belluiClipboardSetImage(const void *png, int length);

// belluiClipboardGetFiles returns a malloc'd array of malloc'd UTF-8 file
// paths, or NULL when the pasteboard holds no file URLs.
char **belluiClipboardGetFiles(int *count);

// belluiClipboardSetFiles replaces the pasteboard content with file URLs.
bool belluiClipboardSetFiles(const char **paths, int count);

// belluiClipboardGetHTML returns the malloc'd HTML flavor, or NULL if absent.
char *belluiClipboardGetHTML(void);

// belluiClipboardSetHTML replaces the pasteboard content with the HTML flavor
// plus its plain-text companion in one declaration.
void belluiClipboardSetHTML(const char *html, const char *plain);

// belluiClipboardSetFlavors replaces the pasteboard content with every present
// flavor in one transaction: text/html/png (NULL/empty = absent) share one
// pasteboard item, each file path becomes a file-URL item. An all-absent
// call just clears the pasteboard.
bool belluiClipboardSetFlavors(const char *text, const char *html,
                             const void *png, int pngLen,
                             const char **paths, int pathCount);
