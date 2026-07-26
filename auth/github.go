package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// authenticateGitHub performs GitHub OAuth 2.0 (not OIDC).
// GitHub doesn't support OIDC discovery, so endpoints are hardcoded.
func authenticateGitHub(cfg OIDCConfig) (*OIDCResult, error) {
	// PKCE.
	verifier, challenge, err := generatePKCE()
	if err != nil {
		return nil, err
	}

	// Callback server.
	cb, err := newCallbackServer()
	if err != nil {
		return nil, err
	}
	defer cb.Close()

	redirectURI := cb.RedirectURI()

	state, err := randomString(32)
	if err != nil {
		return nil, err
	}

	// GitHub scopes: "user:email" for email, "read:user" for profile.
	scopes := cfg.Scopes
	if len(scopes) == 0 {
		scopes = []string{"user:email", "read:user"}
	}

	// Build authorization URL.
	authURL := "https://github.com/login/oauth/authorize?" + url.Values{
		"client_id":             {cfg.ClientID},
		"redirect_uri":          {redirectURI},
		"scope":                 {joinScopes(scopes)},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}.Encode()

	if err := openBrowser(authURL); err != nil {
		return nil, fmt.Errorf("open browser: %w", err)
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	code, callbackState, err := cb.WaitForCode(timeout)
	if err != nil {
		return nil, err
	}
	if callbackState != state {
		return nil, fmt.Errorf("state mismatch")
	}

	// Exchange code.
	tokenResp, err := exchangeGitHubCode(cfg.ClientID, code, redirectURI, verifier)
	if err != nil {
		return nil, err
	}

	// Fetch user info.
	user, err := fetchGitHubUser(tokenResp.AccessToken)
	if err != nil {
		return nil, err
	}

	user.AccessToken = tokenResp.AccessToken
	return user, nil
}

func exchangeGitHubCode(clientID, code, redirectURI, verifier string) (*tokenResponse, error) {
	data := url.Values{
		"client_id":     {clientID},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"code_verifier": {verifier},
	}

	req, err := http.NewRequest("POST", "https://github.com/login/oauth/access_token", nil)
	if err != nil {
		return nil, err
	}
	req.URL.RawQuery = data.Encode()
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("decode token: %w", err)
	}
	if tr.AccessToken == "" {
		return nil, fmt.Errorf("no access token: %s", string(body))
	}
	return &tr, nil
}

func fetchGitHubUser(accessToken string) (*OIDCResult, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var user struct {
		Login     string `json:"login"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
		ID        int    `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}

	name := user.Name
	if name == "" {
		name = user.Login
	}

	return &OIDCResult{
		Sub:     fmt.Sprintf("%d", user.ID),
		Email:   user.Email,
		Name:    name,
		Picture: user.AvatarURL,
	}, nil
}

func joinScopes(scopes []string) string {
	result := ""
	for i, s := range scopes {
		if i > 0 {
			result += " "
		}
		result += s
	}
	return result
}
