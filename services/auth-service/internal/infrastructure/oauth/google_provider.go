package oauth

import (
	"auth-service/internal/config"
	"auth-service/internal/domain/entities"
	domainServices "auth-service/internal/domain/services"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type GoogleProvider struct {
	config *oauth2.Config
}

func NewGoogleProvider(cfg config.GoogleConfig) *GoogleProvider {
	return &GoogleProvider{
		config: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Scopes: []string{
				"https://www.googleapis.com/auth/userinfo.email",
				"https://www.googleapis.com/auth/userinfo.profile",
				"openid",
			},
			Endpoint: google.Endpoint,
		},
	}
}

func (g *GoogleProvider) GetAuthURL(req *domainServices.AuthURLRequest) string {
	if req == nil {
		req = &domainServices.AuthURLRequest{}
	}

	opts := []oauth2.AuthCodeOption{
		oauth2.SetAuthURLParam("include_granted_scopes", "true"),
		oauth2.SetAuthURLParam("prompt", "select_account"),
	}

	if req.CodeChallenge != "" {
		method := req.CodeChallengeMethod
		if method == "" {
			method = "S256"
		}
		opts = append(opts,
			oauth2.SetAuthURLParam("code_challenge", req.CodeChallenge),
			oauth2.SetAuthURLParam("code_challenge_method", method),
		)
	}

	return g.config.AuthCodeURL(req.State, opts...)
}

func (g *GoogleProvider) ExchangeCodeForToken(ctx context.Context, code string) (*entities.GoogleUserInfo, error) {
	// Exchange authorization code for token
	token, err := g.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}

	// Use the token to get user info
	return g.GetUserInfo(ctx, token.AccessToken)
}

func (g *GoogleProvider) GetUserInfo(ctx context.Context, accessToken string) (*entities.GoogleUserInfo, error) {
	client := g.config.Client(ctx, &oauth2.Token{AccessToken: accessToken})

	// Primary endpoint for OIDC profile claims.
	userInfo, rawBody, err := fetchUserInfo(client, "https://openidconnect.googleapis.com/v1/userinfo")
	if err == nil && userInfo.IsValid() {
		return userInfo, nil
	}

	// Fallback endpoint used by older Google OAuth flows.
	legacyUserInfo, legacyRawBody, legacyErr := fetchUserInfo(client, "https://www.googleapis.com/oauth2/v2/userinfo")
	if legacyErr == nil && legacyUserInfo.IsValid() {
		return legacyUserInfo, nil
	}

	primaryErr := ""
	if err != nil {
		primaryErr = err.Error()
	}
	legacyErrText := ""
	if legacyErr != nil {
		legacyErrText = legacyErr.Error()
	}

	return nil, fmt.Errorf(
		"invalid user info received from Google: missing required fields (primary_err=%q primary_body=%q legacy_err=%q legacy_body=%q)",
		primaryErr,
		compactForLog(rawBody),
		legacyErrText,
		compactForLog(legacyRawBody),
	)
}

func fetchUserInfo(client *http.Client, endpoint string) (*entities.GoogleUserInfo, []byte, error) {
	resp, err := client.Get(endpoint)
	if err != nil {
		return nil, nil, fmt.Errorf("request %s failed: %w", endpoint, err)
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, nil, fmt.Errorf("read %s failed: %w", endpoint, readErr)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, body, fmt.Errorf("request %s failed with status %d", endpoint, resp.StatusCode)
	}

	var userInfo entities.GoogleUserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, body, fmt.Errorf("parse %s failed: %w", endpoint, err)
	}

	return &userInfo, body, nil
}

func compactForLog(body []byte) string {
	if len(body) == 0 {
		return ""
	}

	const maxLen = 300
	text := strings.Join(strings.Fields(string(body)), " ")
	if len(text) > maxLen {
		return text[:maxLen] + "..."
	}
	return text
}
