package webview

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeEngine stands in for a platform web engine. The bridge is portable, so
// installing it in place of the eval seam exercises every path on a target
// with no web engine at all — which is also how a backend author can check a
// new port against the same expectations.
type fakeEngine struct {
	scripts []string
	ids     []uint64
}

func (f *fakeEngine) last() string {
	if len(f.scripts) == 0 {
		return ""
	}
	return f.scripts[len(f.scripts)-1]
}

// lastEnvelope decodes the envelope of the most recent dispatch to the page.
func (f *fakeEngine) lastEnvelope(t *testing.T) envelope {
	t.Helper()
	js := f.last()
	require.True(t, strings.HasPrefix(js, "window.__belluiDispatch("), "not a dispatch: %s", js)
	body := strings.TrimSuffix(strings.TrimPrefix(js, "window.__belluiDispatch("), ")")

	var env envelope
	require.NoError(t, json.Unmarshal([]byte(body), &env), "envelope: %s", body)
	return env
}

// newTestView returns a view that looks attached and records what it is asked
// to run, with the bridge enabled for the given policy.
func newTestView(policy OriginPolicy) (*WebView, *fakeEngine) {
	f := &fakeEngine{}
	w := New("")
	w.evalHook = func(js string, callID uint64, _ bool) {
		f.scripts = append(f.scripts, js)
		f.ids = append(f.ids, callID)
	}
	if policy != nil {
		w.EnableBridge(policy)
	}
	return w, f
}

// pageSends feeds a raw envelope in as if the page had posted it.
func pageSends(w *WebView, origin string, env envelope) {
	body, err := json.Marshal(env)
	if err != nil {
		panic(err)
	}
	w.deliverMessage(origin, string(body))
}

func TestBridge_OffByDefault(t *testing.T) {
	w, f := newTestView(nil)

	assert.False(t, w.BridgeEnabled())
	// The strong guarantee: with the bridge off a backend is handed no script
	// at all, so the page has no channel to be rejected on.
	assert.Empty(t, w.bridgeScript("host.post(s)"))

	called := false
	w.OnMessage = func(Message) { called = true }
	pageSends(w, "https://example.com", envelope{V: 1, Kind: kindPost, Payload: json.RawMessage(`1`)})
	assert.False(t, called, "a disabled bridge must not deliver messages")

	var err error
	w.Call("anything", nil, func(_ json.RawMessage, e error) { err = e })
	assert.ErrorIs(t, err, ErrBridgeDisabled)
	assert.Empty(t, f.scripts, "a disabled bridge must not reach the page")
}

func TestBridge_ScriptCarriesPlatformTransport(t *testing.T) {
	w, _ := newTestView(TrustAllOrigins())

	script := w.bridgeScript("host.post(s);")
	assert.Contains(t, script, "host.post(s);", "the platform transport is spliced in")
	assert.Contains(t, script, "window.bellui")
	assert.Contains(t, script, "window.__belluiDispatch")
	assert.NotContains(t, script, "/*POST*/", "the placeholder must be replaced")
}

func TestBridge_OriginPolicyGatesEveryMessage(t *testing.T) {
	w, _ := newTestView(TrustOrigins("https://trusted.example", OriginOpaque))

	var seen []string
	w.OnMessage = func(m Message) { seen = append(seen, m.Origin) }

	post := envelope{V: 1, Kind: kindPost, Payload: json.RawMessage(`"hi"`)}
	pageSends(w, "https://trusted.example", post)
	pageSends(w, OriginOpaque, post)           // a data: URL the app itself loaded
	pageSends(w, "https://evil.example", post) // a frame or a page we navigated to
	pageSends(w, "file://", post)

	assert.Equal(t, []string{"https://trusted.example", OriginOpaque}, seen)
}

func TestBridge_NilPolicyTrustsNothing(t *testing.T) {
	w, _ := newTestView(nil)
	w.EnableBridge(nil)

	called := false
	w.OnMessage = func(Message) { called = true }
	pageSends(w, "https://example.com", envelope{V: 1, Kind: kindPost})
	assert.False(t, called)
}

func TestMessage_Decode(t *testing.T) {
	w, _ := newTestView(TrustAllOrigins())

	var got struct {
		Name string `json:"name"`
		N    int    `json:"n"`
	}
	w.OnMessage = func(m Message) { require.NoError(t, m.Decode(&got)) }
	pageSends(w, "https://example.com", envelope{
		V: 1, Kind: kindPost, Payload: json.RawMessage(`{"name":"ada","n":7}`),
	})

	assert.Equal(t, "ada", got.Name)
	assert.Equal(t, 7, got.N)
}

func TestMessage_PayloadIsNeverNil(t *testing.T) {
	w, _ := newTestView(TrustAllOrigins())

	var m Message
	w.OnMessage = func(got Message) { m = got }
	pageSends(w, "https://example.com", envelope{V: 1, Kind: kindPost})

	assert.Equal(t, json.RawMessage("null"), m.Payload)

	// A never-nil payload means Decode is always safe to call: JSON null
	// leaves the target alone rather than tripping over an empty document.
	n := 5
	assert.NoError(t, m.Decode(&n))
	assert.Equal(t, 5, n)
}

func TestBridge_PageCallIsAnswered(t *testing.T) {
	w, f := newTestView(TrustAllOrigins())
	w.Handle("add", func(m Message) (any, error) {
		var in struct{ A, B int }
		if err := m.Decode(&in); err != nil {
			return nil, err
		}
		return in.A + in.B, nil
	})

	pageSends(w, "https://example.com", envelope{
		V: 1, Kind: kindCall, ID: 9, Name: "add", Payload: json.RawMessage(`{"A":2,"B":3}`),
	})

	env := f.lastEnvelope(t)
	assert.Equal(t, kindReply, env.Kind)
	assert.Equal(t, uint64(9), env.ID, "the reply carries the page's own id")
	assert.Equal(t, json.RawMessage("5"), env.Payload)
	assert.Empty(t, env.Error)
}

func TestBridge_PageCallWithNoHandlerRejects(t *testing.T) {
	w, f := newTestView(TrustAllOrigins())

	pageSends(w, "https://example.com", envelope{V: 1, Kind: kindCall, ID: 4, Name: "missing"})

	env := f.lastEnvelope(t)
	assert.Equal(t, kindReply, env.Kind)
	assert.Equal(t, uint64(4), env.ID)
	assert.Contains(t, env.Error, `no bellui handler named "missing"`,
		"an unanswerable call rejects the page's promise rather than hanging it")
}

func TestBridge_HandlerErrorReachesThePage(t *testing.T) {
	w, f := newTestView(TrustAllOrigins())
	w.Handle("boom", func(Message) (any, error) { return nil, errors.New("not today") })

	pageSends(w, "https://example.com", envelope{V: 1, Kind: kindCall, ID: 1, Name: "boom"})

	env := f.lastEnvelope(t)
	assert.Equal(t, "not today", env.Error)
	assert.Empty(t, env.Payload)
}

func TestBridge_HandleNilRemovesTheHandler(t *testing.T) {
	w, f := newTestView(TrustAllOrigins())
	w.Handle("x", func(Message) (any, error) { return "ok", nil })
	w.Handle("x", nil)

	pageSends(w, "https://example.com", envelope{V: 1, Kind: kindCall, ID: 1, Name: "x"})
	assert.Contains(t, f.lastEnvelope(t).Error, "no bellui handler")
}

func TestBridge_CallCorrelatesConcurrentRequests(t *testing.T) {
	w, f := newTestView(TrustAllOrigins())

	results := map[string]string{}
	w.Call("first", map[string]int{"n": 1}, func(res json.RawMessage, err error) {
		require.NoError(t, err)
		results["first"] = string(res)
	})
	firstID := f.lastEnvelope(t).ID
	w.Call("second", nil, func(res json.RawMessage, err error) {
		require.NoError(t, err)
		results["second"] = string(res)
	})
	secondID := f.lastEnvelope(t).ID
	require.NotEqual(t, firstID, secondID, "each call gets its own id")

	// Reply out of order — correlation, not ordering, is what resolves them.
	pageSends(w, "https://example.com", envelope{V: 1, Kind: kindReply, ID: secondID, Payload: json.RawMessage(`"B"`)})
	pageSends(w, "https://example.com", envelope{V: 1, Kind: kindReply, ID: firstID, Payload: json.RawMessage(`"A"`)})

	assert.Equal(t, map[string]string{"first": `"A"`, "second": `"B"`}, results)
	assert.Empty(t, w.bridge.pending, "a settled call is forgotten")
}

func TestBridge_CallPassesArgumentsAsData(t *testing.T) {
	w, f := newTestView(TrustAllOrigins())

	// A value that would be catastrophic spliced into source.
	w.Call("show", map[string]string{"text": `"); alert(1); ("`}, nil)

	env := f.lastEnvelope(t)
	assert.Equal(t, kindCall, env.Kind)
	assert.Equal(t, "show", env.Name)

	var arg map[string]string
	require.NoError(t, json.Unmarshal(env.Payload, &arg))
	assert.Equal(t, `"); alert(1); ("`, arg["text"], "the value round-trips as data")

	// The payload sits inside a JSON string literal, so the quotes that would
	// have closed it and started a new statement are escaped: the text is
	// carried verbatim without ever being parsed as source.
	assert.NotContains(t, f.last(), `"); alert(1); ("`)
	assert.Contains(t, f.last(), `\"); alert(1); (\"`)
}

func TestBridge_PageRejectionReachesTheCaller(t *testing.T) {
	w, f := newTestView(TrustAllOrigins())

	var err error
	w.Call("ask", nil, func(_ json.RawMessage, e error) { err = e })
	id := f.lastEnvelope(t).ID

	pageSends(w, "https://example.com", envelope{V: 1, Kind: kindReply, ID: id, Error: "page said no"})
	assert.EqualError(t, err, "page said no")
}

func TestBridge_CallFailsWhenTheDispatchCannotRun(t *testing.T) {
	w, f := newTestView(TrustAllOrigins())

	var err error
	w.Call("ask", nil, func(_ json.RawMessage, e error) { err = e })
	require.Len(t, f.ids, 1)

	// The page has no shim — it navigated away, or the load failed. Without
	// this path the caller would wait forever for a reply that cannot come.
	w.deliverEvalResult(f.ids[0], "", "ReferenceError: __belluiDispatch is not defined")
	assert.ErrorContains(t, err, "__belluiDispatch is not defined")
}

func TestBridge_LateReplyIsIgnored(t *testing.T) {
	w, f := newTestView(TrustAllOrigins())

	calls := 0
	w.Call("ask", nil, func(json.RawMessage, error) { calls++ })
	id := f.lastEnvelope(t).ID

	reply := envelope{V: 1, Kind: kindReply, ID: id, Payload: json.RawMessage(`1`)}
	pageSends(w, "https://example.com", reply)
	pageSends(w, "https://example.com", reply)

	assert.Equal(t, 1, calls, "a callback is invoked exactly once")
}

func TestBridge_MalformedMessageIsIgnored(t *testing.T) {
	w, _ := newTestView(TrustAllOrigins())
	w.OnMessage = func(Message) { t.Fatal("malformed input must not be delivered") }

	w.deliverMessage("https://example.com", "not json at all")
	w.deliverMessage("https://example.com", `{"v":1,"kind":"nonsense"}`)
}

func TestBridge_PostCarriesPayloadToThePage(t *testing.T) {
	w, f := newTestView(TrustAllOrigins())
	w.Post(map[string]any{"tick": 3})

	env := f.lastEnvelope(t)
	assert.Equal(t, kindPost, env.Kind)
	assert.Zero(t, env.ID, "a notification needs no correlation")
	assert.JSONEq(t, `{"tick":3}`, string(env.Payload))
}

// ---- lifecycle ----

func TestLifecycle_UnattachedCallsFailRatherThanHang(t *testing.T) {
	w := New("")
	w.EnableBridge(TrustAllOrigins())

	var callErr, evalErr, funcErr error
	w.Call("x", nil, func(_ json.RawMessage, e error) { callErr = e })
	w.EvalResult("1+1", func(_ json.RawMessage, e error) { evalErr = e })
	w.EvalFuncResult("return 1;", nil, func(_ json.RawMessage, e error) { funcErr = e })

	assert.ErrorIs(t, callErr, ErrNotAttached)
	assert.ErrorIs(t, evalErr, ErrNotAttached)
	assert.ErrorIs(t, funcErr, ErrNotAttached)

	// The fire-and-forget forms are documented no-ops, not errors.
	w.Eval("1+1")
	w.Post("hello")
}

func TestLifecycle_DestroyFailsEverythingPending(t *testing.T) {
	w, _ := newTestView(TrustAllOrigins())

	var callErr, evalErr error
	w.Call("x", nil, func(_ json.RawMessage, e error) { callErr = e })
	w.EvalResult("slow()", func(_ json.RawMessage, e error) { evalErr = e })

	w.Destroy()

	assert.ErrorIs(t, callErr, ErrNotAttached)
	assert.ErrorIs(t, evalErr, ErrNotAttached)
	assert.Empty(t, w.bridge.pending)
	assert.Empty(t, w.eval.pending)
}

func TestLifecycle_HandlersSurviveDestroy(t *testing.T) {
	w, f := newTestView(TrustAllOrigins())
	w.Handle("ping", func(Message) (any, error) { return "pong", nil })

	w.Destroy()
	pageSends(w, "https://example.com", envelope{V: 1, Kind: kindCall, ID: 1, Name: "ping"})

	assert.Equal(t, json.RawMessage(`"pong"`), f.lastEnvelope(t).Payload,
		"a view can be re-attached and keep answering")
}

// ---- eval results ----

func TestEvalResult_DeliversJSONValue(t *testing.T) {
	w, f := newTestView(nil)

	var got json.RawMessage
	w.EvalResult("40+2", func(res json.RawMessage, err error) {
		require.NoError(t, err)
		got = res
	})
	require.Len(t, f.ids, 1)

	w.deliverEvalResult(f.ids[0], "42", "")
	assert.Equal(t, json.RawMessage("42"), got)
}

func TestEvalResult_DeliversScriptError(t *testing.T) {
	w, f := newTestView(nil)

	var err error
	w.EvalResult("boom()", func(_ json.RawMessage, e error) { err = e })
	w.deliverEvalResult(f.ids[0], "", "ReferenceError: boom is not defined")

	assert.ErrorContains(t, err, "boom is not defined")
}

func TestEvalResult_NilCallbackIsAllowed(t *testing.T) {
	w, f := newTestView(nil)
	w.EvalResult("sideEffect()", nil)
	w.deliverEvalResult(f.ids[0], "1", "") // must not panic on an absent callback
}

// ---- injection safety ----

func TestScriptFunc_PassesValuesAsData(t *testing.T) {
	js, err := scriptFunc("document.title = title;", Args{"title": `"); alert("pwned`})
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(js, "(function(title){"), "got %s", js)
	assert.NotContains(t, js, `alert("pwned`, "the quote must not close the literal")
	assert.Contains(t, js, `"\"); alert(\"pwned"`)
}

func TestScriptFunc_SortsNamesForDeterminism(t *testing.T) {
	js, err := scriptFunc("return b + a;", Args{"b": 2, "a": 1})
	require.NoError(t, err)
	assert.Equal(t, "(function(a,b){return b + a;\n})(1,2)", js)
}

func TestScriptFunc_EscapesScriptTerminators(t *testing.T) {
	js, err := scriptFunc("f(s);", Args{"s": "</script><script>alert(1)</script>"})
	require.NoError(t, err)
	assert.NotContains(t, js, "</script>", "encoding/json escapes < and >")
}

func TestScriptFunc_RejectsNamesThatAreNotIdentifiers(t *testing.T) {
	for _, name := range []string{"", "1st", "a-b", "a b", "x);alert(1);(", "function", "var", "café"} {
		_, err := scriptFunc("", Args{name: 1})
		assert.Error(t, err, "name %q must be rejected", name)
	}
	for _, name := range []string{"a", "_x", "$el", "camelCase", "n1"} {
		_, err := scriptFunc("", Args{name: 1})
		assert.NoError(t, err, "name %q is a valid identifier", name)
	}
}

func TestScriptFunc_ReportsUnencodableArguments(t *testing.T) {
	_, err := scriptFunc("", Args{"fn": func() {}})
	assert.ErrorContains(t, err, `argument "fn"`)
}

func TestEvalFunc_ReachesTheEngine(t *testing.T) {
	w, f := newTestView(nil)
	w.EvalFunc("document.title = title;", Args{"title": "hi"})
	assert.Equal(t, `(function(title){document.title = title;`+"\n"+`})("hi")`, f.last())
}
