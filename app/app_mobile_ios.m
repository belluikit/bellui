//go:build !ci && ios

#import <UIKit/UIKit.h>

void openURL(char *urlStr) {
    UIApplication *app = [UIApplication sharedApplication];
    NSURL *url = [NSURL URLWithString:[NSString stringWithUTF8String:urlStr]];
    [app openURL:url options:@{} completionHandler:nil];
}

// shareItem presents a UIActivityViewController (the iOS share sheet) for the
// given value: plain text, or a file URL when isFile != 0 (pre-rebrand tracker #711). It is
// shown from the top-most presented view controller on the key window. On iPad
// the sheet is a popover, which UIKit requires be anchored to a source rect —
// omitting it raises an exception — so it is centred on the presenting view.
void shareItem(char *value, int isFile) {
    NSString *s = [NSString stringWithUTF8String:value];
    id item = isFile ? (id)[NSURL fileURLWithPath:s] : (id)s;

    dispatch_async(dispatch_get_main_queue(), ^{
        UIApplication *app = [UIApplication sharedApplication];
        UIWindow *win = nil;
        for (UIWindow *w in app.windows) {
            if (w.isKeyWindow) {
                win = w;
                break;
            }
        }
        if (win == nil) {
            win = app.windows.firstObject;
        }
        UIViewController *root = win.rootViewController;
        if (root == nil) {
            return;
        }
        while (root.presentedViewController != nil) {
            root = root.presentedViewController;
        }

        UIActivityViewController *vc =
            [[UIActivityViewController alloc] initWithActivityItems:@[item]
                                              applicationActivities:nil];
        UIPopoverPresentationController *pop = vc.popoverPresentationController;
        if (pop != nil) {
            pop.sourceView = root.view;
            CGRect b = root.view.bounds;
            pop.sourceRect = CGRectMake(CGRectGetMidX(b), CGRectGetMidY(b), 1, 1);
            pop.permittedArrowDirections = 0;
        }
        [root presentViewController:vc animated:YES completion:nil];
    });
}

