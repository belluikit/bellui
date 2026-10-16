//go:build !no_native_menus

#import <Foundation/Foundation.h>
#import <AppKit/AppKit.h>

const int menuTagMin = 5000;

#if __MAC_OS_X_VERSION_MAX_ALLOWED >= 101400
NSControlStateValue STATE_ON = NSControlStateValueOn;
NSControlStateValue STATE_OFF = NSControlStateValueOff;
#else
NSCellStateValue STATE_ON = NSOnState;
NSCellStateValue STATE_OFF = NSOffState;
#endif


extern void menuCallback(int);
extern BOOL menuEnabled(int);
extern BOOL menuChecked(int);
extern void exceptionCallback(const char*);

@interface BelluiMenuHandler : NSObject {
}
@end

@implementation BelluiMenuHandler
+ (void) tapped:(NSMenuItem*) item {
    menuCallback([item tag]-menuTagMin);
}
+ (BOOL) validateMenuItem:(NSMenuItem*) item {
    BOOL checked = menuChecked([item tag]-menuTagMin);
    if (checked) {
        [item setState:STATE_ON];
    } else {
        [item setState:STATE_OFF];
    }

    return menuEnabled([item tag]-menuTagMin);
}
@end

// forward declaration … we want methods to be ordered alphabetically
NSMenu* nativeMainMenu();

// belluiLiveMenuObjects holds a strong reference to every NSMenu/NSMenuItem that
// bellui hands to AppKit. Without it those objects are owned *only* by AppKit's
// menu-container chain, so resetDarwinMenu()'s removeItem: (which can run mid
// key-equivalent routing, or from the very menu action that triggers a rebuild)
// can deallocate an item while AppKit still holds a raw pointer to it — the
// use-after-free behind the -[OS_dispatch_mach hasSubmenu] crash (pre-rebrand tracker #811). Holding
// our own retain keeps the object alive across that window; the retains are
// released (deferred to the autorelease pool) only in resetDarwinMenu().
static NSMutableArray *belluiLiveMenuObjects = nil;

static void belluiTrackMenuObject(id obj) {
    if (obj == nil) {
        return;
    }
    if (belluiLiveMenuObjects == nil) {
        belluiLiveMenuObjects = [[NSMutableArray alloc] init];
    }
    [belluiLiveMenuObjects addObject:obj]; // NSMutableArray retains
}

// belluiPermanentMenuObjects retains the *static* main-menu scaffolding bellui builds
// once at startup — the menu bar, the app / Window / Services submenus and their
// standard items — for the whole process lifetime. This is the counterpart to
// belluiLiveMenuObjects (the per-generation dynamic menus, dropped on every
// resetDarwinMenu): the scaffolding is never torn down, so nothing here is ever
// freed underfoot. bellui owns the bar instead of GLFW (pre-rebrand tracker #831), so GLFW's orphaned
// Services-menu scaffolding — the macOS-26 use-after-free candidate — never exists.
static NSMutableArray *belluiPermanentMenuObjects = nil;
static BOOL belluiMainMenuBuilt = NO;

static void belluiRetainPermanentMenuObject(id obj) {
    if (obj == nil) {
        return;
    }
    if (belluiPermanentMenuObjects == nil) {
        belluiPermanentMenuObjects = [[NSMutableArray alloc] init];
    }
    [belluiPermanentMenuObjects addObject:obj]; // NSMutableArray retains; never released
}

// createBelluiMainMenu builds the native main menu bar that GLFW's createMenuBar()
// used to build during glfw.Init() (now suppressed via GLFW_COCOA_MENUBAR=false).
// It mirrors that structure exactly — app submenu at index 0, Window menu at
// index 1 — so bellui's existing bar mutation (darwinAppMenu(), completeDarwinMenu())
// and behaviour are unchanged. Idempotent: the bar is constructed at most once.
void createBelluiMainMenu(const char* appNameC) {
    if (belluiMainMenuBuilt) {
        return;
    }
    belluiMainMenuBuilt = YES;

    NSString* appName = nil;
    if (appNameC != NULL) {
        appName = [NSString stringWithUTF8String:appNameC];
    }
    if (appName == nil || [appName isEqualToString:@""]) {
        appName = [[NSProcessInfo processInfo] processName];
    }

    NSMenu* bar = [[NSMenu alloc] init];
    [NSApp setMainMenu:bar];
    belluiRetainPermanentMenuObject(bar);

    NSMenuItem* appMenuItem =
        [bar addItemWithTitle:@"" action:NULL keyEquivalent:@""];
    NSMenu* appMenu = [[NSMenu alloc] init];
    [appMenuItem setSubmenu:appMenu];
    belluiRetainPermanentMenuObject(appMenu);

    [appMenu addItemWithTitle:[NSString stringWithFormat:@"About %@", appName]
                       action:@selector(orderFrontStandardAboutPanel:)
                keyEquivalent:@""];
    [appMenu addItem:[NSMenuItem separatorItem]];
    NSMenu* servicesMenu = [[NSMenu alloc] init];
    [NSApp setServicesMenu:servicesMenu];
    [[appMenu addItemWithTitle:@"Services"
                        action:NULL
                 keyEquivalent:@""] setSubmenu:servicesMenu];
    belluiRetainPermanentMenuObject(servicesMenu);
    [servicesMenu release];
    [appMenu addItem:[NSMenuItem separatorItem]];
    [appMenu addItemWithTitle:[NSString stringWithFormat:@"Hide %@", appName]
                       action:@selector(hide:)
                keyEquivalent:@"h"];
    [[appMenu addItemWithTitle:@"Hide Others"
                        action:@selector(hideOtherApplications:)
                 keyEquivalent:@"h"]
        setKeyEquivalentModifierMask:NSEventModifierFlagOption | NSEventModifierFlagCommand];
    [appMenu addItemWithTitle:@"Show All"
                       action:@selector(unhideAllApplications:)
                keyEquivalent:@""];
    [appMenu addItem:[NSMenuItem separatorItem]];
    [appMenu addItemWithTitle:[NSString stringWithFormat:@"Quit %@", appName]
                       action:@selector(terminate:)
                keyEquivalent:@"q"];

    NSMenuItem* windowMenuItem =
        [bar addItemWithTitle:@"" action:NULL keyEquivalent:@""];
    [bar release]; // retained by NSApp (setMainMenu) + belluiPermanentMenuObjects
    NSMenu* windowMenu = [[NSMenu alloc] initWithTitle:@"Window"];
    [NSApp setWindowsMenu:windowMenu];
    [windowMenuItem setSubmenu:windowMenu];
    belluiRetainPermanentMenuObject(windowMenu);
    [windowMenu release]; // retained by NSApp (setWindowsMenu) + the menu item + array

    [windowMenu addItemWithTitle:@"Minimize"
                          action:@selector(performMiniaturize:)
                   keyEquivalent:@"m"];
    [windowMenu addItemWithTitle:@"Zoom"
                          action:@selector(performZoom:)
                   keyEquivalent:@""];
    [windowMenu addItem:[NSMenuItem separatorItem]];
    [windowMenu addItemWithTitle:@"Bring All to Front"
                          action:@selector(arrangeInFront:)
                   keyEquivalent:@""];
    [windowMenu addItem:[NSMenuItem separatorItem]];
    [[windowMenu addItemWithTitle:@"Enter Full Screen"
                           action:@selector(toggleFullScreen:)
                    keyEquivalent:@"f"]
        setKeyEquivalentModifierMask:NSEventModifierFlagControl | NSEventModifierFlagCommand];

    // Semi-private API that wires the app menu to the Apple/application menu slot.
    SEL setAppleMenuSelector = NSSelectorFromString(@"setAppleMenu:");
    [NSApp performSelector:setAppleMenuSelector withObject:appMenu];
}

// ensureRegularActivationPolicy promotes the process to a regular foreground UI
// application (Dock icon + the ability to become the key window and receive
// keyboard input). GLFW does this during glfw.Init(), but only when its own
// Cocoa menu bar is enabled (cocoa_init.m: `if (_glfw.hints.init.ns.menubar)
// [NSApp setActivationPolicy:Regular]`). bellui now disables that menu bar
// (disableGLFWMenubar) to avoid the macOS-26 native-menu use-after-free (pre-rebrand tracker #831),
// which as a side effect also skipped the activation-policy promotion — leaving
// an *unbundled* app (run from a terminal, `go run`, a bare binary) as a
// background/accessory process whose windows cannot take keyboard focus, so
// typed characters go to the launching terminal instead of the app. bellui owns
// the menu bar now, so it must own this too. Idempotent, and a no-op for a
// bundled .app (already Regular via its Info.plist). Main thread only, after
// AppKit is up (post glfw.Init()).
void ensureRegularActivationPolicy(void) {
    [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
}

void assignDarwinSubmenu(const void* i, const void* m) {
    NSMenu* menu = (NSMenu*)m; // this menu is created in the createDarwinMenu() function
    NSMenuItem *item = (NSMenuItem*)i;
    [item setSubmenu:menu]; // this retains the menu
    [menu release]; // release the menu
}

void completeDarwinMenu(const void* m, bool prepend) {
    NSMenu* main = nativeMainMenu();
    NSMenuItem* top = [[NSMenuItem alloc] initWithTitle:@"" action:nil keyEquivalent:@""];
    [top setTag:menuTagMin];
    if (prepend) {
        [main insertItem:top atIndex:1];
    } else {
        [main addItem:top];
    }
    assignDarwinSubmenu(top, m);
    belluiTrackMenuObject(top);
    [top release]; // retained by the main menu and belluiLiveMenuObjects now
}

const void* createDarwinMenu(const char* label) {
    NSMenu* menu = [[NSMenu alloc] initWithTitle:[NSString stringWithUTF8String:label]];
    belluiTrackMenuObject(menu);
    return (void*)menu; // caller owns the alloc +1; assignDarwinSubmenu releases it
}

const void* darwinAppMenu() {
    return [[nativeMainMenu() itemAtIndex:0] submenu];
}

void getTextColorRGBA(int* r, int* g, int* b, int* a) {
  CGFloat fr, fg, fb, fa;
  NSColor *c = [[NSColor selectedMenuItemTextColor] colorUsingColorSpace: [NSColorSpace sRGBColorSpace]];
  [c getRed: &fr green: &fg blue: &fb alpha: &fa];
  *r = fr*255.0;
  *g = fg*255.0;
  *b = fb*255.0;
  *a = fa*255.0;
}

void handleException(const char* m, id e) {
    exceptionCallback([[NSString stringWithFormat:@"%s failed: %@", m, e] UTF8String]);
}

int replacedAbout = 0;

const void* insertDarwinMenuItem(const void* m, const char* label, const char* keyEquivalent, unsigned int keyEquivalentModifierMask, int nextId, int index, bool isSeparator, const void *imageData, unsigned int imageDataLength) {
    NSMenu* menu = (NSMenu*)m;
    NSMenuItem* item;

    if (strcmp(label, "About") == 0 && !replacedAbout) {
        replacedAbout = 1;
        item = [menu itemArray][0];
        [item setAction:@selector(tapped:)];
        [item setTarget:[BelluiMenuHandler class]];
        [item setTag:nextId+menuTagMin];
        return item;
    }

    if (isSeparator) {
        // +separatorItem returns an autoreleased item we do NOT own, so it is
        // retained here to match the alloc'd branch's +1 — the release below
        // is unconditional.
        item = [[NSMenuItem separatorItem] retain];
    } else {
        item = [[NSMenuItem alloc]
            initWithTitle:[NSString stringWithUTF8String:label]
            action:@selector(tapped:)
            keyEquivalent:[NSString stringWithUTF8String:keyEquivalent]];
        if (keyEquivalentModifierMask) {
            [item setKeyEquivalentModifierMask: keyEquivalentModifierMask];
        }
        [item setTarget:[BelluiMenuHandler class]];
        [item setTag:nextId+menuTagMin];
        if (imageData) {
        char *x = (char *)imageData;
            NSData *data = [[NSData alloc] initWithBytes: imageData length: imageDataLength];
            NSImage *image = [[NSImage alloc] initWithData: data];
            [item setImage: image];
            [data release];
            [image release];
        }
    }

    if (index > -1) {
        [menu insertItem:item atIndex:index];
    } else {
        [menu addItem:item];
    }
    [item release]; // retained by the menu
    belluiTrackMenuObject(item); // hold our own strong ref until the next reset (pre-rebrand tracker #811)
    return item;
}

int menuFontSize() {
  return ceil([[NSFont menuFontOfSize: 0] pointSize]);
}

NSMenu* nativeMainMenu() {
    NSApplication* app = [NSApplication sharedApplication];
    return [app mainMenu];
}

void resetDarwinMenu() {
    NSMenu *root = nativeMainMenu();
    // Copy itemArray to avoid mutating during enumeration — AppKit may be
    // tracking the menu when this is called from a menu action callback.
    NSArray *items = [[root itemArray] copy];

    for (id object in items) {
        if (![object isKindOfClass:[NSMenuItem class]]) {
            continue;
        }
        NSMenuItem *item = object;
        if ([item tag] < menuTagMin) {
            // check for inserted items (like Settings...)
            NSMenu *menu = [item submenu];
            if (menu == nil) {
                continue;
            }
            NSArray *subItems = [[menu itemArray] copy];
            for (id sub in subItems) {
                if (![sub isKindOfClass:[NSMenuItem class]]) {
                    continue;
                }
                NSMenuItem *subItem = sub;
                if ([subItem tag] >= menuTagMin) {
                    [menu removeItem: subItem];
                }
            }
            [subItems release];
            continue;
        }
        [root removeItem: item];
    }
    [items release];

    // Drop bellui's strong references to the previous menu generation, but defer the
    // actual deallocation to the next autorelease-pool drain: -autorelease keeps
    // every held NSMenu/NSMenuItem alive past the current run-loop turn, so an
    // object AppKit is still routing a key event through — or the item whose action
    // triggered this rebuild — is never freed underfoot (pre-rebrand tracker #811).
    [belluiLiveMenuObjects autorelease];
    belluiLiveMenuObjects = nil; // a fresh array is created lazily by belluiTrackMenuObject
}

const void* test_darwinMainMenu() {
    return nativeMainMenu();
}

const void* test_NSMenu_itemAtIndex(const void* m, NSInteger i) {
    NSMenu* menu = (NSMenu*)m;
    @try {
        return [menu itemAtIndex: i];
    } @catch(NSException* e) {
        handleException("test_NSMenu_itemAtIndex", e);
        return NULL;
    }
}

NSInteger test_NSMenu_numberOfItems(const void* m) {
    NSMenu* menu = (NSMenu*)m;
    return [menu numberOfItems];
}

void test_NSMenu_performActionForItemAtIndex(const void* m, NSInteger i) {
    NSMenu* menu = (NSMenu*)m;
    @try {
        // Using performActionForItemAtIndex: would be better but sadly it crashes.
        // We simulate the relevant effect for now.
        // [menu performActionForItemAtIndex:i];
        NSMenuItem* item = [menu itemAtIndex:i];
        [[item target] performSelector:[item action] withObject:item];
    } @catch(NSException* e) {
        handleException("test_NSMenu_performActionForItemAtIndex", e);
    }
}

void test_NSMenu_removeItemAtIndex(const void* m, NSInteger i) {
    NSMenu* menu = (NSMenu*)m;
    @try {
        [menu removeItemAtIndex: i];
    } @catch(NSException* e) {
        handleException("test_NSMenu_removeItemAtIndex", e);
    }
}

const char* test_NSMenu_title(const void* m) {
    NSMenu* menu = (NSMenu*)m;
    return [[menu title] UTF8String];
}

bool test_NSMenuItem_isSeparatorItem(const void* i) {
    NSMenuItem* item = (NSMenuItem*)i;
    return [item isSeparatorItem];
}

const char* test_NSMenuItem_keyEquivalent(const void *i) {
    NSMenuItem* item = (NSMenuItem*)i;
    return [[item keyEquivalent] UTF8String];
}

unsigned long test_NSMenuItem_keyEquivalentModifierMask(const void *i) {
    NSMenuItem* item = (NSMenuItem*)i;
    return [item keyEquivalentModifierMask];
}

const void* test_NSMenuItem_submenu(const void* i) {
    NSMenuItem* item = (NSMenuItem*)i;
    return [item submenu];
}

const char* test_NSMenuItem_title(const void* i) {
    NSMenuItem* item = (NSMenuItem*)i;
    return [[item title] UTF8String];
}

// test_NSObject_retainCount reports an object's strong-reference count, so a
// test can pin bellui's ownership of a menu object rather than waiting for an
// over-release to surface as a use-after-free in the event loop
// (belluikit/studio#5).
NSInteger test_NSObject_retainCount(const void* o) {
    return [(id)o retainCount];
}

// test_popAutoreleasePool / test_pushAutoreleasePool bracket a menu build so a
// test can drain the pending autoreleases it produced (+separatorItem's, among
// others) before reading a retain count — otherwise an undrained pool release
// shows up as an extra reference and masks what bellui actually owns.
static NSAutoreleasePool *belluiTestPool = nil;

void test_popAutoreleasePool(void) {
    [belluiTestPool release];
    belluiTestPool = nil;
}

void test_pushAutoreleasePool(void) {
    belluiTestPool = [[NSAutoreleasePool alloc] init];
}
