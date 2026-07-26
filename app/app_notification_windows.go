//go:build !ci && !android && !ios && !wasm && !tinygo

package app

import (
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"github.com/belluikit/bellui"
)

// This is the Windows rich-notification backend for #516 Phase 2 (#722). It
// replaces the PowerShell toast sender (which has no channel back into the
// process) with WinRT ToastNotification calls plus a COM toast activator
// (INotificationActivationCallback), so a body click and action buttons route
// their callbacks back to the running app via the shared notify registry
// (app/notification.go) — mirroring the Linux D-Bus and macOS UNUserNotification
// backends. It is driven purely through x/sys/windows COM/WinRT syscalls with no
// cgo, matching the #514 native-file-dialog precedent.
//
// Activation requires, per the Windows desktop (unpackaged) toast contract:
//   - an AppUserModelID (AUMID) set on the process,
//   - a Start Menu shortcut carrying that AUMID and a ToastActivatorCLSID, and
//   - a COM server for that CLSID: a LocalServer32 registry entry (so the shell
//     can launch us when closed) plus a runtime CoRegisterClassObject (so the
//     activation reaches the live process).
//
// If any of that setup fails (e.g. pre-Windows-8 with no WinRT), the send
// degrades to the plain PowerShell notification — the notification is never
// dropped, only its callbacks and custom icon are.

// ---- COM / WinRT constants ------------------------------------------------

const (
	roInitMultithreaded    = 1
	clsctxInprocServer     = 0x1
	clsctxLocalServer      = 0x4
	regclsMultipleuse      = 1
	coinitApartmentThreade = 0x2

	sOK           = 0
	eNoInterface  = 0x80004002
	ePointer      = 0x80004003
	classENoAggr  = 0x80040110
	rpcEChangedMd = 0x80010106

	vtLPWSTR = 31
	vtCLSID  = 72

	// toastCallbackLifetime bounds how long a notification's callbacks and temp
	// icon are retained. The desktop toast API gives no dismissed/closed signal
	// without subscribing typed event handlers (a follow-up), so rather than leak
	// handlers for the process lifetime we forget them after a generous window —
	// long enough for a user to act on a toast sitting in the Action Center, after
	// which Windows itself expires it.
	toastCallbackLifetime = 10 * time.Minute
)

// IInspectable adds 6 methods over IUnknown, so WinRT interface methods begin at
// vtable slot 6.
const (
	slotQueryInterface = 0
	slotRelease        = 2

	// IXmlDocumentIO
	slotLoadXml = 6
	// IToastNotificationFactory
	slotCreateToastNotification = 6
	// IToastNotificationManagerStatics
	slotCreateToastNotifierWithID = 7
	// IToastNotifier
	slotShow = 6

	// IShellLinkW::SetPath
	slotSetPath = 20
	// IPropertyStore
	slotSetValue = 6
	slotCommit   = 7
	// IPersistFile::Save
	slotSave = 6
)

var (
	iidIUnknown                         = windows.GUID{Data1: 0x00000000, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidIClassFactory                    = windows.GUID{Data1: 0x00000001, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidINotificationActivationCallback  = windows.GUID{Data1: 0x53E31837, Data2: 0x6600, Data3: 0x4A81, Data4: [8]byte{0x93, 0x95, 0x75, 0xCF, 0xFE, 0x74, 0x6F, 0x94}}
	iidIToastNotificationManagerStatics = windows.GUID{Data1: 0x50AC103F, Data2: 0xD235, Data3: 0x4598, Data4: [8]byte{0xBB, 0xEF, 0x98, 0xFE, 0x4D, 0x1A, 0x3A, 0xD4}}
	iidIToastNotificationFactory        = windows.GUID{Data1: 0x04124B20, Data2: 0x82C6, Data3: 0x4229, Data4: [8]byte{0xB1, 0x09, 0xFD, 0x9E, 0xD4, 0x66, 0x2B, 0x53}}
	iidIXmlDocument                     = windows.GUID{Data1: 0xF7F3A506, Data2: 0x1E87, Data3: 0x42D6, Data4: [8]byte{0xBC, 0xFB, 0xB8, 0xC8, 0x09, 0xFA, 0x54, 0x94}}
	iidIXmlDocumentIO                   = windows.GUID{Data1: 0x6CD0E74E, Data2: 0xEE65, Data3: 0x4489, Data4: [8]byte{0x9E, 0xBF, 0xCA, 0x43, 0xE8, 0x7B, 0xA6, 0x37}}
	clsidShellLink                      = windows.GUID{Data1: 0x00021401, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidIShellLinkW                      = windows.GUID{Data1: 0x000214F9, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidIPersistFile                     = windows.GUID{Data1: 0x0000010B, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidIPropertyStore                   = windows.GUID{Data1: 0x886D8EEB, Data2: 0x8CF2, Data3: 0x4446, Data4: [8]byte{0x8D, 0x02, 0xCD, 0xBA, 0x1D, 0xBD, 0xCF, 0x99}}
	fmtidAppUserModel                   = windows.GUID{Data1: 0x9F4C2855, Data2: 0x9F79, Data3: 0x4B39, Data4: [8]byte{0xA8, 0xD0, 0xE1, 0xD4, 0x2D, 0xE1, 0xD5, 0xF3}}
	pkeyAppUserModelID                  = propertyKey{fmtid: fmtidAppUserModel, pid: 5}
	pkeyAppUserModelToastActivatorCLSID = propertyKey{fmtid: fmtidAppUserModel, pid: 26}
)

var (
	modcombase  = windows.NewLazySystemDLL("combase.dll")
	modole32n   = windows.NewLazySystemDLL("ole32.dll")
	modshell32n = windows.NewLazySystemDLL("shell32.dll")

	procRoInitialize           = modcombase.NewProc("RoInitialize")
	procRoActivateInstance     = modcombase.NewProc("RoActivateInstance")
	procRoGetActivationFactory = modcombase.NewProc("RoGetActivationFactory")
	procWindowsCreateString    = modcombase.NewProc("WindowsCreateString")
	procWindowsDeleteString    = modcombase.NewProc("WindowsDeleteString")
	procCoInitializeExN        = modole32n.NewProc("CoInitializeEx")
	procCoUninitializeN        = modole32n.NewProc("CoUninitialize")
	procCoCreateInstanceN      = modole32n.NewProc("CoCreateInstance")
	procCoRegisterClassObject  = modole32n.NewProc("CoRegisterClassObject")
	procSetCurrentProcessAUMID = modshell32n.NewProc("SetCurrentProcessExplicitAppUserModelID")
)

// propertyKey mirrors the Win32 PROPERTYKEY { GUID fmtid; DWORD pid }.
type propertyKey struct {
	fmtid windows.GUID
	pid   uint32
}

// propVariant is the 24-byte (x64) PROPVARIANT for the value types we set: the
// tag lives in vt, the value pointer at the 8-byte-aligned union offset.
type propVariant struct {
	vt   uint16
	_    [6]byte
	data uintptr
	_    [8]byte
}

// comInterface is the shared layout of a COM interface pointer: a pointer to its
// vtable of function pointers, indexed by slot.
type comInterface struct {
	vtbl *[64]uintptr
}

// ---- Go-implemented COM objects (class factory + activator) ----------------

// classFactoryObj and activatorObj are singletons whose first field is a vtable
// pointer, so &obj is a valid COM interface pointer. Refcounting is a no-op
// (they live for the process), and QueryInterface hands back the same singleton.
type (
	classFactoryObj struct{ vtbl *[5]uintptr }
	activatorObj    struct{ vtbl *[4]uintptr }
)

var (
	theFactory    classFactoryObj
	theActivator  activatorObj
	factoryVtbl   [5]uintptr
	activatorVtbl [4]uintptr

	// toastApp is the app whose notify registry receives routed activations.
	toastApp *belluiApp
)

func setupToastVtables() {
	factoryVtbl = [5]uintptr{
		syscall.NewCallback(factoryQueryInterface),
		syscall.NewCallback(comAddRef),
		syscall.NewCallback(comRelease),
		syscall.NewCallback(factoryCreateInstance),
		syscall.NewCallback(factoryLockServer),
	}
	theFactory.vtbl = &factoryVtbl

	activatorVtbl = [4]uintptr{
		syscall.NewCallback(activatorQueryInterface),
		syscall.NewCallback(comAddRef),
		syscall.NewCallback(comRelease),
		syscall.NewCallback(activatorActivate),
	}
	theActivator.vtbl = &activatorVtbl
}

// The COM callbacks take typed pointer parameters (rather than raw uintptr) so
// that go vet's unsafeptr check is satisfied: converting an incoming address
// back to a Go pointer via unsafe.Pointer(uintptr) is what vet flags, whereas
// syscall.NewCallback delivers pointer arguments as real pointers directly.

func comAddRef(this unsafe.Pointer) uintptr  { return 1 }
func comRelease(this unsafe.Pointer) uintptr { return 1 }

func factoryQueryInterface(this unsafe.Pointer, riid *windows.GUID, ppv *unsafe.Pointer) uintptr {
	if riid == nil || ppv == nil {
		return ePointer
	}
	if guidEqual(riid, &iidIUnknown) || guidEqual(riid, &iidIClassFactory) {
		*ppv = unsafe.Pointer(&theFactory)
		return sOK
	}
	*ppv = nil
	return eNoInterface
}

func factoryCreateInstance(this, outer unsafe.Pointer, riid *windows.GUID, ppv *unsafe.Pointer) uintptr {
	if ppv == nil {
		return ePointer
	}
	if outer != nil {
		return classENoAggr
	}
	if riid != nil && (guidEqual(riid, &iidIUnknown) || guidEqual(riid, &iidINotificationActivationCallback)) {
		*ppv = unsafe.Pointer(&theActivator)
		return sOK
	}
	*ppv = nil
	return eNoInterface
}

func factoryLockServer(this unsafe.Pointer, lock uint32) uintptr { return sOK }

func activatorQueryInterface(this unsafe.Pointer, riid *windows.GUID, ppv *unsafe.Pointer) uintptr {
	if riid == nil || ppv == nil {
		return ePointer
	}
	if guidEqual(riid, &iidIUnknown) || guidEqual(riid, &iidINotificationActivationCallback) {
		*ppv = unsafe.Pointer(&theActivator)
		return sOK
	}
	*ppv = nil
	return eNoInterface
}

// activatorActivate is INotificationActivationCallback::Activate. invokedArgs is
// the toast's launch string (body click) or an action's arguments string; both
// carry the bellui routing token.
func activatorActivate(this unsafe.Pointer, appUserModelID, invokedArgs *uint16, data unsafe.Pointer, count uint32) uintptr {
	args := ""
	if invokedArgs != nil {
		args = windows.UTF16PtrToString(invokedArgs)
	}
	routeToastActivation(args)
	return sOK
}

func guidEqual(a, b *windows.GUID) bool {
	return a.Data1 == b.Data1 && a.Data2 == b.Data2 && a.Data3 == b.Data3 && a.Data4 == b.Data4
}

// ---- toast runtime (dedicated COM thread) ---------------------------------

type toastRuntime struct {
	aumid string
	clsid windows.GUID
	ch    chan string
}

var (
	toastOnce sync.Once
	toastRT   *toastRuntime
	toastErr  error
)

// SendNotificationWithOptions posts a rich WinRT toast whose body click and
// action buttons route back through the shared notify registry. If the toast
// subsystem cannot be initialised it degrades to a plain notification.
func (a *belluiApp) SendNotificationWithOptions(opts *bellui.NotificationOptions) {
	rt, err := a.ensureToast()
	if err != nil {
		bellui.LogError("Falling back to plain Windows notification", err)
		a.SendNotification(plainNotification(opts))
		return
	}

	token := a.registerNotifyCallbacks(opts)
	iconURL, tempIcon := a.toastIconURL(opts)
	xml := buildToastXML(opts, token, iconURL)

	// Best-effort lifetime bound: without a dismissed/closed event we forget the
	// callbacks and remove the temp icon after a generous window.
	if token != 0 || tempIcon != "" {
		time.AfterFunc(toastCallbackLifetime, func() {
			a.forgetNotify(token)
			removeTempIcon(tempIcon)
		})
	}

	// Deliver on the dedicated COM thread; run the hand-off from a goroutine so a
	// stalled COM thread can never block the caller (typically the main goroutine).
	go func() { rt.ch <- xml }()
}

func (a *belluiApp) ensureToast() (*toastRuntime, error) {
	toastOnce.Do(func() {
		toastRT, toastErr = a.initToast()
	})
	return toastRT, toastErr
}

func (a *belluiApp) initToast() (*toastRuntime, error) {
	// WinRT is Windows 8+. If combase or the registration entry points are
	// missing, degrade rather than panic on first use.
	for _, p := range []*windows.LazyProc{
		procRoInitialize, procRoActivateInstance, procRoGetActivationFactory,
		procWindowsCreateString, procCoRegisterClassObject,
	} {
		if err := p.Find(); err != nil {
			return nil, err
		}
	}

	aumid := a.toastAUMID()
	if aumid == "" {
		return nil, errors.New("no application ID for toast AUMID")
	}
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	clsid := toastActivatorCLSID(aumid)

	if p, e := windows.UTF16PtrFromString(aumid); e == nil {
		procSetCurrentProcessAUMID.Call(uintptr(unsafe.Pointer(p)))
		runtime.KeepAlive(p)
	}

	if err := registerToastActivatorCLSID(clsid, exe); err != nil {
		return nil, fmt.Errorf("register toast activator CLSID: %w", err)
	}
	if err := ensureStartMenuShortcut(aumid, clsid, exe, a.toastDisplayName()); err != nil {
		return nil, fmt.Errorf("create start-menu shortcut: %w", err)
	}

	setupToastVtables()
	toastApp = a

	rt := &toastRuntime{aumid: aumid, clsid: clsid, ch: make(chan string, 8)}
	ready := make(chan error, 1)
	go rt.run(ready)
	if err := <-ready; err != nil {
		return nil, err
	}
	return rt, nil
}

// run owns the COM apartment for the process: it initialises WinRT as MTA (so
// incoming activations are dispatched on COM RPC threads with no message pump
// needed), registers the class object, then serves toast-show requests. The OS
// thread is locked and never released so the MTA and registration live for the
// process lifetime.
func (rt *toastRuntime) run(ready chan error) {
	runtime.LockOSThread()

	if hr, _, _ := procRoInitialize.Call(roInitMultithreaded); int32(hr) < 0 && uint32(hr) != rpcEChangedMd {
		ready <- fmt.Errorf("RoInitialize: %w", syscall.Errno(hr))
		return
	}

	var regToken uint32
	if hr, _, _ := procCoRegisterClassObject.Call(
		uintptr(unsafe.Pointer(&rt.clsid)),
		uintptr(unsafe.Pointer(&theFactory)),
		clsctxLocalServer, regclsMultipleuse,
		uintptr(unsafe.Pointer(&regToken)),
	); int32(hr) < 0 {
		ready <- fmt.Errorf("CoRegisterClassObject: %w", syscall.Errno(hr))
		return
	}

	ready <- nil
	for xml := range rt.ch {
		if err := rt.show(xml); err != nil {
			bellui.LogError("Windows toast notification failed", err)
		}
	}
}

// show performs the full WinRT sequence to display one toast from its XML.
func (rt *toastRuntime) show(xml string) error {
	docClass, err := newHString("Windows.Data.Xml.Dom.XmlDocument")
	if err != nil {
		return err
	}
	defer deleteHString(docClass)

	var inspectable unsafe.Pointer
	if hr, _, _ := procRoActivateInstance.Call(docClass, uintptr(unsafe.Pointer(&inspectable))); int32(hr) < 0 || inspectable == nil {
		return fmt.Errorf("RoActivateInstance XmlDocument: %w", syscall.Errno(hr))
	}
	defer release(inspectable)

	docIO, err := queryInterface(inspectable, &iidIXmlDocumentIO)
	if err != nil {
		return fmt.Errorf("QI IXmlDocumentIO: %w", err)
	}
	defer release(docIO)

	hXml, err := newHString(xml)
	if err != nil {
		return err
	}
	defer deleteHString(hXml)
	if hr := vcall(docIO, slotLoadXml, hXml); int32(hr) < 0 {
		return fmt.Errorf("LoadXml: %w", syscall.Errno(hr))
	}

	xmlDoc, err := queryInterface(inspectable, &iidIXmlDocument)
	if err != nil {
		return fmt.Errorf("QI IXmlDocument: %w", err)
	}
	defer release(xmlDoc)

	toastClass, err := newHString("Windows.UI.Notifications.ToastNotification")
	if err != nil {
		return err
	}
	defer deleteHString(toastClass)
	var toastFactory unsafe.Pointer
	if hr, _, _ := procRoGetActivationFactory.Call(toastClass, uintptr(unsafe.Pointer(&iidIToastNotificationFactory)), uintptr(unsafe.Pointer(&toastFactory))); int32(hr) < 0 || toastFactory == nil {
		return fmt.Errorf("RoGetActivationFactory ToastNotification: %w", syscall.Errno(hr))
	}
	defer release(toastFactory)

	var toast unsafe.Pointer
	if hr := vcall(toastFactory, slotCreateToastNotification, uintptr(xmlDoc), uintptr(unsafe.Pointer(&toast))); int32(hr) < 0 || toast == nil {
		return fmt.Errorf("CreateToastNotification: %w", syscall.Errno(hr))
	}
	defer release(toast)

	mgrClass, err := newHString("Windows.UI.Notifications.ToastNotificationManager")
	if err != nil {
		return err
	}
	defer deleteHString(mgrClass)
	var statics unsafe.Pointer
	if hr, _, _ := procRoGetActivationFactory.Call(mgrClass, uintptr(unsafe.Pointer(&iidIToastNotificationManagerStatics)), uintptr(unsafe.Pointer(&statics))); int32(hr) < 0 || statics == nil {
		return fmt.Errorf("RoGetActivationFactory ToastNotificationManager: %w", syscall.Errno(hr))
	}
	defer release(statics)

	hAumid, err := newHString(rt.aumid)
	if err != nil {
		return err
	}
	defer deleteHString(hAumid)
	var notifier unsafe.Pointer
	if hr := vcall(statics, slotCreateToastNotifierWithID, hAumid, uintptr(unsafe.Pointer(&notifier))); int32(hr) < 0 || notifier == nil {
		return fmt.Errorf("CreateToastNotifierWithId: %w", syscall.Errno(hr))
	}
	defer release(notifier)

	if hr := vcall(notifier, slotShow, uintptr(toast)); int32(hr) < 0 {
		return fmt.Errorf("Show: %w", syscall.Errno(hr))
	}
	return nil
}

// ---- COM helpers -----------------------------------------------------------

// vcall invokes a COM method by vtable slot on obj, passing obj as the implicit
// `this`. Go's GC is non-moving, so the argument addresses stashed in the slice
// stay valid; callers keep the referenced locals alive across the call (they
// read the out-params afterward).
func vcall(obj unsafe.Pointer, slot int, args ...uintptr) uintptr {
	o := (*comInterface)(obj)
	all := make([]uintptr, 0, len(args)+1)
	all = append(all, uintptr(obj))
	all = append(all, args...)
	hr, _, _ := syscall.SyscallN(o.vtbl[slot], all...)
	return hr
}

func queryInterface(obj unsafe.Pointer, iid *windows.GUID) (unsafe.Pointer, error) {
	var out unsafe.Pointer
	hr := vcall(obj, slotQueryInterface, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&out)))
	if int32(hr) < 0 || out == nil {
		return nil, syscall.Errno(hr)
	}
	return out, nil
}

func release(obj unsafe.Pointer) {
	if obj != nil {
		vcall(obj, slotRelease)
	}
}

// newHString creates a WinRT HSTRING from s. WindowsCreateString copies the
// buffer, so the source slice need only survive the call.
func newHString(s string) (uintptr, error) {
	p, err := windows.UTF16FromString(s)
	if err != nil {
		return 0, err
	}
	var hs uintptr
	hr, _, _ := procWindowsCreateString.Call(uintptr(unsafe.Pointer(&p[0])), uintptr(len(p)-1), uintptr(unsafe.Pointer(&hs)))
	runtime.KeepAlive(p)
	if int32(hr) < 0 {
		return 0, syscall.Errno(hr)
	}
	return hs, nil
}

func deleteHString(hs uintptr) {
	if hs != 0 {
		procWindowsDeleteString.Call(hs)
	}
}

// ---- AUMID / activator registration ---------------------------------------

// toastActivatorCLSID derives a stable, per-app CLSID from the AUMID (name-based
// UUIDv5 form) so the registry registration, the shortcut property and the
// runtime CoRegisterClassObject all agree without persisting any state.
func toastActivatorCLSID(aumid string) windows.GUID {
	h := sha1.Sum([]byte("bellui-toast-activator:" + aumid))
	var g windows.GUID
	g.Data1 = binary.BigEndian.Uint32(h[0:4])
	g.Data2 = binary.BigEndian.Uint16(h[4:6])
	g.Data3 = binary.BigEndian.Uint16(h[6:8])&0x0FFF | 0x5000 // version 5
	copy(g.Data4[:], h[8:16])
	g.Data4[0] = g.Data4[0]&0x3F | 0x80 // RFC 4122 variant
	return g
}

func (a *belluiApp) toastAUMID() string {
	id := a.UniqueID()
	if id == "" || strings.HasPrefix(id, "missing-id") {
		id = a.Metadata().Name
	}
	return id
}

func (a *belluiApp) toastDisplayName() string {
	if n := a.Metadata().Name; n != "" {
		return n
	}
	return a.toastAUMID()
}

// registerToastActivatorCLSID writes the LocalServer32 entry so the shell can
// COM-launch the app (via os.Executable) to deliver an activation received while
// the app is closed. Activations while running are served by CoRegisterClassObject.
func registerToastActivatorCLSID(clsid windows.GUID, exe string) error {
	base := `Software\Classes\CLSID\` + clsid.String()

	k, _, err := registry.CreateKey(registry.CURRENT_USER, base, registry.WRITE)
	if err != nil {
		return err
	}
	_ = k.SetStringValue("", "Bellui Toast Activator")
	k.Close()

	ls, _, err := registry.CreateKey(registry.CURRENT_USER, base+`\LocalServer32`, registry.WRITE)
	if err != nil {
		return err
	}
	defer ls.Close()
	return ls.SetStringValue("", exe)
}

// ensureStartMenuShortcut creates the Start Menu .lnk (once) carrying the AUMID
// and ToastActivatorCLSID that the notification platform requires to show and
// activate toasts for an unpackaged desktop app.
func ensureStartMenuShortcut(aumid string, clsid windows.GUID, exe, name string) error {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return errors.New("APPDATA is not set")
	}
	dir := filepath.Join(appData, `Microsoft\Windows\Start Menu\Programs`)
	lnk := filepath.Join(dir, sanitizeFileName(name)+".lnk")
	if _, err := os.Stat(lnk); err == nil {
		return nil // already present
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return createShortcut(lnk, exe, aumid, clsid)
}

// createShortcut builds the IShellLink, stamps the AUMID + ToastActivatorCLSID
// via its IPropertyStore and saves it through IPersistFile.
func createShortcut(lnkPath, exe, aumid string, clsid windows.GUID) error {
	// CoInitializeEx/CoUninitialize and every COM call between them must run on
	// one OS thread, so pin this goroutine for the duration.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hr, _, _ := procCoInitializeExN.Call(0, coinitApartmentThreade)
	if int32(hr) >= 0 && uint32(hr) != rpcEChangedMd {
		defer procCoUninitializeN.Call()
	}

	var link unsafe.Pointer
	if hr, _, _ := procCoCreateInstanceN.Call(
		uintptr(unsafe.Pointer(&clsidShellLink)), 0, clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidIShellLinkW)), uintptr(unsafe.Pointer(&link)),
	); int32(hr) < 0 || link == nil {
		return fmt.Errorf("CoCreateInstance ShellLink: %w", syscall.Errno(hr))
	}
	defer release(link)

	exeW, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return err
	}
	if hr := vcall(link, slotSetPath, uintptr(unsafe.Pointer(exeW))); int32(hr) < 0 {
		return fmt.Errorf("SetPath: %w", syscall.Errno(hr))
	}
	runtime.KeepAlive(exeW)

	store, err := queryInterface(link, &iidIPropertyStore)
	if err != nil {
		return fmt.Errorf("QI IPropertyStore: %w", err)
	}
	defer release(store)

	if err := setStringProperty(store, &pkeyAppUserModelID, aumid); err != nil {
		return fmt.Errorf("SetValue AUMID: %w", err)
	}
	if err := setCLSIDProperty(store, &pkeyAppUserModelToastActivatorCLSID, clsid); err != nil {
		return fmt.Errorf("SetValue ToastActivatorCLSID: %w", err)
	}
	if hr := vcall(store, slotCommit); int32(hr) < 0 {
		return fmt.Errorf("Commit property store: %w", syscall.Errno(hr))
	}

	persist, err := queryInterface(link, &iidIPersistFile)
	if err != nil {
		return fmt.Errorf("QI IPersistFile: %w", err)
	}
	defer release(persist)

	lnkW, err := windows.UTF16PtrFromString(lnkPath)
	if err != nil {
		return err
	}
	if hr := vcall(persist, slotSave, uintptr(unsafe.Pointer(lnkW)), 1 /* fRemember */); int32(hr) < 0 {
		return fmt.Errorf("IPersistFile::Save: %w", syscall.Errno(hr))
	}
	runtime.KeepAlive(lnkW)
	return nil
}

// setStringProperty writes a VT_LPWSTR value. IPropertyStore::SetValue deep-copies
// the PROPVARIANT, so the value points at a Go buffer kept alive only across the
// call — no CoTaskMem allocation (and thus no PropVariantClear) is needed.
func setStringProperty(store unsafe.Pointer, key *propertyKey, val string) error {
	buf, err := windows.UTF16FromString(val)
	if err != nil {
		return err
	}
	pv := propVariant{vt: vtLPWSTR, data: uintptr(unsafe.Pointer(&buf[0]))}
	hr := vcall(store, slotSetValue, uintptr(unsafe.Pointer(key)), uintptr(unsafe.Pointer(&pv)))
	runtime.KeepAlive(buf)
	runtime.KeepAlive(&pv)
	if int32(hr) < 0 {
		return syscall.Errno(hr)
	}
	return nil
}

// setCLSIDProperty writes a VT_CLSID value over a stack copy of the GUID, kept
// alive across the (deep-copying) SetValue call.
func setCLSIDProperty(store unsafe.Pointer, key *propertyKey, clsid windows.GUID) error {
	g := clsid
	pv := propVariant{vt: vtCLSID, data: uintptr(unsafe.Pointer(&g))}
	hr := vcall(store, slotSetValue, uintptr(unsafe.Pointer(key)), uintptr(unsafe.Pointer(&pv)))
	runtime.KeepAlive(&g)
	runtime.KeepAlive(&pv)
	if int32(hr) < 0 {
		return syscall.Errno(hr)
	}
	return nil
}

func sanitizeFileName(s string) string {
	if s == "" {
		return "Bellui App"
	}
	return strings.NewReplacer(
		`\`, "-", "/", "-", ":", "-", "*", "-", "?", "-",
		`"`, "-", "<", "-", ">", "-", "|", "-",
	).Replace(s)
}

// ---- toast XML + activation routing ----------------------------------------

const toastArgPrefix = "bellui"

func toastBodyArgs(token uint64) string { return fmt.Sprintf("%s;body;%d", toastArgPrefix, token) }
func toastActionArgs(token uint64, idx int) string {
	return fmt.Sprintf("%s;action;%d;%d", toastArgPrefix, idx, token)
}

// toastActivation is the decoded form of an Activate argument string.
type toastActivation struct {
	body   bool   // body click vs. an action button
	token  uint64 // routing token into the notify registry
	action int    // action index (only meaningful when body is false)
}

// parseToastArgs decodes a toast launch/arguments string. ok is false for
// anything that is not one of our bellui-encoded strings.
func parseToastArgs(args string) (toastActivation, bool) {
	parts := strings.Split(args, ";")
	if len(parts) < 3 || parts[0] != toastArgPrefix {
		return toastActivation{}, false
	}
	switch parts[1] {
	case "body":
		tok, err := strconv.ParseUint(parts[2], 10, 64)
		if err != nil {
			return toastActivation{}, false
		}
		return toastActivation{body: true, token: tok}, true
	case "action":
		if len(parts) < 4 {
			return toastActivation{}, false
		}
		idx, e1 := strconv.Atoi(parts[2])
		tok, e2 := strconv.ParseUint(parts[3], 10, 64)
		if e1 != nil || e2 != nil {
			return toastActivation{}, false
		}
		return toastActivation{token: tok, action: idx}, true
	}
	return toastActivation{}, false
}

// routeToastActivation parses an Activate argument string and fires the matching
// callback via the shared registry (which hops to the main goroutine).
func routeToastActivation(args string) {
	a := toastApp
	if a == nil {
		return
	}
	act, ok := parseToastArgs(args)
	if !ok {
		return
	}
	if act.body {
		a.fireNotifyTapped(act.token)
	} else {
		a.fireNotifyAction(act.token, act.action)
	}
}

// toastIconURL resolves the file:// URL for the toast image, writing a temp file
// for a per-notification icon (returned so it can later be removed) and else
// falling back to the app's cached icon.
func (a *belluiApp) toastIconURL(opts *bellui.NotificationOptions) (iconURL, tempPath string) {
	path := a.cachedIconPath()
	if opts.Icon != nil {
		if p, err := writeTempNotificationIcon(opts.Icon, "bellui-notify-icon-*.png"); err == nil {
			path, tempPath = p, p
		} else {
			bellui.LogError("Unable to write notification icon", err)
		}
	}
	if path == "" {
		return "", tempPath
	}
	return "file:///" + filepath.ToSlash(path), tempPath
}

// buildToastXML renders the ToastGeneric XML: the launch string carries the body
// token, each action's arguments carry the action index + token, and the icon
// (if any) is the appLogoOverride.
func buildToastXML(opts *bellui.NotificationOptions, token uint64, iconURL string) string {
	var b strings.Builder
	b.WriteString(`<toast activationType="foreground" launch="`)
	b.WriteString(xmlEscape(toastBodyArgs(token)))
	b.WriteString(`"><visual><binding template="ToastGeneric">`)
	if iconURL != "" {
		b.WriteString(`<image placement="appLogoOverride" src="`)
		b.WriteString(xmlEscape(iconURL))
		b.WriteString(`"/>`)
	}
	b.WriteString(`<text>`)
	b.WriteString(xmlEscape(opts.Title))
	b.WriteString(`</text><text>`)
	b.WriteString(xmlEscape(opts.Content))
	b.WriteString(`</text></binding></visual>`)
	if len(opts.Actions) > 0 {
		b.WriteString(`<actions>`)
		for i, act := range opts.Actions {
			b.WriteString(`<action activationType="foreground" content="`)
			b.WriteString(xmlEscape(act.Label))
			b.WriteString(`" arguments="`)
			b.WriteString(xmlEscape(toastActionArgs(token, i)))
			b.WriteString(`"/>`)
		}
		b.WriteString(`</actions>`)
	}
	b.WriteString(`</toast>`)
	return b.String()
}

var xmlEscaper = strings.NewReplacer(
	"&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;",
)

func xmlEscape(s string) string { return xmlEscaper.Replace(s) }
