//go:build android

package auth

/*
#include <stdbool.h>
#include <stdint.h>
#include <stdlib.h>

bool belluiAuthInit(uintptr_t jniEnv, uintptr_t ctx);
void belluiAuthGoogleSignIn(uintptr_t jniEnv, uint64_t token, const char *serverClientID, const char *nonce);
*/
import "C"

import (
	"errors"
	"fmt"
	"time"
	"unsafe"

	"github.com/belluikit/bellui/driver"
)

func init() {
	nativeSheets["google"] = nativeSheet{run: googleSheet}
	newRedirectFlow = func(cfg OIDCConfig) (redirectFlow, error) {
		return nil, fmt.Errorf("provider %q has no sign-in flow on Android: only \"google\" does", cfg.Provider)
	}
	loopbackFlows = false
}

// googleLibraries names what an app ships in android/libs to sign in with
// Google, for the error an app without them gets.
const googleLibraries = "androidx.credentials:credentials, androidx.credentials:credentials-play-services-auth " +
	"and com.google.android.libraries.identity.googleid:googleid, with their dependencies"

//export belluiAuthGoogleDone
func belluiAuthGoogleDone(token C.uint64_t, status C.int, idToken, email, name, msg *C.char) {
	complete(uint64(token), nativeDone{
		status:  int(status),
		idToken: C.GoString(idToken),
		email:   C.GoString(email),
		name:    C.GoString(name),
		msg:     C.GoString(msg),
	})
}

// googleSheet signs in through Credential Manager. clientID is the server's
// (Web) client ID, which Credential Manager puts in the token's audience.
func googleSheet(clientID, nonce string, timeout time.Duration) (*nativeCredential, error) {
	token, ch := register()
	cID, cNonce := C.CString(clientID), C.CString(nonce)
	defer C.free(unsafe.Pointer(cID))
	defer C.free(unsafe.Pointer(cNonce))

	started := false
	err := driver.RunNative(func(ctx any) error {
		android, ok := ctx.(*driver.AndroidContext)
		if !ok {
			return errors.New("no Android context")
		}
		if !bool(C.belluiAuthInit(C.uintptr_t(android.Env), C.uintptr_t(android.Ctx))) {
			return nil
		}
		started = true
		C.belluiAuthGoogleSignIn(C.uintptr_t(android.Env), C.uint64_t(token), cID, cNonce)
		return nil
	})
	if err == nil && !started {
		err = errors.New("this app's embedded Android support predates native sign-in; rebuild it with a newer bellui")
	}
	if err != nil {
		forget(token)
		return nil, err
	}

	select {
	case d := <-ch:
		switch d.status {
		case nativeOK:
			return &nativeCredential{IDToken: d.idToken, Email: d.email, Name: d.name}, nil
		case nativeCancelled:
			return nil, ErrCancelled
		case nativeMissing:
			return nil, fmt.Errorf("sign in with Google needs %s in android/libs (missing %s)", googleLibraries, d.msg)
		default:
			return nil, fmt.Errorf("sign in with Google failed: %s", d.msg)
		}
	case <-time.After(timeout):
		forget(token)
		return nil, fmt.Errorf("authentication timed out after %v", timeout)
	}
}
