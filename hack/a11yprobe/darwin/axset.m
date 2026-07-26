// axset: raw AX API driver for the text-protocol verify.
// usage: axset <pid>
#import <Cocoa/Cocoa.h>
#import <ApplicationServices/ApplicationServices.h>

static AXUIElementRef findTextArea(AXUIElementRef el, int depth) {
    if (depth > 12) return NULL;
    CFTypeRef role = NULL;
    AXUIElementCopyAttributeValue(el, kAXRoleAttribute, &role);
    if (role && CFEqual(role, CFSTR("AXTextArea"))) { CFRelease(role); return (AXUIElementRef)CFRetain(el); }
    if (role) CFRelease(role);
    CFTypeRef children = NULL;
    AXUIElementCopyAttributeValue(el, kAXChildrenAttribute, &children);
    if (!children) return NULL;
    AXUIElementRef found = NULL;
    for (CFIndex i = 0; i < CFArrayGetCount(children) && !found; i++) {
        found = findTextArea((AXUIElementRef)CFArrayGetValueAtIndex(children, i), depth+1);
    }
    CFRelease(children);
    return found;
}

int main(int argc, char** argv) {
    pid_t pid = atoi(argv[1]);
    AXUIElementRef app = AXUIElementCreateApplication(pid);
    AXUIElementRef ta = findTextArea(app, 0);
    if (!ta) { printf("no text area\n"); return 1; }

    CFRange want = CFRangeMake(7, 5);
    AXValueRef v = AXValueCreate(kAXValueTypeCFRange, &want);
    AXError err = AXUIElementSetAttributeValue(ta, kAXSelectedTextRangeAttribute, v);
    printf("set err=%d\n", err);
    usleep(500000);

    CFTypeRef got = NULL;
    AXUIElementCopyAttributeValue(ta, kAXSelectedTextRangeAttribute, &got);
    if (got) {
        CFRange r; AXValueGetValue((AXValueRef)got, kAXValueTypeCFRange, &r);
        printf("readback loc=%ld len=%ld\n", r.location, r.length);
    }
    CFTypeRef sel = NULL;
    AXUIElementCopyAttributeValue(ta, kAXSelectedTextAttribute, &sel);
    if (sel) printf("selected text: %s\n", [[NSString stringWithString:(NSString*)sel] UTF8String]);

    // line for index 12 + range for line 1 (parameterized attributes)
    CFTypeRef line = NULL;
    CFNumberRef idx = CFNumberCreate(NULL, kCFNumberIntType, (int[]){12});
    AXUIElementCopyParameterizedAttributeValue(ta, kAXLineForIndexParameterizedAttribute, idx, &line);
    if (line) { int l; CFNumberGetValue((CFNumberRef)line, kCFNumberIntType, &l); printf("lineForIndex(12)=%d\n", l); }
    return 0;
}
