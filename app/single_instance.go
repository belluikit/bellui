package app

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"

	"github.com/trendvidia/protowire-go/encoding/pxf"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal/app/instancepb"
	"github.com/belluikit/bellui/internal/pxfmigrate"
)

// siMaxPayload caps a single forwarded-argument message (defensive against a
// hostile local peer streaming an unbounded length prefix).
const siMaxPayload = 1 << 20 // 1 MiB

// Declare conformity with the optional SingleInstanceApp interface.
var _ bellui.SingleInstanceApp = (*belluiApp)(nil)

// errNoUniqueID is returned when single-instance mode is requested without an app ID.
var errNoUniqueID = errors.New("single instance mode requires a unique ID, use app.NewWithID() or the manifest.pxf ID field")

// SetSingleInstance implements bellui.SingleInstanceApp.
//
// The platform-specific transport lives in singleInstanceBind (see the
// single_instance_{unix,windows,other}.go files). Forwarded arguments are routed
// through onForwardedArgs, which both feeds the open-with dispatch (see
// openevent.go) and invokes this callback, so an app that registers file/URL
// open handlers receives already-running "open with" events for free once
// single-instance mode is on.
func (a *belluiApp) SetSingleInstance(onOpen func(args []string)) (bool, error) {
	a.openMu.Lock()
	a.siOnOpen = onOpen
	a.openMu.Unlock()

	return a.ensureSingleInstance()
}

// ensureSingleInstance binds the transport once, keyed on the app's UniqueID.
// SetSingleInstance and any internal caller reach single-instance mode through
// it; repeated calls report the same role without rebinding.
func (a *belluiApp) ensureSingleInstance() (bool, error) {
	if a.siEnabled {
		return a.siPrimary, nil
	}

	if a.uniqueID == "" && a.Metadata().ID == "" {
		return false, errNoUniqueID
	}

	primary, closer, err := singleInstanceBind(a.UniqueID(), a.onForwardedArgs)
	if err != nil {
		return false, err
	}

	a.siEnabled = true
	a.siPrimary = primary
	a.siCloser = closer
	return primary, nil
}

// onForwardedArgs is the transport's delivery callback, invoked on a background
// accept goroutine when another launch forwards its arguments. It fans out to
// the open-with dispatch and to the app's own single-instance callback; both
// paths hop to the main goroutine.
func (a *belluiApp) onForwardedArgs(args []string) {
	a.dispatchOpenArgs(args) // classifies + queues onto the main goroutine

	a.openMu.Lock()
	cb := a.siOnOpen
	a.openMu.Unlock()
	if cb != nil {
		bellui.Do(func() { cb(args) })
	}
}

// writeArgs sends a length-prefixed message of arguments to a forwarding peer.
// The wire format (4-byte big-endian length + PXF instancepb.ForwardedArgs) is
// shared by every transport. An argument that is not valid UTF-8, such as a
// file name in another encoding, is sent as JSON sent it, with U+FFFD for each
// bad byte: PXF would send it as it is, and the running copy would then refuse
// the whole message.
func writeArgs(w io.Writer, args []string) error {
	msg := &instancepb.ForwardedArgs{Args: make([]string, len(args))}
	for i, a := range args {
		msg.Args[i] = pxfmigrate.ValidUTF8(a)
	}
	payload, err := pxf.Marshal(msg)
	if err != nil {
		return err
	}
	if len(payload) > siMaxPayload {
		return errors.New("single instance argument payload too large")
	}
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(payload)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err = w.Write(payload)
	return err
}

// readArgs decodes a message written by writeArgs, or by a version before #523,
// which sent a JSON array: the running copy and the launch forwarding to it can
// be different versions of the app during an upgrade.
func readArgs(r io.Reader) ([]string, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(hdr[:])
	if n > siMaxPayload {
		return nil, errors.New("single instance argument payload too large")
	}
	payload := make([]byte, n)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}
	if pxfmigrate.IsJSON(payload, '[') {
		var args []string
		if err := json.Unmarshal(payload, &args); err != nil {
			return nil, err
		}
		return args, nil
	}
	var msg instancepb.ForwardedArgs
	if err := pxf.Unmarshal(payload, &msg); err != nil {
		return nil, err
	}
	return msg.GetArgs(), nil
}
