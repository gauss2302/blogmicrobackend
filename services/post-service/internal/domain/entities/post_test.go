package entities

import (
	"strings"
	"testing"
)

// Limits count characters, like the VARCHAR(200) column, not UTF-8 bytes.
func TestIsValidCountsCharacters(t *testing.T) {
	valid := func(title string) error {
		p := Post{ID: "id", UserID: "u", Title: title, Content: "c", Slug: "a-slug"}
		return p.IsValid()
	}
	if err := valid(strings.Repeat("я", 200)); err != nil { // 400 bytes
		t.Errorf("200 Cyrillic characters: %v", err)
	}
	if err := valid(strings.Repeat("я", 201)); err == nil {
		t.Error("201 characters accepted")
	}
}
