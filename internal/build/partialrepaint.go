package build

import "os"

// PartialRepaint is whether the drivers repaint only the dirty region: each
// frame clears and redraws only what refreshed or moved objects cover in the
// persistent canvas texture, and keeps the rest. It is on by default on
// desktop, where it is validated pixel for pixel against a full repaint
// (BELLUI_PARTIAL_VERIFY, #496), and on Android and iOS with the default
// WebGPU painter, validated the same way on devices (#593). It is off on the
// web (#648) and with the GL painter on mobile (-tags gl) until they are
// validated too. The partialrepaint and nopartialrepaint build tags turn it on or
// off anywhere; BELLUI_PARTIAL_REPAINT=0 turns it off at run time, without a
// rebuild. It is read once at start-up.
var PartialRepaint = partialRepaintDefault && partialRepaintAllowed(os.Getenv("BELLUI_PARTIAL_REPAINT"))

// partialRepaintAllowed reports whether the BELLUI_PARTIAL_REPAINT value
// leaves partial repaint on. Only an explicit "off" turns it off; the variable
// can't turn it on where the build has it off.
func partialRepaintAllowed(v string) bool {
	switch v {
	case "0", "false", "off", "no":
		return false
	}
	return true
}
