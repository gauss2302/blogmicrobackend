package services

import (
	"context"
	stderrors "errors"
	"fmt"
	"post-service/internal/infrastructure/messaging"

	"post-service/internal/application/dto"
	"post-service/internal/application/errors"
	"post-service/internal/domain/entities"
	"post-service/internal/domain/repositories"
	"post-service/pkg/logger"

	"github.com/google/uuid"
)

// maxSlugAttempts bounds how many generated slugs CreatePost tries before giving up.
const maxSlugAttempts = 5

// PostService has no search indexer: the posts table's triggers queue every
// change for search (search.Relay sends it), so writes never wait for Kafka.
type PostService struct {
	postRepo       repositories.PostRepository
	eventPublisher *messaging.EventPublisher
	logger         *logger.Logger
}

func NewPostService(postRepo repositories.PostRepository, eventPublisher *messaging.EventPublisher, logger *logger.Logger) *PostService {
	return &PostService{
		postRepo:       postRepo,
		eventPublisher: eventPublisher,
		logger:         logger,
	}
}

func (s *PostService) CreatePost(ctx context.Context, req *dto.CreatePostRequest, userID string) (*dto.PostResponse, error) {
	s.logger.Info(fmt.Sprintf("Creating post for user: %s", userID))

	// Create post entity
	post := &entities.Post{
		ID:        uuid.New().String(),
		UserID:    userID,
		Title:     req.Title,
		Content:   req.Content,
		Slug:      req.Slug,
		Published: req.Published,
	}

	// A client-supplied slug is used as is and a conflict is a 409. A generated
	// one gets a random suffix and another try when it is taken; the unique
	// index is the only check, so concurrent creates cannot race past it.
	autoSlug := post.Slug == ""
	for attempt := 1; ; attempt++ {
		if autoSlug {
			post.GenerateSlug(attempt > 1)
		}
		post.Sanitize()
		if err := post.IsValid(); err != nil {
			s.logger.Warn(fmt.Sprintf("Post validation failed: %v", err))
			return nil, errors.ErrInvalidPostData
		}

		err := s.postRepo.Create(ctx, post)
		if err == nil {
			break
		}
		if !stderrors.Is(err, repositories.ErrSlugTaken) {
			s.logger.Error(fmt.Sprintf("Failed to create post: %v", err))
			return nil, errors.ErrPostCreationFailed
		}
		if !autoSlug || attempt == maxSlugAttempts {
			return nil, errors.ErrPostAlreadyExists
		}
		s.logger.Info(fmt.Sprintf("Slug %s is taken, retrying with a suffix", post.Slug))
	}

	s.logger.Info(fmt.Sprintf("Post created successfully: %s", post.ID))

	if s.eventPublisher != nil {
		event := messaging.PostCreatedEvent{
			PostID:    post.ID,
			UserID:    post.UserID,
			Title:     post.Title,
			Slug:      post.Slug,
			Published: post.Published,
			CreatedAt: post.CreatedAt,
		}

		if err := s.eventPublisher.PublishPostCreated(event); err != nil {
			s.logger.Error(fmt.Sprintf("failed to publish post created event: %v", err))
		} else {
			s.logger.Info(fmt.Sprintf("published post created event for post: %s", post.ID))
		}
	}

	return &dto.PostResponse{
		ID:        post.ID,
		UserID:    post.UserID,
		Title:     post.Title,
		Content:   post.Content,
		Slug:      post.Slug,
		Published: post.Published,
		CreatedAt: post.CreatedAt,
		UpdatedAt: post.UpdatedAt,
	}, nil
}

func (s *PostService) GetPost(ctx context.Context, id string, userID string) (*dto.PostResponse, error) {
	s.logger.Info(fmt.Sprintf("Getting post: %s for user: %s", id, userID))

	post, err := s.postRepo.GetByID(ctx, id)
	if err != nil {
		s.logger.Warn(fmt.Sprintf("Post not found: %s", id))
		return nil, errors.ErrPostNotFound
	}

	// Check if user owns the post or if it's published
	if post.UserID != userID && !post.Published {
		return nil, errors.ErrUnauthorizedAccess
	}

	return &dto.PostResponse{
		ID:        post.ID,
		UserID:    post.UserID,
		Title:     post.Title,
		Content:   post.Content,
		Slug:      post.Slug,
		Published: post.Published,
		CreatedAt: post.CreatedAt,
		UpdatedAt: post.UpdatedAt,
	}, nil
}

func (s *PostService) GetPostBySlug(ctx context.Context, slug string) (*dto.PostResponse, error) {
	s.logger.Info(fmt.Sprintf("Getting post by slug: %s", slug))

	post, err := s.postRepo.GetBySlug(ctx, slug)
	if err != nil {
		s.logger.Warn(fmt.Sprintf("Post not found by slug: %s", slug))
		return nil, errors.ErrPostNotFound
	}

	return &dto.PostResponse{
		ID:        post.ID,
		UserID:    post.UserID,
		Title:     post.Title,
		Content:   post.Content,
		Slug:      post.Slug,
		Published: post.Published,
		CreatedAt: post.CreatedAt,
		UpdatedAt: post.UpdatedAt,
	}, nil
}

func (s *PostService) UpdatePost(ctx context.Context, id string, req *dto.UpdatePostRequest, userID string) (*dto.PostResponse, error) {
	s.logger.Info(fmt.Sprintf("Updating post: %s by user: %s", id, userID))

	// Read, change and write under one row lock: a concurrent update of other
	// fields is not overwritten, and updated_at comes back from the database.
	post, err := s.postRepo.Update(ctx, id, func(post *entities.Post) error {
		if post.UserID != userID {
			return errors.ErrUnauthorizedAccess
		}
		if req.Title != nil {
			post.Title = *req.Title
		}
		if req.Content != nil {
			post.Content = *req.Content
		}
		if req.Slug != nil {
			post.Slug = *req.Slug
		}
		if req.Published != nil {
			post.Published = *req.Published
		}

		post.Sanitize()
		if err := post.IsValid(); err != nil {
			s.logger.Warn(fmt.Sprintf("Post validation failed on update: %v", err))
			return errors.ErrInvalidPostData
		}
		return nil
	})
	if err != nil {
		var postErr *errors.PostError
		switch {
		case stderrors.As(err, &postErr):
			return nil, postErr
		case stderrors.Is(err, repositories.ErrSlugTaken): // the unique index; drafts included
			return nil, errors.ErrPostAlreadyExists
		case stderrors.Is(err, repositories.ErrPostNotFound):
			s.logger.Warn(fmt.Sprintf("Post not found for update: %s", id))
			return nil, errors.ErrPostNotFound
		}
		s.logger.Error(fmt.Sprintf("Failed to update post: %v", err))
		return nil, errors.ErrPostUpdateFailed
	}

	s.logger.Info(fmt.Sprintf("Post updated successfully: %s", post.ID))

	// Publish event after successful update
	if s.eventPublisher != nil {
		event := messaging.PostUpdatedEvent{
			PostID:    post.ID,
			UserID:    post.UserID,
			Title:     post.Title,
			Slug:      post.Slug,
			Published: post.Published,
			UpdatedAt: post.UpdatedAt,
		}

		if err := s.eventPublisher.PublishPostUpdated(event); err != nil {
			s.logger.Error(fmt.Sprintf("Failed to publish post updated event: %v", err))
			// Don't fail the request, just log the error
		} else {
			s.logger.Info(fmt.Sprintf("Published post updated event for post: %s", post.ID))
		}
	}

	return &dto.PostResponse{
		ID:        post.ID,
		UserID:    post.UserID,
		Title:     post.Title,
		Content:   post.Content,
		Slug:      post.Slug,
		Published: post.Published,
		CreatedAt: post.CreatedAt,
		UpdatedAt: post.UpdatedAt,
	}, nil
}

func (s *PostService) DeletePost(ctx context.Context, id string, userID string) error {
	s.logger.Info(fmt.Sprintf("Deleting post: %s by user: %s", id, userID))

	// Get existing post to check ownership and for event data
	post, err := s.postRepo.GetByID(ctx, id)
	if err != nil {
		s.logger.Warn(fmt.Sprintf("Post not found for deletion: %s", id))
		return errors.ErrPostNotFound
	}

	// Check if user owns the post
	if post.UserID != userID {
		return errors.ErrUnauthorizedAccess
	}

	// Store data for event before deletion
	postTitle := post.Title
	postUserID := post.UserID

	if err := s.postRepo.Delete(ctx, id); err != nil {
		s.logger.Error(fmt.Sprintf("Failed to delete post: %v", err))
		return errors.ErrPostDeletionFailed
	}

	s.logger.Info(fmt.Sprintf("Post deleted successfully: %s", id))

	// Publish event after successful deletion
	if s.eventPublisher != nil {
		event := messaging.PostDeletedEvent{
			PostID:    id,
			UserID:    postUserID,
			Title:     postTitle,
			DeletedAt: post.UpdatedAt, // Use updated time as deletion time
		}

		if err := s.eventPublisher.PublishPostDeleted(event); err != nil {
			s.logger.Error(fmt.Sprintf("Failed to publish post deleted event: %v", err))
			// Don't fail the request, just log the error
		} else {
			s.logger.Info(fmt.Sprintf("Published post deleted event for post: %s", id))
		}
	}

	return nil
}

// UnpublishUserPosts hides every post of userID (account deletion): they leave
// public lists, and the same transaction queues their removal from search, so
// success means the removals will reach the index once Kafka is up. Rows stay.
// No RabbitMQ events, so nobody is notified. Idempotent.
func (s *PostService) UnpublishUserPosts(ctx context.Context, userID string) error {
	n, err := s.postRepo.UnpublishByUserID(ctx, userID)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Failed to unpublish posts of user %s: %v", userID, err))
		return errors.ErrPostUpdateFailed
	}
	s.logger.Info(fmt.Sprintf("Unpublished %d posts of user %s", n, userID))
	return nil
}

func (s *PostService) ListPosts(ctx context.Context, req *dto.ListPostsRequest) (*dto.ListPostsResponse, error) {
	// Public listing never exposes drafts, regardless of a caller-supplied
	// published_only flag. Authors read their own drafts via GetUserPosts/GetPost.
	req.PublishedOnly = true
	s.logger.Info(fmt.Sprintf("Listing posts: limit=%d, offset=%d, published_only=%t", req.Limit, req.Offset, req.PublishedOnly))

	posts, err := s.postRepo.List(ctx, req.Limit, req.Offset, req.PublishedOnly)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Failed to list posts: %v", err))
		return nil, errors.ErrPostListFailed
	}

	var postResponses []*dto.PostSummaryResponse
	for _, post := range posts {
		postResponses = append(postResponses, &dto.PostSummaryResponse{
			ID:        post.ID,
			UserID:    post.UserID,
			Title:     post.Title,
			Slug:      post.Slug,
			Published: post.Published,
			CreatedAt: post.CreatedAt,
			UpdatedAt: post.UpdatedAt,
		})
	}

	return &dto.ListPostsResponse{
		Posts:  postResponses,
		Limit:  req.Limit,
		Offset: req.Offset,
		Total:  len(postResponses),
	}, nil
}

// GetUserPosts lists userID's posts. Drafts are included only for the author
// (actorID == userID); everyone else, anonymous included, sees published posts.
func (s *PostService) GetUserPosts(ctx context.Context, userID, actorID string, req *dto.UserPostsRequest) (*dto.ListPostsResponse, error) {
	includeDrafts := actorID != "" && actorID == userID
	s.logger.Info(fmt.Sprintf("Getting posts for user: %s, limit=%d, offset=%d, drafts=%t", userID, req.Limit, req.Offset, includeDrafts))

	posts, err := s.postRepo.GetByUserID(ctx, userID, req.Limit, req.Offset, includeDrafts)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Failed to get user posts: %v", err))
		return nil, errors.ErrPostListFailed
	}

	var postResponses []*dto.PostSummaryResponse
	for _, post := range posts {
		postResponses = append(postResponses, &dto.PostSummaryResponse{
			ID:        post.ID,
			UserID:    post.UserID,
			Title:     post.Title,
			Slug:      post.Slug,
			Published: post.Published,
			CreatedAt: post.CreatedAt,
			UpdatedAt: post.UpdatedAt,
		})
	}

	return &dto.ListPostsResponse{
		Posts:  postResponses,
		Limit:  req.Limit,
		Offset: req.Offset,
		Total:  len(postResponses),
	}, nil
}

func (s *PostService) SearchPosts(ctx context.Context, req *dto.SearchPostsRequest) (*dto.ListPostsResponse, error) {
	// Search never exposes drafts, regardless of the requested published_only.
	req.PublishedOnly = true
	s.logger.Info(fmt.Sprintf("Searching posts: query=%s, limit=%d, offset=%d, published_only=%t", req.Query, req.Limit, req.Offset, req.PublishedOnly))

	posts, err := s.postRepo.Search(ctx, req.Query, req.Limit, req.Offset, req.PublishedOnly)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Failed to search posts: %v", err))
		return nil, errors.ErrPostSearchFailed
	}

	var postResponses []*dto.PostSummaryResponse
	for _, post := range posts {
		postResponses = append(postResponses, &dto.PostSummaryResponse{
			ID:        post.ID,
			UserID:    post.UserID,
			Title:     post.Title,
			Slug:      post.Slug,
			Published: post.Published,
			CreatedAt: post.CreatedAt,
			UpdatedAt: post.UpdatedAt,
		})
	}

	return &dto.ListPostsResponse{
		Posts:  postResponses,
		Limit:  req.Limit,
		Offset: req.Offset,
		Total:  len(postResponses),
	}, nil
}

func (s *PostService) GetStats(ctx context.Context, userID string) (*dto.PostStatsResponse, error) {
	s.logger.Info("Getting post statistics")

	publishedCount, err := s.postRepo.GetPublishedCount(ctx)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Failed to get published posts count: %v", err))
		return nil, errors.ErrPostStatsFailed
	}

	response := &dto.PostStatsResponse{
		TotalPublishedPosts: publishedCount,
	}

	// If userID is provided, get user's post count
	if userID != "" {
		userCount, err := s.postRepo.GetUserPostsCount(ctx, userID)
		if err != nil {
			s.logger.Error(fmt.Sprintf("Failed to get user posts count: %v", err))
			return nil, errors.ErrPostStatsFailed
		}
		response.UserPostsCount = userCount
	}

	return response, nil
}

// GetPostOwner returns the user ID of the post owner
// This is used for authorization checks at the gRPC level
func (s *PostService) GetPostOwner(ctx context.Context, postID string) (string, error) {
	post, err := s.postRepo.GetByID(ctx, postID)
	if err != nil {
		s.logger.Warn(fmt.Sprintf("Post not found: %s", postID))
		return "", errors.ErrPostNotFound
	}

	return post.UserID, nil
}
