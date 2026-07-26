//go:build ios

#import <UIKit/UIKit.h>

#include <stdbool.h>
#include <stdlib.h>
#include <string.h>

// belluiClipboardSetImage places a PNG-encoded image (len bytes at data) on the
// general pasteboard. UIPasteboard is thread-safe, so no main-thread hop is
// needed. Returns false if the bytes don't decode to an image.
bool belluiClipboardSetImage(void *data, int len) {
    @autoreleasepool {
        NSData *png = [NSData dataWithBytes:data length:len];
        UIImage *img = [UIImage imageWithData:png];
        if (img == nil) {
            return false;
        }
        [UIPasteboard generalPasteboard].image = img;
        return true;
    }
}

// registerFlavorData adds one eagerly-loadable data representation to the
// item provider. Registration order is NSItemProvider fidelity order.
static void registerFlavorData(NSItemProvider *provider, NSString *uti, NSData *data) {
    [provider registerDataRepresentationForTypeIdentifier:uti
                                               visibility:NSItemProviderRepresentationVisibilityAll
                                              loadHandler:^NSProgress *(void (^completionHandler)(NSData *, NSError *)) {
        completionHandler(data, nil);
        return nil;
    }];
}

// belluiClipboardSetFlavors replaces the pasteboard content with every present
// flavor in one transaction (fyne#1142): a single pasteboard item carrying
// the HTML/PNG/text representations. The item is published as an
// NSItemProvider so the representations carry an explicit fidelity order
// (HTML, then PNG, then plain text) — a bare UIPasteboard items dictionary
// with all three UTIs has no such order and Notes pastes nothing from it
// (empirically; any two of the three paste fine). NULL/empty arguments mark
// a flavor absent; an all-absent call clears the pasteboard.
bool belluiClipboardSetFlavors(const char *text, const char *html, void *png, int pngLen) {
    @autoreleasepool {
        NSItemProvider *provider = [[[NSItemProvider alloc] init] autorelease];
        int flavors = 0;
        if (html != NULL) {
            NSString *s = [NSString stringWithUTF8String:html];
            if (s != nil) {
                registerFlavorData(provider, @"public.html", [s dataUsingEncoding:NSUTF8StringEncoding]);
                flavors++;
            }
        }
        if (png != NULL && pngLen > 0) {
            registerFlavorData(provider, @"public.png", [NSData dataWithBytes:png length:pngLen]);
            flavors++;
        }
        if (text != NULL) {
            NSString *s = [NSString stringWithUTF8String:text];
            if (s != nil) {
                registerFlavorData(provider, @"public.utf8-plain-text", [s dataUsingEncoding:NSUTF8StringEncoding]);
                flavors++;
            }
        }

        UIPasteboard *pb = [UIPasteboard generalPasteboard];
        if (flavors == 0) {
            pb.items = @[]; // an all-absent write is a clear
            return true;
        }
        pb.itemProviders = @[ provider ];
        return true;
    }
}

// belluiClipboardGetImage returns malloc'd PNG bytes of the pasteboard image (or
// NULL when none); the Go caller frees the buffer and sets *outLen to the length.
void *belluiClipboardGetImage(int *outLen) {
    @autoreleasepool {
        *outLen = 0;
        UIPasteboard *pb = [UIPasteboard generalPasteboard];
        if (!pb.hasImages) {
            return NULL;
        }
        UIImage *img = pb.image;
        if (img == nil) {
            return NULL;
        }
        NSData *png = UIImagePNGRepresentation(img);
        if (png == nil || png.length == 0) {
            return NULL;
        }
        void *buf = malloc(png.length);
        if (buf == NULL) {
            return NULL;
        }
        memcpy(buf, png.bytes, png.length);
        *outLen = (int)png.length;
        return buf;
    }
}
