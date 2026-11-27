package services

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"post-service/internal/application/dto"
	appErrors "post-service/internal/application/errors"
	"post-service/internal/domain/entities"
	"post-service/internal/domain/repositories"
	"post-service/pkg/logger"
)

// fakeRepo keeps posts in memory and enforces slug uniqueness like the
// posts_slug_key index. Methods the tests don't use panic via the nil embed.
type fakeRepo struct {
	repositories.PostRepository
	posts           map[string]*entities.Post // by ID
	createCalls     int
	failNextCreates int // Create reports ErrSlugTaken this many times first
}

func newFakeRepo() *fakeRepo { return &fakeRepo{posts: map[string]*entities.Post{}} }

func (r *fakeRepo) slugTaken(post *entities.Post) bool {
	for _, p := range r.posts {
		if p.Slug == post.Slug && p.ID != post.ID {
			return true
		}
	}
	return false
}

func (r *fakeRepo) Create(_ context.Context, post *entities.Post) error {
	r.createCalls++
	if r.failNextCreates > 0 {
		r.failNextCreates--
		return fmt.Errorf("%w: %s", repositories.ErrSlugTaken, post.Slug)
	}
	if r.slugTaken(post) {
		return fmt.Errorf("%w: %s", repositories.ErrSlugTaken, post.Slug)
	}
	cp := *post
	r.posts[post.ID] = &cp
	return nil
}

func (r *fakeRepo) Update(_ context.Context, post *entities.Post) error {
	if r.slugTaken(post) {
		return fmt.Errorf("%w: %s", repositories.ErrSlugTaken, post.Slug)
	}
	cp := *post
	r.posts[post.ID] = &cp
	return nil
}

func (r *fakeRepo) GetByID(_ context.Context, id string) (*entities.Post, error) {
	p, ok := r.posts[id]
	if !ok {
		return nil, fmt.Errorf("post not found")
	}
	cp := *p
	return &cp, nil
}

func (r *fakeRepo) GetByUserID(_ context.Context, userID string, _, _ int, includeDrafts bool) ([]*entities.Post, error) {
	var out []*entities.Post
	for _, p := range r.posts {
		if p.UserID == userID && (p.Published || includeDrafts) {
			cp := *p
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *fakeRepo) UnpublishByUserID(_ context.Context, userID string) (int64, error) {
	var n int64
	for _, p := range r.posts {
		if p.UserID == userID && p.Published {
			p.Published = false
			n++
		}
	}
	return n, nil
}

func newTestService(repo *fakeRepo) *PostService {
	return NewPostService(repo, nil, logger.New("error"))
}

func create(t *testing.T, s *PostService, title, slug string) *dto.PostResponse {
	t.Helper()
	resp, err := s.CreatePost(context.Background(), &dto.CreatePostRequest{Title: title, Content: "body", Slug: slug}, "user-1")
	if err != nil {
		t.Fatalf("CreatePost(%q, slug %q) error: %v", title, slug, err)
	}
	return resp
}

func TestCreatePost_UniqueLatinTitleKeepsPlainSlug(t *testing.T) {
	s := newTestService(newFakeRepo())
	if got := create(t, s, "Hello World", "").Slug; got != "hello-world" {
		t.Errorf("slug = %q, want %q", got, "hello-world")
	}
}

func TestCreatePost_SameLatinTitleGetsDistinctSlugs(t *testing.T) {
	s := newTestService(newFakeRepo())
	first := create(t, s, "Hello World", "")
	second := create(t, s, "Hello World", "")
	if first.Slug == second.Slug {
		t.Fatalf("both posts got slug %q", first.Slug)
	}
	if !strings.HasPrefix(second.Slug, "hello-world-") {
		t.Errorf("second slug = %q, want hello-world-<suffix>", second.Slug)
	}
}

func TestCreatePost_CyrillicTitleGetsValidDistinctSlugs(t *testing.T) {
	s := newTestService(newFakeRepo())
	first := create(t, s, "Привет мир", "")
	second := create(t, s, "Привет мир", "")
	if first.Slug == second.Slug {
		t.Fatalf("both posts got slug %q", first.Slug)
	}
	for _, slug := range []string{first.Slug, second.Slug} {
		p := entities.Post{ID: "id", UserID: "u", Title: "t", Content: "c", Slug: slug}
		if err := p.IsValid(); err != nil {
			t.Errorf("slug %q is invalid: %v", slug, err)
		}
	}
}

func TestCreatePost_ExplicitTakenSlugIsConflict(t *testing.T) {
	repo := newFakeRepo()
	s := newTestService(repo)
	create(t, s, "First", "my-post")
	_, err := s.CreatePost(context.Background(), &dto.CreatePostRequest{Title: "Second", Content: "body", Slug: "my-post"}, "user-2")
	if err != appErrors.ErrPostAlreadyExists {
		t.Fatalf("err = %v, want ErrPostAlreadyExists", err)
	}
	if repo.createCalls != 2 {
		t.Errorf("Create called %d times, want 2 (no retry for a client slug)", repo.createCalls)
	}
}

func TestCreatePost_RetriesGeneratedSlugAfterUniqueViolation(t *testing.T) {
	repo := newFakeRepo()
	repo.failNextCreates = 1 // e.g. a concurrent create took the slug after validation
	s := newTestService(repo)
	resp := create(t, s, "Hello World", "")
	if repo.createCalls != 2 {
		t.Errorf("Create called %d times, want 2", repo.createCalls)
	}
	if !strings.HasPrefix(resp.Slug, "hello-world-") {
		t.Errorf("slug = %q, want hello-world-<suffix>", resp.Slug)
	}
}

func TestUpdatePost_TakenSlugIsConflict(t *testing.T) {
	s := newTestService(newFakeRepo())
	create(t, s, "First", "first-post")
	second := create(t, s, "Second", "second-post")

	taken := "first-post"
	_, err := s.UpdatePost(context.Background(), second.ID, &dto.UpdatePostRequest{Slug: &taken}, "user-1")
	if err != appErrors.ErrPostAlreadyExists {
		t.Fatalf("err = %v, want ErrPostAlreadyExists", err)
	}

	own := "second-post"
	if _, err := s.UpdatePost(context.Background(), second.ID, &dto.UpdatePostRequest{Slug: &own}, "user-1"); err != nil {
		t.Errorf("keeping own slug: err = %v, want nil", err)
	}
}

func TestGetUserPosts_DraftsOnlyForAuthor(t *testing.T) {
	s := newTestService(newFakeRepo())
	ctx := context.Background()
	if _, err := s.CreatePost(ctx, &dto.CreatePostRequest{Title: "Live", Content: "body", Published: true}, "author"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreatePost(ctx, &dto.CreatePostRequest{Title: "Draft", Content: "body"}, "author"); err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		actor string
		want  int
	}{
		{"author", 2},
		{"someone-else", 1},
		{"", 1}, // anonymous
	} {
		resp, err := s.GetUserPosts(ctx, "author", tt.actor, &dto.UserPostsRequest{Limit: 20})
		if err != nil {
			t.Fatalf("actor %q: %v", tt.actor, err)
		}
		if len(resp.Posts) != tt.want {
			t.Errorf("actor %q sees %d posts, want %d", tt.actor, len(resp.Posts), tt.want)
		}
		for _, p := range resp.Posts {
			if !p.Published && tt.actor != "author" {
				t.Errorf("actor %q sees draft %q", tt.actor, p.Title)
			}
		}
	}
}

func TestUnpublishUserPosts_HidesOnlyTheAuthorsPosts(t *testing.T) {
	repo := newFakeRepo()
	s := newTestService(repo)
	ctx := context.Background()
	var authorIDs []string
	for _, published := range []bool{true, false} {
		resp, err := s.CreatePost(ctx, &dto.CreatePostRequest{Title: "Post", Content: "body", Published: published}, "author")
		if err != nil {
			t.Fatal(err)
		}
		authorIDs = append(authorIDs, resp.ID)
	}
	other, err := s.CreatePost(ctx, &dto.CreatePostRequest{Title: "Other", Content: "body", Published: true}, "other")
	if err != nil {
		t.Fatal(err)
	}

	for attempt := 1; attempt <= 2; attempt++ { // idempotent
		if err := s.UnpublishUserPosts(ctx, "author"); err != nil {
			t.Fatalf("attempt %d: %v", attempt, err)
		}
	}
	for _, id := range authorIDs {
		if repo.posts[id].Published {
			t.Errorf("post %s still published", id)
		}
	}
	if !repo.posts[other.ID].Published {
		t.Error("another user's post was unpublished")
	}
}
