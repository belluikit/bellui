// axlist: prints every element of a process's windows with its role and
// label, indented by depth, for checking what a screen reader can reach
// (#1172). Usage: axlist <pid>
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

static void dump(AXUIElementRef el, int depth) {
    if (depth > 30) return;
    printf("%*s%s title='%s' desc='%s' placeholder='%s'\n", depth * 2, "",
           strAttr(el, kAXRoleAttribute).UTF8String,
           strAttr(el, kAXTitleAttribute).UTF8String,
           strAttr(el, kAXDescriptionAttribute).UTF8String,
           strAttr(el, kAXPlaceholderValueAttribute).UTF8String);
    CFTypeRef children = NULL;
    AXUIElementCopyAttributeValue(el, kAXChildrenAttribute, &children);
    if (!children) return;
    for (CFIndex i = 0; i < CFArrayGetCount(children); i++)
        dump((AXUIElementRef)CFArrayGetValueAtIndex(children, i), depth + 1);
    CFRelease(children);
}

int main(int argc, char** argv) {
    @autoreleasepool {
        if (argc < 2) { fprintf(stderr, "usage: axlist <pid>\n"); return 2; }
        AXUIElementRef app = AXUIElementCreateApplication(atoi(argv[1]));
        CFTypeRef windows = NULL;
        AXUIElementCopyAttributeValue(app, kAXWindowsAttribute, &windows);
        if (!windows) { fprintf(stderr, "no windows (not trusted, or no such pid)\n"); return 1; }
        for (CFIndex i = 0; i < CFArrayGetCount(windows); i++)
            dump((AXUIElementRef)CFArrayGetValueAtIndex(windows, i), 0);
        CFRelease(windows);
    }
    return 0;
}
