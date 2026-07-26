//go:build darwin && !ios && !wasm && !js

#import <Foundation/Foundation.h>
#import <AppKit/AppKit.h>
#include <string.h>
#include "dragsource_darwin.h"

// belluiDragOutCompleted is the Go-exported drag completion callback (see
// dragsource_darwin.go); accepted is non-zero when a target took the drop.
extern void belluiDragOutCompleted(int accepted);

// BelluiDragSource is the NSDraggingSource for every drag-out. A single retained
// instance serves all drags (it carries no per-drag state), so it is never
// released — the counterpart to the retained menu scaffolding, avoiding a
// dangling source during AppKit's asynchronous drag session.
@interface BelluiDragSource : NSObject <NSDraggingSource>
@end

@implementation BelluiDragSource
- (NSDragOperation)draggingSession:(NSDraggingSession *)session
    sourceOperationMaskForDraggingContext:(NSDraggingContext)context {
    return NSDragOperationCopy | NSDragOperationGeneric;
}

// draggingSession:endedAtPoint:operation: reports the final drop outcome. A
// non-NSDragOperationNone operation means a target accepted the drop; forward it
// to Go so an optional OnCompleted callback can fire on the main goroutine.
- (void)draggingSession:(NSDraggingSession *)session
           endedAtPoint:(NSPoint)screenPoint
              operation:(NSDragOperation)operation {
    belluiDragOutCompleted(operation != NSDragOperationNone ? 1 : 0);
}
@end

static BelluiDragSource *gBelluiDragSource = nil;

// dragImageOrDefault returns the supplied image, else the file's Finder icon,
// else the application icon — so a drag always has a visible representation.
static NSImage *dragImageOrDefault(NSImage *supplied, NSString *forFile) {
    if (supplied != nil) {
        return supplied;
    }
    if (forFile != nil) {
        return [[NSWorkspace sharedWorkspace] iconForFile:forFile];
    }
    return [NSApp applicationIconImage];
}

bool belluiStartDragOut(const char* text, const char* filePathsNL, const void* imageData, int imageLen) {
    NSWindow *window = [NSApp keyWindow];
    if (window == nil) {
        window = [NSApp mainWindow];
    }
    if (window == nil) {
        return false;
    }
    NSView *view = [window contentView];
    if (view == nil) {
        return false;
    }

    // The session must be seeded by the in-flight mouse event; a drag-out is
    // always started from a Dragged handler, so [NSApp currentEvent] is it.
    NSEvent *event = [NSApp currentEvent];
    if (event == nil) {
        return false;
    }
    NSPoint loc = [event locationInWindow];

    NSImage *supplied = nil;
    if (imageData != NULL && imageLen > 0) {
        NSData *data = [NSData dataWithBytes:imageData length:(NSUInteger)imageLen];
        supplied = [[[NSImage alloc] initWithData:data] autorelease];
    }

    NSMutableArray<NSDraggingItem *> *items = [NSMutableArray array];

    if (filePathsNL != NULL && strlen(filePathsNL) > 0) {
        NSString *joined = [NSString stringWithUTF8String:filePathsNL];
        for (NSString *path in [joined componentsSeparatedByString:@"\n"]) {
            if ([path length] == 0) {
                continue;
            }
            NSURL *url = [NSURL fileURLWithPath:path];
            NSDraggingItem *item = [[[NSDraggingItem alloc] initWithPasteboardWriter:url] autorelease];
            NSImage *img = dragImageOrDefault(supplied, path);
            [item setDraggingFrame:NSMakeRect(loc.x, loc.y, img.size.width, img.size.height) contents:img];
            [items addObject:item];
        }
    }

    if (text != NULL && strlen(text) > 0) {
        NSString *str = [NSString stringWithUTF8String:text];
        NSDraggingItem *item = [[[NSDraggingItem alloc] initWithPasteboardWriter:str] autorelease];
        NSImage *img = dragImageOrDefault(supplied, nil);
        [item setDraggingFrame:NSMakeRect(loc.x, loc.y, img.size.width, img.size.height) contents:img];
        [items addObject:item];
    }

    if ([items count] == 0) {
        return false;
    }

    if (gBelluiDragSource == nil) {
        gBelluiDragSource = [[BelluiDragSource alloc] init]; // retained for process lifetime
    }

    // beginDraggingSessionWithItems:event:source: is non-blocking — AppKit drives
    // the drag in its own loop, so bellui's event loop is not stalled.
    [view beginDraggingSessionWithItems:items event:event source:gBelluiDragSource];
    return true;
}
