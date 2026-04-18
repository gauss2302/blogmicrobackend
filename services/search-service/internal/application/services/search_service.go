package services

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	searchv1 "github.com/nikitashilov/microblog_grpc/proto/search/v1"
	userv1 "github.com/nikitashilov/microblog_grpc/proto/user/v1"
	"search-service/internal/infrastructure/opensearch"
	"search-service/pkg/logger"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

type SearchService struct {
	os         *opensearch.Client
	usersIndex string
	postsIndex string
	userConn   *grpc.ClientConn
	userClient userv1.UserServiceClient
	log        *logger.Logger
}

type GRPCTLSOptions struct {
	Enabled  bool
	CAFile   string
	CertFile string
	KeyFile  string
}

func NewSearchService(os *opensearch.Client, usersIndex, postsIndex string, userServiceAddr string, tlsOpts GRPCTLSOptions, log *logger.Logger) (*SearchService, error) {
	transportCreds, err := buildClientTransportCredentials(tlsOpts)
	if err != nil {
		return nil, fmt.Errorf("build user-service transport credentials: %w", err)
	}

	conn, err := grpc.NewClient(userServiceAddr, grpc.WithTransportCredentials(transportCreds))
	if err != nil {
		return nil, fmt.Errorf("user service gRPC client: %w", err)
	}
	return &SearchService{
		os:         os,
		usersIndex: usersIndex,
		postsIndex: postsIndex,
		userConn:   conn,
		userClient: userv1.NewUserServiceClient(conn),
		log:        log,
	}, nil
}

func (s *SearchService) Close() error {
	if s.userConn != nil {
		return s.userConn.Close()
	}
	return nil
}

func (s *SearchService) Search(ctx context.Context, req *searchv1.SearchRequest) (*searchv1.SearchResponse, error) {
	query := strings.TrimSpace(req.GetQuery())
	if query == "" {
		return &searchv1.SearchResponse{Users: nil, Posts: nil}, nil
	}

	usersLimit := int(req.GetUsersLimit())
	if usersLimit <= 0 || usersLimit > 50 {
		usersLimit = 20
	}
	postsLimit := int(req.GetPostsLimit())
	if postsLimit <= 0 || postsLimit > 50 {
		postsLimit = 20
	}

	usersFrom := decodeCursor(req.GetUsersCursor())
	postsFrom := decodeCursor(req.GetPostsCursor())

	resp := &searchv1.SearchResponse{
		Users:        nil,
		Posts:        nil,
		UsersPartial: false,
		PostsPartial: false,
	}

	// Search users and posts in parallel
	var userHits []searchv1.SearchUserHit
	var userNext int
	var userPartial bool
	var userErr error
	var postHits []searchv1.SearchPostHit
	var postNext int
	var postPartial bool
	var postErr error

	done := make(chan struct{})
	go func() {
		userHits, userNext, userPartial, userErr = s.searchUsers(ctx, query, usersLimit, usersFrom)
		done <- struct{}{}
	}()
	go func() {
		postHits, postNext, postPartial, postErr = s.searchPosts(ctx, query, postsLimit, postsFrom)
		done <- struct{}{}
	}()
	<-done
	<-done

	if userErr != nil {
		s.log.Warn("search users: " + userErr.Error())
		resp.UsersPartial = true
	} else {
		resp.Users = usersToPtrs(userHits)
		resp.UsersNextCursor = encodeCursor(userNext)
		resp.UsersPartial = userPartial
	}

	if postErr != nil {
		s.log.Warn("search posts: " + postErr.Error())
		resp.PostsPartial = true
	} else {
		resp.Posts = postsToPtrs(postHits)
		resp.PostsNextCursor = encodeCursor(postNext)
		resp.PostsPartial = postPartial
	}

	// Demote already-followed users: call AreFollowed and reorder
	if len(resp.Users) > 0 && req.GetRequestingUserId() != "" {
		followed, err := s.areFollowed(ctx, req.GetRequestingUserId(), ptrsToUsers(resp.Users))
		if err != nil {
			s.log.Warn("are_followed: " + err.Error())
		} else if len(followed) > 0 {
			resp.Users = usersToPtrs(demoteFollowedUsers(ptrsToUsers(resp.Users), followed))
		}
	}

	return resp, nil
}

func (s *SearchService) searchUsers(ctx context.Context, query string, limit, from int) ([]searchv1.SearchUserHit, int, bool, error) {
	if s.os == nil {
		return nil, 0, true, fmt.Errorf("opensearch not configured")
	}
	body := buildUserSearchBody(query, limit, from)
	res, err := s.os.DoSearch(ctx, s.usersIndex, bytes.NewReader(body), &limit, &from)
	if err != nil {
		return nil, 0, true, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 && res.StatusCode != 404 {
		return nil, 0, true, fmt.Errorf("opensearch users: %d", res.StatusCode)
	}
	var out struct {
		Hits struct {
			Hits []struct {
				Source struct {
					ID      string `json:"id"`
					Name    string `json:"name"`
					Picture string `json:"picture"`
					Bio     string `json:"bio"`
				} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
		Shards struct {
			Failed int `json:"failed"`
		} `json:"_shards"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, 0, true, err
	}
	partial := out.Shards.Failed > 0
	hits := make([]searchv1.SearchUserHit, 0, len(out.Hits.Hits))
	for _, h := range out.Hits.Hits {
		hits = append(hits, searchv1.SearchUserHit{
			Id:      h.Source.ID,
			Name:    h.Source.Name,
			Picture: h.Source.Picture,
			Bio:     h.Source.Bio,
		})
	}
	nextFrom := from + len(hits)
	return hits, nextFrom, partial, nil
}

func (s *SearchService) searchPosts(ctx context.Context, query string, limit, from int) ([]searchv1.SearchPostHit, int, bool, error) {
	if s.os == nil {
		return nil, 0, true, fmt.Errorf("opensearch not configured")
	}
	body := buildPostSearchBody(query, limit, from)
	res, err := s.os.DoSearch(ctx, s.postsIndex, bytes.NewReader(body), &limit, &from)
	if err != nil {
		return nil, 0, true, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 && res.StatusCode != 404 {
		return nil, 0, true, fmt.Errorf("opensearch posts: %d", res.StatusCode)
	}
	var out struct {
		Hits struct {
			Hits []struct {
				Source struct {
					ID        string `json:"id"`
					UserID    string `json:"user_id"`
					Title     string `json:"title"`
					Slug      string `json:"slug"`
					Content   string `json:"content"`
					Published bool   `json:"published"`
				} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
		Shards struct {
			Failed int `json:"failed"`
		} `json:"_shards"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, 0, true, err
	}
	partial := out.Shards.Failed > 0
	hits := make([]searchv1.SearchPostHit, 0, len(out.Hits.Hits))
	for _, h := range out.Hits.Hits {
		preview := h.Source.Content
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		hits = append(hits, searchv1.SearchPostHit{
			Id:             h.Source.ID,
			UserId:         h.Source.UserID,
			Title:          h.Source.Title,
			Slug:           h.Source.Slug,
			ContentPreview: preview,
			Published:      h.Source.Published,
		})
	}
	nextFrom := from + len(hits)
	return hits, nextFrom, partial, nil
}

func buildUserSearchBody(query string, size, from int) []byte {
	// Prefix + fuzzy on name (and bio)
	q := map[string]interface{}{
		"from": from,
		"size": size,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"should": []map[string]interface{}{
					{"prefix": map[string]interface{}{"name": map[string]interface{}{"value": query, "boost": 2}}},
					{"match": map[string]interface{}{"name": map[string]interface{}{"query": query, "fuzziness": "AUTO"}}},
					{"match": map[string]interface{}{"bio": map[string]interface{}{"query": query, "fuzziness": "AUTO"}}},
				},
			},
		},
	}
	b, _ := json.Marshal(q)
	return b
}

func buildPostSearchBody(query string, size, from int) []byte {
	q := map[string]interface{}{
		"from": from,
		"size": size,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"filter": []map[string]interface{}{
					{"term": map[string]interface{}{"published": true}},
				},
				"should": []map[string]interface{}{
					{"prefix": map[string]interface{}{"title": map[string]interface{}{"value": query, "boost": 2}}},
					{"match": map[string]interface{}{"title": map[string]interface{}{"query": query, "fuzziness": "AUTO"}}},
					{"match": map[string]interface{}{"content": map[string]interface{}{"query": query, "fuzziness": "AUTO"}}},
				},
				"minimum_should_match": 1,
			},
		},
	}
	b, _ := json.Marshal(q)
	return b
}

func (s *SearchService) areFollowed(ctx context.Context, followerID string, users []searchv1.SearchUserHit) (map[string]bool, error) {
	if len(users) == 0 {
		return nil, nil
	}
	ids := make([]string, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.Id)
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	resp, err := s.userClient.AreFollowed(ctx, &userv1.AreFollowedRequest{
		FollowerId:  followerID,
		FolloweeIds: ids,
	})
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool)
	for _, id := range resp.GetFollowedIds() {
		set[id] = true
	}
	return set, nil
}

func demoteFollowedUsers(users []searchv1.SearchUserHit, followed map[string]bool) []searchv1.SearchUserHit {
	var notFollowed, followedList []searchv1.SearchUserHit
	for _, u := range users {
		if followed[u.Id] {
			followedList = append(followedList, u)
		} else {
			notFollowed = append(notFollowed, u)
		}
	}
	// Not followed first, then followed (demoted)
	return append(notFollowed, followedList...)
}

func encodeCursor(from int) string {
	if from <= 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString([]byte(strconv.Itoa(from)))
}

func decodeCursor(cursor string) int {
	if cursor == "" {
		return 0
	}
	b, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(string(b))
	if n < 0 {
		return 0
	}
	return n
}

func usersToPtrs(h []searchv1.SearchUserHit) []*searchv1.SearchUserHit {
	if len(h) == 0 {
		return nil
	}
	out := make([]*searchv1.SearchUserHit, len(h))
	for i := range h {
		out[i] = &h[i]
	}
	return out
}

func ptrsToUsers(p []*searchv1.SearchUserHit) []searchv1.SearchUserHit {
	if len(p) == 0 {
		return nil
	}
	out := make([]searchv1.SearchUserHit, len(p))
	for i := range p {
		if p[i] != nil {
			out[i] = *p[i]
		}
	}
	return out
}

func postsToPtrs(h []searchv1.SearchPostHit) []*searchv1.SearchPostHit {
	if len(h) == 0 {
		return nil
	}
	out := make([]*searchv1.SearchPostHit, len(h))
	for i := range h {
		out[i] = &h[i]
	}
	return out
}

func buildClientTransportCredentials(tlsOpts GRPCTLSOptions) (credentials.TransportCredentials, error) {
	if !tlsOpts.Enabled {
		return insecure.NewCredentials(), nil
	}

	caPEM, err := os.ReadFile(tlsOpts.CAFile)
	if err != nil {
		return nil, fmt.Errorf("read gRPC CA file: %w", err)
	}

	rootCAs := x509.NewCertPool()
	if ok := rootCAs.AppendCertsFromPEM(caPEM); !ok {
		return nil, fmt.Errorf("parse gRPC CA certificate")
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    rootCAs,
	}

	if tlsOpts.CertFile != "" && tlsOpts.KeyFile != "" {
		clientCert, certErr := tls.LoadX509KeyPair(tlsOpts.CertFile, tlsOpts.KeyFile)
		if certErr != nil {
			return nil, fmt.Errorf("load gRPC client certificate: %w", certErr)
		}
		tlsConfig.Certificates = []tls.Certificate{clientCert}
	}

	return credentials.NewTLS(tlsConfig), nil
}
