// axsel: #556 verify — selection protocol on a selectable static-text label.
#import <Cocoa/Cocoa.h>
#import <ApplicationServices/ApplicationServices.h>

static AXUIElementRef findLabel(AXUIElementRef el, int depth) {
    if (depth > 12) return NULL;
    CFTypeRef role = NULL, title = NULL;
    AXUIElementCopyAttributeValue(el, kAXRoleAttribute, &role);
    bool isText = role && CFEqual(role, kAXStaticTextRole);
    if (role) CFRelease(role);
    if (isText) {
        AXUIElementCopyAttributeValue(el, kAXTitleAttribute, &title);
        bool match = title && [(NSString*)title containsString:@"selectme"];
        if (title) CFRelease(title);
        if (match) return (AXUIElementRef)CFRetain(el);
    }
    CFTypeRef children = NULL;
    AXUIElementCopyAttributeValue(el, kAXChildrenAttribute, &children);
    if (!children) return NULL;
    AXUIElementRef found = NULL;
    for (CFIndex i = 0; i < CFArrayGetCount(children) && !found; i++)
        found = findLabel((AXUIElementRef)CFArrayGetValueAtIndex(children, i), depth+1);
    CFRelease(children);
    return found;
}

int main(int argc, char** argv) {
    if (argc < 2) {
        fprintf(stderr, "usage: %s <pid of a11yprobe>\n", argv[0]);
        return 2;
    }
    AXUIElementRef app = AXUIElementCreateApplication(atoi(argv[1]));
    AXUIElementRef label = findLabel(app, 0);
    if (!label) { printf("no selectable label found\n"); return 1; }

    CFTypeRef n = NULL;
    AXUIElementCopyAttributeValue(label, kAXNumberOfCharactersAttribute, &n);
    printf("label chars=%s\n", [[NSString stringWithFormat:@"%@", (id)n] UTF8String]);

    CFRange want = CFRangeMake(9, 6); // "grabme"
    AXValueRef v = AXValueCreate(kAXValueTypeCFRange, &want);
    AXError err = AXUIElementSetAttributeValue(label, kAXSelectedTextRangeAttribute, v);
    printf("set err=%d\n", err);
    usleep(500000);

    CFTypeRef sel = NULL;
    AXUIElementCopyAttributeValue(label, kAXSelectedTextAttribute, &sel);
    printf("selected: '%s'\n", sel ? [(NSString*)sel UTF8String] : "(nil)");
    CFTypeRef got = NULL;
    AXUIElementCopyAttributeValue(label, kAXSelectedTextRangeAttribute, &got);
    if (got) {
        CFRange r; AXValueGetValue((AXValueRef)got, kAXValueTypeCFRange, &r);
        printf("readback loc=%ld len=%ld\n", r.location, r.length);
    }
    return 0;
}
