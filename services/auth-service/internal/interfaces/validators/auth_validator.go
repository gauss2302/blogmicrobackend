package validators

import (
	"auth-service/internal/application/services/dto"
	"fmt"
	"strings"
)

type AuthValidator struct{}

func NewAuthValidator() *AuthValidator {
	return &AuthValidator{}
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
	
	return nil
}