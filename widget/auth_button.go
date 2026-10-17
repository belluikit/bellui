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
type AuthButton struct {
	BaseWidget

	// Provider is the provider key (e.g., "google", "microsoft").
	Provider string

	// ClientID is the OAuth 2.0 client identifier.
	ClientID string

	// Issuer overrides the OIDC discovery URL. Required for "okta",
	// "keycloak", and "custom" providers.
	//
	// When the button saves a refresh token, an Issuer of the app's own (any
	// value other than the provider preset's) is part of the token's key, so
	// two buttons for one provider with different issuers, such as two
	// Keycloak realms, keep separate tokens. Read such a token back with
	// auth.LoadTokenFor(Provider, Issuer); for a preset issuer, auth.LoadToken
	// (Provider) is enough.
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
// If an authentication is already in progress, the tap is ignored.
func (b *AuthButton) Tapped(_ *event.PointEvent) {
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
			if err := saveToken(b.Provider, b.tokenIssuer(), res.result.RefreshToken, res.result.Email); err != nil {
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

// tokenIssuer is the issuer the button's saved token is keyed by: the app's
// own Issuer, or "" when the button uses the provider preset's (#356).
func (b *AuthButton) tokenIssuer() string {
	if b.Issuer == "" || b.Issuer == b.preset().Issuer {
		return ""
	}
	return b.Issuer
}

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

	objects := []core.CanvasObject{bg, label}
	if logoImg != nil {
		objects = append(objects, logoImg)
	}

	return &authButtonRenderer{
		BaseRenderer: internalWidget.NewBaseRenderer(objects),
		bg:           bg,
		label:        label,
		logoImg:      logoImg,
		btn:          b,
	}
}

type authButtonRenderer struct {
	internalWidget.BaseRenderer
	bg      *canvas.Rectangle
	label   *canvas.Text
	logoImg *canvas.Image
	btn     *AuthButton
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
	// the left; the full-width bg maps onto itself.
	mirrorIfRTL(r.Objects(), size.Width)
}

func (r *authButtonRenderer) Refresh() {
	preset := r.btn.preset()
	r.bg.FillColor = preset.BgColor
	r.bg.Refresh()

	buttonText := preset.ButtonText
	if r.btn.Text != "" {
		buttonText = r.btn.Text
	}
	r.label.Text = buttonText
	r.label.Color = preset.TextColor
	r.label.Refresh()

	r.Layout(r.btn.Size())
	canvas.Refresh(r.btn.super())
}
