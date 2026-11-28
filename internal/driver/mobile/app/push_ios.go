//go:build darwin && ios

package app

/*
#include <stdlib.h>

void belluiRemotePushRegister(void);
void belluiRemotePushPresent(long long id, int show);
void belluiRequestNotificationAuth(long long id);
*/
import "C"

import (
	"sync"
	"unsafe"
)

// Remote push on iOS (#1517). The app delegate (darwin_ios.m) is the
// notification center's delegate from launch to exit and reports everything
// here: the device token or its failure, a tapped notification, a push that
// arrives in the foreground, and a tap on one of the app's own rich local
// notifications. The bellui app package sets the handlers before the app
// runs; a tap that arrives earlier is held for them.

// PushHandlers are the callbacks the bellui app package installs. They run on
// the UIKit main thread or a notification-center queue, so each must hand its
// work to the main goroutine itself and return without blocking.
type PushHandlers struct {
	// Token gets the device token as lowercase hex.
	Token func(hex string)
	// RegisterError gets the reason registration failed.
	RegisterError func(msg string)
	// Opened gets the JSON payload of a tapped remote notification.
	Opened func(payload []byte)
	// Received gets the JSON payload of a remote notification that arrived
	// in the foreground, and must call reply once with whether to show it.
	Received func(payload []byte, reply func(show bool))
	// LocalResponse gets a tap on a rich local notification: its token and
	// -1 for the body, the action's index for a button, -2 for a dismiss.
	LocalResponse func(token uint64, action int)
}

var push struct {
	sync.Mutex
	h          *PushHandlers
	held       [][]byte // taps before the handlers were set
	nextID     int64
	permission map[int64]func(granted bool, errMsg string)
}

// SetPushHandlers installs h and passes it the taps held until now.
func SetPushHandlers(h PushHandlers) {
	push.Lock()
	push.h = &h
	held := push.held
	push.held = nil
	push.Unlock()
	if h.Opened != nil {
		for _, p := range held {
			h.Opened(p)
		}
	}
}

func pushHandlers() *PushHandlers {
	push.Lock()
	defer push.Unlock()
	return push.h
}

// RegisterForPush asks iOS for a device token; the answer comes to the Token
// or RegisterError handler.
func RegisterForPush() {
	C.belluiRemotePushRegister()
}

// RequestNotificationPermission asks for alert and sound authorization and
// passes the answer to done, on a notification-center queue. errMsg is
// non-empty only when the question could not be asked.
func RequestNotificationPermission(done func(granted bool, errMsg string)) {
	push.Lock()
	push.nextID++
	id := push.nextID
	if push.permission == nil {
		push.permission = map[int64]func(bool, string){}
	}
	push.permission[id] = done
	push.Unlock()
	C.belluiRequestNotificationAuth(C.longlong(id))
}

//export onNotificationPermission
func onNotificationPermission(id C.longlong, granted C.int, errMsg *C.char) {
	push.Lock()
	done := push.permission[int64(id)]
	delete(push.permission, int64(id))
	push.Unlock()
	if done != nil {
		done(granted != 0, C.GoString(errMsg))
	}
}

//export onPushToken
func onPushToken(hex *C.char) {
	if h := pushHandlers(); h != nil && h.Token != nil {
		h.Token(C.GoString(hex))
	}
}

//export onPushRegisterError
func onPushRegisterError(msg *C.char) {
	if h := pushHandlers(); h != nil && h.RegisterError != nil {
		h.RegisterError(C.GoString(msg))
	}
}

//export onPushOpened
func onPushOpened(payload *C.char, n C.int) {
	p := C.GoBytes(unsafe.Pointer(payload), n)
	push.Lock()
	h := push.h
	if h == nil {
		push.held = append(push.held, p)
	}
	push.Unlock()
	if h != nil && h.Opened != nil {
		h.Opened(p)
	}
}

// onPushWillPresent asks whether to show a foreground push. With no handler
// it is shown, as every foreground notification was before #1517.
//
//export onPushWillPresent
func onPushWillPresent(id C.longlong, payload *C.char, n C.int) {
	reply := func(show bool) {
		s := C.int(0)
		if show {
			s = 1
		}
		C.belluiRemotePushPresent(id, s)
	}
	h := pushHandlers()
	if h == nil || h.Received == nil {
		reply(true)
		return
	}
	h.Received(C.GoBytes(unsafe.Pointer(payload), n), reply)
}

//export onLocalNotificationResponse
func onLocalNotificationResponse(token C.ulonglong, action C.int) {
	if h := pushHandlers(); h != nil && h.LocalResponse != nil {
		h.LocalResponse(uint64(token), int(action))
	}
}
