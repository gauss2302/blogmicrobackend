package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"testing"

	searchv1 "github.com/nikitashilov/microblog_grpc/proto/search/v1"
	"search-service/pkg/logger"
)

func TestEncodeDecodeCursor(t *testing.T) {
	tests := []struct {
		name   string
		from   int
		expect int // decode(encode(from)) for from > 0; decode("") = 0
	}{
		{"zero", 0, 0},
		{"one", 1, 1},
		{"twenty", 20, 20},
		{"large", 10000, 10000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := encodeCursor(tt.from)
			if tt.from <= 0 {
				if c != "" {
					t.Errorf("encodeCursor(%d) want \"\", got %q", tt.from, c)
				}
				return
			}
			got := decodeCursor(c)
			if got != tt.expect {
				t.Errorf("decodeCursor(encodeCursor(%d)) = %d, want %d", tt.from, got, tt.expect)
			}
		})
	}
}

func TestDecodeCursorInvalid(t *testing.T) {
	if got := decodeCursor(""); got != 0 {
		t.Errorf("decodeCursor(\"\") = %d, want 0", got)
	}
	if got := decodeCursor("not-base64!!"); got != 0 {
		t.Errorf("decodeCursor(\"not-base64!!\") = %d, want 0", got)
	}
	// Negative offset encoded as base64; decoder should return 0
	negEnc := base64.StdEncoding.EncodeToString([]byte(strconv.Itoa(-5)))
	if got := decodeCursor(negEnc); got != 0 {
		t.Errorf("decodeCursor(negative) = %d, want 0", got)
	}
}

func TestBuildUserSearchBody(t *testing.T) {
	body := buildUserSearchBody("alice", 10, 0)
	var m map[string]interface{}
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("buildUserSearchBody produced invalid JSON: %v", err)
	}
	if _, ok := m["query"]; !ok {
		t.Error("buildUserSearchBody: missing query")
	}
	if size, ok := m["size"].(float64); !ok || int(size) != 10 {
		t.Errorf("buildUserSearchBody: size want 10, got %v", m["size"])
	}
	if from, ok := m["from"].(float64); !ok || int(from) != 0 {
		t.Errorf("buildUserSearchBody: from want 0, got %v", m["from"])
	}
}

func TestBuildPostSearchBody(t *testing.T) {
	body := buildPostSearchBody("hello", 5, 20)
	var m map[string]interface{}
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("buildPostSearchBody produced invalid JSON: %v", err)
	}
	if _, ok := m["query"]; !ok {
		t.Error("buildPostSearchBody: missing query")
	}
	if size, ok := m["size"].(float64); !ok || int(size) != 5 {
		t.Errorf("buildPostSearchBody: size want 5, got %v", m["size"])
	}
	if from, ok := m["from"].(float64); !ok || int(from) != 20 {
		t.Errorf("buildPostSearchBody: from want 20, got %v", m["from"])
	}
}

func TestDemoteFollowedUsers(t *testing.T) {
	users := []searchv1.SearchUserHit{
		{Id: "a", Name: "A"},
		{Id: "b", Name: "B"},
		{Id: "c", Name: "C"},
	}
	followed := map[string]bool{"b": true}
	out := demoteFollowedUsers(users, followed)
	if len(out) != 3 {
		t.Fatalf("len(out) = %d, want 3", len(out))
	}
	if out[0].Id != "a" || out[1].Id != "c" || out[2].Id != "b" {
		t.Errorf("demote order: got %v, want [a, c, b] (followed last)", []string{out[0].Id, out[1].Id, out[2].Id})
	}
}

func TestDemoteFollowedUsersEmpty(t *testing.T) {
	out := demoteFollowedUsers(nil, nil)
	if out != nil {
		t.Errorf("demoteFollowedUsers(nil, nil) = %v, want nil", out)
	}
	out = demoteFollowedUsers([]searchv1.SearchUserHit{}, map[string]bool{})
	if len(out) != 0 {
		t.Errorf("demoteFollowedUsers(empty, empty) len = %d", len(out))
	}
}

// TestSearch_OpenSearchUnavailable_PartialResult verifies that when OpenSearch is nil
// (unavailable), Search returns partial branches and no top-level error.
func TestSearch_OpenSearchUnavailable_PartialResult(t *testing.T) {
	s := &SearchService{
		os:         nil,
		usersIndex:  "users",
		postsIndex:  "posts",
		userConn:   nil,
		userClient: nil,
		log:        logger.New("info"),
	}
	ctx := context.Background()
	req := &searchv1.SearchRequest{Query: "test"}
	resp, err := s.Search(ctx, req)
	if err != nil {
		t.Fatalf("Search with nil OpenSearch should not return error, got %v", err)
	}
	if resp == nil {
		t.Fatal("Search returned nil response")
	}
	if !resp.UsersPartial {
		t.Error("expected UsersPartial true when OpenSearch unavailable")
	}
	if !resp.PostsPartial {
		t.Error("expected PostsPartial true when OpenSearch unavailable")
	}
	if len(resp.GetUsers()) != 0 {
		t.Errorf("expected no users when OpenSearch nil, got %d", len(resp.GetUsers()))
	}
	if len(resp.GetPosts()) != 0 {
		t.Errorf("expected no posts when OpenSearch nil, got %d", len(resp.GetPosts()))
	}
}
