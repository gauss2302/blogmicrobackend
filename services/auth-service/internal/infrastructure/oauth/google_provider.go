package oauth

import (
	"auth-service/internal/config"
	"auth-service/internal/domain/entities"
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"io"
	"net/http"
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

func (g *GoogleProvider) GetAuthURL(state string) string {
	return g.config.AuthCodeURL(state,
		oauth2.AccessTypeOffline, // Gets refresh token
		oauth2.ApprovalForce,     // Force consent screen
	)
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

	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("user info request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read user info response: %w", err)
	}

	var userInfo entities.GoogleUserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, fmt.Errorf("failed to parse user info: %w", err)
	}

	if !userInfo.IsValid() {
		return nil, fmt.Errorf("invalid user info received from Google: missing required fields")
	}

	return &userInfo, nil
}
