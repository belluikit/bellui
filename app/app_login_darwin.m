//go:build !ci && !ios && !wasm && !test_web_driver && !mobile

#import <Foundation/Foundation.h>
#import <ServiceManagement/ServiceManagement.h>

// launchAtLoginSupported reports whether SMAppService can be used: macOS 13+
// and a bundled app (mainAppService uses the main bundle identifier).
int launchAtLoginSupported(void) {
    if (@available(macOS 13.0, *)) {
        return [[NSBundle mainBundle] bundleIdentifier] != nil ? 1 : 0;
    }
    return 0;
}

// setLaunchAtLogin registers (enable != 0) or unregisters the main app as a
// login item. Returns 0 on success, 1 on failure.
int setLaunchAtLogin(int enable) {
    if (@available(macOS 13.0, *)) {
        SMAppService *service = [SMAppService mainAppService];
        NSError *err = nil;
        BOOL ok;
        if (enable) {
            ok = [service registerAndReturnError:&err];
        } else {
            ok = [service unregisterAndReturnError:&err];
        }
        if (!ok) {
            if (err != nil) {
                NSLog(@"launch-at-login change failed: %@", err);
            }
            return 1;
        }
        return 0;
    }
    return 1;
}

// launchAtLoginEnabled returns 1 when the login item is registered and enabled.
int launchAtLoginEnabled(void) {
    if (@available(macOS 13.0, *)) {
        return [[SMAppService mainAppService] status] == SMAppServiceStatusEnabled ? 1 : 0;
    }
    return 0;
}
