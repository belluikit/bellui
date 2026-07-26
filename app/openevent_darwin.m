//go:build !ci && !ios && !wasm && !test_web_driver && !mobile

#import <Foundation/Foundation.h>
#import <CoreServices/CoreServices.h>

// The GetURL Apple Event (Internet Config suite) constants are not exported by
// the CoreServices umbrella header; define their canonical FourCharCode values.
#ifndef kInternetEventClass
#define kInternetEventClass 'GURL'
#endif
#ifndef kAEGetURL
#define kAEGetURL 'GURL'
#endif

// Implemented in Go (openevent_darwin.go).
extern void belluiHandleOpenURL(char *url);
extern void belluiHandleOpenFile(char *path);

@interface BelluiOpenEventHandler : NSObject
- (void)handleGetURLEvent:(NSAppleEventDescriptor *)event withReplyEvent:(NSAppleEventDescriptor *)reply;
- (void)handleOpenDocsEvent:(NSAppleEventDescriptor *)event withReplyEvent:(NSAppleEventDescriptor *)reply;
@end

static BelluiOpenEventHandler *belluiOpenHandler = nil;

@implementation BelluiOpenEventHandler

- (void)handleGetURLEvent:(NSAppleEventDescriptor *)event withReplyEvent:(NSAppleEventDescriptor *)reply {
    NSString *urlStr = [[event paramDescriptorForKeyword:keyDirectObject] stringValue];
    if (urlStr != nil && [urlStr length] > 0) {
        belluiHandleOpenURL((char *)[urlStr UTF8String]);
    }
}

- (void)handleOpenDocsEvent:(NSAppleEventDescriptor *)event withReplyEvent:(NSAppleEventDescriptor *)reply {
    NSAppleEventDescriptor *docList = [event paramDescriptorForKeyword:keyDirectObject];
    NSInteger count = [docList numberOfItems];
    for (NSInteger i = 1; i <= count; i++) { // Apple Event lists are 1-indexed
        NSAppleEventDescriptor *desc = [[docList descriptorAtIndex:i] coerceToDescriptorType:typeFileURL];
        if (desc == nil) {
            continue;
        }
        NSString *urlStr = [[NSString alloc] initWithData:[desc data] encoding:NSUTF8StringEncoding];
        NSString *path = [[NSURL URLWithString:urlStr] path];
        if (path != nil && [path length] > 0) {
            belluiHandleOpenFile((char *)[path UTF8String]);
        }
        [urlStr release];
    }
}

@end

// registerOpenEventHandlers installs the odoc (open documents) and GURL (open
// URL) Apple Event handlers. NSAppleEventManager exists independently of NSApp,
// and the launch event Launch Services queues is delivered once the driver's
// Cocoa event loop starts pumping, so registering here (before the loop runs)
// catches the launch payload. Idempotent.
void registerOpenEventHandlers() {
    if (belluiOpenHandler != nil) {
        return;
    }
    belluiOpenHandler = [[BelluiOpenEventHandler alloc] init];

    NSAppleEventManager *aem = [NSAppleEventManager sharedAppleEventManager];
    [aem setEventHandler:belluiOpenHandler
             andSelector:@selector(handleGetURLEvent:withReplyEvent:)
           forEventClass:kInternetEventClass
              andEventID:kAEGetURL];
    [aem setEventHandler:belluiOpenHandler
             andSelector:@selector(handleOpenDocsEvent:withReplyEvent:)
           forEventClass:kCoreEventClass
              andEventID:kAEOpenDocuments];
}
