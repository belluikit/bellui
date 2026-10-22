package app

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode/utf8"

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
// shared by every transport. Each argument is escaped with escapeArg, so a file
// name that is not valid UTF-8 (a Latin-1 name on Linux, say) reaches the
// running copy byte for byte: PXF refuses to read invalid UTF-8 back, and the
// JSON message before #523 replaced each bad byte with U+FFFD, naming a file
// that does not exist (#661).
func writeArgs(w io.Writer, args []string) error {
	msg := &instancepb.ForwardedArgs{Args: make([]string, len(args))}
	for i, a := range args {
		msg.Args[i] = escapeArg(a)
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
	args := msg.GetArgs()
	for i, a := range args {
		args[i] = unescapeArg(a)
	}
	return args, nil
}

// argEscapeBase maps the bytes 0x80..0xFF to the private-use code points
// U+EF80..U+EFFF. Part of the forwarded-arguments wire format: a running copy
// and the launch forwarding to it must agree on it, so it never changes.
const argEscapeBase = 0xEF00

func isArgEscape(r rune) bool { return r >= argEscapeBase+0x80 && r <= argEscapeBase+0xFF }

// escapeArg returns s as valid UTF-8 that unescapeArg turns back into s's exact
// bytes. Valid UTF-8 is kept as it is, except a code point in U+EF80..U+EFFF;
// each byte of that, and each byte that is not part of valid UTF-8, becomes
// its escape. It works without knowing the bytes' encoding, which Linux does
// not record for a file name.
func escapeArg(s string) string {
	if utf8.ValidString(s) && !strings.ContainsFunc(s, isArgEscape) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRuneInString(s[i:])
		if (r == utf8.RuneError && n == 1) || isArgEscape(r) {
			for j := i; j < i+n; j++ {
				b.WriteRune(argEscapeBase + rune(s[j]))
			}
		} else {
			b.WriteString(s[i : i+n])
		}
		i += n
	}
	return b.String()
}

// unescapeArg reverses escapeArg.
func unescapeArg(s string) string {
	if !strings.ContainsFunc(s, isArgEscape) {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		if isArgEscape(r) {
			b.WriteByte(byte(r - argEscapeBase))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
