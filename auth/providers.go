package auth

import (
	_ "embed"
	"image/color"

	"github.com/belluikit/bellui"
)

//go:embed logos/google.svg
var googleLogo []byte

//go:embed logos/microsoft.svg
var microsoftLogo []byte

//go:embed logos/apple.svg
var appleLogo []byte

//go:embed logos/github.svg
var githubLogo []byte

//go:embed logos/okta.svg
var oktaLogo []byte

// ProviderPreset defines the visual identity and discovery URL for a
// known authentication provider.
type ProviderPreset struct {
	// Name is the human-readable provider name.
	Name string

	// Issuer is the OIDC discovery base URL.
	// Empty for providers that need a tenant-specific issuer (Okta, Keycloak).
	Issuer string

	// Logo is the provider's brand icon.
	Logo bellui.Resource

	// BgColor is the brand-compliant button background color.
	BgColor color.NRGBA

	// TextColor is the brand-compliant button text color.
	TextColor color.NRGBA

	// ButtonText is the default button label (e.g., "Sign in with Google").
	ButtonText string
}

// Providers maps provider keys to their presets.
var Providers = map[string]ProviderPreset{
	"google": {
		Name:       "Google",
		Issuer:     "https://accounts.google.com",
		Logo:       bellui.NewStaticResource("google.svg", googleLogo),
		BgColor:    color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}, // white
		TextColor:  color.NRGBA{R: 0x1F, G: 0x1F, B: 0x1F, A: 0xFF}, // dark grey
		ButtonText: "Sign in with Google",
	},
	"microsoft": {
		Name:       "Microsoft",
		Issuer:     "https://login.microsoftonline.com/common/v2.0",
		Logo:       bellui.NewStaticResource("microsoft.svg", microsoftLogo),
		BgColor:    color.NRGBA{R: 0x2F, G: 0x2F, B: 0x2F, A: 0xFF}, // dark
		TextColor:  color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}, // white
		ButtonText: "Sign in with Microsoft",
	},
	"apple": {
		Name:       "Apple",
		Issuer:     "https://appleid.apple.com",
		Logo:       bellui.NewStaticResource("apple.svg", appleLogo),
		BgColor:    color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xFF}, // black
		TextColor:  color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}, // white
		ButtonText: "Sign in with Apple",
	},
	"github": {
		Name:       "GitHub",
		Issuer:     "", // GitHub doesn't use OIDC discovery
		Logo:       bellui.NewStaticResource("github.svg", githubLogo),
		BgColor:    color.NRGBA{R: 0x24, G: 0x29, B: 0x2E, A: 0xFF}, // dark
		TextColor:  color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}, // white
		ButtonText: "Sign in with GitHub",
	},
	"okta": {
		Name:       "Okta",
		Issuer:     "", // tenant-specific, must be provided via config
		Logo:       bellui.NewStaticResource("okta.svg", oktaLogo),
		BgColor:    color.NRGBA{R: 0x00, G: 0x7D, B: 0xC1, A: 0xFF}, // Okta blue
		TextColor:  color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}, // white
		ButtonText: "Sign in with Okta",
	},
	"keycloak": {
		Name:       "Keycloak",
		Issuer:     "", // tenant-specific
		Logo:       nil,
		BgColor:    color.NRGBA{R: 0x4D, G: 0x4D, B: 0x4D, A: 0xFF}, // grey
		TextColor:  color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}, // white
		ButtonText: "Sign in with Keycloak",
	},
}
