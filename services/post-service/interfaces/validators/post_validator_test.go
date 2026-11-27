package validators

import (
	"strings"
	"testing"

	"post-service/internal/application/dto"
)

// Limits count characters, not UTF-8 bytes.
func TestValidatorCountsCharacters(t *testing.T) {
	v := NewPostValidator()
	ok := &dto.CreatePostRequest{Title: strings.Repeat("я", 200), Content: "body"}
	if err := v.ValidateCreatePostRequest(ok); err != nil {
		t.Errorf("200 Cyrillic characters: %v", err)
	}
	long := &dto.CreatePostRequest{Title: strings.Repeat("я", 201), Content: "body"}
	if err := v.ValidateCreatePostRequest(long); err == nil {
		t.Error("201 characters accepted")
	}
	if err := v.ValidateSearchPostsRequest(&dto.SearchPostsRequest{Query: strings.Repeat("я", 100)}); err != nil {
		t.Errorf("100-character Cyrillic query: %v", err) // 200 bytes
	}
	if err := v.ValidateSearchPostsRequest(&dto.SearchPostsRequest{Query: strings.Repeat("я", 101)}); err == nil {
		t.Error("101-character query accepted")
	}
}
