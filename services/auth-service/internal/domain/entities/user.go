package entities

import "strings"

type GoogleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	VerifiedEmail bool   `json:"verified_email"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Locale        string `json:"locale"`
	Sub           string `json:"sub,omitempty"`
	EmailVerified bool   `json:"email_verified,omitempty"`
}

func (u *GoogleUserInfo) Normalize() {
	if u == nil {
		return
	}

	if u.ID == "" {
		u.ID = strings.TrimSpace(u.Sub)
	}
	if !u.VerifiedEmail {
		u.VerifiedEmail = u.EmailVerified
	}
	u.Email = strings.TrimSpace(u.Email)
}

func (u *GoogleUserInfo) IsValid() bool {
	u.Normalize()
	return u.ID != "" && u.Email != "" && u.VerifiedEmail
}