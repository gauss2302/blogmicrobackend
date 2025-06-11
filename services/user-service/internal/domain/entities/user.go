package entities

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

type User struct {
	ID        string    `json:"id" db:"id"`
	Email     string    `json:"email" db:"email"`
	Name      string    `json:"name" db:"name"`
	Picture   string    `json:"picture,omitempty" db:"picture"`
	Bio       string    `json:"bio,omitempty" db:"bio"`
	Location  string    `json:"location,omitempty" db:"location"`
	Website   string    `json:"website,omitempty" db:"website"`
	IsActive  bool      `json:"is_active" db:"is_active"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type UserProfile struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Picture  string `json:"picture,omitempty"`
	Bio      string `json:"bio,omitempty"`
	Location string `json:"location,omitempty"`
	Website  string `json:"website,omitempty"`
}

func (u *User) ToProfile() *UserProfile {
	return &UserProfile{
		ID:       u.ID,
		Email:    u.Email,
		Name:     u.Name,
		Picture:  u.Picture,
		Bio:      u.Bio,
		Location: u.Location,
		Website:  u.Website,
	}
}

func (u *User) IsValid() error {
	if strings.TrimSpace(u.ID) == "" {
		return fmt.Errorf("user ID is required")
	}
	
	if strings.TrimSpace(u.Email) == "" {
		return fmt.Errorf("email is required")
	}
	
	if !isValidEmail(u.Email) {
		return fmt.Errorf("invalid email format")
	}
	
	if strings.TrimSpace(u.Name) == "" {
		return fmt.Errorf("name is required")
	}
	
	if len(u.Name) > 100 {
		return fmt.Errorf("name must be less than 100 characters")
	}
	
	if len(u.Bio) > 500 {
		return fmt.Errorf("bio must be less than 500 characters")
	}
	
	if u.Website != "" && !isValidURL(u.Website) {
		return fmt.Errorf("invalid website URL")
	}
	
	return nil
}

func (u *User) Sanitize() {
	u.Email = strings.ToLower(strings.TrimSpace(u.Email))
	u.Name = strings.TrimSpace(u.Name)
	u.Bio = strings.TrimSpace(u.Bio)
	u.Location = strings.TrimSpace(u.Location)
	u.Website = strings.TrimSpace(u.Website)
}

func isValidEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

func isValidURL(url string) bool {
	urlRegex := regexp.MustCompile(`^https?://[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}(/.*)?$`)
	return urlRegex.MatchString(url)
}