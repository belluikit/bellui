//go:build darwin && !wasm && !test_web_driver

#import <AppKit/AppKit.h>
#import <Foundation/Foundation.h>

#include <stdlib.h>
#include <string.h>

#include "clipboard_flavors_darwin.h"

// This file is compiled with manual retain/release (no ARC), matching the
// other ObjC bridges in this driver: alloc'd objects are released explicitly
// and every entry point runs inside its own autorelease pool. Returned
// buffers are malloc'd copies — never interior pointers like [s UTF8String].

void *belluiClipboardGetImage(int *length) {
	@autoreleasepool {
		NSPasteboard *pb = [NSPasteboard generalPasteboard];
		NSData *png = [pb dataForType:NSPasteboardTypePNG];
		if (png == nil) {
			NSData *tiff = [pb dataForType:NSPasteboardTypeTIFF];
			if (tiff != nil) {
				NSBitmapImageRep *rep = [[NSBitmapImageRep alloc] initWithData:tiff];
				if (rep != nil) {
					png = [rep representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
					[rep release];
				}
			}
		}
		if (png == nil || [png length] == 0) {
			*length = 0;
			return NULL;
		}

		void *buf = malloc([png length]);
		if (buf == NULL) {
			*length = 0;
			return NULL;
		}
		memcpy(buf, [png bytes], [png length]);
		*length = (int)[png length];
		return buf;
	}
}

bool belluiClipboardSetImage(const void *png, int length) {
	@autoreleasepool {
		NSData *data = [NSData dataWithBytes:png length:length];
		NSPasteboard *pb = [NSPasteboard generalPasteboard];
		[pb declareTypes:@[ NSPasteboardTypePNG, NSPasteboardTypeTIFF ] owner:nil];
		bool ok = [pb setData:data forType:NSPasteboardTypePNG];

		NSBitmapImageRep *rep = [[NSBitmapImageRep alloc] initWithData:data];
		if (rep != nil) {
			NSData *tiff = [rep TIFFRepresentation];
			if (tiff != nil) {
				[pb setData:tiff forType:NSPasteboardTypeTIFF];
			}
			[rep release];
		}
		return ok;
	}
}

char **belluiClipboardGetFiles(int *count) {
	@autoreleasepool {
		*count = 0;
		NSPasteboard *pb = [NSPasteboard generalPasteboard];
		NSArray *urls = [pb readObjectsForClasses:@[ [NSURL class] ]
		                                  options:@{NSPasteboardURLReadingFileURLsOnlyKey : @YES}];
		if (urls == nil || [urls count] == 0) {
			return NULL;
		}

		char **paths = malloc(sizeof(char *) * [urls count]);
		if (paths == NULL) {
			return NULL;
		}
		int n = 0;
		for (NSURL *url in urls) {
			const char *path = [[url path] UTF8String];
			if (path == NULL) {
				continue;
			}
			paths[n] = strdup(path);
			if (paths[n] != NULL) {
				n++;
			}
		}
		if (n == 0) {
			free(paths);
			return NULL;
		}
		*count = n;
		return paths;
	}
}

bool belluiClipboardSetFiles(const char **paths, int count) {
	@autoreleasepool {
		NSMutableArray *urls = [NSMutableArray arrayWithCapacity:count];
		for (int i = 0; i < count; i++) {
			NSString *path = [NSString stringWithUTF8String:paths[i]];
			if (path == nil) {
				continue;
			}
			NSURL *url = [NSURL fileURLWithPath:path];
			if (url != nil) {
				[urls addObject:url];
			}
		}

		NSPasteboard *pb = [NSPasteboard generalPasteboard];
		[pb clearContents];
		if ([urls count] == 0) {
			return false;
		}
		bool ok = [pb writeObjects:urls];

		// writeObjects: registers promise-backed data; without a reader the
		// promises beyond the first item are lost when the app terminates.
		// Reading every item's data back forces resolution into the
		// pasteboard server so the file list outlives this process.
		for (NSPasteboardItem *item in [pb pasteboardItems]) {
			for (NSString *type in [item types]) {
				[item dataForType:type];
			}
		}
		// The readback resolves data promises, but delivery of the items
		// beyond the first to the pasteboard server is still asynchronous —
		// an immediate process exit loses them (see belluiClipboardSetFlavors).
		if ([urls count] > 1) {
			usleep(50000);
		}
		return ok;
	}
}

char *belluiClipboardGetHTML(void) {
	@autoreleasepool {
		NSString *html = [[NSPasteboard generalPasteboard] stringForType:NSPasteboardTypeHTML];
		if (html == nil) {
			return NULL;
		}
		const char *utf8 = [html UTF8String];
		if (utf8 == NULL) {
			return NULL;
		}
		return strdup(utf8);
	}
}

bool belluiClipboardSetFlavors(const char *text, const char *html,
                             const void *png, int pngLen,
                             const char **paths, int pathCount) {
	@autoreleasepool {
		NSMutableArray *items = [NSMutableArray array];

		// The text, HTML and image flavors are alternative representations
		// of the same logical content, so they share one pasteboard item.
		NSPasteboardItem *main = [[[NSPasteboardItem alloc] init] autorelease];
		bool hasMain = false;
		if (text != NULL) {
			NSString *s = [NSString stringWithUTF8String:text];
			if (s != nil) {
				[main setString:s forType:NSPasteboardTypeString];
				hasMain = true;
			}
		}
		if (html != NULL) {
			NSString *s = [NSString stringWithUTF8String:html];
			if (s != nil) {
				[main setString:s forType:NSPasteboardTypeHTML];
				hasMain = true;
			}
		}
		if (png != NULL && pngLen > 0) {
			NSData *data = [NSData dataWithBytes:png length:pngLen];
			[main setData:data forType:NSPasteboardTypePNG];
			NSBitmapImageRep *rep = [[NSBitmapImageRep alloc] initWithData:data];
			if (rep != nil) {
				NSData *tiff = [rep TIFFRepresentation];
				if (tiff != nil) {
					[main setData:tiff forType:NSPasteboardTypeTIFF];
				}
				[rep release];
			}
			hasMain = true;
		}
		if (hasMain) {
			[items addObject:main];
		}

		for (int i = 0; i < pathCount; i++) {
			NSString *path = [NSString stringWithUTF8String:paths[i]];
			if (path == nil) {
				continue;
			}
			NSURL *url = [NSURL fileURLWithPath:path];
			if (url == nil) {
				continue;
			}
			NSPasteboardItem *file = [[[NSPasteboardItem alloc] init] autorelease];
			[file setString:[url absoluteString] forType:NSPasteboardTypeFileURL];
			[items addObject:file];
		}

		NSPasteboard *pb = [NSPasteboard generalPasteboard];
		[pb clearContents];
		if ([items count] == 0) {
			return true; // an all-absent write is a clear
		}
		bool ok = [pb writeObjects:items];

		// Even with data set directly on the items, writeObjects: delivers
		// items beyond the first to the pasteboard server asynchronously; a
		// process that exits right after the call loses them (the readback
		// trick used for promise-backed NSURLs does not help — it is served
		// from the local cache). A short wait lets that delivery complete;
		// single-item writes are delivered synchronously and skip it.
		if ([items count] > 1) {
			usleep(50000);
		}
		return ok;
	}
}

void belluiClipboardSetHTML(const char *html, const char *plain) {
	@autoreleasepool {
		NSString *htmlStr = [NSString stringWithUTF8String:html];
		NSString *plainStr = [NSString stringWithUTF8String:plain];
		if (htmlStr == nil || plainStr == nil) {
			return;
		}

		NSPasteboard *pb = [NSPasteboard generalPasteboard];
		[pb declareTypes:@[ NSPasteboardTypeHTML, NSPasteboardTypeString ] owner:nil];
		[pb setString:htmlStr forType:NSPasteboardTypeHTML];
		[pb setString:plainStr forType:NSPasteboardTypeString];
	}
}
