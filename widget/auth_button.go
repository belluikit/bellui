package widget

import (
	"context"
	"image/color"
	"strings"
	"sync"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/auth"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	internalWidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/theme"
)

// AuthButton is a branded sign-in button that triggers an OIDC or OAuth 2.0
// authentication flow for a specific provider. It displays the provider's
// logo and brand-compliant colors.
//
// Supported providers: "google", "microsoft", "apple", "github", "okta",
// "keycloak", and "custom".
//
// A disabled AuthButton keeps its place in the disabled look and takes no tap
// (#1212). Disabling it does not cancel a sign-in already running; call
// CancelAuth for that.
type AuthButton struct {
	DisableableWidget
	pressTarget

	// Provider is the provider key (e.g., "google", "microsoft").
	Provider string

	// ClientID is the OAuth 2.0 client identifier.
	ClientID string

	// Issuer overrides the OIDC discovery URL. Required for "okta",
	// "keycloak", and "custom" providers.
	//
	// When the button saves a refresh token, an Issuer of the app's own (any
	// value other than the provider preset's, a Microsoft tenant included) is
	// part of the token's key, so two buttons for one provider with different
	// issuers, such as two Keycloak realms, keep separate tokens. Read a
	// button's token back with auth.LoadTokenFor(Provider, Issuer) and remove
	// it with auth.ClearTokenFor; for an empty or preset Issuer,
	// auth.LoadToken(Provider) finds the same token.
	Issuer string

	// Scopes is a space-separated list of OAuth scopes.
	// Defaults to "openid email profile" for OIDC providers and to
	// "user:email read:user" for GitHub.
	Scopes string

	// Text overrides the button label. Defaults to the provider preset.
	Text string

	// Logo overrides the button icon. Defaults to the provider preset.
	Logo bellui.Resource

	// OnAuthenticated is called on the main thread after a successful
	// authentication with the user's claims.
	OnAuthenticated func(result auth.OIDCResult) `json:"-"`

	// OnError is called on the main thread if authentication fails.
	OnError func(err error) `json:"-"`

	cancel context.CancelFunc
	mu     sync.Mutex
}

// NewAuthButton creates a new authentication button for the given provider.
func NewAuthButton(provider, clientID string, onAuth func(auth.OIDCResult)) *AuthButton {
	b := &AuthButton{
		Provider:        provider,
		ClientID:        clientID,
		OnAuthenticated: onAuth,
	}
	b.ExtendBaseWidget(b)
	return b
}

// Tapped triggers the authentication flow in a background goroutine.
// If an authentication is already in progress, or the button is disabled, the
// tap is ignored.
func (b *AuthButton) Tapped(ev *event.PointEvent) {
	if b.Disabled() {
		return
	}
	// The press shows before the lock is taken: pressShape takes it too, and
	// a sync.Mutex is not re-entrant, so showing it under the lock hung the
	// main goroutine on the first tap of a drawn button.
	internalWidget.TapPress(&b.pressTarget, ev)
	b.mu.Lock()
	if b.cancel != nil {
		b.mu.Unlock()
		return // already authenticating
	}
	ctx, cancel := context.WithCancel(context.Background())
	b.cancel = cancel
	b.mu.Unlock()

	go b.authenticate(ctx)
}

// HoverIn does nothing: the button is Hoverable for HoverOut.
func (b *AuthButton) HoverIn(*event.HoverEvent) {}

// HoverMoved does nothing.
func (b *AuthButton) HoverMoved(*event.HoverEvent) {}

// HoverOut ends a press the pointer took off the button: it gets no MouseUp
// for a release elsewhere.
func (b *AuthButton) HoverOut() {
	internalWidget.CancelPress(&b.pressTarget)
}

// pressShape is the whole button, in the provider's text colour, unless an
// authentication is already running or the button is disabled, and a tap
// would do nothing (#1091, #1212).
func (b *AuthButton) pressShape() (internalWidget.PressShape, bool) {
	if b.Disabled() {
		return internalWidget.PressShape{}, false
	}
	b.mu.Lock()
	busy := b.cancel != nil
	b.mu.Unlock()
	if busy {
		return internalWidget.PressShape{}, false
	}
	return internalWidget.PressShape{
		Size:         b.Size(),
		CornerRadius: 4, // the button's own corners
		Content:      b.preset().TextColor,
	}, true
}

// CancelAuth cancels any in-flight authentication flow.
func (b *AuthButton) CancelAuth() {
	b.mu.Lock()
	if b.cancel != nil {
		b.cancel()
		b.cancel = nil
	}
	b.mu.Unlock()
}

func (b *AuthButton) authenticate(ctx context.Context) {
	defer func() {
		b.mu.Lock()
		b.cancel = nil
		b.mu.Unlock()
	}()

	scopes := strings.Fields(b.Scopes)
	cfg := auth.OIDCConfig{
		Provider: b.Provider,
		Issuer:   b.Issuer,
		ClientID: b.ClientID,
		Scopes:   scopes,
	}

	// Run Authenticate in a sub-goroutine so we can select on ctx.Done().
	type authResult struct {
		result *auth.OIDCResult
		err    error
	}
	ch := make(chan authResult, 1)
	go func() {
		result, err := authenticate(cfg)
		ch <- authResult{result, err}
	}()

	select {
	case <-ctx.Done():
		return
	case res := <-ch:
		if ctx.Err() != nil {
			return // cancelled while waiting
		}
		if res.err != nil {
			if b.OnError != nil {
				bellui.Do(func() { b.OnError(res.err) })
			}
			return
		}

		// Persist the refresh token so the app can read it back with
		// auth.LoadToken or auth.LoadTokenFor (exchanging it for new tokens is
		// up to the app; the auth package does not do it). Authentication
		// itself succeeded, so a persistence failure is logged, not routed to
		// OnError.
		if res.result.RefreshToken != "" {
			if err := saveToken(b.Provider, b.Issuer, res.result.RefreshToken, res.result.Email); err != nil {
				bellui.LogError("auth: failed to persist refresh token", err)
			}
		}

		if b.OnAuthenticated != nil {
			r := *res.result
			bellui.Do(func() { b.OnAuthenticated(r) })
		}
	}
}

// authenticate and saveToken are the auth package's sign-in and token store,
// replaceable in tests so that no browser opens and no token is written to the
// user's config directory.
var (
	authenticate = auth.Authenticate
	saveToken    = auth.SaveTokenFor
)

func (b *AuthButton) preset() auth.ProviderPreset {
	if p, ok := auth.Providers[b.Provider]; ok {
		return p
	}
	return auth.ProviderPreset{
		Name:       "Custom",
		BgColor:    color.NRGBA{R: 0x4D, G: 0x4D, B: 0x4D, A: 0xFF},
		TextColor:  color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF},
		ButtonText: "Sign in",
	}
}

func (b *AuthButton) CreateRenderer() core.WidgetRenderer {
	b.ExtendBaseWidget(b)
	preset := b.preset()

	bgColor := preset.BgColor
	textColor := preset.TextColor
	buttonText := preset.ButtonText
	if b.Text != "" {
		buttonText = b.Text
	}

	bg := canvas.NewRectangle(bgColor)
	bg.CornerRadius = 4

	label := canvas.NewText(buttonText, textColor)
	label.TextSize = b.Theme().Size(theme.SizeNameBodyLarge)
	label.TextStyle.Bold = true

	logo := b.Logo
	if logo == nil {
		logo = preset.Logo
	}

	var logoImg *canvas.Image
	if logo != nil {
		logoImg = canvas.NewImageFromResource(logo)
		logoImg.FillMode = canvas.ImageFillContain
	}

	objects := []core.CanvasObject{bg, internalWidget.AttachPress(&b.pressTarget, b.pressShape), label}
	if logoImg != nil {
		objects = append(objects, logoImg)
	}

	r := &authButtonRenderer{
		BaseRenderer: internalWidget.NewBaseRenderer(objects),
		bg:           bg,
		label:        label,
		logoImg:      logoImg,
		logo:         logo,
		btn:          b,
	}
	r.applyColors()
	return r
}

type authButtonRenderer struct {
	internalWidget.BaseRenderer
	bg      *canvas.Rectangle
	label   *canvas.Text
	logoImg *canvas.Image
	logo    bellui.Resource // the logo as given, before a disabled tint
	btn     *AuthButton
}

// applyColors paints the provider's brand colours, or the disabled look: the
// background in the disabled button colour, the label and logo in the
// disabled content colour, as for a disabled Button (#1212).
func (r *authButtonRenderer) applyColors() {
	preset := r.btn.preset()
	if !r.btn.Disabled() {
		r.bg.FillColor = preset.BgColor
		r.label.Color = preset.TextColor
		if r.logoImg != nil {
			r.logoImg.Resource = r.logo
		}
		return
	}
	th := r.btn.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	r.bg.FillColor = th.Color(theme.ColorNameDisabledButton, v)
	r.label.Color = th.Color(theme.ColorNameDisabled, v)
	if r.logoImg != nil {
		r.logoImg.Resource = theme.NewDisabledResource(r.logo)
	}
}

func (r *authButtonRenderer) MinSize() geo.Size {
	textSize := r.label.MinSize()
	h := textSize.Height + 24 // vertical padding
	w := textSize.Width + 48  // horizontal padding
	if r.logoImg != nil {
		w += h // logo takes a square area
	}
	return geo.NewSize(w, h)
}

func (r *authButtonRenderer) Layout(size geo.Size) {
	r.bg.Resize(size)

	padding := float32(12)
	logoSize := size.Height - padding*2
	x := padding

	if r.logoImg != nil {
		r.logoImg.Move(geo.NewPos(x, padding))
		r.logoImg.Resize(geo.NewSize(logoSize, logoSize))
		x += logoSize + padding
	}

	labelSize := r.label.MinSize()
	r.label.Move(geo.NewPos(x, (size.Height-labelSize.Height)/2))
	r.label.Resize(labelSize) // tight box so the RTL reflection lands correctly

	// Under RTL reflect the row so the logo sits on the right and the label on
	// the left; the full-width bg maps onto itself. The press ripple is left
	// out: a press places it.
	mirrored := []core.CanvasObject{r.bg, r.label}
	if r.logoImg != nil {
		mirrored = append(mirrored, r.logoImg)
	}
	mirrorIfRTL(mirrored, size.Width)
}

func (r *authButtonRenderer) Refresh() {
	preset := r.btn.preset()
	r.applyColors()
	r.bg.Refresh()

	buttonText := preset.ButtonText
	if r.btn.Text != "" {
		buttonText = r.btn.Text
	}
	r.label.Text = buttonText
	r.label.Refresh()
	if r.logoImg != nil {
		r.logoImg.Refresh()
	}

	r.Layout(r.btn.Size())
	canvas.Refresh(r.btn.super())
}
