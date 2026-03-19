package validators

import (
	"auth-service/internal/application/services/dto"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

type AuthValidator struct{}

var pkceAllowedPattern = regexp.MustCompile(`^[A-Za-z0-9\-._~]+$`)

func NewAuthValidator() *AuthValidator {
	return &AuthValidator{}
}

func (v *AuthValidator) ValidateGoogleAuthURLRequest(req *dto.GoogleAuthURLRequest) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}

	platform := strings.ToLower(strings.TrimSpace(string(req.Platform)))
	if platform != "" && platform != string(dto.OAuthPlatformWeb) && platform != string(dto.OAuthPlatformMobile) {
		return fmt.Errorf("platform must be web or mobile")
	}

	if redirectURI := strings.TrimSpace(req.ClientRedirectURI); redirectURI != "" {
		if _, err := url.ParseRequestURI(redirectURI); err != nil {
			return fmt.Errorf("redirect_uri must be a valid absolute URI")
		}
	}

	codeChallenge := strings.TrimSpace(req.CodeChallenge)
	if codeChallenge != "" {
		if len(codeChallenge) < 43 || len(codeChallenge) > 128 || !pkceAllowedPattern.MatchString(codeChallenge) {
			return fmt.Errorf("code_challenge is invalid")
		}
	}

	method := strings.ToUpper(strings.TrimSpace(req.CodeChallengeMethod))
	if method != "" && method != "S256" && method != "PLAIN" {
		return fmt.Errorf("code_challenge_method must be S256 or plain")
	}
	if method != "" && codeChallenge == "" {
		return fmt.Errorf("code_challenge is required when code_challenge_method is provided")
	}

	return nil
}

func (v *AuthValidator) ValidateRefreshTokenRequest(req *dto.RefreshTokenRequest) error {
	if strings.TrimSpace(req.RefreshToken) == "" {
		return fmt.Errorf("refresh token is required")
	}

	if len(req.RefreshToken) < 20 {
		return fmt.Errorf("refresh token appears to be invalid")
	}

	return nil
}

func (v *AuthValidator) ValidateLogoutRequest(req *dto.LogoutRequest) error {
	if strings.TrimSpace(req.AccessToken) == "" {
		return fmt.Errorf("access token is required")
	}

	if len(req.AccessToken) < 20 {
		return fmt.Errorf("access token appears to be invalid")
	}

	return nil
}

func (v *AuthValidator) ValidateGoogleCallbackRequest(req *dto.GoogleCallbackRequest) error {
	if strings.TrimSpace(req.State) == "" {
		return fmt.Errorf("state parameter is required")
	}

	if strings.TrimSpace(req.Code) == "" {
		return fmt.Errorf("code parameter is required")
	}

	if len(req.Code) < 10 {
		return fmt.Errorf("authorization code appears to be invalid")
	}

	return nil
}

func (v *AuthValidator) ValidateExchangeAuthCodeRequest(req *dto.ExchangeAuthCodeRequest) error {
	if strings.TrimSpace(req.AuthCode) == "" {
		return fmt.Errorf("auth code is required")
	}

	if len(req.AuthCode) < 10 {
		return fmt.Errorf("auth code appears to be invalid")
	}

	if verifier := strings.TrimSpace(req.CodeVerifier); verifier != "" {
		if len(verifier) < 43 || len(verifier) > 128 || !pkceAllowedPattern.MatchString(verifier) {
			return fmt.Errorf("code verifier appears to be invalid")
		}
	}

	return nil
}
