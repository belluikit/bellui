// Command signinprobe signs in through auth.Authenticate on a phone (#1042):
// Sign in with Apple and Google on iOS, Google through Credential Manager on
// Android. Each attempt prints one line with what Authenticate verified,
// or that it was cancelled or failed:
//
//	signinprobe provider=apple ok iss=https://appleid.apple.com aud=… nonce=match sub=… email=a***@example.com code=yes
//	signinprobe provider=google cancelled
//
// and, on success, the raw ID token, so it can be checked again off the device
// against the provider's published keys:
//
//	signinprobe idtoken provider=apple eyJ…
//
// The token is a credential for about an hour: keep it to the machine the
// device is attached to.
//
// The client IDs below are the probe's own, from the owner's Google Cloud
// project; they are public identifiers, not secrets. The Apple audience is
// the app's bundle ID, com.trendvidia.signinprobe.
//
// Packaging:
//
//   - iOS: `bellui package --target ios --app-id com.trendvidia.signinprobe`.
//     ios/Entitlements.plist carries the Sign in with Apple entitlement.
//   - Android: run ./fetch-libs.sh first, then
//     `bellui package --target android/arm64 --app-id com.trendvidia.signinprobe`.
//
// It quits by itself after fifteen minutes.
package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/auth"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/widget"
)

const (
	// googleWebClient is the audience of a Credential Manager token.
	googleWebClient = "303374290916-ev21c4rqrd1ltj97i186a9h8745kkuv7.apps.googleusercontent.com"
	// googleIOSClient signs in through the browser session on iOS.
	googleIOSClient = "303374290916-ohpe1svna6ake7jrk2npbqbhen2gn0ic.apps.googleusercontent.com"
)

func main() {
	a := app.New()
	w := a.NewWindow("signinprobe")
	status := widget.NewLabel("Tap a provider to sign in.")
	status.Wrapping = bellui.TextWrapWord

	signIn := func(provider, clientID string) {
		nonce := newNonce()
		status.SetText("Signing in with " + provider + "…")
		go func() {
			r, err := auth.Authenticate(auth.OIDCConfig{Provider: provider, ClientID: clientID, Nonce: nonce})
			line := report(provider, nonce, r, err)
			fmt.Println(line)
			if err == nil {
				fmt.Println("signinprobe idtoken provider=" + provider + " " + r.IDToken)
			}
			bellui.Do(func() { status.SetText(line) })
		}()
	}

	buttons := container.NewVBox()
	if runtime.GOOS == "ios" {
		buttons.Add(widget.NewButton("Sign in with Apple", func() { signIn("apple", "") }))
		buttons.Add(widget.NewButton("Sign in with Google", func() { signIn("google", googleIOSClient) }))
	} else {
		buttons.Add(widget.NewButton("Sign in with Google", func() { signIn("google", googleWebClient) }))
	}
	w.SetContent(container.NewVBox(buttons, status))

	// App.Quit does nothing on a phone, whose guidelines forbid an app
	// quitting itself, so the self-stop exits the process.
	go func() {
		time.Sleep(15 * time.Minute)
		fmt.Println("signinprobe done")
		os.Exit(0)
	}()
	fmt.Println("signinprobe ready os=" + runtime.GOOS)
	bellui.ShowAndRun(w)
}

func newNonce() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "signinprobe-" + hex.EncodeToString(b)
}

// report describes one attempt. The claims it prints are read from the token
// Authenticate already verified; the nonce is compared with the one this
// attempt sent, hashed first for Apple, whose sheet takes the hash.
func report(provider, nonce string, r *auth.OIDCResult, err error) string {
	switch {
	case errors.Is(err, auth.ErrCancelled):
		return "signinprobe provider=" + provider + " cancelled"
	case err != nil:
		return "signinprobe provider=" + provider + " error=" + err.Error()
	}
	var c struct {
		Iss   string          `json:"iss"`
		Aud   json.RawMessage `json:"aud"`
		Nonce string          `json:"nonce"`
		Exp   int64           `json:"exp"`
	}
	if parts := strings.Split(r.IDToken, "."); len(parts) == 3 {
		if p, err := base64.RawURLEncoding.DecodeString(parts[1]); err == nil {
			_ = json.Unmarshal(p, &c)
		}
	}
	want := nonce
	if provider == "apple" {
		want = sha256Hex(nonce)
	}
	match := "mismatch"
	if c.Nonce == want {
		match = "match"
	}
	code := "no"
	if r.AuthorizationCode != "" {
		code = "yes"
	}
	return fmt.Sprintf("signinprobe provider=%s ok iss=%s aud=%s nonce=%s exp_in=%s sub=%s email=%s name=%t code=%s",
		provider, c.Iss, string(c.Aud), match, time.Until(time.Unix(c.Exp, 0)).Round(time.Minute),
		r.Sub, mask(r.Email), r.Name != "", code)
}

// mask keeps an email's first letter and domain, so a log line says an email
// came back without carrying the address.
func mask(email string) string {
	at := strings.IndexByte(email, '@')
	if at < 1 {
		return "none"
	}
	return email[:1] + "***" + email[at:]
}
