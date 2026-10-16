//go:build darwin && !ios && !wasm && !js

#import <AppKit/AppKit.h>
#import <Foundation/Foundation.h>
#import <UniformTypeIdentifiers/UniformTypeIdentifiers.h>

#include <stdlib.h>

#include "file_darwin_native.h"

// Exported from Go (file_darwin_native.go). Called on the main thread from the
// panel completion handler with the chosen paths (count 0 when cancelled).
extern void belluiFileDialogResponse(int handle, const char **paths, int count, int cancelled);

// This file is compiled with manual retain/release (no ARC), matching the
// other ObjC bridges in this driver. Each entry point runs inside its own
// autorelease pool. The panel is retained for the lifetime of the async
// presentation and released in the completion handler.

// buildAllowedTypes converts the extension and MIME lists into an array of
// UTType for -[NSSavePanel setAllowedContentTypes:] (macOS 11+). Returns nil
// when there is nothing to restrict, leaving the panel unfiltered.
static NSArray *buildAllowedTypes(const char **exts, int extCount,
                                  const char **mimes, int mimeCount)
    API_AVAILABLE(macos(11.0)) {
	NSMutableArray<UTType *> *types = [NSMutableArray array];

	for (int i = 0; i < extCount; i++) {
		NSString *ext = [NSString stringWithUTF8String:exts[i]];
		UTType *t = [UTType typeWithFilenameExtension:ext];
		if (t != nil) {
			[types addObject:t];
		}
	}
	for (int i = 0; i < mimeCount; i++) {
		NSString *mime = [NSString stringWithUTF8String:mimes[i]];
		UTType *t = [UTType typeWithMIMEType:mime];
		if (t != nil) {
			[types addObject:t];
		}
	}

	if ([types count] == 0) {
		return nil;
	}
	return types;
}

// legacyAllowedFileTypes is the pre-macOS-11 fallback: an array of bare
// extension strings for -[NSSavePanel setAllowedFileTypes:]. MIME filters
// cannot be expressed this way and are ignored (extension filters are the
// common case; the MIME path is covered on macOS 11+).
static NSArray *legacyAllowedFileTypes(const char **exts, int extCount) {
	if (extCount == 0) {
		return nil;
	}
	NSMutableArray<NSString *> *types = [NSMutableArray array];
	for (int i = 0; i < extCount; i++) {
		[types addObject:[NSString stringWithUTF8String:exts[i]]];
	}
	return types;
}

static void applyFilters(NSSavePanel *panel, const char **exts, int extCount,
                         const char **mimes, int mimeCount,
                         const char **legacyExts, int legacyExtCount) {
	if (@available(macOS 11.0, *)) {
		NSArray *types = buildAllowedTypes(exts, extCount, mimes, mimeCount);
		if (types != nil) {
			[panel setAllowedContentTypes:types];
		}
	} else {
		// Pre-11 setAllowedFileTypes: takes bare extensions only, so MIME
		// filters are pre-mapped to extensions on the Go side (pre-rebrand tracker #632); use that
		// combined list rather than the extension-only one.
		NSArray *types = legacyAllowedFileTypes(legacyExts, legacyExtCount);
		if (types != nil) {
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
			[panel setAllowedFileTypes:types];
#pragma clang diagnostic pop
		}
	}
}

void belluiShowFilePanel(
    uintptr_t parentWindow,
    int forSave,
    int directory,
    int multi,
    const char *title,
    const char *prompt,
    const char *initialDir,
    const char *initialName,
    const char **exts, int extCount,
    const char **mimes, int mimeCount,
    const char **legacyExts, int legacyExtCount,
    int handle) {
	// Copy every borrowed C buffer into ObjC objects before returning; the Go
	// caller frees its buffers as soon as this function returns.
	NSString *titleStr = title != NULL ? [NSString stringWithUTF8String:title] : nil;
	NSString *promptStr = (prompt != NULL && prompt[0] != '\0')
	                          ? [NSString stringWithUTF8String:prompt] : nil;
	NSString *initialDirStr = (initialDir != NULL && initialDir[0] != '\0')
	                              ? [NSString stringWithUTF8String:initialDir] : nil;
	NSString *initialNameStr = (initialName != NULL && initialName[0] != '\0')
	                               ? [NSString stringWithUTF8String:initialName] : nil;

	NSMutableArray<NSString *> *extList = [NSMutableArray array];
	for (int i = 0; i < extCount; i++) {
		[extList addObject:[NSString stringWithUTF8String:exts[i]]];
	}
	NSMutableArray<NSString *> *mimeList = [NSMutableArray array];
	for (int i = 0; i < mimeCount; i++) {
		[mimeList addObject:[NSString stringWithUTF8String:mimes[i]]];
	}
	NSMutableArray<NSString *> *legacyExtList = [NSMutableArray array];
	for (int i = 0; i < legacyExtCount; i++) {
		[legacyExtList addObject:[NSString stringWithUTF8String:legacyExts[i]]];
	}

	dispatch_async(dispatch_get_main_queue(), ^{
		@autoreleasepool {
			NSSavePanel *panel;
			if (forSave) {
				panel = [NSSavePanel savePanel];
			} else {
				NSOpenPanel *open = [NSOpenPanel openPanel];
				[open setCanChooseFiles:directory ? NO : YES];
				[open setCanChooseDirectories:directory ? YES : NO];
				[open setAllowsMultipleSelection:multi ? YES : NO];
				[open setResolvesAliases:YES];
				panel = open;
			}
			[panel retain];

			if (titleStr != nil) {
				[panel setMessage:titleStr];
			}
			if (promptStr != nil) {
				[panel setPrompt:promptStr];
			}
			if (initialDirStr != nil) {
				[panel setDirectoryURL:[NSURL fileURLWithPath:initialDirStr
				                                  isDirectory:YES]];
			}
			if (forSave && initialNameStr != nil) {
				[panel setNameFieldStringValue:initialNameStr];
			}

			// Only file selection honours a type filter; a folder picker and a
			// save panel with a fixed name keep every location reachable.
			if (!directory) {
				NSUInteger extC = [extList count];
				NSUInteger mimeC = [mimeList count];
				NSUInteger legacyC = [legacyExtList count];
				const char **extPtrs = extC ? malloc(sizeof(char *) * extC) : NULL;
				const char **mimePtrs = mimeC ? malloc(sizeof(char *) * mimeC) : NULL;
				const char **legacyPtrs = legacyC ? malloc(sizeof(char *) * legacyC) : NULL;
				for (NSUInteger i = 0; i < extC; i++) {
					extPtrs[i] = [[extList objectAtIndex:i] UTF8String];
				}
				for (NSUInteger i = 0; i < mimeC; i++) {
					mimePtrs[i] = [[mimeList objectAtIndex:i] UTF8String];
				}
				for (NSUInteger i = 0; i < legacyC; i++) {
					legacyPtrs[i] = [[legacyExtList objectAtIndex:i] UTF8String];
				}
				applyFilters(panel, extPtrs, (int)extC, mimePtrs, (int)mimeC,
				             legacyPtrs, (int)legacyC);
				free(extPtrs);
				free(mimePtrs);
				free(legacyPtrs);
			}

			void (^completion)(NSModalResponse) = ^(NSModalResponse result) {
				// Collect the chosen file URLs. A save panel and a single-select
				// open panel expose one URL; a multi-select open panel exposes an
				// array (pre-rebrand tracker #716). Every entry is copied into a C array that lives
				// only for the duration of the Go callback.
				NSMutableArray<NSString *> *paths = [NSMutableArray array];
				if (result == NSModalResponseOK) {
					if (!forSave && multi &&
					    [panel isKindOfClass:[NSOpenPanel class]]) {
						for (NSURL *url in [(NSOpenPanel *)panel URLs]) {
							if (url != nil && [url isFileURL]) {
								[paths addObject:[url path]];
							}
						}
					} else {
						NSURL *url = [panel URL];
						if (url != nil && [url isFileURL]) {
							[paths addObject:[url path]];
						}
					}
				}

				int count = (int)[paths count];
				const char **cpaths = count ? malloc(sizeof(char *) * count) : NULL;
				for (int i = 0; i < count; i++) {
					cpaths[i] = [[paths objectAtIndex:i] UTF8String];
				}
				belluiFileDialogResponse(handle, cpaths, count, count == 0 ? 1 : 0);
				free(cpaths);
				[panel release];
			};

			NSWindow *parent = (NSWindow *)parentWindow;
			if (parent != nil) {
				[panel beginSheetModalForWindow:parent completionHandler:completion];
			} else {
				[panel beginWithCompletionHandler:completion];
			}
		}
	});
}
