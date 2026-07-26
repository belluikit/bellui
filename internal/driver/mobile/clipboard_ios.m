//go:build ios

#import <UIKit/UIKit.h>
#import <MobileCoreServices/MobileCoreServices.h>

#include <string.h>

void setClipboardContent(char *content) {
    @autoreleasepool {
        NSString *value = [NSString stringWithUTF8String:content];
        [[UIPasteboard generalPasteboard] setString:value];
    }
}

// Returns a malloc'd copy (or NULL); the Go caller frees it.
char *getClipboardContent() {
    @autoreleasepool {
        NSString *str = [[UIPasteboard generalPasteboard] string];
        const char *utf8 = [str UTF8String];

        return utf8 ? strdup(utf8) : NULL;
    }
}
