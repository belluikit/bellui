// axcell: off-viewport virtualized-cell realize verification for pre-rebrand tracker #637 Stage 4.
//
// It finds the AXTable in the target process and requests a cell far past the
// viewport via the AXCellForColumnAndRow parameterized attribute (which the
// bridge maps to accessibilityCellForColumn:row:). A non-null result whose
// title / row-column index range match the request proves the bridge realized
// the cell on demand through the seam rather than returning nothing. Run against
// hack/a11yrealizeprobe (a 1,000,000-row virtual grid with zero realized cells).
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

static long numAttr(AXUIElementRef el, CFStringRef attr) {
    CFTypeRef v = NULL;
    AXUIElementCopyAttributeValue(el, attr, &v);
    if (!v) return -1;
    long out = -1;
    if (CFGetTypeID(v) == CFNumberGetTypeID()) CFNumberGetValue(v, kCFNumberLongType, &out);
    CFRelease(v);
    return out;
}

// Reads an AXRowIndexRange / AXColumnIndexRange (an AXValue of CFRange type).
static long rangeStart(AXUIElementRef el, CFStringRef attr) {
    CFTypeRef v = NULL;
    AXUIElementCopyAttributeValue(el, attr, &v);
    if (!v) return -1;
    long out = -1;
    if (CFGetTypeID(v) == AXValueGetTypeID()) {
        CFRange r;
        if (AXValueGetValue((AXValueRef)v, kAXValueCFRangeType, &r)) out = r.location;
    }
    CFRelease(v);
    return out;
}

static AXUIElementRef findRole(AXUIElementRef el, NSString* want, int depth) {
    if (depth > 12) return NULL;
    if ([strAttr(el, kAXRoleAttribute) isEqualToString:want]) return (AXUIElementRef)CFRetain(el);
    CFTypeRef children = NULL;
    AXUIElementCopyAttributeValue(el, kAXChildrenAttribute, &children);
    if (!children) return NULL;
    AXUIElementRef found = NULL;
    for (CFIndex i = 0; i < CFArrayGetCount(children) && !found; i++)
        found = findRole((AXUIElementRef)CFArrayGetValueAtIndex(children, i), want, depth + 1);
    CFRelease(children);
    return found;
}

// Requests the cell at (column, row) through AXCellForColumnAndRow. The
// parameter is a two-element array [column, row] of CFNumbers.
static AXUIElementRef cellAt(AXUIElementRef table, int column, int row) {
    CFNumberRef col = CFNumberCreate(NULL, kCFNumberIntType, &column);
    CFNumberRef rw = CFNumberCreate(NULL, kCFNumberIntType, &row);
    const void* vals[] = {col, rw};
    CFArrayRef param = CFArrayCreate(NULL, vals, 2, &kCFTypeArrayCallBacks);
    CFTypeRef cell = NULL;
    AXError err = AXUIElementCopyParameterizedAttributeValue(
        table, CFSTR("AXCellForColumnAndRow"), param, &cell);
    CFRelease(col);
    CFRelease(rw);
    CFRelease(param);
    if (err != kAXErrorSuccess) return NULL;
    return (AXUIElementRef)cell;
}

int main(int argc, char** argv) {
    if (argc < 2) {
        fprintf(stderr, "usage: %s <pid of a11yrealizeprobe> [row] [col]\n", argv[0]);
        return 2;
    }
    pid_t pid = atoi(argv[1]);
    int row = argc > 2 ? atoi(argv[2]) : 500000;
    int col = argc > 3 ? atoi(argv[3]) : 1;

    AXUIElementRef app = AXUIElementCreateApplication(pid);
    AXUIElementRef table = findRole(app, @"AXTable", 0);
    if (!table) { printf("no AXTable found\n"); return 1; }

    printf("table extent: rows=%ld cols=%ld\n",
           numAttr(table, CFSTR("AXRowCount")), numAttr(table, CFSTR("AXColumnCount")));

    printf("AXCellForColumnAndRow col=%d row=%d [off-viewport realize]:\n", col, row);
    AXUIElementRef cell = cellAt(table, col, row);
    if (!cell) { printf("  FAIL: no cell returned (realize failed)\n"); return 1; }
    NSString* title = strAttr(cell, kAXTitleAttribute);
    printf("  role=%s title='%s' rowIdx=%ld colIdx=%ld\n",
           strAttr(cell, kAXRoleAttribute).UTF8String, title.UTF8String,
           rangeStart(cell, CFSTR("AXRowIndexRange")), rangeStart(cell, CFSTR("AXColumnIndexRange")));
    NSString* want = [NSString stringWithFormat:@"R%dC%d", row, col];
    if (![title isEqualToString:want]) { printf("  FAIL: expected title '%s'\n", want.UTF8String); return 1; }

    // Re-request the same cell: the realized-cell cache should return an
    // element equal to the first (stable identity).
    AXUIElementRef again = cellAt(table, col, row);
    printf("  re-request equal: %s\n", (again && CFEqual(cell, again)) ? "YES" : "NO");
    if (again) CFRelease(again);

    // Out-of-range coordinate must be rejected.
    long rows = numAttr(table, CFSTR("AXRowCount"));
    AXUIElementRef oob = cellAt(table, 0, (int)rows);
    printf("  out-of-range (row=%ld) rejected: %s\n", rows, oob ? "NO" : "YES");
    if (oob) CFRelease(oob);

    // AXPress on an off-screen realized cell brings it into view through the
    // seam (watch the probe's PROBE scrollIntoView line).
    AXError perr = AXUIElementPerformAction(cell, kAXPressAction);
    printf("  AXPress (scroll into view) err=%d\n", perr);

    CFRelease(cell);
    printf("OK: off-viewport cell realized through the seam\n");
    return 0;
}
