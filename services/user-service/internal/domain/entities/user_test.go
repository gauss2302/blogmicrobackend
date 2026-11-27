package entities

import (
	"strings"
	"testing"
)

func validUser() User {
	return User{ID: "u1", Email: "a@example.com", Name: "Alice", Website: "https://example.com"}
}

// A-80: limits are the VARCHAR sizes, counted in characters; NUL and invalid
// UTF-8 are rejected in every text field.
func TestIsValidTextFields(t *testing.T) {
	fields := []struct {
		name string
		max  int
		set  func(*User, string)
		fill func(n int) string // a valid value of n characters
	}{
		{"id", 255, func(u *User, v string) { u.ID = v }, func(n int) string { return strings.Repeat("я", n) }},
		{"email", 255, func(u *User, v string) { u.Email = v }, func(n int) string { return strings.Repeat("a", n-len("@example.com")) + "@example.com" }},
		{"name", 100, func(u *User, v string) { u.Name = v }, func(n int) string { return strings.Repeat("я", n) }},
		{"picture", 500, func(u *User, v string) { u.Picture = v }, func(n int) string { return strings.Repeat("я", n) }},
		{"bio", 500, func(u *User, v string) { u.Bio = v }, func(n int) string { return strings.Repeat("я", n) }},
		{"location", 100, func(u *User, v string) { u.Location = v }, func(n int) string { return strings.Repeat("я", n) }},
		{"website", 255, func(u *User, v string) { u.Website = v }, func(n int) string { return "https://e.com/" + strings.Repeat("x", n-len("https://e.com/")) }},
	}
	for _, f := range fields {
		t.Run(f.name, func(t *testing.T) {
			u := validUser()
			f.set(&u, f.fill(f.max))
			if err := u.IsValid(); err != nil {
				t.Errorf("%d characters: %v", f.max, err)
			}

			u = validUser()
			f.set(&u, f.fill(f.max+1))
			if err := u.IsValid(); err == nil || !strings.Contains(err.Error(), "at most") {
				t.Errorf("%d characters: err = %v", f.max+1, err)
			}

			for _, bad := range []string{"\x00", "\xff"} {
				u = validUser()
				f.set(&u, bad+f.fill(20))
				// The message tells this check apart from the email and URL formats.
				if err := u.IsValid(); err == nil || !strings.Contains(err.Error(), "NUL or invalid UTF-8") {
					t.Errorf("%q: err = %v", bad, err)
				}
			}
		})
	}
}

func TestIsValidCyrillicName(t *testing.T) {
	u := validUser()
	u.Name = strings.Repeat("ж", 60) // 120 bytes
	if err := u.IsValid(); err != nil {
		t.Fatalf("60 Cyrillic characters: %v", err)
	}
}
