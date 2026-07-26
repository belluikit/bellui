//go:build !ci && !ios && !wasm && !test_web_driver && !mobile

#import <Foundation/Foundation.h>
#import <AppKit/AppKit.h>

// shareItems presents an NSSharingServicePicker for the given items (plain
// strings, or file URLs when isFile != 0), anchored at the centre of the key
// window's content view. Returns 0 when the request was accepted, nonzero when
// there is nothing to share. The picker itself is shown asynchronously on the
// main thread; if no window is available it silently does nothing.
int shareItems(const char **items, int count, int isFile) {
    if (count <= 0) {
        return 1;
    }

    NSMutableArray *objects = [NSMutableArray arrayWithCapacity:count];
    for (int i = 0; i < count; i++) {
        NSString *s = [NSString stringWithUTF8String:items[i]];
        if (isFile) {
            [objects addObject:[NSURL fileURLWithPath:s]];
        } else {
            [objects addObject:s];
        }
    }
    if ([objects count] == 0) {
        return 1;
    }

    dispatch_async(dispatch_get_main_queue(), ^{
        NSWindow *win = [NSApp keyWindow];
        if (win == nil) {
            win = [NSApp mainWindow];
        }
        NSView *view = [win contentView];
        if (view == nil) {
            return;
        }

        NSSharingServicePicker *picker =
            [[NSSharingServicePicker alloc] initWithItems:objects];
        NSRect bounds = [view bounds];
        [picker showRelativeToRect:NSMakeRect(NSMidX(bounds), NSMidY(bounds), 1, 1)
                            ofView:view
                     preferredEdge:NSMinYEdge];
    });
    return 0;
}
