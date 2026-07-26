//go:build !ci && !ios && !wasm && !test_web_driver && !mobile

#import <AppKit/AppKit.h>

// setDockBadge sets the application's dock-tile badge label. An empty string
// clears it. The dock tile is UI state, so mutate it on the main thread.
void setDockBadge(char *cLabel) {
    NSString *label = [NSString stringWithUTF8String:cLabel];
    dispatch_async(dispatch_get_main_queue(), ^{
        [[NSApp dockTile] setBadgeLabel:[label length] > 0 ? label : nil];
    });
}
