package validators

import (
	"fmt"
	"regexp"
	"strings"

	"post-service/internal/application/dto"
)

type PostValidator struct{}

func NewPostValidator() *PostValidator {
	return &PostValidator{}
}

func (v *PostValidator) ValidateCreatePostRequest(req *dto.CreatePostRequest) error {
	if strings.TrimSpace(req.Title) == "" {
		return fmt.Errorf("title is required")
	}

	if len(req.Title) > 200 {
		return fmt.Errorf("title must be less than 200 characters")
	}

	if strings.TrimSpace(req.Content) == "" {
		return fmt.Errorf("content is required")
	}

	if len(req.Content) > 50000 {
		return fmt.Errorf("content must be less than 50,000 characters")
	}

	if req.Slug != "" {
		if err := v.validateSlug(req.Slug); err != nil {
			return err
		}
	}

	return nil
}

func (v *PostValidator) ValidateUpdatePostRequest(req *dto.UpdatePostRequest) error {
	if req.Title != nil {
		if strings.TrimSpace(*req.Title) == "" {
			return fmt.Errorf("title cannot be empty")
		}
		if len(*req.Title) > 200 {
			return fmt.Errorf("title must be less than 200 characters")
		}
	}

	if req.Content != nil {
		if strings.TrimSpace(*req.Content) == "" {
			return fmt.Errorf("content cannot be empty")
		}
		if len(*req.Content) > 50000 {
			return fmt.Errorf("content must be less than 50,000 characters")
		}
	}

	if req.Slug != nil {
		if err := v.validateSlug(*req.Slug); err != nil {
			return err
		}
	}

	return nil
}

func (v *PostValidator) ValidateSearchPostsRequest(req *dto.SearchPostsRequest) error {
	if strings.TrimSpace(req.Query) == "" {
		return fmt.Errorf("search query is required")
	}

	if len(req.Query) < 2 {
		return fmt.Errorf("search query must be at least 2 characters")
	}

	if len(req.Query) > 100 {
		return fmt.Errorf("search query must be less than 100 characters")
	}

	return nil
}

func (v *PostValidator) validateSlug(slug string) error {
	if len(slug) < 3 {
		return fmt.Errorf("slug must be at least 3 characters")
	}

	if len(slug) > 100 {
		return fmt.Errorf("slug must be less than 100 characters")
	}

	// Check slug format: lowercase letters, numbers, and hyphens only
	slugRegex := regexp.MustCompile(`^[a-z0-9-]+$`)
	if !slugRegex.MatchString(slug) {
		return fmt.Errorf("slug can only contain lowercase letters, numbers, and hyphens")
	}

	// Check that slug doesn't start or end with hyphen
	if strings.HasPrefix(slug, "-") || strings.HasSuffix(slug, "-") {
		return fmt.Errorf("slug cannot start or end with a hyphen")
	}

	// Check for consecutive hyphens
	if strings.Contains(slug, "--") {
		return fmt.Errorf("slug cannot contain consecutive hyphens")
	}

	return nil
}