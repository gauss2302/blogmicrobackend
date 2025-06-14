package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"auth-service/internal/config"
	"auth-service/internal/domain/entities"
)

type GoogleProvider struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	httpClient   *http.Client
}

type GoogleTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	IDToken      string `json:"id_token"`
	Scope        string `json:"scope"`
}

func NewGoogleProvider(cfg config.GoogleConfig) *GoogleProvider {
	return &GoogleProvider{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (g *GoogleProvider) GetAuthURL(state string) string {
	baseURL := "https://accounts.google.com/o/oauth2/auth"
	params := url.Values{}
	params.Add("client_id", g.ClientID)
	params.Add("redirect_uri", g.RedirectURL)
	params.Add("scope", "openid email profile")
	params.Add("response_type", "code")
	params.Add("state", state)
	params.Add("access_type", "offline")
	params.Add("prompt", "consent") // Ensures refresh token is returned
	
	return fmt.Sprintf("%s?%s", baseURL, params.Encode())
}

func (g *GoogleProvider) ExchangeCodeForToken(ctx context.Context, code string) (*entities.GoogleUserInfo, error) {
	// Step 1: Exchange authorization code for access token
	tokenResp, err := g.exchangeCodeForAccessToken(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code for token: %w", err)
	}

	// Step 2: Get user info using access token
	userInfo, err := g.GetUserInfo(ctx, tokenResp.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}

	return userInfo, nil
}

func (g *GoogleProvider) exchangeCodeForAccessToken(ctx context.Context, code string) (*GoogleTokenResponse, error) {
	tokenURL := "https://oauth2.googleapis.com/token"
	
	data := url.Values{}
	data.Set("client_id", g.ClientID)
	data.Set("client_secret", g.ClientSecret)
	data.Set("code", code)
	data.Set("grant_type", "authorization_code")
	data.Set("redirect_uri", g.RedirectURL)

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Parse error response for better debugging
		var errorResp struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		if parseErr := json.Unmarshal(body, &errorResp); parseErr == nil {
			return nil, fmt.Errorf("token exchange failed (status %d): %s - %s", 
				resp.StatusCode, errorResp.Error, errorResp.ErrorDescription)
		}
		return nil, fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResponse GoogleTokenResponse
	if err := json.Unmarshal(body, &tokenResponse); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	if tokenResponse.AccessToken == "" {
		return nil, fmt.Errorf("no access token received from Google")
	}

	return &tokenResponse, nil
}

func (g *GoogleProvider) GetUserInfo(ctx context.Context, accessToken string) (*entities.GoogleUserInfo, error) {
	userInfoURL := "https://www.googleapis.com/oauth2/v2/userinfo"
	
	req, err := http.NewRequestWithContext(ctx, "GET", userInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create user info request: %w", err)
	}
	
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read user info response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("user info request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var userInfo entities.GoogleUserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, fmt.Errorf("failed to parse user info: %w", err)
	}

	// Validate essential fields
	if !userInfo.IsValid() {
		return nil, fmt.Errorf("invalid user info received from Google: missing required fields")
	}

	return &userInfo, nil
}