// axtree: outline/tree-row verification for pre-rebrand tracker #501.
#import <Cocoa/Cocoa.h>
#import <ApplicationServices/ApplicationServices.h>

static NSString* strAttr(AXUIElementRef el, CFStringRef attr) {
    CFTypeRef v = NULL;
    AXUIElementCopyAttributeValue(el, attr, &v);
    if (!v) return @"";
    if (CFGetTypeID(v) == CFStringGetTypeID()) return [(NSString*)v autorelease];
    NSString* d = [NSString stringWithFormat:@"%@", (id)v];
    CFRelease(v);
    return d;
}

static AXUIElementRef findRole(AXUIElementRef el, NSString* want, int depth) {
    if (depth > 12) return NULL;
    NSString* role = strAttr(el, kAXRoleAttribute);
    if ([role isEqualToString:want]) return (AXUIElementRef)CFRetain(el);
    CFTypeRef children = NULL;
    AXUIElementCopyAttributeValue(el, kAXChildrenAttribute, &children);
    if (!children) return NULL;
    AXUIElementRef found = NULL;
    for (CFIndex i = 0; i < CFArrayGetCount(children) && !found; i++)
        found = findRole((AXUIElementRef)CFArrayGetValueAtIndex(children, i), want, depth+1);
    CFRelease(children);
    return found;
}

static void dumpRows(AXUIElementRef outline) {
    CFTypeRef children = NULL;
    AXUIElementCopyAttributeValue(outline, kAXChildrenAttribute, &children);
    if (!children) { printf("  (no children)\n"); return; }
    for (CFIndex i = 0; i < CFArrayGetCount(children); i++) {
        AXUIElementRef row = (AXUIElementRef)CFArrayGetValueAtIndex(children, i);
        CFTypeRef actions = NULL;
        AXUIElementCopyAttributeValue(row, CFSTR("AXCustomActions"), &actions);
        NSString* acts = actions ? [NSString stringWithFormat:@"%@", (id)actions] : @"(none)";
        printf("  row %ld: role=%s sub=%s title='%s' disclosing=%s lvl=%s custom=%s\n", (long)i,
               strAttr(row, kAXRoleAttribute).UTF8String,
               strAttr(row, kAXSubroleAttribute).UTF8String,
               strAttr(row, kAXTitleAttribute).UTF8String,
               strAttr(row, kAXDisclosingAttribute).UTF8String,
               strAttr(row, kAXDisclosureLevelAttribute).UTF8String,
               [[acts stringByReplacingOccurrencesOfString:@"\n" withString:@" "] UTF8String]);
        if (actions) CFRelease(actions);
    }
    CFRelease(children);
}

int main(int argc, char** argv) {
    if (argc < 2) {
        fprintf(stderr, "usage: %s <pid of a11yprobe>\n", argv[0]);
        return 2;
    }
    pid_t pid = atoi(argv[1]);
    AXUIElementRef app = AXUIElementCreateApplication(pid);
    AXUIElementRef outline = findRole(app, @"AXOutline", 0);
    if (!outline) { printf("no outline\n"); return 1; }
    printf("before expand:\n");
    dumpRows(outline);

    // expand the first disclosable row
    CFTypeRef children = NULL;
    AXUIElementCopyAttributeValue(outline, kAXChildrenAttribute, &children);
    if (children && CFArrayGetCount(children) > 0) {
        AXUIElementRef row = (AXUIElementRef)CFArrayGetValueAtIndex(children, 0);
        AXError err = AXUIElementSetAttributeValue(row, kAXDisclosingAttribute, kCFBooleanTrue);
        printf("set disclosing err=%d\n", err);
    }
    usleep(900000);
    printf("after expand:\n");
    dumpRows(outline);
    return 0;
}
