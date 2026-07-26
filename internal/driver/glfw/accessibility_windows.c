//go:build accessibility && windows && !wasm && !test_web_driver

// UI Automation provider bridge for the bellui accessibility layer (#487).
//
// The Go side (accessibility_windows.go) reconciles the platform-neutral
// semantic tree into a forest of BelluiUIAElement providers attached to each
// window's HWND; this file implements the UIA provider interfaces over those
// elements in plain C (manual vtables over the mingw-w64 C interface
// definitions — no C++, so it builds with both mingw flavors CI uses).
//
// Threading: everything runs on the main thread. The providers declare
// ProviderOptions_UseComThreading, so UIA marshals client calls onto this
// thread's STA message pump (GLFW's WaitEventsTimeout); callbacks into Go
// therefore arrive on the main goroutine, the same re-entry model as the
// GLFW input callbacks and the darwin backend — no locking anywhere.

#include "accessibility_windows.h"

#include <windows.h>
#include <ole2.h>
#include <oleauto.h>
#include <uiautomationcore.h>
#include <uiautomationcoreapi.h>

#include "_cgo_export.h"

// Numeric UIA IDs (spec-frozen; the definitions live in uiautomationclient.h,
// which is a client-side header we do not otherwise need).
#define UIA_ControlTypePropertyId 30003
#define UIA_NamePropertyId 30005
#define UIA_HasKeyboardFocusPropertyId 30008
#define UIA_IsKeyboardFocusablePropertyId 30009
#define UIA_IsEnabledPropertyId 30010
#define UIA_AutomationIdPropertyId 30011
#define UIA_HelpTextPropertyId 30013
#define UIA_IsControlElementPropertyId 30016
#define UIA_IsContentElementPropertyId 30017
#define UIA_IsPasswordPropertyId 30019
#define UIA_IsOffscreenPropertyId 30022
#define UIA_FrameworkIdPropertyId 30024
#define UIA_ValueValuePropertyId 30045
#define UIA_RangeValueValuePropertyId 30047
#define UIA_ExpandCollapseExpandCollapseStatePropertyId 30070
#define UIA_SelectionItemIsSelectedPropertyId 30079
#define UIA_ToggleToggleStatePropertyId 30086
#define UIA_LevelPropertyId 30154
#define UIA_HeadingLevelPropertyId 30173
#define UIA_IsDialogPropertyId 30174

#define UIA_InvokePatternId 10000
#define UIA_ValuePatternId 10002
#define UIA_RangeValuePatternId 10003
#define UIA_ExpandCollapsePatternId 10005
#define UIA_GridPatternId 10006
#define UIA_GridItemPatternId 10007
#define UIA_SelectionItemPatternId 10010
#define UIA_TextPatternId 10014
#define UIA_TogglePatternId 10015
#define UIA_VirtualizedItemPatternId 10020

#define UIA_StructureChangedEventId 20002
#define UIA_AutomationFocusChangedEventId 20005
#define UIA_Invoke_InvokedEventId 20009
#define UIA_SelectionItem_ElementSelectedEventId 20012
#define UIA_Text_TextSelectionChangedEventId 20014
#define UIA_Text_TextChangedEventId 20015

#define UIA_IsReadOnlyAttributeId 40015

// HEADINGLEVELID values for UIA_HeadingLevelPropertyId.
#define HeadingLevel_None 80050
#define HeadingLevel1 80051

// Control types that act as selection containers for SelectionItem children.
#define UIA_ListControlTypeId 50008
#define UIA_TabControlTypeId 50018
#define UIA_TreeControlTypeId 50023
#define UIA_DataGridControlTypeId 50028
#define UIA_TableControlTypeId 50036

// Raised on the window to defer the focus event out of the WM_SETFOCUS
// handler: UIA clients commonly sit inside a SendMessage when the window
// activates, and raising synchronously there can deadlock.
#define WM_BELLUI_A11Y_FOCUS (WM_APP + 100)

// ============================================================
// Element
// ============================================================

typedef struct BelluiUIAElement BelluiUIAElement;

struct BelluiUIAElement {
    // Interface parts; each is a struct holding just an lpVtbl pointer, so
    // the containing element is recovered with offsetof arithmetic.
    IRawElementProviderSimple simple;
    IRawElementProviderFragment fragment;
    IRawElementProviderFragmentRoot fragRoot; // answered by roots only
    IInvokeProvider invoke;
    IToggleProvider toggle;
    IExpandCollapseProvider expandCollapse;
    IRangeValueProvider range;
    ISelectionItemProvider selItem;
    IValueProvider value;
    ITextProvider text;
    IGridProvider grid;
    IGridItemProvider gridItem;
    IVirtualizedItemProvider vitem; // realized off-viewport cells only

    LONG refCount;
    BOOL isRoot;
    BOOL disconnected;
    BOOL everUpdated;
    HWND hwnd;
    uintptr_t handle; // Go-side registry key; doubles as the runtime id

    BelluiUIAElement* parent; // not ref-counted: parents own children
    BelluiUIAElement** children;
    int childCount;

    // Properties, diffed by WinAccessibilityElementUpdate.
    int controlType;
    WCHAR* name;
    WCHAR* help;
    WCHAR* valueText;
    double x, y, w, h; // client-area pixels, top-left origin
    BOOL enabled, focusable, focused, offscreen;
    BOOL isPassword, isDialog, structural;
    BOOL invokable;
    int toggleState;
    int expandState;
    int selectable;
    BOOL hasRange;
    double rangeMin, rangeMax, rangeValue;
    BOOL rangeSettable;
    BOOL textElement;
    BOOL readOnly;
    long selStart, selEnd, textVersion;
    int level;
    int headingLevel;
    BOOL hasGridDims;
    int gridRows, gridCols;
    BOOL hasCell;
    int cellRow, cellCol;
    // A cell realized on demand for a screen reader that navigated off the
    // viewport (fyne#637): it is not part of its grid's child structure — its
    // parent is set directly to the grid so fragment navigation resolves — and
    // it exposes the VirtualizedItem pattern so the client can Realize (scroll)
    // it into view.
    BOOL virtualizedItem;
};

#define ELEM_FROM(iface, member) \
    ((BelluiUIAElement*)((char*)(iface) - offsetof(BelluiUIAElement, member)))

// A text range: [start, end) in rune offsets against the element's live
// text (resolved through the Go callbacks on every query, so ranges never
// hold stale content).
typedef struct {
    ITextRangeProvider iface;
    LONG refCount;
    BelluiUIAElement* elem; // ref-counted
    long start, end;
} BelluiUIATextRange;

static struct IRawElementProviderSimpleVtbl g_simpleVtbl;
static struct IRawElementProviderFragmentVtbl g_fragmentVtbl;
static struct IRawElementProviderFragmentRootVtbl g_fragRootVtbl;
static struct IInvokeProviderVtbl g_invokeVtbl;
static struct IToggleProviderVtbl g_toggleVtbl;
static struct IExpandCollapseProviderVtbl g_expandVtbl;
static struct IRangeValueProviderVtbl g_rangeVtbl;
static struct ISelectionItemProviderVtbl g_selItemVtbl;
static struct IValueProviderVtbl g_valueVtbl;
static struct ITextProviderVtbl g_textVtbl;
static struct IGridProviderVtbl g_gridVtbl;
static struct IGridItemProviderVtbl g_gridItemVtbl;
static struct IVirtualizedItemProviderVtbl g_vitemVtbl;
static struct ITextRangeProviderVtbl g_textRangeVtbl;
static BOOL g_vtblInit = FALSE;
static void initVtbls(void);

// UiaRaiseNotificationEvent needs Windows 10 1709; loaded dynamically so the
// binary still starts on older systems (announcements just no-op there).
typedef HRESULT(WINAPI* PFN_UiaRaiseNotificationEvent)(
    IRawElementProviderSimple*, enum NotificationKind, enum NotificationProcessing, BSTR, BSTR);
static PFN_UiaRaiseNotificationEvent pfnRaiseNotification = NULL;
static BOOL g_notificationLoaded = FALSE;

// ============================================================
// Per-window registry
// ============================================================

typedef struct BelluiUIAWindow {
    HWND hwnd;
    WNDPROC origWndProc;
    BelluiUIAElement* root;    // synthetic fragment root, children = forest roots
    BelluiUIAElement* focused; // ref-counted; NULL when nothing focused
    struct BelluiUIAWindow* next;
} BelluiUIAWindow;

static BelluiUIAWindow* g_windows = NULL;

static BelluiUIAWindow* windowFor(HWND hwnd) {
    for (BelluiUIAWindow* w = g_windows; w; w = w->next) {
        if (w->hwnd == hwnd) {
            return w;
        }
    }
    return NULL;
}

// ============================================================
// Helpers
// ============================================================

static WCHAR* utf8ToWide(const char* utf8) {
    if (!utf8) {
        utf8 = "";
    }
    int len = MultiByteToWideChar(CP_UTF8, 0, utf8, -1, NULL, 0);
    if (len <= 0) {
        len = 1;
    }
    WCHAR* w = (WCHAR*)calloc(len, sizeof(WCHAR));
    if (w && len > 1) {
        MultiByteToWideChar(CP_UTF8, 0, utf8, -1, w, len);
    }
    return w;
}

// Wraps a Go-allocated C string (C.CString) as a BSTR and frees it.
static BSTR bstrTakeUTF8(char* s) {
    if (!s) {
        return SysAllocString(L"");
    }
    int len = MultiByteToWideChar(CP_UTF8, 0, s, -1, NULL, 0);
    BSTR b = SysAllocStringLen(NULL, len > 0 ? len - 1 : 0);
    if (b && len > 1) {
        MultiByteToWideChar(CP_UTF8, 0, s, -1, b, len);
    }
    free(s);
    return b;
}

static char* wideToUTF8(const WCHAR* w) {
    if (!w) {
        w = L"";
    }
    int len = WideCharToMultiByte(CP_UTF8, 0, w, -1, NULL, 0, NULL, NULL);
    if (len <= 0) {
        len = 1;
    }
    char* s = (char*)calloc(len, 1);
    if (s && len > 1) {
        WideCharToMultiByte(CP_UTF8, 0, w, -1, s, len, NULL, NULL);
    }
    return s;
}

static BOOL clientsListening(void) {
    return UiaClientsAreListening();
}

static void raisePropChangedBSTR(BelluiUIAElement* e, PROPERTYID pid, const WCHAR* oldVal, const WCHAR* newVal) {
    VARIANT vOld, vNew;
    VariantInit(&vOld);
    VariantInit(&vNew);
    vOld.vt = VT_BSTR;
    vOld.bstrVal = SysAllocString(oldVal ? oldVal : L"");
    vNew.vt = VT_BSTR;
    vNew.bstrVal = SysAllocString(newVal ? newVal : L"");
    UiaRaiseAutomationPropertyChangedEvent(&e->simple, pid, vOld, vNew);
    VariantClear(&vOld);
    VariantClear(&vNew);
}

static void raisePropChangedI4(BelluiUIAElement* e, PROPERTYID pid, int oldVal, int newVal) {
    VARIANT vOld, vNew;
    VariantInit(&vOld);
    VariantInit(&vNew);
    vOld.vt = VT_I4;
    vOld.lVal = oldVal;
    vNew.vt = VT_I4;
    vNew.lVal = newVal;
    UiaRaiseAutomationPropertyChangedEvent(&e->simple, pid, vOld, vNew);
}

static void raisePropChangedR8(BelluiUIAElement* e, PROPERTYID pid, double oldVal, double newVal) {
    VARIANT vOld, vNew;
    VariantInit(&vOld);
    VariantInit(&vNew);
    vOld.vt = VT_R8;
    vOld.dblVal = oldVal;
    vNew.vt = VT_R8;
    vNew.dblVal = newVal;
    UiaRaiseAutomationPropertyChangedEvent(&e->simple, pid, vOld, vNew);
}

static void raisePropChangedBool(BelluiUIAElement* e, PROPERTYID pid, BOOL oldVal, BOOL newVal) {
    VARIANT vOld, vNew;
    VariantInit(&vOld);
    VariantInit(&vNew);
    vOld.vt = VT_BOOL;
    vOld.boolVal = oldVal ? VARIANT_TRUE : VARIANT_FALSE;
    vNew.vt = VT_BOOL;
    vNew.boolVal = newVal ? VARIANT_TRUE : VARIANT_FALSE;
    UiaRaiseAutomationPropertyChangedEvent(&e->simple, pid, vOld, vNew);
}

// ============================================================
// QueryInterface / ref counting (shared by every interface part)
// ============================================================

static void elemFree(BelluiUIAElement* e) {
    free(e->name);
    free(e->help);
    free(e->valueText);
    free(e->children);
    free(e);
}

static ULONG elemAddRef(BelluiUIAElement* e) {
    return (ULONG)InterlockedIncrement(&e->refCount);
}

static ULONG elemRelease(BelluiUIAElement* e) {
    LONG c = InterlockedDecrement(&e->refCount);
    if (c == 0) {
        elemFree(e);
    }
    return (ULONG)c;
}

static HRESULT elemQI(BelluiUIAElement* e, REFIID riid, void** ppv) {
    if (!ppv) {
        return E_POINTER;
    }
    void* out = NULL;
    if (IsEqualIID(riid, &IID_IUnknown) || IsEqualIID(riid, &IID_IRawElementProviderSimple)) {
        out = &e->simple;
    } else if (IsEqualIID(riid, &IID_IRawElementProviderFragment)) {
        out = &e->fragment;
    } else if (e->isRoot && IsEqualIID(riid, &IID_IRawElementProviderFragmentRoot)) {
        out = &e->fragRoot;
    } else if (e->invokable && IsEqualIID(riid, &IID_IInvokeProvider)) {
        out = &e->invoke;
    } else if (e->toggleState >= 0 && IsEqualIID(riid, &IID_IToggleProvider)) {
        out = &e->toggle;
    } else if (e->expandState >= 0 && IsEqualIID(riid, &IID_IExpandCollapseProvider)) {
        out = &e->expandCollapse;
    } else if (e->hasRange && IsEqualIID(riid, &IID_IRangeValueProvider)) {
        out = &e->range;
    } else if (e->selectable >= 0 && IsEqualIID(riid, &IID_ISelectionItemProvider)) {
        out = &e->selItem;
    } else if ((e->textElement || (e->valueText && e->valueText[0])) && IsEqualIID(riid, &IID_IValueProvider)) {
        out = &e->value;
    } else if (e->textElement && IsEqualIID(riid, &IID_ITextProvider)) {
        out = &e->text;
    } else if (e->hasGridDims && IsEqualIID(riid, &IID_IGridProvider)) {
        out = &e->grid;
    } else if (e->hasCell && IsEqualIID(riid, &IID_IGridItemProvider)) {
        out = &e->gridItem;
    } else if (e->virtualizedItem && IsEqualIID(riid, &IID_IVirtualizedItemProvider)) {
        out = &e->vitem;
    }
    if (!out) {
        *ppv = NULL;
        return E_NOINTERFACE;
    }
    elemAddRef(e);
    *ppv = out;
    return S_OK;
}

// Per-interface QI/AddRef/Release thunks.
#define ELEM_IUNKNOWN(prefix, itype, member)                                                       \
    static HRESULT STDMETHODCALLTYPE prefix##_QI(itype* This, REFIID riid, void** ppv) {           \
        return elemQI(ELEM_FROM(This, member), riid, ppv);                                         \
    }                                                                                              \
    static ULONG STDMETHODCALLTYPE prefix##_AddRef(itype* This) {                                  \
        return elemAddRef(ELEM_FROM(This, member));                                                \
    }                                                                                              \
    static ULONG STDMETHODCALLTYPE prefix##_Release(itype* This) {                                 \
        return elemRelease(ELEM_FROM(This, member));                                               \
    }

ELEM_IUNKNOWN(S, IRawElementProviderSimple, simple)
ELEM_IUNKNOWN(F, IRawElementProviderFragment, fragment)
ELEM_IUNKNOWN(FR, IRawElementProviderFragmentRoot, fragRoot)
ELEM_IUNKNOWN(INV, IInvokeProvider, invoke)
ELEM_IUNKNOWN(TG, IToggleProvider, toggle)
ELEM_IUNKNOWN(EC, IExpandCollapseProvider, expandCollapse)
ELEM_IUNKNOWN(RV, IRangeValueProvider, range)
ELEM_IUNKNOWN(SI, ISelectionItemProvider, selItem)
ELEM_IUNKNOWN(VAL, IValueProvider, value)
ELEM_IUNKNOWN(TXT, ITextProvider, text)
ELEM_IUNKNOWN(GR, IGridProvider, grid)
ELEM_IUNKNOWN(GI, IGridItemProvider, gridItem)
ELEM_IUNKNOWN(VI, IVirtualizedItemProvider, vitem)

// ============================================================
// IRawElementProviderSimple
// ============================================================

static HRESULT STDMETHODCALLTYPE S_get_ProviderOptions(IRawElementProviderSimple* This, enum ProviderOptions* pRetVal) {
    (void)This;
    if (!pRetVal) {
        return E_POINTER;
    }
    *pRetVal = (enum ProviderOptions)(ProviderOptions_ServerSideProvider | ProviderOptions_UseComThreading);
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE S_GetPatternProvider(IRawElementProviderSimple* This, PATTERNID patternId, IUnknown** pRetVal) {
    if (!pRetVal) {
        return E_POINTER;
    }
    *pRetVal = NULL;
    BelluiUIAElement* e = ELEM_FROM(This, simple);
    void* out = NULL;
    switch (patternId) {
    case UIA_InvokePatternId:
        if (e->invokable) {
            out = &e->invoke;
        }
        break;
    case UIA_TogglePatternId:
        if (e->toggleState >= 0) {
            out = &e->toggle;
        }
        break;
    case UIA_ExpandCollapsePatternId:
        if (e->expandState >= 0) {
            out = &e->expandCollapse;
        }
        break;
    case UIA_RangeValuePatternId:
        if (e->hasRange) {
            out = &e->range;
        }
        break;
    case UIA_SelectionItemPatternId:
        if (e->selectable >= 0) {
            out = &e->selItem;
        }
        break;
    case UIA_ValuePatternId:
        if (e->textElement || (e->valueText && e->valueText[0])) {
            out = &e->value;
        }
        break;
    case UIA_TextPatternId:
        if (e->textElement) {
            out = &e->text;
        }
        break;
    case UIA_GridPatternId:
        if (e->hasGridDims) {
            out = &e->grid;
        }
        break;
    case UIA_GridItemPatternId:
        if (e->hasCell) {
            out = &e->gridItem;
        }
        break;
    case UIA_VirtualizedItemPatternId:
        if (e->virtualizedItem) {
            out = &e->vitem;
        }
        break;
    }
    if (out) {
        elemAddRef(e);
        *pRetVal = (IUnknown*)out;
    }
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE S_GetPropertyValue(IRawElementProviderSimple* This, PROPERTYID pid, VARIANT* pRetVal) {
    if (!pRetVal) {
        return E_POINTER;
    }
    VariantInit(pRetVal);
    BelluiUIAElement* e = ELEM_FROM(This, simple);

    if (e->isRoot) {
        // The host provider (from the HWND) answers most root properties;
        // it reports IsKeyboardFocusable=FALSE, so override focus here.
        switch (pid) {
        case UIA_IsKeyboardFocusablePropertyId:
        case UIA_IsControlElementPropertyId:
        case UIA_IsContentElementPropertyId:
        case UIA_IsEnabledPropertyId:
            pRetVal->vt = VT_BOOL;
            pRetVal->boolVal = VARIANT_TRUE;
            break;
        case UIA_HasKeyboardFocusPropertyId:
            pRetVal->vt = VT_BOOL;
            pRetVal->boolVal = (GetForegroundWindow() == e->hwnd) ? VARIANT_TRUE : VARIANT_FALSE;
            break;
        case UIA_FrameworkIdPropertyId:
            pRetVal->vt = VT_BSTR;
            pRetVal->bstrVal = SysAllocString(L"Bellui");
            break;
        }
        return S_OK;
    }

    switch (pid) {
    case UIA_ControlTypePropertyId:
        pRetVal->vt = VT_I4;
        pRetVal->lVal = e->controlType;
        break;
    case UIA_NamePropertyId:
        pRetVal->vt = VT_BSTR;
        pRetVal->bstrVal = SysAllocString(e->name ? e->name : L"");
        break;
    case UIA_HelpTextPropertyId:
        if (e->help && e->help[0]) {
            pRetVal->vt = VT_BSTR;
            pRetVal->bstrVal = SysAllocString(e->help);
        }
        break;
    case UIA_AutomationIdPropertyId: {
        WCHAR buf[32];
        wsprintfW(buf, L"bellui_%u", (unsigned int)e->handle);
        pRetVal->vt = VT_BSTR;
        pRetVal->bstrVal = SysAllocString(buf);
        break;
    }
    case UIA_FrameworkIdPropertyId:
        pRetVal->vt = VT_BSTR;
        pRetVal->bstrVal = SysAllocString(L"Bellui");
        break;
    case UIA_IsEnabledPropertyId:
        pRetVal->vt = VT_BOOL;
        pRetVal->boolVal = e->enabled ? VARIANT_TRUE : VARIANT_FALSE;
        break;
    case UIA_IsKeyboardFocusablePropertyId:
        pRetVal->vt = VT_BOOL;
        pRetVal->boolVal = e->focusable ? VARIANT_TRUE : VARIANT_FALSE;
        break;
    case UIA_HasKeyboardFocusPropertyId:
        pRetVal->vt = VT_BOOL;
        pRetVal->boolVal = e->focused ? VARIANT_TRUE : VARIANT_FALSE;
        break;
    case UIA_IsControlElementPropertyId:
    case UIA_IsContentElementPropertyId:
        pRetVal->vt = VT_BOOL;
        pRetVal->boolVal = e->structural ? VARIANT_FALSE : VARIANT_TRUE;
        break;
    case UIA_IsOffscreenPropertyId:
        pRetVal->vt = VT_BOOL;
        pRetVal->boolVal = e->offscreen ? VARIANT_TRUE : VARIANT_FALSE;
        break;
    case UIA_IsPasswordPropertyId:
        pRetVal->vt = VT_BOOL;
        pRetVal->boolVal = e->isPassword ? VARIANT_TRUE : VARIANT_FALSE;
        break;
    case UIA_IsDialogPropertyId:
        pRetVal->vt = VT_BOOL;
        pRetVal->boolVal = e->isDialog ? VARIANT_TRUE : VARIANT_FALSE;
        break;
    case UIA_LevelPropertyId:
        if (e->level > 0) {
            pRetVal->vt = VT_I4;
            pRetVal->lVal = e->level;
        }
        break;
    case UIA_HeadingLevelPropertyId:
        pRetVal->vt = VT_I4;
        pRetVal->lVal = e->headingLevel > 0 ? HeadingLevel1 + (e->headingLevel - 1) : HeadingLevel_None;
        break;
    }
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE S_get_HostRawElementProvider(IRawElementProviderSimple* This, IRawElementProviderSimple** pRetVal) {
    if (!pRetVal) {
        return E_POINTER;
    }
    *pRetVal = NULL;
    BelluiUIAElement* e = ELEM_FROM(This, simple);
    if (e->isRoot) {
        return UiaHostProviderFromHwnd(e->hwnd, pRetVal);
    }
    return S_OK;
}

// ============================================================
// IRawElementProviderFragment
// ============================================================

static HRESULT STDMETHODCALLTYPE F_Navigate(IRawElementProviderFragment* This, enum NavigateDirection direction, IRawElementProviderFragment** pRetVal) {
    if (!pRetVal) {
        return E_POINTER;
    }
    *pRetVal = NULL;
    BelluiUIAElement* e = ELEM_FROM(This, fragment);
    BelluiUIAElement* target = NULL;

    switch (direction) {
    case NavigateDirection_Parent:
        target = e->parent;
        break;
    case NavigateDirection_FirstChild:
        if (e->childCount > 0) {
            target = e->children[0];
        }
        break;
    case NavigateDirection_LastChild:
        if (e->childCount > 0) {
            target = e->children[e->childCount - 1];
        }
        break;
    case NavigateDirection_NextSibling:
    case NavigateDirection_PreviousSibling: {
        BelluiUIAElement* p = e->parent;
        if (!p) {
            break;
        }
        int idx = -1;
        for (int i = 0; i < p->childCount; i++) {
            if (p->children[i] == e) {
                idx = i;
                break;
            }
        }
        if (idx < 0) {
            break;
        }
        idx += (direction == NavigateDirection_NextSibling) ? 1 : -1;
        if (idx >= 0 && idx < p->childCount) {
            target = p->children[idx];
        }
        break;
    }
    default:
        break;
    }

    if (target) {
        elemAddRef(target);
        *pRetVal = &target->fragment;
    }
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE F_GetRuntimeId(IRawElementProviderFragment* This, SAFEARRAY** pRetVal) {
    if (!pRetVal) {
        return E_POINTER;
    }
    BelluiUIAElement* e = ELEM_FROM(This, fragment);
    if (e->isRoot) {
        // The host provider supplies the root's runtime id.
        *pRetVal = NULL;
        return S_OK;
    }
    SAFEARRAY* psa = SafeArrayCreateVector(VT_I4, 0, 2);
    if (!psa) {
        return E_OUTOFMEMORY;
    }
    LONG idx = 0;
    int val = UiaAppendRuntimeId;
    SafeArrayPutElement(psa, &idx, &val);
    idx = 1;
    val = (int)e->handle;
    SafeArrayPutElement(psa, &idx, &val);
    *pRetVal = psa;
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE F_get_BoundingRectangle(IRawElementProviderFragment* This, struct UiaRect* pRetVal) {
    if (!pRetVal) {
        return E_POINTER;
    }
    BelluiUIAElement* e = ELEM_FROM(This, fragment);
    if (e->isRoot) {
        RECT rc;
        GetClientRect(e->hwnd, &rc);
        POINT pt = {0, 0};
        ClientToScreen(e->hwnd, &pt);
        pRetVal->left = pt.x;
        pRetVal->top = pt.y;
        pRetVal->width = rc.right - rc.left;
        pRetVal->height = rc.bottom - rc.top;
        return S_OK;
    }
    POINT pt = {(LONG)e->x, (LONG)e->y};
    ClientToScreen(e->hwnd, &pt);
    pRetVal->left = pt.x;
    pRetVal->top = pt.y;
    pRetVal->width = e->w;
    pRetVal->height = e->h;
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE F_GetEmbeddedFragmentRoots(IRawElementProviderFragment* This, SAFEARRAY** pRetVal) {
    (void)This;
    if (pRetVal) {
        *pRetVal = NULL;
    }
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE F_SetFocus(IRawElementProviderFragment* This) {
    BelluiUIAElement* e = ELEM_FROM(This, fragment);
    if (!e->isRoot && !e->disconnected) {
        belluiWinA11ySetFocus(e->handle);
    }
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE F_get_FragmentRoot(IRawElementProviderFragment* This, IRawElementProviderFragmentRoot** pRetVal) {
    if (!pRetVal) {
        return E_POINTER;
    }
    *pRetVal = NULL;
    BelluiUIAElement* e = ELEM_FROM(This, fragment);
    while (e && !e->isRoot) {
        e = e->parent;
    }
    if (e) {
        elemAddRef(e);
        *pRetVal = &e->fragRoot;
    }
    return S_OK;
}

// ============================================================
// IRawElementProviderFragmentRoot (window roots only)
// ============================================================

static BelluiUIAElement* hitTest(BelluiUIAElement* e, double cx, double cy) {
    for (int i = e->childCount - 1; i >= 0; i--) {
        BelluiUIAElement* hit = hitTest(e->children[i], cx, cy);
        if (hit) {
            return hit;
        }
    }
    if (!e->isRoot && !e->structural &&
        cx >= e->x && cx < e->x + e->w && cy >= e->y && cy < e->y + e->h) {
        return e;
    }
    return NULL;
}

static HRESULT STDMETHODCALLTYPE FR_ElementProviderFromPoint(IRawElementProviderFragmentRoot* This, double x, double y, IRawElementProviderFragment** pRetVal) {
    if (!pRetVal) {
        return E_POINTER;
    }
    *pRetVal = NULL;
    BelluiUIAElement* root = ELEM_FROM(This, fragRoot);
    POINT pt = {(LONG)x, (LONG)y};
    ScreenToClient(root->hwnd, &pt);
    BelluiUIAElement* hit = hitTest(root, pt.x, pt.y);
    if (hit) {
        elemAddRef(hit);
        *pRetVal = &hit->fragment;
    }
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE FR_GetFocus(IRawElementProviderFragmentRoot* This, IRawElementProviderFragment** pRetVal) {
    if (!pRetVal) {
        return E_POINTER;
    }
    *pRetVal = NULL;
    BelluiUIAElement* root = ELEM_FROM(This, fragRoot);
    BelluiUIAWindow* w = windowFor(root->hwnd);
    if (w && w->focused) {
        elemAddRef(w->focused);
        *pRetVal = &w->focused->fragment;
    }
    return S_OK;
}

// ============================================================
// Action patterns
// ============================================================

static HRESULT STDMETHODCALLTYPE INV_Invoke(IInvokeProvider* This) {
    BelluiUIAElement* e = ELEM_FROM(This, invoke);
    if (e->disconnected || !e->enabled) {
        return UIA_E_ELEMENTNOTENABLED;
    }
    if (!belluiWinA11yInvoke(e->handle)) {
        return E_FAIL;
    }
    if (clientsListening()) {
        UiaRaiseAutomationEvent(&e->simple, UIA_Invoke_InvokedEventId);
    }
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE TG_Toggle(IToggleProvider* This) {
    BelluiUIAElement* e = ELEM_FROM(This, toggle);
    if (e->disconnected || !e->enabled) {
        return UIA_E_ELEMENTNOTENABLED;
    }
    return belluiWinA11yToggle(e->handle) ? S_OK : E_FAIL;
}

static HRESULT STDMETHODCALLTYPE TG_get_ToggleState(IToggleProvider* This, enum ToggleState* pRetVal) {
    if (!pRetVal) {
        return E_POINTER;
    }
    BelluiUIAElement* e = ELEM_FROM(This, toggle);
    *pRetVal = (enum ToggleState)(e->toggleState >= 0 ? e->toggleState : ToggleState_Off);
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE EC_Expand(IExpandCollapseProvider* This) {
    BelluiUIAElement* e = ELEM_FROM(This, expandCollapse);
    if (e->disconnected || !e->enabled) {
        return UIA_E_ELEMENTNOTENABLED;
    }
    return belluiWinA11yExpandCollapse(e->handle, 1) ? S_OK : E_FAIL;
}

static HRESULT STDMETHODCALLTYPE EC_Collapse(IExpandCollapseProvider* This) {
    BelluiUIAElement* e = ELEM_FROM(This, expandCollapse);
    if (e->disconnected || !e->enabled) {
        return UIA_E_ELEMENTNOTENABLED;
    }
    return belluiWinA11yExpandCollapse(e->handle, 0) ? S_OK : E_FAIL;
}

static HRESULT STDMETHODCALLTYPE EC_get_ExpandCollapseState(IExpandCollapseProvider* This, enum ExpandCollapseState* pRetVal) {
    if (!pRetVal) {
        return E_POINTER;
    }
    BelluiUIAElement* e = ELEM_FROM(This, expandCollapse);
    *pRetVal = e->expandState == 1 ? ExpandCollapseState_Expanded : ExpandCollapseState_Collapsed;
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE RV_SetValue(IRangeValueProvider* This, double val) {
    BelluiUIAElement* e = ELEM_FROM(This, range);
    if (e->disconnected || !e->enabled || !e->rangeSettable) {
        return UIA_E_ELEMENTNOTENABLED;
    }
    return belluiWinA11yRangeSetValue(e->handle, val) ? S_OK : E_FAIL;
}

static HRESULT STDMETHODCALLTYPE RV_get_Value(IRangeValueProvider* This, double* pRetVal) {
    if (!pRetVal) {
        return E_POINTER;
    }
    *pRetVal = ELEM_FROM(This, range)->rangeValue;
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE RV_get_IsReadOnly(IRangeValueProvider* This, WINBOOL* pRetVal) {
    if (!pRetVal) {
        return E_POINTER;
    }
    BelluiUIAElement* e = ELEM_FROM(This, range);
    *pRetVal = (!e->rangeSettable || !e->enabled) ? TRUE : FALSE;
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE RV_get_Maximum(IRangeValueProvider* This, double* pRetVal) {
    if (!pRetVal) {
        return E_POINTER;
    }
    *pRetVal = ELEM_FROM(This, range)->rangeMax;
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE RV_get_Minimum(IRangeValueProvider* This, double* pRetVal) {
    if (!pRetVal) {
        return E_POINTER;
    }
    *pRetVal = ELEM_FROM(This, range)->rangeMin;
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE RV_get_LargeChange(IRangeValueProvider* This, double* pRetVal) {
    (void)This;
    if (!pRetVal) {
        return E_POINTER;
    }
    *pRetVal = 0;
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE RV_get_SmallChange(IRangeValueProvider* This, double* pRetVal) {
    (void)This;
    if (!pRetVal) {
        return E_POINTER;
    }
    *pRetVal = 0;
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE SI_Select(ISelectionItemProvider* This) {
    BelluiUIAElement* e = ELEM_FROM(This, selItem);
    if (e->disconnected || !e->enabled) {
        return UIA_E_ELEMENTNOTENABLED;
    }
    return belluiWinA11ySelect(e->handle) ? S_OK : E_FAIL;
}

static HRESULT STDMETHODCALLTYPE SI_AddToSelection(ISelectionItemProvider* This) {
    return SI_Select(This);
}

static HRESULT STDMETHODCALLTYPE SI_RemoveFromSelection(ISelectionItemProvider* This) {
    (void)This;
    return E_FAIL; // bellui selection containers have no explicit deselect
}

static HRESULT STDMETHODCALLTYPE SI_get_IsSelected(ISelectionItemProvider* This, WINBOOL* pRetVal) {
    if (!pRetVal) {
        return E_POINTER;
    }
    *pRetVal = ELEM_FROM(This, selItem)->selectable == 1 ? TRUE : FALSE;
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE SI_get_SelectionContainer(ISelectionItemProvider* This, IRawElementProviderSimple** pRetVal) {
    if (!pRetVal) {
        return E_POINTER;
    }
    *pRetVal = NULL;
    BelluiUIAElement* e = ELEM_FROM(This, selItem)->parent;
    while (e && !e->isRoot) {
        switch (e->controlType) {
        case UIA_ListControlTypeId:
        case UIA_TreeControlTypeId:
        case UIA_TabControlTypeId:
        case UIA_TableControlTypeId:
        case UIA_DataGridControlTypeId:
            elemAddRef(e);
            *pRetVal = &e->simple;
            return S_OK;
        }
        e = e->parent;
    }
    return S_OK;
}

// ============================================================
// Value pattern
// ============================================================

static HRESULT STDMETHODCALLTYPE VAL_SetValue(IValueProvider* This, LPCWSTR val) {
    BelluiUIAElement* e = ELEM_FROM(This, value);
    if (e->disconnected || !e->enabled || e->readOnly || !e->textElement) {
        return UIA_E_ELEMENTNOTENABLED;
    }
    char* utf8 = wideToUTF8(val);
    belluiWinA11yValueSet(e->handle, utf8);
    free(utf8);
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE VAL_get_Value(IValueProvider* This, BSTR* pRetVal) {
    if (!pRetVal) {
        return E_POINTER;
    }
    BelluiUIAElement* e = ELEM_FROM(This, value);
    if (e->textElement && !e->disconnected) {
        // Resolved live against the widget so the value is never a stale
        // per-frame copy (and huge documents are not snapshotted on paint).
        *pRetVal = bstrTakeUTF8(belluiWinA11yTextForRange(e->handle, 0, -1));
        return S_OK;
    }
    *pRetVal = SysAllocString(e->valueText ? e->valueText : L"");
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE VAL_get_IsReadOnly(IValueProvider* This, WINBOOL* pRetVal) {
    if (!pRetVal) {
        return E_POINTER;
    }
    BelluiUIAElement* e = ELEM_FROM(This, value);
    *pRetVal = (e->readOnly || !e->textElement) ? TRUE : FALSE;
    return S_OK;
}

// ============================================================
// Grid / GridItem patterns
// ============================================================

static BelluiUIAElement* findCell(BelluiUIAElement* e, int row, int col) {
    if (e->hasCell && e->cellRow == row && e->cellCol == col) {
        return e;
    }
    for (int i = 0; i < e->childCount; i++) {
        BelluiUIAElement* hit = findCell(e->children[i], row, col);
        if (hit) {
            return hit;
        }
    }
    return NULL;
}

static HRESULT STDMETHODCALLTYPE GR_GetItem(IGridProvider* This, int row, int column, IRawElementProviderSimple** pRetVal) {
    if (!pRetVal) {
        return E_POINTER;
    }
    *pRetVal = NULL;
    BelluiUIAElement* e = ELEM_FROM(This, grid);
    if (row < 0 || column < 0 || row >= e->gridRows || column >= e->gridCols) {
        return E_INVALIDARG;
    }
    // Visible cells exist in the tree already; a cell scrolled off the viewport
    // is realized on demand through the neutral seam (fyne#637) so a screen
    // reader can read past the viewport in a million-row grid without
    // materializing the rows between.
    BelluiUIAElement* cell = findCell(e, row, column);
    if (!cell) {
        cell = (BelluiUIAElement*)belluiWinA11yRealizeCell(e->handle, row, column);
    }
    if (cell) {
        elemAddRef(cell);
        *pRetVal = &cell->simple;
    }
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE GR_get_RowCount(IGridProvider* This, int* pRetVal) {
    if (!pRetVal) {
        return E_POINTER;
    }
    *pRetVal = ELEM_FROM(This, grid)->gridRows;
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE GR_get_ColumnCount(IGridProvider* This, int* pRetVal) {
    if (!pRetVal) {
        return E_POINTER;
    }
    *pRetVal = ELEM_FROM(This, grid)->gridCols;
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE GI_get_Row(IGridItemProvider* This, int* pRetVal) {
    if (!pRetVal) {
        return E_POINTER;
    }
    *pRetVal = ELEM_FROM(This, gridItem)->cellRow;
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE GI_get_Column(IGridItemProvider* This, int* pRetVal) {
    if (!pRetVal) {
        return E_POINTER;
    }
    *pRetVal = ELEM_FROM(This, gridItem)->cellCol;
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE GI_get_RowSpan(IGridItemProvider* This, int* pRetVal) {
    (void)This;
    if (!pRetVal) {
        return E_POINTER;
    }
    *pRetVal = 1;
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE GI_get_ColumnSpan(IGridItemProvider* This, int* pRetVal) {
    (void)This;
    if (!pRetVal) {
        return E_POINTER;
    }
    *pRetVal = 1;
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE GI_get_ContainingGrid(IGridItemProvider* This, IRawElementProviderSimple** pRetVal) {
    if (!pRetVal) {
        return E_POINTER;
    }
    *pRetVal = NULL;
    BelluiUIAElement* e = ELEM_FROM(This, gridItem)->parent;
    while (e && !e->isRoot) {
        if (e->hasGridDims) {
            elemAddRef(e);
            *pRetVal = &e->simple;
            return S_OK;
        }
        e = e->parent;
    }
    return S_OK;
}

// ============================================================
// IVirtualizedItemProvider (realized off-viewport cells)
// ============================================================

// VI_Realize brings a cell realized on demand into the viewport so it becomes a
// real host: the client (a screen reader moving its cursor into an off-screen
// cell) calls this, and Go routes it to the grid's AccessibleScrollCellIntoView
// through the neutral realize seam. Reading the cell has no side effect; only
// this explicit realize scrolls (fyne#637).
static HRESULT STDMETHODCALLTYPE VI_Realize(IVirtualizedItemProvider* This) {
    BelluiUIAElement* e = ELEM_FROM(This, vitem);
    if (e->disconnected) {
        return S_OK;
    }
    belluiWinA11yRealize(e->handle);
    return S_OK;
}

// ============================================================
// Text pattern
// ============================================================

static BelluiUIATextRange* rangeCreate(BelluiUIAElement* elem, long start, long end);

static HRESULT STDMETHODCALLTYPE R_QI(ITextRangeProvider* This, REFIID riid, void** ppv) {
    if (!ppv) {
        return E_POINTER;
    }
    if (IsEqualIID(riid, &IID_IUnknown) || IsEqualIID(riid, &IID_ITextRangeProvider)) {
        BelluiUIATextRange* r = (BelluiUIATextRange*)This;
        InterlockedIncrement(&r->refCount);
        *ppv = This;
        return S_OK;
    }
    *ppv = NULL;
    return E_NOINTERFACE;
}

static ULONG STDMETHODCALLTYPE R_AddRef(ITextRangeProvider* This) {
    return (ULONG)InterlockedIncrement(&((BelluiUIATextRange*)This)->refCount);
}

static ULONG STDMETHODCALLTYPE R_Release(ITextRangeProvider* This) {
    BelluiUIATextRange* r = (BelluiUIATextRange*)This;
    LONG c = InterlockedDecrement(&r->refCount);
    if (c == 0) {
        elemRelease(r->elem);
        free(r);
    }
    return (ULONG)c;
}

static HRESULT STDMETHODCALLTYPE R_Clone(ITextRangeProvider* This, ITextRangeProvider** pRetVal) {
    if (!pRetVal) {
        return E_POINTER;
    }
    BelluiUIATextRange* r = (BelluiUIATextRange*)This;
    BelluiUIATextRange* clone = rangeCreate(r->elem, r->start, r->end);
    if (!clone) {
        return E_OUTOFMEMORY;
    }
    *pRetVal = &clone->iface;
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE R_Compare(ITextRangeProvider* This, ITextRangeProvider* range, WINBOOL* pRetVal) {
    if (!pRetVal) {
        return E_POINTER;
    }
    *pRetVal = FALSE;
    if (!range || range->lpVtbl != &g_textRangeVtbl) {
        return S_OK;
    }
    BelluiUIATextRange* a = (BelluiUIATextRange*)This;
    BelluiUIATextRange* b = (BelluiUIATextRange*)range;
    *pRetVal = (a->elem == b->elem && a->start == b->start && a->end == b->end) ? TRUE : FALSE;
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE R_CompareEndpoints(ITextRangeProvider* This, enum TextPatternRangeEndpoint endpoint,
    ITextRangeProvider* targetRange, enum TextPatternRangeEndpoint targetEndpoint, int* pRetVal) {
    if (!pRetVal) {
        return E_POINTER;
    }
    if (!targetRange || targetRange->lpVtbl != &g_textRangeVtbl) {
        return E_INVALIDARG;
    }
    BelluiUIATextRange* a = (BelluiUIATextRange*)This;
    BelluiUIATextRange* b = (BelluiUIATextRange*)targetRange;
    long av = (endpoint == TextPatternRangeEndpoint_Start) ? a->start : a->end;
    long bv = (targetEndpoint == TextPatternRangeEndpoint_Start) ? b->start : b->end;
    *pRetVal = (av < bv) ? -1 : (av > bv) ? 1 : 0;
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE R_ExpandToEnclosingUnit(ITextRangeProvider* This, enum TextUnit unit) {
    BelluiUIATextRange* r = (BelluiUIATextRange*)This;
    long start = 0, end = 0;
    belluiWinA11yTextUnitBounds(r->elem->handle, (int)unit, r->start, &start, &end);
    r->start = start;
    r->end = end;
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE R_FindAttribute(ITextRangeProvider* This, TEXTATTRIBUTEID attributeId, VARIANT val,
    WINBOOL backward, ITextRangeProvider** pRetVal) {
    (void)This;
    (void)attributeId;
    (void)val;
    (void)backward;
    if (!pRetVal) {
        return E_POINTER;
    }
    *pRetVal = NULL;
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE R_FindText(ITextRangeProvider* This, BSTR text, WINBOOL backward, WINBOOL ignoreCase,
    ITextRangeProvider** pRetVal) {
    (void)This;
    (void)text;
    (void)backward;
    (void)ignoreCase;
    if (!pRetVal) {
        return E_POINTER;
    }
    *pRetVal = NULL;
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE R_GetAttributeValue(ITextRangeProvider* This, TEXTATTRIBUTEID attributeId, VARIANT* pRetVal) {
    if (!pRetVal) {
        return E_POINTER;
    }
    VariantInit(pRetVal);
    BelluiUIATextRange* r = (BelluiUIATextRange*)This;
    if (attributeId == UIA_IsReadOnlyAttributeId) {
        pRetVal->vt = VT_BOOL;
        pRetVal->boolVal = r->elem->readOnly ? VARIANT_TRUE : VARIANT_FALSE;
        return S_OK;
    }
    IUnknown* notSupported = NULL;
    if (SUCCEEDED(UiaGetReservedNotSupportedValue(&notSupported))) {
        pRetVal->vt = VT_UNKNOWN;
        pRetVal->punkVal = notSupported;
    }
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE R_GetBoundingRectangles(ITextRangeProvider* This, SAFEARRAY** pRetVal) {
    if (!pRetVal) {
        return E_POINTER;
    }
    // Per-range geometry (#990): Go answers one l,t,w,h rectangle per visible
    // visual line of the range in client pixels (widgets without layout
    // geometry answer none); offset by the window origin into screen
    // coordinates. An empty array tells the client to fall back to the
    // element bounds.
    BelluiUIATextRange* r = (BelluiUIATextRange*)This;
    double* rects = NULL;
    int n = belluiWinA11yTextRangeRects(r->elem->handle, r->start, r->end, &rects);
    if (n <= 0 || !rects) {
        *pRetVal = SafeArrayCreateVector(VT_R8, 0, 0);
        return S_OK;
    }
    POINT origin = {0, 0};
    ClientToScreen(r->elem->hwnd, &origin);
    SAFEARRAY* psa = SafeArrayCreateVector(VT_R8, 0, (ULONG)(n * 4));
    if (!psa) {
        free(rects);
        return E_OUTOFMEMORY;
    }
    for (LONG i = 0; i < n; i++) {
        double vals[4] = {
            rects[i * 4 + 0] + (double)origin.x,
            rects[i * 4 + 1] + (double)origin.y,
            rects[i * 4 + 2],
            rects[i * 4 + 3],
        };
        for (LONG j = 0; j < 4; j++) {
            LONG idx = i * 4 + j;
            SafeArrayPutElement(psa, &idx, &vals[j]);
        }
    }
    free(rects);
    *pRetVal = psa;
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE R_GetEnclosingElement(ITextRangeProvider* This, IRawElementProviderSimple** pRetVal) {
    if (!pRetVal) {
        return E_POINTER;
    }
    BelluiUIATextRange* r = (BelluiUIATextRange*)This;
    elemAddRef(r->elem);
    *pRetVal = &r->elem->simple;
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE R_GetText(ITextRangeProvider* This, int maxLength, BSTR* pRetVal) {
    if (!pRetVal) {
        return E_POINTER;
    }
    BelluiUIATextRange* r = (BelluiUIATextRange*)This;
    long end = r->end;
    if (maxLength >= 0 && r->start + maxLength < end) {
        end = r->start + maxLength;
    }
    *pRetVal = bstrTakeUTF8(belluiWinA11yTextForRange(r->elem->handle, r->start, end));
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE R_Move(ITextRangeProvider* This, enum TextUnit unit, int count, int* pRetVal) {
    if (!pRetVal) {
        return E_POINTER;
    }
    *pRetVal = 0;
    if (count == 0) {
        return S_OK;
    }
    BelluiUIATextRange* r = (BelluiUIATextRange*)This;
    int moved = 0;
    long newStart = belluiWinA11yTextUnitStep(r->elem->handle, (int)unit, r->start, count, &moved);
    if (r->start == r->end) {
        r->start = r->end = newStart;
    } else {
        long s = 0, e = 0;
        belluiWinA11yTextUnitBounds(r->elem->handle, (int)unit, newStart, &s, &e);
        r->start = s;
        r->end = e;
    }
    *pRetVal = moved;
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE R_MoveEndpointByUnit(ITextRangeProvider* This, enum TextPatternRangeEndpoint endpoint,
    enum TextUnit unit, int count, int* pRetVal) {
    if (!pRetVal) {
        return E_POINTER;
    }
    *pRetVal = 0;
    if (count == 0) {
        return S_OK;
    }
    BelluiUIATextRange* r = (BelluiUIATextRange*)This;
    int moved = 0;
    if (endpoint == TextPatternRangeEndpoint_Start) {
        r->start = belluiWinA11yTextUnitStep(r->elem->handle, (int)unit, r->start, count, &moved);
        if (r->start > r->end) {
            r->end = r->start;
        }
    } else {
        r->end = belluiWinA11yTextUnitStep(r->elem->handle, (int)unit, r->end, count, &moved);
        if (r->end < r->start) {
            r->start = r->end;
        }
    }
    *pRetVal = moved;
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE R_MoveEndpointByRange(ITextRangeProvider* This, enum TextPatternRangeEndpoint endpoint,
    ITextRangeProvider* targetRange, enum TextPatternRangeEndpoint targetEndpoint) {
    if (!targetRange || targetRange->lpVtbl != &g_textRangeVtbl) {
        return E_INVALIDARG;
    }
    BelluiUIATextRange* r = (BelluiUIATextRange*)This;
    BelluiUIATextRange* t = (BelluiUIATextRange*)targetRange;
    long v = (targetEndpoint == TextPatternRangeEndpoint_Start) ? t->start : t->end;
    if (endpoint == TextPatternRangeEndpoint_Start) {
        r->start = v;
        if (r->end < r->start) {
            r->end = r->start;
        }
    } else {
        r->end = v;
        if (r->start > r->end) {
            r->start = r->end;
        }
    }
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE R_Select(ITextRangeProvider* This) {
    BelluiUIATextRange* r = (BelluiUIATextRange*)This;
    if (r->elem->disconnected) {
        return UIA_E_ELEMENTNOTENABLED;
    }
    belluiWinA11yTextSetSelection(r->elem->handle, r->start, r->end);
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE R_AddToSelection(ITextRangeProvider* This) {
    (void)This;
    return E_FAIL; // single selection only
}

static HRESULT STDMETHODCALLTYPE R_RemoveFromSelection(ITextRangeProvider* This) {
    (void)This;
    return E_FAIL;
}

static HRESULT STDMETHODCALLTYPE R_ScrollIntoView(ITextRangeProvider* This, WINBOOL alignToTop) {
    (void)This;
    (void)alignToTop;
    return S_OK; // bellui text widgets keep the caret visible themselves
}

static HRESULT STDMETHODCALLTYPE R_GetChildren(ITextRangeProvider* This, SAFEARRAY** pRetVal) {
    (void)This;
    if (!pRetVal) {
        return E_POINTER;
    }
    *pRetVal = NULL;
    return S_OK;
}

static BelluiUIATextRange* rangeCreate(BelluiUIAElement* elem, long start, long end) {
    BelluiUIATextRange* r = (BelluiUIATextRange*)calloc(1, sizeof(BelluiUIATextRange));
    if (!r) {
        return NULL;
    }
    r->iface.lpVtbl = &g_textRangeVtbl;
    r->refCount = 1;
    r->elem = elem;
    elemAddRef(elem);
    r->start = start;
    r->end = end;
    return r;
}

// Wraps a single range in the SAFEARRAY(VT_UNKNOWN) the ITextProvider
// enumeration methods return, transferring the reference.
static HRESULT rangeArray(BelluiUIATextRange* r, SAFEARRAY** pRetVal) {
    if (!r) {
        return E_OUTOFMEMORY;
    }
    SAFEARRAY* psa = SafeArrayCreateVector(VT_UNKNOWN, 0, 1);
    if (!psa) {
        R_Release(&r->iface);
        return E_OUTOFMEMORY;
    }
    LONG idx = 0;
    SafeArrayPutElement(psa, &idx, (IUnknown*)&r->iface); // AddRefs
    R_Release(&r->iface);
    *pRetVal = psa;
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE TXT_GetSelection(ITextProvider* This, SAFEARRAY** pRetVal) {
    if (!pRetVal) {
        return E_POINTER;
    }
    *pRetVal = NULL;
    BelluiUIAElement* e = ELEM_FROM(This, text);
    long start = 0, end = 0;
    belluiWinA11yTextSelection(e->handle, &start, &end);
    return rangeArray(rangeCreate(e, start, end), pRetVal);
}

static HRESULT STDMETHODCALLTYPE TXT_GetVisibleRanges(ITextProvider* This, SAFEARRAY** pRetVal) {
    if (!pRetVal) {
        return E_POINTER;
    }
    *pRetVal = NULL;
    BelluiUIAElement* e = ELEM_FROM(This, text);
    return rangeArray(rangeCreate(e, 0, belluiWinA11yTextLength(e->handle)), pRetVal);
}

static HRESULT STDMETHODCALLTYPE TXT_RangeFromChild(ITextProvider* This, IRawElementProviderSimple* childElement,
    ITextRangeProvider** pRetVal) {
    (void)childElement;
    if (!pRetVal) {
        return E_POINTER;
    }
    BelluiUIAElement* e = ELEM_FROM(This, text);
    BelluiUIATextRange* r = rangeCreate(e, 0, belluiWinA11yTextLength(e->handle));
    if (!r) {
        return E_OUTOFMEMORY;
    }
    *pRetVal = &r->iface;
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE TXT_RangeFromPoint(ITextProvider* This, struct UiaPoint point, ITextRangeProvider** pRetVal) {
    (void)point;
    if (!pRetVal) {
        return E_POINTER;
    }
    // No per-character geometry; a degenerate range at the caret is the
    // most useful approximation.
    BelluiUIAElement* e = ELEM_FROM(This, text);
    long start = 0, end = 0;
    belluiWinA11yTextSelection(e->handle, &start, &end);
    BelluiUIATextRange* r = rangeCreate(e, start, start);
    if (!r) {
        return E_OUTOFMEMORY;
    }
    *pRetVal = &r->iface;
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE TXT_get_DocumentRange(ITextProvider* This, ITextRangeProvider** pRetVal) {
    if (!pRetVal) {
        return E_POINTER;
    }
    BelluiUIAElement* e = ELEM_FROM(This, text);
    BelluiUIATextRange* r = rangeCreate(e, 0, belluiWinA11yTextLength(e->handle));
    if (!r) {
        return E_OUTOFMEMORY;
    }
    *pRetVal = &r->iface;
    return S_OK;
}

static HRESULT STDMETHODCALLTYPE TXT_get_SupportedTextSelection(ITextProvider* This, enum SupportedTextSelection* pRetVal) {
    (void)This;
    if (!pRetVal) {
        return E_POINTER;
    }
    *pRetVal = SupportedTextSelection_Single;
    return S_OK;
}

// ============================================================
// Vtable setup
// ============================================================

static void initVtbls(void) {
    if (g_vtblInit) {
        return;
    }

    g_simpleVtbl.QueryInterface = S_QI;
    g_simpleVtbl.AddRef = S_AddRef;
    g_simpleVtbl.Release = S_Release;
    g_simpleVtbl.get_ProviderOptions = S_get_ProviderOptions;
    g_simpleVtbl.GetPatternProvider = S_GetPatternProvider;
    g_simpleVtbl.GetPropertyValue = S_GetPropertyValue;
    g_simpleVtbl.get_HostRawElementProvider = S_get_HostRawElementProvider;

    g_fragmentVtbl.QueryInterface = F_QI;
    g_fragmentVtbl.AddRef = F_AddRef;
    g_fragmentVtbl.Release = F_Release;
    g_fragmentVtbl.Navigate = F_Navigate;
    g_fragmentVtbl.GetRuntimeId = F_GetRuntimeId;
    g_fragmentVtbl.get_BoundingRectangle = F_get_BoundingRectangle;
    g_fragmentVtbl.GetEmbeddedFragmentRoots = F_GetEmbeddedFragmentRoots;
    g_fragmentVtbl.SetFocus = F_SetFocus;
    g_fragmentVtbl.get_FragmentRoot = F_get_FragmentRoot;

    g_fragRootVtbl.QueryInterface = FR_QI;
    g_fragRootVtbl.AddRef = FR_AddRef;
    g_fragRootVtbl.Release = FR_Release;
    g_fragRootVtbl.ElementProviderFromPoint = FR_ElementProviderFromPoint;
    g_fragRootVtbl.GetFocus = FR_GetFocus;

    g_invokeVtbl.QueryInterface = INV_QI;
    g_invokeVtbl.AddRef = INV_AddRef;
    g_invokeVtbl.Release = INV_Release;
    g_invokeVtbl.Invoke = INV_Invoke;

    g_toggleVtbl.QueryInterface = TG_QI;
    g_toggleVtbl.AddRef = TG_AddRef;
    g_toggleVtbl.Release = TG_Release;
    g_toggleVtbl.Toggle = TG_Toggle;
    g_toggleVtbl.get_ToggleState = TG_get_ToggleState;

    g_expandVtbl.QueryInterface = EC_QI;
    g_expandVtbl.AddRef = EC_AddRef;
    g_expandVtbl.Release = EC_Release;
    g_expandVtbl.Expand = EC_Expand;
    g_expandVtbl.Collapse = EC_Collapse;
    g_expandVtbl.get_ExpandCollapseState = EC_get_ExpandCollapseState;

    g_rangeVtbl.QueryInterface = RV_QI;
    g_rangeVtbl.AddRef = RV_AddRef;
    g_rangeVtbl.Release = RV_Release;
    g_rangeVtbl.SetValue = RV_SetValue;
    g_rangeVtbl.get_Value = RV_get_Value;
    g_rangeVtbl.get_IsReadOnly = RV_get_IsReadOnly;
    g_rangeVtbl.get_Maximum = RV_get_Maximum;
    g_rangeVtbl.get_Minimum = RV_get_Minimum;
    g_rangeVtbl.get_LargeChange = RV_get_LargeChange;
    g_rangeVtbl.get_SmallChange = RV_get_SmallChange;

    g_selItemVtbl.QueryInterface = SI_QI;
    g_selItemVtbl.AddRef = SI_AddRef;
    g_selItemVtbl.Release = SI_Release;
    g_selItemVtbl.Select = SI_Select;
    g_selItemVtbl.AddToSelection = SI_AddToSelection;
    g_selItemVtbl.RemoveFromSelection = SI_RemoveFromSelection;
    g_selItemVtbl.get_IsSelected = SI_get_IsSelected;
    g_selItemVtbl.get_SelectionContainer = SI_get_SelectionContainer;

    g_valueVtbl.QueryInterface = VAL_QI;
    g_valueVtbl.AddRef = VAL_AddRef;
    g_valueVtbl.Release = VAL_Release;
    g_valueVtbl.SetValue = VAL_SetValue;
    g_valueVtbl.get_Value = VAL_get_Value;
    g_valueVtbl.get_IsReadOnly = VAL_get_IsReadOnly;

    g_textVtbl.QueryInterface = TXT_QI;
    g_textVtbl.AddRef = TXT_AddRef;
    g_textVtbl.Release = TXT_Release;
    g_textVtbl.GetSelection = TXT_GetSelection;
    g_textVtbl.GetVisibleRanges = TXT_GetVisibleRanges;
    g_textVtbl.RangeFromChild = TXT_RangeFromChild;
    g_textVtbl.RangeFromPoint = TXT_RangeFromPoint;
    g_textVtbl.get_DocumentRange = TXT_get_DocumentRange;
    g_textVtbl.get_SupportedTextSelection = TXT_get_SupportedTextSelection;

    g_gridVtbl.QueryInterface = GR_QI;
    g_gridVtbl.AddRef = GR_AddRef;
    g_gridVtbl.Release = GR_Release;
    g_gridVtbl.GetItem = GR_GetItem;
    g_gridVtbl.get_RowCount = GR_get_RowCount;
    g_gridVtbl.get_ColumnCount = GR_get_ColumnCount;

    g_gridItemVtbl.QueryInterface = GI_QI;
    g_gridItemVtbl.AddRef = GI_AddRef;
    g_gridItemVtbl.Release = GI_Release;
    g_gridItemVtbl.get_Row = GI_get_Row;
    g_gridItemVtbl.get_Column = GI_get_Column;
    g_gridItemVtbl.get_RowSpan = GI_get_RowSpan;
    g_gridItemVtbl.get_ColumnSpan = GI_get_ColumnSpan;
    g_gridItemVtbl.get_ContainingGrid = GI_get_ContainingGrid;

    g_vitemVtbl.QueryInterface = VI_QI;
    g_vitemVtbl.AddRef = VI_AddRef;
    g_vitemVtbl.Release = VI_Release;
    g_vitemVtbl.Realize = VI_Realize;

    g_textRangeVtbl.QueryInterface = R_QI;
    g_textRangeVtbl.AddRef = R_AddRef;
    g_textRangeVtbl.Release = R_Release;
    g_textRangeVtbl.Clone = R_Clone;
    g_textRangeVtbl.Compare = R_Compare;
    g_textRangeVtbl.CompareEndpoints = R_CompareEndpoints;
    g_textRangeVtbl.ExpandToEnclosingUnit = R_ExpandToEnclosingUnit;
    g_textRangeVtbl.FindAttribute = R_FindAttribute;
    g_textRangeVtbl.FindText = R_FindText;
    g_textRangeVtbl.GetAttributeValue = R_GetAttributeValue;
    g_textRangeVtbl.GetBoundingRectangles = R_GetBoundingRectangles;
    g_textRangeVtbl.GetEnclosingElement = R_GetEnclosingElement;
    g_textRangeVtbl.GetText = R_GetText;
    g_textRangeVtbl.Move = R_Move;
    g_textRangeVtbl.MoveEndpointByUnit = R_MoveEndpointByUnit;
    g_textRangeVtbl.MoveEndpointByRange = R_MoveEndpointByRange;
    g_textRangeVtbl.Select = R_Select;
    g_textRangeVtbl.AddToSelection = R_AddToSelection;
    g_textRangeVtbl.RemoveFromSelection = R_RemoveFromSelection;
    g_textRangeVtbl.ScrollIntoView = R_ScrollIntoView;
    g_textRangeVtbl.GetChildren = R_GetChildren;

    g_vtblInit = TRUE;
}

static BelluiUIAElement* createElement(uintptr_t handle, HWND hwnd, BOOL isRoot) {
    initVtbls();
    BelluiUIAElement* e = (BelluiUIAElement*)calloc(1, sizeof(BelluiUIAElement));
    if (!e) {
        return NULL;
    }
    e->simple.lpVtbl = &g_simpleVtbl;
    e->fragment.lpVtbl = &g_fragmentVtbl;
    e->fragRoot.lpVtbl = &g_fragRootVtbl;
    e->invoke.lpVtbl = &g_invokeVtbl;
    e->toggle.lpVtbl = &g_toggleVtbl;
    e->expandCollapse.lpVtbl = &g_expandVtbl;
    e->range.lpVtbl = &g_rangeVtbl;
    e->selItem.lpVtbl = &g_selItemVtbl;
    e->value.lpVtbl = &g_valueVtbl;
    e->text.lpVtbl = &g_textVtbl;
    e->grid.lpVtbl = &g_gridVtbl;
    e->gridItem.lpVtbl = &g_gridItemVtbl;
    e->vitem.lpVtbl = &g_vitemVtbl;
    e->refCount = 1;
    e->isRoot = isRoot;
    e->hwnd = hwnd;
    e->handle = handle;
    e->enabled = TRUE;
    e->toggleState = -1;
    e->expandState = -1;
    e->selectable = -1;
    return e;
}

// ============================================================
// Window subclass
// ============================================================

static LRESULT CALLBACK a11yWndProc(HWND hwnd, UINT msg, WPARAM wParam, LPARAM lParam) {
    BelluiUIAWindow* w = windowFor(hwnd);
    WNDPROC orig = w ? w->origWndProc : NULL;

    if (msg == WM_GETOBJECT && w && w->root && (int)(LONG)lParam == UiaRootObjectId) {
        return UiaReturnRawElementProvider(hwnd, wParam, lParam, &w->root->simple);
    }

    if (msg == WM_SETFOCUS || (msg == WM_ACTIVATE && LOWORD(wParam) != 0)) {
        PostMessageW(hwnd, WM_BELLUI_A11Y_FOCUS, 0, 0);
    }

    if (msg == WM_BELLUI_A11Y_FOCUS) {
        if (w && w->root && clientsListening()) {
            BelluiUIAElement* target = w->focused ? w->focused : w->root;
            UiaRaiseAutomationEvent(&target->simple, UIA_AutomationFocusChangedEventId);
        }
        return 0;
    }

    if (orig) {
        return CallWindowProcW(orig, hwnd, msg, wParam, lParam);
    }
    return DefWindowProcW(hwnd, msg, wParam, lParam);
}

// ============================================================
// Public API (called from Go)
// ============================================================

WinAccessibilityElementRef WinAccessibilityElementCreate(uintptr_t handle, void* hwnd) {
    return createElement(handle, (HWND)hwnd, FALSE);
}

// WinAccessibilityRealizedElementConfigure finishes an element realized on
// demand for an off-viewport cell (fyne#637): it is not part of its grid's
// child structure, so Go sets its parent directly to the grid element here (so
// fragment navigation up to the fragment root resolves) and marks it as
// exposing the VirtualizedItem pattern. The parent link is not ref-counted, in
// keeping with the "parents own children" model — Go owns the sole reference to
// a realized cell and destroys it in the same synchronous update pass that
// tears down its grid (no UIA client call interleaves), so the link never
// dangles across a client access.
void WinAccessibilityRealizedElementConfigure(WinAccessibilityElementRef ref, WinAccessibilityElementRef grid) {
    BelluiUIAElement* e = (BelluiUIAElement*)ref;
    if (!e) {
        return;
    }
    e->virtualizedItem = TRUE;
    e->parent = (BelluiUIAElement*)grid;
}

bool WinAccessibilityElementUpdate(WinAccessibilityElementRef ref, WinAccessibilityElementProps* p) {
    BelluiUIAElement* e = (BelluiUIAElement*)ref;
    bool changed = FALSE;
    BOOL fresh = !e->everUpdated;
    BOOL raise = !fresh && clientsListening();

    if (e->controlType != p->controlType) {
        e->controlType = p->controlType;
        changed = TRUE;
    }

    WCHAR* name = utf8ToWide(p->name);
    if (!e->name || wcscmp(e->name, name) != 0) {
        if (raise) {
            raisePropChangedBSTR(e, UIA_NamePropertyId, e->name, name);
        }
        free(e->name);
        e->name = name;
        changed = TRUE;
    } else {
        free(name);
    }

    WCHAR* help = utf8ToWide(p->help);
    if (!e->help || wcscmp(e->help, help) != 0) {
        free(e->help);
        e->help = help;
        changed = TRUE;
    } else {
        free(help);
    }

    WCHAR* value = utf8ToWide(p->value);
    if (!e->valueText || wcscmp(e->valueText, value) != 0) {
        if (raise) {
            raisePropChangedBSTR(e, UIA_ValueValuePropertyId, e->valueText, value);
        }
        free(e->valueText);
        e->valueText = value;
        changed = TRUE;
    } else {
        free(value);
    }

    if (e->x != p->x || e->y != p->y || e->w != p->width || e->h != p->height) {
        e->x = p->x;
        e->y = p->y;
        e->w = p->width;
        e->h = p->height;
        changed = TRUE;
    }

    if (e->enabled != (BOOL)p->enabled) {
        e->enabled = p->enabled;
        changed = TRUE;
    }
    if (e->focusable != (BOOL)p->focusable) {
        e->focusable = p->focusable;
        changed = TRUE;
    }
    if (e->focused != (BOOL)p->focused) {
        e->focused = p->focused;
        changed = TRUE;
    }
    if (e->offscreen != (BOOL)p->offscreen) {
        e->offscreen = p->offscreen;
        changed = TRUE;
    }
    if (e->isPassword != (BOOL)p->isPassword) {
        e->isPassword = p->isPassword;
        changed = TRUE;
    }
    if (e->isDialog != (BOOL)p->isDialog) {
        e->isDialog = p->isDialog;
        changed = TRUE;
    }
    if (e->structural != (BOOL)p->structural) {
        e->structural = p->structural;
        changed = TRUE;
    }
    if (e->invokable != (BOOL)p->invokable) {
        e->invokable = p->invokable;
        changed = TRUE;
    }
    if (e->toggleState != p->toggleState) {
        if (raise && e->toggleState >= 0 && p->toggleState >= 0) {
            raisePropChangedI4(e, UIA_ToggleToggleStatePropertyId, e->toggleState, p->toggleState);
        }
        e->toggleState = p->toggleState;
        changed = TRUE;
    }
    if (e->expandState != p->expandState) {
        if (raise && e->expandState >= 0 && p->expandState >= 0) {
            raisePropChangedI4(e, UIA_ExpandCollapseExpandCollapseStatePropertyId,
                e->expandState == 1 ? ExpandCollapseState_Expanded : ExpandCollapseState_Collapsed,
                p->expandState == 1 ? ExpandCollapseState_Expanded : ExpandCollapseState_Collapsed);
        }
        e->expandState = p->expandState;
        changed = TRUE;
    }
    if (e->selectable != p->selectable) {
        if (raise && e->selectable >= 0 && p->selectable >= 0) {
            raisePropChangedBool(e, UIA_SelectionItemIsSelectedPropertyId, e->selectable == 1, p->selectable == 1);
            if (p->selectable == 1) {
                UiaRaiseAutomationEvent(&e->simple, UIA_SelectionItem_ElementSelectedEventId);
            }
        }
        e->selectable = p->selectable;
        changed = TRUE;
    }
    if (e->hasRange != (BOOL)p->hasRange || e->rangeMin != p->rangeMin ||
        e->rangeMax != p->rangeMax || e->rangeValue != p->rangeValue) {
        if (raise && e->hasRange && p->hasRange && e->rangeValue != p->rangeValue) {
            raisePropChangedR8(e, UIA_RangeValueValuePropertyId, e->rangeValue, p->rangeValue);
        }
        e->hasRange = p->hasRange;
        e->rangeMin = p->rangeMin;
        e->rangeMax = p->rangeMax;
        e->rangeValue = p->rangeValue;
        changed = TRUE;
    }
    if (e->rangeSettable != (BOOL)p->rangeSettable) {
        e->rangeSettable = p->rangeSettable;
        changed = TRUE;
    }
    if (e->textElement != (BOOL)p->textElement) {
        e->textElement = p->textElement;
        changed = TRUE;
    }
    if (e->readOnly != (BOOL)p->readOnly) {
        e->readOnly = p->readOnly;
        changed = TRUE;
    }
    if (e->textVersion != p->textVersion) {
        if (raise && e->textElement) {
            UiaRaiseAutomationEvent(&e->simple, UIA_Text_TextChangedEventId);
        }
        e->textVersion = p->textVersion;
        changed = TRUE;
    }
    if (e->selStart != p->selStart || e->selEnd != p->selEnd) {
        if (raise && e->textElement) {
            UiaRaiseAutomationEvent(&e->simple, UIA_Text_TextSelectionChangedEventId);
        }
        e->selStart = p->selStart;
        e->selEnd = p->selEnd;
        changed = TRUE;
    }
    if (e->level != p->level) {
        e->level = p->level;
        changed = TRUE;
    }
    if (e->headingLevel != p->headingLevel) {
        e->headingLevel = p->headingLevel;
        changed = TRUE;
    }
    if (e->hasGridDims != (BOOL)p->hasGridDims || e->gridRows != p->gridRows || e->gridCols != p->gridCols) {
        e->hasGridDims = p->hasGridDims;
        e->gridRows = p->gridRows;
        e->gridCols = p->gridCols;
        changed = TRUE;
    }
    if (e->hasCell != (BOOL)p->hasCell || e->cellRow != p->cellRow || e->cellCol != p->cellCol) {
        e->hasCell = p->hasCell;
        e->cellRow = p->cellRow;
        e->cellCol = p->cellCol;
        changed = TRUE;
    }

    e->everUpdated = TRUE;
    return changed;
}

bool WinAccessibilityElementSetChildren(WinAccessibilityElementRef ref, WinAccessibilityElementRef* children, int count) {
    BelluiUIAElement* e = (BelluiUIAElement*)ref;

    BOOL same = (e->childCount == count);
    if (same) {
        for (int i = 0; i < count; i++) {
            if (e->children[i] != (BelluiUIAElement*)children[i]) {
                same = FALSE;
                break;
            }
        }
    }
    if (same) {
        return FALSE;
    }

    BelluiUIAElement** arr = NULL;
    if (count > 0) {
        arr = (BelluiUIAElement**)malloc(count * sizeof(BelluiUIAElement*));
        if (!arr) {
            return FALSE;
        }
        for (int i = 0; i < count; i++) {
            arr[i] = (BelluiUIAElement*)children[i];
            elemAddRef(arr[i]);
            arr[i]->parent = e;
        }
    }
    BelluiUIAElement** old = e->children;
    int oldCount = e->childCount;
    e->children = arr;
    e->childCount = count;
    for (int i = 0; i < oldCount; i++) {
        if (old[i]->parent == e) {
            // Still pointing here: either removed from the tree (about to be
            // destroyed) or re-parented later this update pass.
            BOOL kept = FALSE;
            for (int j = 0; j < count; j++) {
                if (arr && arr[j] == old[i]) {
                    kept = TRUE;
                    break;
                }
            }
            if (!kept) {
                old[i]->parent = NULL;
            }
        }
        elemRelease(old[i]);
    }
    free(old);

    if (e->everUpdated && clientsListening()) {
        int runtimeId[2] = {UiaAppendRuntimeId, (int)e->handle};
        UiaRaiseStructureChangedEvent(&e->simple, StructureChangeType_ChildrenInvalidated, runtimeId, 2);
    }
    return TRUE;
}

void WinAccessibilityElementDestroy(WinAccessibilityElementRef ref) {
    if (!ref) {
        return;
    }
    BelluiUIAElement* e = (BelluiUIAElement*)ref;
    e->disconnected = TRUE;
    UiaDisconnectProvider(&e->simple);
    for (int i = 0; i < e->childCount; i++) {
        if (e->children[i]->parent == e) {
            e->children[i]->parent = NULL;
        }
        elemRelease(e->children[i]);
    }
    free(e->children);
    e->children = NULL;
    e->childCount = 0;
    e->parent = NULL;
    elemRelease(e); // Go's reference
}

bool WinAccessibilityWindowSetRoots(void* hwndPtr, WinAccessibilityElementRef* roots, int count,
    WinAccessibilityElementRef focused) {
    HWND hwnd = (HWND)hwndPtr;
    BelluiUIAWindow* w = windowFor(hwnd);
    if (!w) {
        w = (BelluiUIAWindow*)calloc(1, sizeof(BelluiUIAWindow));
        if (!w) {
            return FALSE;
        }
        w->hwnd = hwnd;
        w->root = createElement(0, hwnd, TRUE);
        if (!w->root) {
            free(w);
            return FALSE;
        }
        w->origWndProc = (WNDPROC)SetWindowLongPtrW(hwnd, GWLP_WNDPROC, (LONG_PTR)a11yWndProc);
        w->next = g_windows;
        g_windows = w;
    }

    bool changed = WinAccessibilityElementSetChildren(w->root, roots, count);
    w->root->everUpdated = TRUE;

    BelluiUIAElement* newFocus = (BelluiUIAElement*)focused;
    if (w->focused != newFocus) {
        if (newFocus) {
            elemAddRef(newFocus);
        }
        if (w->focused) {
            elemRelease(w->focused);
        }
        w->focused = newFocus;
        changed = TRUE;
        if (newFocus && clientsListening() && GetForegroundWindow() == hwnd) {
            UiaRaiseAutomationEvent(&newFocus->simple, UIA_AutomationFocusChangedEventId);
        }
    }
    return changed;
}

void WinAccessibilityWindowCleanup(void* hwndPtr) {
    HWND hwnd = (HWND)hwndPtr;
    BelluiUIAWindow** link = &g_windows;
    while (*link && (*link)->hwnd != hwnd) {
        link = &(*link)->next;
    }
    BelluiUIAWindow* w = *link;
    if (!w) {
        return;
    }
    *link = w->next;

    if (w->origWndProc) {
        SetWindowLongPtrW(hwnd, GWLP_WNDPROC, (LONG_PTR)w->origWndProc);
    }
    if (w->focused) {
        elemRelease(w->focused);
    }
    if (w->root) {
        WinAccessibilityElementDestroy(w->root);
    }
    free(w);
}

void WinAccessibilityAnnounce(const char* text, bool assertive) {
    if (!text || !text[0]) {
        return;
    }
    if (!g_notificationLoaded) {
        g_notificationLoaded = TRUE;
        HMODULE core = GetModuleHandleW(L"uiautomationcore.dll");
        if (!core) {
            core = LoadLibraryW(L"uiautomationcore.dll");
        }
        if (core) {
            pfnRaiseNotification =
                (PFN_UiaRaiseNotificationEvent)GetProcAddress(core, "UiaRaiseNotificationEvent");
        }
    }
    if (!pfnRaiseNotification || !clientsListening()) {
        return;
    }

    // Deliver on the foreground bellui window's provider so the announcement
    // is associated with the UI the user is in; fall back to any window.
    BelluiUIAWindow* w = windowFor(GetForegroundWindow());
    if (!w) {
        w = g_windows;
    }
    if (!w || !w->root) {
        return;
    }

    WCHAR* wide = utf8ToWide(text);
    BSTR msg = SysAllocString(wide ? wide : L"");
    free(wide);
    BSTR activity = SysAllocString(L"bellui-announcement");
    pfnRaiseNotification(&w->root->simple, NotificationKind_ActionCompleted,
        assertive ? NotificationProcessing_ImportantAll : NotificationProcessing_All,
        msg, activity);
    SysFreeString(msg);
    SysFreeString(activity);
}
