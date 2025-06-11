package dto

// Existing DTOs (keeping as they were)
type GoogleLoginRequest struct {
	Code  string `json:"code" binding:"required"`  
	State string `json:"state,omitempty"`          
}

type GoogleAuthURLResponse struct {
	AuthURL string `json:"auth_url"`
	State   string `json:"state"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type LogoutRequest struct {
	AccessToken string `json:"access_token" binding:"required"`
}

type AuthResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int       `json:"expires_in"`
	User         *UserInfo `json:"user"`
}

type UserInfo struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name,omitempty"`
	Picture string `json:"picture,omitempty"`
}

type TokenValidationResponse struct {
	Valid  bool   `json:"valid"`
	UserID string `json:"user_id,omitempty"`
	Email  string `json:"email,omitempty"`
}

// Adding the missing DTOs that I referenced in the service
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

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

// Removing the duplicated ones I mistakenly added
type GoogleAuthURLRequest struct {
	// Optional: could add state or other parameters if needed
}