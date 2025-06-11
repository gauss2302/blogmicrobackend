// internal/interfaces/validators/user_validator.go
package validators

import (
	"fmt"
	"regexp"
	"strings"

	"user-service/internal/application/dto"
)

type UserValidator struct{}

func NewUserValidator() *UserValidator {
	return &UserValidator{}
}

func (v *UserValidator) ValidateCreateUserRequest(req *dto.CreateUserRequest) error {
	if strings.TrimSpace(req.ID) == "" {
		return fmt.Errorf("user ID is required")
	}
	
	if strings.TrimSpace(req.Email) == "" {
		return fmt.Errorf("email is required")
	}
	
	if !isValidEmail(req.Email) {
		return fmt.Errorf("invalid email format")
	}
	
	if strings.TrimSpace(req.Name) == "" {
		return fmt.Errorf("name is required")
	}
	
	if len(req.Name) > 100 {
		return fmt.Errorf("name must be less than 100 characters")
	}
	
	return nil
}

func (v *UserValidator) ValidateUpdateUserRequest(req *dto.UpdateUserRequest) error {
	if req.Name != nil {
		if strings.TrimSpace(*req.Name) == "" {
			return fmt.Errorf("name cannot be empty")
		}
		if len(*req.Name) > 100 {
			return fmt.Errorf("name must be less than 100 characters")
		}
	}
	
	if req.Bio != nil && len(*req.Bio) > 500 {
		return fmt.Errorf("bio must be less than 500 characters")
	}
	
	if req.Location != nil && len(*req.Location) > 100 {
		return fmt.Errorf("location must be less than 100 characters")
	}
	
	if req.Website != nil && *req.Website != "" {
		if !isValidURL(*req.Website) {
			return fmt.Errorf("invalid website URL")
		}
	}
	
	return nil
}

func (v *UserValidator) ValidateSearchUsersRequest(req *dto.SearchUsersRequest) error {
	if strings.TrimSpace(req.Query) == "" {
		return fmt.Errorf("search query is required")
	}
	
	if len(req.Query) < 2 {
		return fmt.Errorf("search query must be at least 2 characters")
	}
	
	return nil
}

func isValidEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

func isValidURL(url string) bool {
	urlRegex := regexp.MustCompile(`^https?://[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}(/.*)?$`)
	return urlRegex.MatchString(url)
}