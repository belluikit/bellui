//go:build !ci && !mobile && !tinygo

#import <UIKit/UIKit.h>
#import <UserNotifications/UserNotifications.h>

// applyIOSBadge sets the icon badge on the main thread. iOS 16+ exposes the
// non-deprecated UNUserNotificationCenter setBadgeCount:; older systems use the
// UIApplication API.
static void applyIOSBadge(long count) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (@available(iOS 16.0, *)) {
            [[UNUserNotificationCenter currentNotificationCenter]
                setBadgeCount:(NSInteger)count withCompletionHandler:nil];
        } else {
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
            [[UIApplication sharedApplication] setApplicationIconBadgeNumber:(NSInteger)count];
#pragma clang diagnostic pop
        }
    });
}

// setIOSBadge sets the application icon badge number (0 clears it). Unlike the
// macOS dock badge, the iOS icon badge only displays if the app holds badge
// authorization, so request it first (the system prompts once, then caches the
// grant and returns immediately on later calls) and apply the count in the
// completion handler.
void setIOSBadge(long count) {
    UNUserNotificationCenter *center = [UNUserNotificationCenter currentNotificationCenter];
    [center requestAuthorizationWithOptions:UNAuthorizationOptionBadge
        completionHandler:^(BOOL granted, NSError *_Nullable error) {
            if (error != nil) {
                NSLog(@"Error requesting badge authorization: %@", error);
            }
            applyIOSBadge(count);
        }];
}
