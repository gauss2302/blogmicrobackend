package dto

type GoogleAuthURLResponse struct {
	AuthURL string `json:"auth_url"`
	State   string `json:"state"`
}

type GoogleCallbackRequest struct {
	State string `form:"state" binding:"required"`
	Code  string `form:"code" binding:"required"`
}

type GoogleCallbackResponse struct {
	AuthCode string `json:"auth_code"`
}

type ExchangeAuthCodeRequest struct {
	AuthCode string `json:"auth_code" binding:"required"`
}

type ExchangeAuthCodeResponse struct {
	User   *UserInfo    `json:"user"`
	Tokens *TokenPair   `json:"tokens"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// NEW: Make refresh token response consistent with exchange response
type RefreshTokenResponse struct {
	User   *UserInfo    `json:"user"`
	Tokens *TokenPair   `json:"tokens"`
}

type LogoutRequest struct {
	AccessToken string `json:"access_token" binding:"required"`
}

type TokenValidationResponse struct {
	Valid  bool   `json:"valid"`
	UserID string `json:"user_id,omitempty"`
	Email  string `json:"email,omitempty"`
}

type UserInfo struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name,omitempty"`
	Picture string `json:"picture,omitempty"`
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}