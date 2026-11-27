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

// A-80: NUL and invalid UTF-8 in title or content are invalid input.
func TestIsValidRejectsNULAndInvalidUTF8(t *testing.T) {
	for _, bad := range []string{"a\x00b", "a\xffb"} {
		for field, p := range map[string]Post{
			"title":   {ID: "id", UserID: "u", Title: bad, Content: "c", Slug: "a-slug"},
			"content": {ID: "id", UserID: "u", Title: "t", Content: bad, Slug: "a-slug"},
		} {
			if err := p.IsValid(); err == nil {
				t.Errorf("%s %q accepted", field, bad)
			}
		}
	}
}
