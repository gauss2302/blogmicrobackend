package oauth

import (
	"auth-service/internal/config"
	"auth-service/internal/domain/entities"
	domainServices "auth-service/internal/domain/services"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

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

// GetAuthURL builds the Google consent URL. The client's PKCE challenge stays
// in our OAuth state and is checked on /auth/exchange. Google never sees it:
// the backend has no verifier to send on its own code exchange, so Google
// would answer invalid_grant.
func (g *GoogleProvider) GetAuthURL(req *domainServices.AuthURLRequest) string {
	if req == nil {
		req = &domainServices.AuthURLRequest{}
	}

	opts := []oauth2.AuthCodeOption{
		oauth2.SetAuthURLParam("include_granted_scopes", "true"),
		oauth2.SetAuthURLParam("prompt", "select_account"),
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

// userInfoTimeout bounds each userinfo request; the default HTTP client has none.
const userInfoTimeout = 10 * time.Second

func (g *GoogleProvider) GetUserInfo(ctx context.Context, accessToken string) (*entities.GoogleUserInfo, error) {
	client := g.config.Client(ctx, &oauth2.Token{AccessToken: accessToken})

	// Primary endpoint for OIDC profile claims.
	userInfo, err := fetchUserInfo(ctx, client, "https://openidconnect.googleapis.com/v1/userinfo")
	if err == nil && userInfo.IsValid() {
		return userInfo, nil
	}

	// Fallback endpoint used by older Google OAuth flows.
	legacyUserInfo, legacyErr := fetchUserInfo(ctx, client, "https://www.googleapis.com/oauth2/v2/userinfo")
	if legacyErr == nil && legacyUserInfo.IsValid() {
		return legacyUserInfo, nil
	}

	// Response bodies carry the user's profile; keep them out of the error,
	// which ends up in logs.
	return nil, fmt.Errorf("invalid user info received from Google: missing required fields (primary_err=%v legacy_err=%v)", err, legacyErr)
}

func fetchUserInfo(ctx context.Context, client *http.Client, endpoint string) (*entities.GoogleUserInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, userInfoTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build %s request: %w", endpoint, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request %s failed: %w", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request %s failed with status %d", endpoint, resp.StatusCode)
	}

	var userInfo entities.GoogleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("parse %s failed: %w", endpoint, err)
	}
	return &userInfo, nil
}
