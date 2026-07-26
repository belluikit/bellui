package gsportal

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	portalDest   = "org.freedesktop.portal.Desktop"
	portalPath   = "/org/freedesktop/portal/desktop"
	gsIface      = "org.freedesktop.portal.GlobalShortcuts"
	requestIface = "org.freedesktop.portal.Request"

	// createSessionTimeout bounds the wait for the (non-interactive) CreateSession
	// reply. Binding, by contrast, is not waited on — it may show a consent
	// dialog, so it is fire-and-accept per the GlobalShortcutApp contract.
	createSessionTimeout = 30 * time.Second
)

// ErrUnavailable is reported when the GlobalShortcuts portal is not on the bus
// (a compositor/xdg-desktop-portal without GlobalShortcuts support, e.g. GNOME
// before 45). Callers surface this as "unsupported" rather than a hard failure.
var ErrUnavailable = errors.New("org.freedesktop.portal.GlobalShortcuts is not available")

// entry is one registered shortcut.
type entry struct {
	id          string // stable portal shortcut id (CustomShortcut.ShortcutName())
	description string // human-readable, shown in the consent dialog
	trigger     string // preferred_trigger hint (see PortalTrigger)
	handler     func()
}

// Client wraps a GlobalShortcuts portal session. It lazily creates the session
// on the first Register and (re)binds the full shortcut set through the portal,
// which prompts the user for consent. Activations arrive as D-Bus signals and
// are routed to the registered handler via deliver (typically bellui.Do, so the
// handler runs on the main goroutine).
type Client struct {
	conn    *dbus.Conn
	sender  string // unique bus name with the leading ':' stripped, '.'→'_'
	deliver func(func())

	mu        sync.Mutex
	tokenSeq  uint64
	session   dbus.ObjectPath
	entries   map[string]*entry                    // id → entry
	responses map[dbus.ObjectPath]chan responseMsg // pending request path → waiter
}

type responseMsg struct {
	code    uint32
	results map[string]dbus.Variant
}

// NewClient connects to the session bus, verifies the GlobalShortcuts portal is
// present, subscribes to its Activated signal and to portal Request responses,
// and starts routing. deliver schedules a handler onto the app's main goroutine.
//
// appID is the application's unique id. On a non-sandboxed (non-Flatpak) app the
// portal cannot otherwise determine it, and backends such as GNOME reject
// GlobalShortcuts with "an app id is required"; NewClient registers it via the
// host portal Registry so subsequent portal calls on this connection are
// attributed to the app.
func NewClient(appID string, deliver func(func())) (*Client, error) {
	conn, err := dbus.SessionBus() // shared connection, don't close
	if err != nil {
		return nil, err
	}
	if !portalAvailable(conn) {
		return nil, ErrUnavailable
	}

	c := &Client{
		conn:      conn,
		sender:    senderToken(conn.Names()),
		deliver:   deliver,
		entries:   map[string]*entry{},
		responses: map[dbus.ObjectPath]chan responseMsg{},
	}

	c.registerAppID(appID)

	// Watch the GlobalShortcuts Activated signal and every portal Request Response.
	if err := conn.AddMatchSignal(
		dbus.WithMatchInterface(gsIface),
		dbus.WithMatchMember("Activated"),
	); err != nil {
		return nil, err
	}
	if err := conn.AddMatchSignal(
		dbus.WithMatchInterface(requestIface),
		dbus.WithMatchMember("Response"),
	); err != nil {
		return nil, err
	}

	ch := make(chan *dbus.Signal, 16)
	conn.Signal(ch)
	go c.consume(ch)
	return c, nil
}

// registerAppID associates appID with this connection via the host portal
// Registry (org.freedesktop.host.portal.Registry.Register), so a non-sandboxed
// app is identifiable to GlobalShortcuts. Best-effort: an empty id, or an older
// portal without the Registry interface (where the id is resolved from the app
// scope instead), is a non-fatal skip.
func (c *Client) registerAppID(appID string) {
	if appID == "" {
		return
	}
	obj := c.conn.Object(portalDest, portalPath)
	obj.Call("org.freedesktop.host.portal.Registry.Register", 0, appID, map[string]dbus.Variant{})
}

// portalAvailable reports whether the GlobalShortcuts interface answers a
// property read (its "version"), which fails cleanly when the portal is absent.
func portalAvailable(conn *dbus.Conn) bool {
	obj := conn.Object(portalDest, portalPath)
	var v dbus.Variant
	err := obj.Call("org.freedesktop.DBus.Properties.Get", 0, gsIface, "version").Store(&v)
	return err == nil
}

// Register adds or replaces a shortcut and (re)binds the full set through the
// portal. It returns once the request has been accepted (the consent dialog, if
// any, resolves asynchronously) — a nil error means "accepted", per the
// GlobalShortcutApp contract, not "confirmed live".
func (c *Client) Register(id, description, trigger string, handler func()) error {
	c.mu.Lock()
	c.entries[id] = &entry{id: id, description: description, trigger: trigger, handler: handler}
	c.mu.Unlock()

	if err := c.ensureSession(); err != nil {
		return err
	}
	return c.bindAll()
}

// Unregister removes a shortcut. The remaining set is re-bound; unknown ids are
// ignored. (The portal has no per-shortcut unbind, so the set is rebound.)
func (c *Client) Unregister(id string) {
	c.mu.Lock()
	_, ok := c.entries[id]
	if ok {
		delete(c.entries, id)
	}
	c.mu.Unlock()
	if !ok || c.sessionPath() == "" {
		return
	}
	_ = c.bindAll()
}

func (c *Client) sessionPath() dbus.ObjectPath {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.session
}

// ensureSession creates the portal session once. CreateSession is
// non-interactive, so the reply is awaited (bounded).
func (c *Client) ensureSession() error {
	c.mu.Lock()
	if c.session != "" {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	sessionToken := c.newToken()
	res, err := c.callWithRequest(createSessionTimeout, gsIface+".CreateSession", func(opts map[string]dbus.Variant) {
		opts["session_handle_token"] = dbus.MakeVariant(sessionToken)
	})
	if err != nil {
		// GNOME rejects the session with "an app id is required" when the app is
		// not identifiable — a non-sandboxed app must be installed with a .desktop
		// file whose id matches its UniqueID (see registerAppID). Add the hint.
		if strings.Contains(strings.ToLower(err.Error()), "app id") {
			return fmt.Errorf("%w — the app must be installed with a .desktop file matching its UniqueID for global shortcuts", err)
		}
		return err
	}
	handle, ok := res.results["session_handle"]
	if !ok {
		return errors.New("gsportal: CreateSession returned no session_handle")
	}
	path, ok := handle.Value().(string)
	if !ok {
		return fmt.Errorf("gsportal: unexpected session_handle type %T", handle.Value())
	}

	c.mu.Lock()
	c.session = dbus.ObjectPath(path)
	c.mu.Unlock()
	return nil
}

// bindAll binds the full current shortcut set. It does not wait for the response
// (which may follow a user consent dialog); a failure to even issue the call is
// returned, success means the request was accepted.
func (c *Client) bindAll() error {
	c.mu.Lock()
	session := c.session
	list := make([]portalShortcut, 0, len(c.entries))
	for _, e := range c.entries {
		meta := map[string]dbus.Variant{"description": dbus.MakeVariant(e.description)}
		if e.trigger != "" {
			meta["preferred_trigger"] = dbus.MakeVariant(e.trigger)
		}
		list = append(list, portalShortcut{ID: e.id, Meta: meta})
	}
	c.mu.Unlock()

	if session == "" {
		return errors.New("gsportal: no session")
	}

	token := c.newToken()
	opts := map[string]dbus.Variant{"handle_token": dbus.MakeVariant(token)}
	obj := c.conn.Object(portalDest, portalPath)
	// BindShortcuts(session o, shortcuts a(sa{sv}), parent_window s, options a{sv})
	call := obj.Call(gsIface+".BindShortcuts", 0, session, list, "", opts)
	return call.Err
}

// portalShortcut encodes one a(sa{sv}) element: (id, {description, preferred_trigger}).
type portalShortcut struct {
	ID   string
	Meta map[string]dbus.Variant
}

// callWithRequest issues a portal method whose result is delivered via the
// Request Response signal. It registers a waiter at the *predicted* request path
// (from the handle_token it injects) before the call, so a fast response cannot
// be missed, then waits up to timeout for the response.
func (c *Client) callWithRequest(timeout time.Duration, method string, buildOpts func(map[string]dbus.Variant), args ...interface{}) (responseMsg, error) {
	token := c.newToken()
	reqPath := requestPath(c.sender, token)

	opts := map[string]dbus.Variant{"handle_token": dbus.MakeVariant(token)}
	if buildOpts != nil {
		buildOpts(opts)
	}

	ch := make(chan responseMsg, 1)
	c.mu.Lock()
	c.responses[reqPath] = ch
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.responses, reqPath)
		c.mu.Unlock()
	}()

	obj := c.conn.Object(portalDest, portalPath)
	call := obj.Call(method, 0, append(args, opts)...)
	if call.Err != nil {
		return responseMsg{}, call.Err
	}
	// The portal returns the actual request handle; honour it if it differs from
	// the prediction (spec guarantees the predicted path, but be defensive).
	var actual dbus.ObjectPath
	if err := call.Store(&actual); err == nil && actual != "" && actual != reqPath {
		c.mu.Lock()
		delete(c.responses, reqPath)
		c.responses[actual] = ch
		c.mu.Unlock()
		reqPath = actual
	}

	select {
	case msg := <-ch:
		if msg.code != 0 {
			return msg, fmt.Errorf("gsportal: %s request failed (response %d)", method, msg.code)
		}
		return msg, nil
	case <-time.After(timeout):
		return responseMsg{}, fmt.Errorf("gsportal: timed out waiting for %s response", method)
	}
}

// consume routes incoming Activated and Request Response signals.
func (c *Client) consume(ch chan *dbus.Signal) {
	for sig := range ch {
		switch sig.Name {
		case requestIface + ".Response":
			c.routeResponse(sig)
		case gsIface + ".Activated":
			c.routeActivated(sig)
		}
	}
}

// routeResponse dispatches a Request Response(u response, a{sv} results) to the
// waiter registered for the request's object path.
func (c *Client) routeResponse(sig *dbus.Signal) {
	if len(sig.Body) < 2 {
		return
	}
	code, _ := sig.Body[0].(uint32)
	results, _ := sig.Body[1].(map[string]dbus.Variant)

	c.mu.Lock()
	ch := c.responses[sig.Path]
	c.mu.Unlock()
	if ch != nil {
		select {
		case ch <- responseMsg{code: code, results: results}:
		default:
		}
	}
}

// routeActivated fires the handler for an Activated(o session, s shortcut_id, …)
// signal on the delivery goroutine.
func (c *Client) routeActivated(sig *dbus.Signal) {
	if len(sig.Body) < 2 {
		return
	}
	id, _ := sig.Body[1].(string)

	c.mu.Lock()
	e := c.entries[id]
	c.mu.Unlock()
	if e != nil && e.handler != nil {
		c.deliver(e.handler)
	}
}

// newToken returns a per-client-unique token for handle_token / session_handle_token.
func (c *Client) newToken() string {
	c.mu.Lock()
	c.tokenSeq++
	seq := c.tokenSeq
	c.mu.Unlock()
	return "bellui" + strconv.FormatUint(seq, 10)
}

// senderToken derives the request-path sender segment from the connection's
// unique name: strip the leading ':' and replace '.' with '_'
// (e.g. ":1.42" → "1_42").
func senderToken(names []string) string {
	if len(names) == 0 {
		return ""
	}
	s := strings.TrimPrefix(names[0], ":")
	return strings.ReplaceAll(s, ".", "_")
}

// requestPath computes the predicted portal Request object path for a token,
// per the xdg-desktop-portal convention.
func requestPath(sender, token string) dbus.ObjectPath {
	return dbus.ObjectPath("/org/freedesktop/portal/desktop/request/" + sender + "/" + token)
}
