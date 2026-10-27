//go:build ios

package audio

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework AVFoundation -framework Foundation
#include <stdlib.h>

char *mmediaSetPlaybackSession(void);
*/
import "C"

import (
	"errors"
	"unsafe"

	"github.com/belluikit/bellui"
)

// setupSession sets the iOS audio session to Playback before the device
// opens. iOS otherwise uses SoloAmbient, which goes quiet when the app is in
// the background or the silent switch is on (#845). Playback is what a media
// app wants: it keeps playing in the background, as long as the app declares
// UIBackgroundModes audio in its ios/Info.plist, and through silent mode.
//
// A failure is logged and does not stop the device from opening: audio still
// plays in the foreground under the default category.
func setupSession() {
	if e := C.mmediaSetPlaybackSession(); e != nil {
		bellui.LogError("x/mmedia: setting the audio session to Playback", errors.New(C.GoString(e)))
		C.free(unsafe.Pointer(e))
	}
}
