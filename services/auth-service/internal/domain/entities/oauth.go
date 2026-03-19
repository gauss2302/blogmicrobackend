package entities

type OAuthPlatform string

const (
	OAuthPlatformWeb    OAuthPlatform = "web"
	OAuthPlatformMobile OAuthPlatform = "mobile"
)

type OAuthState struct {
	State               string        `json:"state"`
	Platform            OAuthPlatform `json:"platform"`
	ClientRedirectURI   string        `json:"client_redirect_uri"`
	ClientState         string        `json:"client_state,omitempty"`
	CodeChallenge       string        `json:"code_challenge,omitempty"`
	CodeChallengeMethod string        `json:"code_challenge_method,omitempty"`
}

type AuthCodePayload struct {
	User                *GoogleUserInfo `json:"user"`
	Platform            OAuthPlatform   `json:"platform"`
	ClientRedirectURI   string          `json:"client_redirect_uri"`
	ClientState         string          `json:"client_state,omitempty"`
	CodeChallenge       string          `json:"code_challenge,omitempty"`
	CodeChallengeMethod string          `json:"code_challenge_method,omitempty"`
}
