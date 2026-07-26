//go:build !ci && !wasm && !test_web_driver && !mobile

#import <Foundation/Foundation.h>
#if __MAC_OS_X_VERSION_MAX_ALLOWED >= 101400 || TARGET_OS_IPHONE || TARGET_IPHONE_SIMULATOR
#import <UserNotifications/UserNotifications.h>
#endif

static int notifyNum = 0;

extern void fallbackSend(char *cTitle, char *cBody);
extern void notifyCallbackDarwin(unsigned long long token, int actionIndex);

bool isBundled() {
    return [[NSBundle mainBundle] bundleIdentifier] != nil;
}

#if __MAC_OS_X_VERSION_MAX_ALLOWED >= 101400 || TARGET_OS_IPHONE || TARGET_IPHONE_SIMULATOR

// BelluiUNDelegate receives click and action-button activations for rich
// notifications and routes them back to Go by the token carried in the
// notification's userInfo. It also presents banners while the app is
// foreground, which the system otherwise suppresses.
@interface BelluiUNDelegate : NSObject <UNUserNotificationCenterDelegate>
+ (instancetype)shared;
@end

@implementation BelluiUNDelegate
+ (instancetype)shared {
    static BelluiUNDelegate *shared = nil;
    static dispatch_once_t once;
    dispatch_once(&once, ^{ shared = [[BelluiUNDelegate alloc] init]; });
    return shared;
}

- (void)userNotificationCenter:(UNUserNotificationCenter *)center
       willPresentNotification:(UNNotification *)notification
         withCompletionHandler:(void (^)(UNNotificationPresentationOptions))completionHandler {
    if (@available(macOS 11.0, iOS 14.0, *)) {
        completionHandler(UNNotificationPresentationOptionBanner | UNNotificationPresentationOptionSound);
    } else {
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
        completionHandler(UNNotificationPresentationOptionAlert | UNNotificationPresentationOptionSound);
#pragma clang diagnostic pop
    }
}

- (void)userNotificationCenter:(UNUserNotificationCenter *)center
didReceiveNotificationResponse:(UNNotificationResponse *)response
         withCompletionHandler:(void (^)(void))completionHandler {
    NSNumber *tokenNum = response.notification.request.content.userInfo[@"belluiToken"];
    if (tokenNum != nil) {
        unsigned long long token = [tokenNum unsignedLongLongValue];
        NSString *actionID = response.actionIdentifier;
        int idx = -2; // default: cleanup only (dismiss / unknown)
        if ([actionID isEqualToString:UNNotificationDefaultActionIdentifier]) {
            idx = -1; // body click
        } else if ([actionID hasPrefix:@"bellui-action-"]) {
            idx = [[actionID substringFromIndex:[@"bellui-action-" length]] intValue];
        }
        notifyCallbackDarwin(token, idx);
    }
    completionHandler();
}
@end

// Accumulated categories, re-set on each rich send (setNotificationCategories
// replaces the whole set, so we keep the union).
static NSMutableDictionary<NSString *, UNNotificationCategory *> *belluiCategories = nil;

void doSendNotification(UNUserNotificationCenter *center, NSString *title, NSString *body) {
    UNMutableNotificationContent *content = [UNMutableNotificationContent new];
    [content autorelease];
    content.title = title;
    content.body = body;

    notifyNum++;
    NSString *identifier = [NSString stringWithFormat:@"bellui-notify-%d", notifyNum];
    UNNotificationRequest *request = [UNNotificationRequest requestWithIdentifier:identifier
        content:content trigger:nil];

    [center addNotificationRequest:request withCompletionHandler:^(NSError * _Nullable error) {
        if (error != nil) {
            NSLog(@"Could not send notification: %@", error);
        }
    }];
}

void sendNotification(char *cTitle, char *cBody) {
    UNUserNotificationCenter *center = [UNUserNotificationCenter currentNotificationCenter];
    NSString *title = [NSString stringWithUTF8String:cTitle];
    NSString *body = [NSString stringWithUTF8String:cBody];

    UNAuthorizationOptions options = UNAuthorizationOptionAlert;
    [center requestAuthorizationWithOptions:options
        completionHandler:^(BOOL granted, NSError *_Nullable error) {
            if (!granted) {
                if (error != NULL) {
                    NSLog(@"Error asking for permission to send notifications %@", error);
                    // this happens if our app was not signed, so do it the old way
                    fallbackSend((char *)[title UTF8String], (char *)[body UTF8String]);
                } else {
                    NSLog(@"Unable to get permission to send notifications");
                }
            } else {
                doSendNotification(center, title, body);
            }
        }];
}

// belluiScheduleNotification attaches the optional icon and schedules the request.
// Split out so it can run either directly or deferred until a just-registered
// category has committed (see doSendNotificationWithOptions).
static void belluiScheduleNotification(UNUserNotificationCenter *center,
        UNMutableNotificationContent *content, NSString *iconPath) {
    if ([iconPath length] > 0) {
        NSURL *url = [NSURL fileURLWithPath:iconPath];
        NSError *attErr = nil;
        UNNotificationAttachment *att = [UNNotificationAttachment attachmentWithIdentifier:@"icon"
            URL:url options:nil error:&attErr];
        if (att != nil) {
            content.attachments = @[att];
        } else if (attErr != nil) {
            NSLog(@"Could not attach notification icon: %@", attErr);
        }
    }

    notifyNum++;
    NSString *identifier = [NSString stringWithFormat:@"bellui-notify-%d", notifyNum];
    UNNotificationRequest *request = [UNNotificationRequest requestWithIdentifier:identifier
        content:content trigger:nil];
    [center addNotificationRequest:request withCompletionHandler:^(NSError * _Nullable error) {
        if (error != nil) {
            NSLog(@"Could not send notification: %@", error);
        }
    }];
}

// doSendNotificationWithOptions builds the rich content (userInfo token, optional
// action category, optional icon attachment) and schedules it. Called inside the
// authorization completion handler.
void doSendNotificationWithOptions(UNUserNotificationCenter *center, unsigned long long token,
        NSString *title, NSString *body, NSString *iconPath, NSArray<NSString *> *actionLabels) {
    UNMutableNotificationContent *content = [UNMutableNotificationContent new];
    [content autorelease];
    content.title = title;
    content.body = body;
    content.userInfo = @{@"belluiToken": @(token)};

    BOOL registeredNewCategory = NO;
    if ([actionLabels count] > 0) {
        NSMutableArray<UNNotificationAction *> *actions = [NSMutableArray array];
        for (NSUInteger i = 0; i < [actionLabels count]; i++) {
            NSString *ident = [NSString stringWithFormat:@"bellui-action-%lu", (unsigned long)i];
            UNNotificationAction *act = [UNNotificationAction actionWithIdentifier:ident
                title:actionLabels[i] options:UNNotificationActionOptionForeground];
            [actions addObject:act];
        }
        // Key the category by the label set (FNV-1a of the joined labels), not by
        // token, so repeated sends with the same actions reuse one category rather
        // than accumulating one per notification for the app's lifetime.
        unsigned long long h = 1469598103934665603ULL;
        NSString *joined = [actionLabels componentsJoinedByString:@"\x1f"];
        const char *jb = [joined UTF8String];
        for (const char *p = jb; *p; p++) { h ^= (unsigned char)*p; h *= 1099511628211ULL; }
        NSString *catID = [NSString stringWithFormat:@"bellui-cat-%llx", h];
        if (belluiCategories == nil) {
            belluiCategories = [[NSMutableDictionary alloc] init];
        }
        if (belluiCategories[catID] == nil) {
            UNNotificationCategory *cat = [UNNotificationCategory categoryWithIdentifier:catID
                actions:actions intentIdentifiers:@[] options:UNNotificationCategoryOptionNone];
            belluiCategories[catID] = cat;
            [center setNotificationCategories:[NSSet setWithArray:[belluiCategories allValues]]];
            registeredNewCategory = YES;
        }
        content.categoryIdentifier = catID;
    }

    if (registeredNewCategory) {
        // setNotificationCategories is asynchronous relative to addNotificationRequest;
        // scheduling immediately after registering a *new* category races the
        // registration, so the first notification of that action set shows without its
        // buttons (only the system "Show"). getNotificationCategories' completion runs
        // after the set has been applied, so scheduling from there is ordered. Already
        // registered categories skip this and schedule directly.
        [center getNotificationCategoriesWithCompletionHandler:^(NSSet<UNNotificationCategory *> *categories) {
            belluiScheduleNotification(center, content, iconPath);
        }];
    } else {
        belluiScheduleNotification(center, content, iconPath);
    }
}

void sendNotificationWithOptions(unsigned long long token, char *cTitle, char *cBody,
        char *cIconPath, char **cActionLabels, int actionCount) {
    UNUserNotificationCenter *center = [UNUserNotificationCenter currentNotificationCenter];
    center.delegate = [BelluiUNDelegate shared];

    NSString *title = [NSString stringWithUTF8String:cTitle];
    NSString *body = [NSString stringWithUTF8String:cBody];
    NSString *iconPath = (cIconPath != NULL) ? [NSString stringWithUTF8String:cIconPath] : @"";

    NSMutableArray<NSString *> *labels = [NSMutableArray array];
    for (int i = 0; i < actionCount; i++) {
        [labels addObject:[NSString stringWithUTF8String:cActionLabels[i]]];
    }

    UNAuthorizationOptions options = UNAuthorizationOptionAlert | UNAuthorizationOptionSound;
    [center requestAuthorizationWithOptions:options
        completionHandler:^(BOOL granted, NSError *_Nullable error) {
            if (!granted) {
                if (error != NULL) {
                    NSLog(@"Error asking for permission to send notifications %@", error);
                    // unsigned/unbundled app: degrade to a plain notification (no callbacks)
                    fallbackSend((char *)[title UTF8String], (char *)[body UTF8String]);
                } else {
                    NSLog(@"Unable to get permission to send notifications");
                }
            } else {
                doSendNotificationWithOptions(center, token, title, body, iconPath, labels);
            }
        }];
}
#else
void sendNotification(char *cTitle, char *cBody) {
	fallbackSend(cTitle, cBody);
}

void sendNotificationWithOptions(unsigned long long token, char *cTitle, char *cBody,
        char *cIconPath, char **cActionLabels, int actionCount) {
	fallbackSend(cTitle, cBody);
}
#endif
