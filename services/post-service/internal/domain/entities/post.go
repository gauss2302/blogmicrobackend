package entities

import (
	"fmt"
	"strings"
	"time"
)

type Post struct {
	ID        string    `json:"id" db:"id"`
	UserID    string    `json:"user_id" db:"user_id"`
	Title     string    `json:"title" db:"title"`
	Content   string    `json:"content" db:"content"`
	Slug      string    `json:"slug" db:"slug"`
	Published bool      `json:"published" db:"published"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type PostSummary struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Title     string    `json:"title"`
	Slug      string    `json:"slug"`
	Published bool      `json:"published"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (p *Post) ToSummary() *PostSummary {
	return &PostSummary{
		ID:        p.ID,
		UserID:    p.UserID,
		Title:     p.Title,
		Slug:      p.Slug,
		Published: p.Published,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
}

func (p *Post) IsValid() error {
	if strings.TrimSpace(p.ID) == "" {
		return fmt.Errorf("post ID is required")
	}

	if strings.TrimSpace(p.UserID) == "" {
		return fmt.Errorf("user ID is required")
	}

	if strings.TrimSpace(p.Title) == "" {
		return fmt.Errorf("title is required")
	}

	if len(p.Title) > 200 {
		return fmt.Errorf("title must be less than 200 characters")
	}

	if strings.TrimSpace(p.Content) == "" {
		return fmt.Errorf("content is required")
	}

	if len(p.Content) > 50000 {
		return fmt.Errorf("content must be less than 50,000 characters")
	}

	if strings.TrimSpace(p.Slug) == "" {
		return fmt.Errorf("slug is required")
	}

	if !isValidSlug(p.Slug) {
		return fmt.Errorf("invalid slug format")
	}

	return nil
}

func (p *Post) Sanitize() {
	p.Title = strings.TrimSpace(p.Title)
	p.Content = strings.TrimSpace(p.Content)
	p.Slug = strings.ToLower(strings.TrimSpace(p.Slug))
}

func (p *Post) GenerateSlug() {
	if p.Slug == "" {
		p.Slug = slugify(p.Title)
	}
}


func isValidSlug(slug string) bool {
	if len(slug) < 3 || len(slug) > 100 {
		return false
	}

	for _, char := range slug {
		if !((char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-') {
			return false
		}
	}

	return !strings.HasPrefix(slug, "-") && !strings.HasSuffix(slug, "-")
}

func slugify(text string) string {
	text = strings.ToLower(text)
	text = strings.ReplaceAll(text, " ", "-")
	
	var result strings.Builder
	for _, char := range text {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-' {
			result.WriteRune(char)
		}
	}
	
	slug := result.String()
	slug = strings.Trim(slug, "-")
	
	// Remove consecutive dashes
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	
	if len(slug) > 100 {
		slug = slug[:100]
		slug = strings.TrimSuffix(slug, "-")
	}
	
	if len(slug) < 3 {
		slug = "post"
	}
	
	return slug
}