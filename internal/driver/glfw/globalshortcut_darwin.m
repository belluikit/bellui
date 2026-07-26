//go:build darwin

#import <Carbon/Carbon.h>

// Implemented in Go (exported).
extern void belluiGlobalHotKeyFired(unsigned int id);

static EventHandlerRef gHotKeyHandlerRef = NULL;

// belluiHotKeyHandler receives every hotkey-pressed event and forwards the
// registered id back to Go.
static OSStatus belluiHotKeyHandler(EventHandlerCallRef nextHandler, EventRef theEvent,
                                  void *userData) {
    EventHotKeyID hkID;
    OSStatus err = GetEventParameter(theEvent, kEventParamDirectObject, typeEventHotKeyID,
                                     NULL, sizeof(hkID), NULL, &hkID);
    if (err == noErr) {
        belluiGlobalHotKeyFired(hkID.id);
    }
    return noErr;
}

// ensureHotKeyHandler installs the application-wide hotkey event handler once.
static void ensureHotKeyHandler(void) {
    if (gHotKeyHandlerRef != NULL) {
        return;
    }
    EventTypeSpec spec;
    spec.eventClass = kEventClassKeyboard;
    spec.eventKind = kEventHotKeyPressed;
    InstallApplicationEventHandler(NewEventHandlerUPP(belluiHotKeyHandler), 1, &spec, NULL,
                                   &gHotKeyHandlerRef);
}

// registerHotKey registers a system-wide hotkey and returns its
// EventHotKeyRef as an integer handle (0 on failure).
unsigned long registerHotKey(unsigned int id, unsigned int keyCode, unsigned int modifiers) {
    ensureHotKeyHandler();

    EventHotKeyID hkID;
    hkID.signature = 'blui';
    hkID.id = id;

    EventHotKeyRef ref = NULL;
    OSStatus status = RegisterEventHotKey(keyCode, modifiers, hkID,
                                          GetApplicationEventTarget(), 0, &ref);
    if (status != noErr || ref == NULL) {
        return 0;
    }
    return (unsigned long)ref;
}

// unregisterHotKey releases a hotkey previously returned by registerHotKey.
void unregisterHotKey(unsigned long ref) {
    if (ref != 0) {
        UnregisterEventHotKey((EventHotKeyRef)ref);
    }
}
