//go:build windows && !wasm && !test_web_driver

// Plain C over the Win32 clipboard API - no C++, so it builds under both CI
// mingw flavors (winlibs gcc x64 and llvm-mingw arm64), matching the UIA
// bridge. All calls arrive on the main thread via runOnClipboardThread; the
// clipboard is opened with a NULL owner window, which is sufficient for the
// immediate (non-delayed) rendering used here.

#include <windows.h>
#include <shellapi.h>
#include <shlobj.h> // DROPFILES
#include <stdlib.h>
#include <string.h>

#include "clipboard_flavors_windows.h"

static UINT belluiPngFormat(void) {
	static UINT fmt = 0;
	if (fmt == 0) {
		fmt = RegisterClipboardFormatW(L"PNG");
	}
	return fmt;
}

static UINT belluiHTMLFormat(void) {
	static UINT fmt = 0;
	if (fmt == 0) {
		fmt = RegisterClipboardFormatW(L"HTML Format");
	}
	return fmt;
}

// copyClipboardData copies the clipboard block for the given format into
// malloc'd memory. The clipboard must already be open. Handles returned by
// GetClipboardData belong to the clipboard and are never freed here.
static int copyClipboardData(UINT format, void **out, int *length) {
	HANDLE h = GetClipboardData(format);
	if (h == NULL) {
		return BELLUI_CB_NO_DATA;
	}
	SIZE_T size = GlobalSize(h);
	if (size == 0) {
		return BELLUI_CB_NO_DATA;
	}
	void *src = GlobalLock(h);
	if (src == NULL) {
		return BELLUI_CB_ERR_OTHER;
	}
	void *buf = malloc(size);
	if (buf == NULL) {
		GlobalUnlock(h);
		return BELLUI_CB_ERR_OTHER;
	}
	memcpy(buf, src, size);
	GlobalUnlock(h);
	*out = buf;
	*length = (int)size;
	return BELLUI_CB_OK;
}

// putClipboardData copies the payload into a movable global block and hands
// it to the clipboard. On success the system owns the block; it is freed
// here only when SetClipboardData rejects it.
static int putClipboardData(UINT format, const void *payload, SIZE_T size) {
	HGLOBAL h = GlobalAlloc(GMEM_MOVEABLE, size);
	if (h == NULL) {
		return BELLUI_CB_ERR_OTHER;
	}
	void *dst = GlobalLock(h);
	if (dst == NULL) {
		GlobalFree(h);
		return BELLUI_CB_ERR_OTHER;
	}
	memcpy(dst, payload, size);
	GlobalUnlock(h);
	if (SetClipboardData(format, h) == NULL) {
		GlobalFree(h);
		return BELLUI_CB_ERR_OTHER;
	}
	return BELLUI_CB_OK;
}

// putClipboardText converts UTF-8 to a CF_UNICODETEXT block and hands it to
// the (already open) clipboard.
static int putClipboardText(const char *utf8) {
	int wlen = MultiByteToWideChar(CP_UTF8, 0, utf8, -1, NULL, 0);
	if (wlen <= 0) {
		return BELLUI_CB_ERR_OTHER;
	}
	HGLOBAL h = GlobalAlloc(GMEM_MOVEABLE, (SIZE_T)wlen * sizeof(WCHAR));
	if (h == NULL) {
		return BELLUI_CB_ERR_OTHER;
	}
	WCHAR *dst = (WCHAR *)GlobalLock(h);
	if (dst == NULL) {
		GlobalFree(h);
		return BELLUI_CB_ERR_OTHER;
	}
	MultiByteToWideChar(CP_UTF8, 0, utf8, -1, dst, wlen);
	GlobalUnlock(h);
	if (SetClipboardData(CF_UNICODETEXT, h) == NULL) {
		GlobalFree(h);
		return BELLUI_CB_ERR_OTHER;
	}
	return BELLUI_CB_OK;
}

// buildDropFiles assembles a CF_HDROP global block from UTF-8 paths, or NULL
// on failure. The caller owns the block until SetClipboardData accepts it.
static HGLOBAL buildDropFiles(const char **utf8paths, int count) {
	size_t totalChars = 1; // trailing extra NUL of the double-NUL list
	for (int i = 0; i < count; i++) {
		int wlen = MultiByteToWideChar(CP_UTF8, 0, utf8paths[i], -1, NULL, 0);
		if (wlen <= 0) {
			return NULL;
		}
		totalChars += (size_t)wlen;
	}

	SIZE_T size = sizeof(DROPFILES) + totalChars * sizeof(WCHAR);
	HGLOBAL h = GlobalAlloc(GMEM_MOVEABLE, size);
	if (h == NULL) {
		return NULL;
	}
	DROPFILES *df = (DROPFILES *)GlobalLock(h);
	if (df == NULL) {
		GlobalFree(h);
		return NULL;
	}
	memset(df, 0, size);
	df->pFiles = sizeof(DROPFILES);
	df->fWide = TRUE;
	WCHAR *dst = (WCHAR *)((BYTE *)df + sizeof(DROPFILES));
	size_t remaining = totalChars;
	for (int i = 0; i < count; i++) {
		int wlen = MultiByteToWideChar(CP_UTF8, 0, utf8paths[i], -1, dst, (int)remaining);
		if (wlen <= 0) {
			GlobalUnlock(h);
			GlobalFree(h);
			return NULL;
		}
		dst += wlen;
		remaining -= (size_t)wlen;
	}
	GlobalUnlock(h);
	return h;
}

int belluiWinClipboardGetImage(void **data, int *length, int *format) {
	*data = NULL;
	*length = 0;
	*format = 0;
	if (!OpenClipboard(NULL)) {
		return BELLUI_CB_ERR_OPEN;
	}

	int status = BELLUI_CB_NO_DATA;
	if (IsClipboardFormatAvailable(belluiPngFormat())) {
		status = copyClipboardData(belluiPngFormat(), data, length);
		if (status == BELLUI_CB_OK) {
			*format = 1;
		}
	}
	if (status != BELLUI_CB_OK) {
		// The system synthesizes CF_DIB from CF_BITMAP/CF_DIBV5 writers.
		int dibStatus = copyClipboardData(CF_DIB, data, length);
		if (dibStatus == BELLUI_CB_OK) {
			*format = 2;
			status = BELLUI_CB_OK;
		} else if (status == BELLUI_CB_NO_DATA) {
			status = dibStatus;
		}
	}
	CloseClipboard();
	return status;
}

int belluiWinClipboardSetImage(const void *png, int pngLen, const void *dib, int dibLen) {
	if (!OpenClipboard(NULL)) {
		return BELLUI_CB_ERR_OPEN;
	}
	if (!EmptyClipboard()) {
		CloseClipboard();
		return BELLUI_CB_ERR_OTHER;
	}
	int status = putClipboardData(belluiPngFormat(), png, (SIZE_T)pngLen);
	int dibStatus = putClipboardData(CF_DIB, dib, (SIZE_T)dibLen);
	if (status == BELLUI_CB_OK) {
		status = dibStatus;
	}
	CloseClipboard();
	return status;
}

int belluiWinClipboardGetFiles(char **paths, int *count, int *length) {
	*paths = NULL;
	*count = 0;
	*length = 0;
	if (!OpenClipboard(NULL)) {
		return BELLUI_CB_ERR_OPEN;
	}
	HDROP drop = (HDROP)GetClipboardData(CF_HDROP);
	if (drop == NULL) {
		CloseClipboard();
		return BELLUI_CB_NO_DATA;
	}

	UINT n = DragQueryFileW(drop, 0xFFFFFFFF, NULL, 0);
	char *buf = NULL;
	size_t used = 0;
	int found = 0;
	for (UINT i = 0; i < n; i++) {
		// Size query per file - paths from the shell may exceed MAX_PATH.
		UINT wlen = DragQueryFileW(drop, i, NULL, 0);
		if (wlen == 0) {
			continue;
		}
		WCHAR *wbuf = malloc(((size_t)wlen + 1) * sizeof(WCHAR));
		if (wbuf == NULL) {
			free(buf);
			CloseClipboard();
			return BELLUI_CB_ERR_OTHER;
		}
		if (DragQueryFileW(drop, i, wbuf, wlen + 1) == 0) {
			free(wbuf);
			continue;
		}
		int u8len = WideCharToMultiByte(CP_UTF8, 0, wbuf, -1, NULL, 0, NULL, NULL);
		if (u8len <= 0) {
			free(wbuf);
			continue;
		}
		char *grown = realloc(buf, used + (size_t)u8len);
		if (grown == NULL) {
			free(wbuf);
			free(buf);
			CloseClipboard();
			return BELLUI_CB_ERR_OTHER;
		}
		buf = grown;
		WideCharToMultiByte(CP_UTF8, 0, wbuf, -1, buf + used, u8len, NULL, NULL);
		used += (size_t)u8len; // u8len includes the NUL separator
		free(wbuf);
		found++;
	}
	CloseClipboard();

	if (found == 0) {
		free(buf);
		return BELLUI_CB_NO_DATA;
	}
	char *terminated = realloc(buf, used + 1);
	if (terminated == NULL) {
		free(buf);
		return BELLUI_CB_ERR_OTHER;
	}
	terminated[used] = 0; // double-NUL termination
	*paths = terminated;
	*count = found;
	*length = (int)used;
	return BELLUI_CB_OK;
}

int belluiWinClipboardSetFiles(const char **utf8paths, int count) {
	HGLOBAL h = buildDropFiles(utf8paths, count);
	if (h == NULL) {
		return BELLUI_CB_ERR_OTHER;
	}

	if (!OpenClipboard(NULL)) {
		GlobalFree(h);
		return BELLUI_CB_ERR_OPEN;
	}
	if (!EmptyClipboard()) {
		CloseClipboard();
		GlobalFree(h);
		return BELLUI_CB_ERR_OTHER;
	}
	int status = BELLUI_CB_OK;
	if (SetClipboardData(CF_HDROP, h) == NULL) {
		GlobalFree(h);
		status = BELLUI_CB_ERR_OTHER;
	}
	CloseClipboard();
	return status;
}

int belluiWinClipboardGetHTML(char **payload) {
	*payload = NULL;
	if (!OpenClipboard(NULL)) {
		return BELLUI_CB_ERR_OPEN;
	}
	HANDLE h = GetClipboardData(belluiHTMLFormat());
	if (h == NULL) {
		CloseClipboard();
		return BELLUI_CB_NO_DATA;
	}
	SIZE_T size = GlobalSize(h);
	char *src = (char *)GlobalLock(h);
	if (src == NULL) {
		CloseClipboard();
		return BELLUI_CB_ERR_OTHER;
	}
	char *buf = malloc(size + 1);
	if (buf == NULL) {
		GlobalUnlock(h);
		CloseClipboard();
		return BELLUI_CB_ERR_OTHER;
	}
	memcpy(buf, src, size);
	buf[size] = 0;
	GlobalUnlock(h);
	CloseClipboard();
	*payload = buf;
	return BELLUI_CB_OK;
}

int belluiWinClipboardSetHTML(const char *cfhtml, const char *plainUtf8) {
	if (!OpenClipboard(NULL)) {
		return BELLUI_CB_ERR_OPEN;
	}
	if (!EmptyClipboard()) {
		CloseClipboard();
		return BELLUI_CB_ERR_OTHER;
	}
	// The registered "HTML Format" payload is UTF-8 text including its
	// terminating NUL (consumers tolerate and some expect it).
	int status = putClipboardData(belluiHTMLFormat(), cfhtml, strlen(cfhtml) + 1);

	int textStatus = putClipboardText(plainUtf8);
	if (status == BELLUI_CB_OK) {
		status = textStatus;
	}
	CloseClipboard();
	return status;
}

int belluiWinClipboardSetFlavors(const char *plainUtf8, const char *cfhtml,
                               const void *png, int pngLen,
                               const void *dib, int dibLen,
                               const char **utf8paths, int fileCount) {
	// The CF_HDROP block can fail to build; do so before opening the
	// clipboard so a failure never leaves it emptied.
	HGLOBAL drop = NULL;
	if (fileCount > 0) {
		drop = buildDropFiles(utf8paths, fileCount);
		if (drop == NULL) {
			return BELLUI_CB_ERR_OTHER;
		}
	}

	if (!OpenClipboard(NULL)) {
		if (drop != NULL) {
			GlobalFree(drop);
		}
		return BELLUI_CB_ERR_OPEN;
	}
	if (!EmptyClipboard()) {
		CloseClipboard();
		if (drop != NULL) {
			GlobalFree(drop);
		}
		return BELLUI_CB_ERR_OTHER;
	}

	// Write every present flavor, reporting the first failure while still
	// attempting the rest of the transaction.
	int status = BELLUI_CB_OK;
	int s;
	if (plainUtf8 != NULL) {
		s = putClipboardText(plainUtf8);
		if (status == BELLUI_CB_OK) {
			status = s;
		}
	}
	if (cfhtml != NULL) {
		s = putClipboardData(belluiHTMLFormat(), cfhtml, strlen(cfhtml) + 1);
		if (status == BELLUI_CB_OK) {
			status = s;
		}
	}
	if (png != NULL && pngLen > 0) {
		s = putClipboardData(belluiPngFormat(), png, (SIZE_T)pngLen);
		if (status == BELLUI_CB_OK) {
			status = s;
		}
	}
	if (dib != NULL && dibLen > 0) {
		s = putClipboardData(CF_DIB, dib, (SIZE_T)dibLen);
		if (status == BELLUI_CB_OK) {
			status = s;
		}
	}
	if (drop != NULL && SetClipboardData(CF_HDROP, drop) == NULL) {
		GlobalFree(drop);
		if (status == BELLUI_CB_OK) {
			status = BELLUI_CB_ERR_OTHER;
		}
	}
	CloseClipboard();
	return status;
}
