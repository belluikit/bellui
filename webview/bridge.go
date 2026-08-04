package webview

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/belluikit/bellui"
)

// Errors reported to a bridge or eval callback instead of leaving it pending.
// A callback this package accepts is always called exactly once, so a caller
// never has to distinguish "failed" from "never answered".
//
// Since: 1.0
var (
	// ErrNotAttached is reported when the view has no native web engine —
	// either it has not been attached to a window yet, or it has been
	// destroyed, or the platform has no web engine binding at all
	// ([Supported] is false).
	ErrNotAttached = errors.New("webview: not attached to a window")

	// ErrBridgeDisabled is reported when a bridge operation is attempted on a
	// view whose bridge was never enabled with [WebView.EnableBridge].
	ErrBridgeDisabled = errors.New("webview: bridge is not enabled")
)

// OriginOpaque is the origin reported for page content that has no origin of
// its own — a data: URL, about:blank, or a sandboxed frame. It matches the
// HTML specification's serialization of an opaque origin.
//
// Content served from a file: URL reports "file://" instead, and ordinary web
// content reports scheme, host and non-default port, e.g. "https://example.com"
// or "http://localhost:8080".
//
// Since: 1.0
const OriginOpaque = "null"

// OriginPolicy reports whether page content served from origin may use the
// bridge. It is consulted for every inbound message, including messages from
// sub-frames, so a page that navigates or embeds a third-party frame cannot
// widen its own access.
//
// Since: 1.0
type OriginPolicy func(origin string) bool

// TrustAllOrigins allows every page loaded in the view to use the bridge.
//
// Use it only for content the app itself controls end to end. A view that can
// reach the open web — because it loads a remote URL, follows a link, or hosts
// a third-party frame — must not be given this policy: it would hand any page
// that turns up the same access to the app's [WebView.Handle] functions as the
// app's own content. Constrain what may be loaded with [WebView.OnNavigate] as
// well if the answer is not obviously "nothing else can be".
//
// Since: 1.0
func TrustAllOrigins() OriginPolicy {
	return func(string) bool { return true }
}

// TrustOrigins allows only the listed origins, compared exactly against the
// serialization described on [OriginOpaque]. Pass [OriginOpaque] to trust
// content loaded from a data: URL or about:blank, and "file://" to trust
// content the app loaded from disk.
//
// Since: 1.0
func TrustOrigins(origins ...string) OriginPolicy {
	allowed := make(map[string]bool, len(origins))
	for _, o := range origins {
		allowed[o] = true
	}
	return func(origin string) bool { return allowed[origin] }
}

// Message is one inbound communication from the page: a notification posted
// with window.bellui.post, or a request made with window.bellui.call.
//
// Since: 1.0
type Message struct {
	// Name is the handler the page addressed, for a request made with
	// window.bellui.call. It is empty for a posted notification.
	Name string

	// Payload is the value the page sent, still encoded. Read it with
	// [Message.Decode]. It is the JSON null literal when the page sent
	// nothing, never nil.
	Payload json.RawMessage

	// Origin is the serialized security origin of the frame that sent the
	// message — see [OriginOpaque]. It has already been accepted by the
	// view's [OriginPolicy]; it is reported so a handler shared between
	// several trusted origins can still tell them apart.
	Origin string
}

// Decode unmarshals the message payload into v, which must be a pointer.
//
// Since: 1.0
func (m Message) Decode(v any) error {
	if len(m.Payload) == 0 {
		return errors.New("webview: message has no payload")
	}
	return json.Unmarshal(m.Payload, v)
}

// Handler answers a request the page made with window.bellui.call. Its result
// is marshalled to JSON and resolves the page's promise; a non-nil error
// rejects that promise with the error's message, so failure is reported to the
// page rather than swallowed.
//
// It runs on the main goroutine, with the engine's own callback waiting behind
// it, so it must return promptly. Hand slow work to a goroutine and report it
// later with [WebView.Post].
//
// Since: 1.0
type Handler func(Message) (any, error)

// bridgeState is the portable half of the bridge: who may talk to us, what we
// answer, and which of our own requests to the page are still outstanding.
type bridgeState struct {
	enabled  bool
	policy   OriginPolicy
	handlers map[string]Handler
	pending  map[uint64]func(json.RawMessage, error)
	nextID   uint64
}

// EnableBridge installs the page-side bridge, admitting only content that
// policy accepts. It must be called before [WebView.Attach]: the bridge is
// part of the web view's configuration, and a view attached without it has no
// window.bellui object and no message channel behind it — a page cannot reach
// bellui at all, rather than reaching it and being ignored. Calling it on an
// attached view is a logged no-op.
//
// The bridge is off by default, and deliberately so. This package can show
// content the app did not write, and a message channel installed for a local
// trusted page is the same channel a remote page inherits when the user
// follows a link — the same stance [widget.RichText] takes with
// AllowRemoteContent, where parsed content is not trusted merely because it
// was rendered. A nil policy trusts nothing, which leaves the bridge present
// in the page but silent; use [TrustAllOrigins] or [TrustOrigins] to say what
// is trusted.
//
// Since: 1.0
func (w *WebView) EnableBridge(policy OriginPolicy) {
	if w.attachedRaw() {
		bellui.LogError("webview: EnableBridge must be called before Attach", nil)
		return
	}
	w.bridge.enabled = true
	w.bridge.policy = policy
}

// BridgeEnabled reports whether [WebView.EnableBridge] has been called.
//
// Since: 1.0
func (w *WebView) BridgeEnabled() bool {
	return w.bridge.enabled
}

// Handle registers fn as the answer to window.bellui.call(name, arg) in the
// page. Registering a nil function removes the handler. Handlers can be
// registered at any time; a request that arrives for a name with no handler
// rejects the page's promise with an error rather than hanging it.
//
// Since: 1.0
func (w *WebView) Handle(name string, fn Handler) {
	if fn == nil {
		delete(w.bridge.handlers, name)
		return
	}
	if w.bridge.handlers == nil {
		w.bridge.handlers = map[string]Handler{}
	}
	w.bridge.handlers[name] = fn
}

// Post sends a one-way notification to the page, delivered to whatever the
// page assigned to window.bellui.onMessage. payload is marshalled to JSON, so
// it crosses as data and can never become executable source in the page.
//
// It is the counterpart of [WebView.OnMessage]. Use [WebView.Call] when an
// answer is wanted.
//
// Since: 1.0
func (w *WebView) Post(payload any) {
	if !w.bridge.enabled || !w.attached() {
		return
	}
	data, err := jsonOrNull(payload)
	if err != nil {
		bellui.LogError("webview: cannot encode posted payload", err)
		return
	}
	w.dispatchToPage(envelope{V: bridgeVersion, Kind: kindPost, Payload: data}, nil)
}

// Call invokes the page function registered with window.bellui.handle(name, fn)
// and reports its result to onResult, on the main goroutine. arg is marshalled
// to JSON, so values from outside the app cross as data and cannot become
// executable source in the page — prefer this to building a script with
// [WebView.Eval].
//
// onResult may be nil to ignore the answer. When it is not, it is called
// exactly once: with the page's resolved value, or with an error if the page
// rejected, has no handler by that name, or the view is not attached
// ([ErrNotAttached]) or has no bridge ([ErrBridgeDisabled]).
//
// A page handler that never settles its promise leaves the callback pending
// until the view is destroyed, which fails it — the correlation is honoured,
// but the page's own liveness is the page's responsibility.
//
// Since: 1.0
func (w *WebView) Call(name string, arg any, onResult func(json.RawMessage, error)) {
	if !w.bridge.enabled {
		reportResult(onResult, ErrBridgeDisabled)
		return
	}
	if !w.attached() {
		reportResult(onResult, ErrNotAttached)
		return
	}
	data, err := jsonOrNull(arg)
	if err != nil {
		reportResult(onResult, err)
		return
	}

	w.bridge.nextID++
	id := w.bridge.nextID
	if onResult != nil {
		if w.bridge.pending == nil {
			w.bridge.pending = map[uint64]func(json.RawMessage, error){}
		}
		w.bridge.pending[id] = onResult
	}

	// Sent as an eval with a result so a page that has no bridge shim yet —
	// it navigated away, or the load failed — fails the call at once instead
	// of leaving it waiting for a reply that cannot come.
	w.dispatchToPage(envelope{V: bridgeVersion, Kind: kindCall, ID: id, Name: name, Payload: data},
		func(evalErr error) { w.settleCall(id, nil, evalErr) })
}

// ---- inbound ----

// deliverMessage is the backend entry point for a script message from the
// page. body is the JSON envelope the page-side shim posted.
func (w *WebView) deliverMessage(origin, body string) {
	if !w.bridge.enabled {
		return // no handler should have been installed at all
	}
	if w.bridge.policy == nil || !w.bridge.policy(origin) {
		bellui.LogError("webview: bridge message rejected from untrusted origin "+origin, nil)
		return
	}

	var env envelope
	if err := json.Unmarshal([]byte(body), &env); err != nil {
		bellui.LogError("webview: malformed bridge message", err)
		return
	}
	switch env.Kind {
	case kindPost:
		if w.OnMessage != nil {
			w.OnMessage(Message{Payload: payloadOrNull(env.Payload), Origin: origin})
		}
	case kindCall:
		w.answerPageCall(env, origin)
	case kindReply:
		w.settleCall(env.ID, env.Payload, replyError(env.Error))
	}
}

// answerPageCall runs the registered handler for a page request and replies
// with its result, or with an error — including "there is no such handler",
// which the page sees as a rejected promise rather than one that never
// settles.
func (w *WebView) answerPageCall(env envelope, origin string) {
	h := w.bridge.handlers[env.Name]
	if h == nil {
		w.replyToPage(env.ID, nil, fmt.Errorf("webview: no bellui handler named %q", env.Name))
		return
	}
	res, err := h(Message{Name: env.Name, Payload: payloadOrNull(env.Payload), Origin: origin})
	w.replyToPage(env.ID, res, err)
}

func (w *WebView) replyToPage(id uint64, res any, err error) {
	reply := envelope{V: bridgeVersion, Kind: kindReply, ID: id}
	if err != nil {
		reply.Error = err.Error()
	} else {
		data, encErr := jsonOrNull(res)
		if encErr != nil {
			reply.Error = "webview: handler result cannot be encoded: " + encErr.Error()
		} else {
			reply.Payload = data
		}
	}
	w.dispatchToPage(reply, nil)
}

// settleCall completes a Go-initiated call exactly once.
func (w *WebView) settleCall(id uint64, payload json.RawMessage, err error) {
	cb := w.bridge.pending[id]
	if cb == nil {
		return
	}
	delete(w.bridge.pending, id)
	cb(payloadOrNull(payload), err)
}

// dispatchToPage hands an envelope to the page-side shim. onEvalErr, when
// set, reports a failure to run the dispatch at all.
func (w *WebView) dispatchToPage(env envelope, onEvalErr func(error)) {
	data, err := json.Marshal(env)
	if err != nil { // envelope fields are all encodable; belt and braces
		bellui.LogError("webview: cannot encode bridge envelope", err)
		return
	}
	// encoding/json escapes <, > and the U+2028/U+2029 line separators, so the
	// literal is safe to splice into JavaScript source.
	js := "window.__belluiDispatch(" + string(data) + ")"
	if onEvalErr == nil {
		w.Eval(js)
		return
	}
	w.EvalResult(js, func(_ json.RawMessage, err error) {
		if err != nil {
			onEvalErr(err)
		}
	})
}

// failPending fails every outstanding call, so a destroyed view never leaves a
// callback waiting forever.
func (w *WebView) failPending(err error) {
	pending := w.bridge.pending
	w.bridge.pending = nil
	for _, cb := range pending {
		cb(json.RawMessage("null"), err)
	}
	w.failPendingEvals(err)
}

// ---- wire format ----

// The envelope is the one wire format the bridge speaks, in both directions
// and on every platform. Standardising it here is the point: an app that asks
// the page a question should not have to invent an id scheme, and a backend
// should not get to invent a different one.
const bridgeVersion = 1

const (
	kindPost  = "post"  // one-way notification, no reply
	kindCall  = "call"  // request; the receiver must send a reply with this ID
	kindReply = "reply" // response to a call, carrying Payload or Error
)

type envelope struct {
	V       int             `json:"v"`
	Kind    string          `json:"kind"`
	ID      uint64          `json:"id,omitempty"`
	Name    string          `json:"name,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Error   string          `json:"error,omitempty"`
}

func payloadOrNull(p json.RawMessage) json.RawMessage {
	if len(p) == 0 {
		return json.RawMessage("null")
	}
	return p
}

func replyError(msg string) error {
	if msg == "" {
		return nil
	}
	return errors.New(msg)
}

func reportResult(cb func(json.RawMessage, error), err error) {
	if cb != nil {
		cb(json.RawMessage("null"), err)
	}
}

// ---- the page side ----

// bridgeScript returns the JavaScript a backend injects at document start,
// or "" when the bridge is disabled — in which case a backend must install no
// message channel at all.
func (w *WebView) bridgeScript(post string) string {
	if !w.bridge.enabled {
		return ""
	}
	return strings.Replace(bridgeShim, "/*POST*/", post, 1)
}

// bridgeShim is the page-side half of the bridge, shared by every backend so
// that window.bellui means the same thing everywhere. /*POST*/ is replaced
// with the platform's own way of handing a string to the host — the only part
// that differs between WKScriptMessageHandler, @JavascriptInterface,
// postMessage and WebMessageReceived.
//
// It is injected at document start on every navigation, so the page can use
// window.bellui from its first line, and re-establishes itself after a
// navigation without the app doing anything.
const bridgeShim = `(function () {
  if (window.__belluiDispatch) { return; }
  function __post(s) { /*POST*/ }
  var __next = 1, __pending = {}, __handlers = {};
  function __send(env) { __post(JSON.stringify(env)); }
  function __err(e) { return String(e && e.message ? e.message : e); }
  function __value(v) { return v === undefined ? null : v; }

  window.bellui = {
    post: function (payload) {
      __send({v: 1, kind: "post", payload: __value(payload)});
    },
    call: function (name, arg) {
      var id = __next++;
      return new Promise(function (resolve, reject) {
        __pending[id] = {resolve: resolve, reject: reject};
        __send({v: 1, kind: "call", id: id, name: String(name), payload: __value(arg)});
      });
    },
    handle: function (name, fn) {
      if (fn) { __handlers[String(name)] = fn; } else { delete __handlers[String(name)]; }
    },
    onMessage: null
  };

  window.__belluiDispatch = function (env) {
    if (!env) { return; }
    if (env.kind === "reply") {
      var p = __pending[env.id];
      if (!p) { return; }
      delete __pending[env.id];
      if (env.error) { p.reject(new Error(env.error)); } else { p.resolve(__value(env.payload)); }
      return;
    }
    if (env.kind === "post") {
      if (typeof window.bellui.onMessage === "function") {
        window.bellui.onMessage(__value(env.payload));
      }
      return;
    }
    if (env.kind === "call") {
      var fn = __handlers[env.name];
      if (!fn) {
        __send({v: 1, kind: "reply", id: env.id, error: "webview: no page handler named " + env.name});
        return;
      }
      try {
        Promise.resolve(fn(__value(env.payload))).then(function (v) {
          __send({v: 1, kind: "reply", id: env.id, payload: __value(v)});
        }, function (e) {
          __send({v: 1, kind: "reply", id: env.id, error: __err(e)});
        });
      } catch (e) {
        __send({v: 1, kind: "reply", id: env.id, error: __err(e)});
      }
    }
  };
})();`
