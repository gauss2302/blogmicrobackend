package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"api-gateway/internal/models"
	"api-gateway/pkg/logger"
)

type PostClient struct {
	baseURL    string
	httpClient *http.Client
	logger     *logger.Logger
}

func NewPostClient(baseURL string, logger *logger.Logger) *PostClient {
	return &PostClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

func (c *PostClient) CreatePost(ctx context.Context, req map[string]interface{}, userID string) (*models.PostResponse, error) {
	return c.makePostRequest(ctx, "POST", "/api/v1/posts", req, userID)
}

func (c *PostClient) GetPost(ctx context.Context, id string, userID string) (*models.PostResponse, error) {
	endpoint := fmt.Sprintf("/api/v1/posts/%s", id)
	
	url := c.baseURL + endpoint

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	if userID != "" {
		req.Header.Set("X-User-ID", userID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var response models.APIResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !response.Success {
		return nil, fmt.Errorf("request failed: %s", response.Error.Message)
	}

	// Parse the data field for post response
	dataBytes, err := json.Marshal(response.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}

	var postResp models.PostResponse
	if err := json.Unmarshal(dataBytes, &postResp); err != nil {
		return nil, fmt.Errorf("failed to parse post data: %w", err)
	}

	return &postResp, nil
}

func (c *PostClient) GetPostBySlug(ctx context.Context, slug string) (*models.PostResponse, error) {
	endpoint := fmt.Sprintf("/api/v1/posts/slug/%s", slug)
	resp, err := c.makePublicRequest(ctx, "GET", endpoint)
	if err != nil {
		return nil, err
	}

	// Parse response into post format
	dataBytes, err := json.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	var post models.PostResponse
	if err := json.Unmarshal(dataBytes, &post); err != nil {
		return nil, fmt.Errorf("failed to parse post response: %w", err)
	}

	return &post, nil
}

func (c *PostClient) UpdatePost(ctx context.Context, id string, req map[string]interface{}, userID string) (*models.PostResponse, error) {
	endpoint := fmt.Sprintf("/api/v1/posts/%s", id)
	return c.makePostRequest(ctx, "PUT", endpoint, req, userID)
}

func (c *PostClient) DeletePost(ctx context.Context, id string, userID string) error {
	endpoint := fmt.Sprintf("/api/v1/posts/%s", id)
	_, err := c.makePostRequest(ctx, "DELETE", endpoint, nil, userID)
	return err
}

func (c *PostClient) ListPosts(ctx context.Context, limit, offset int, publishedOnly bool) (*models.ListPostsResponse, error) {
	params := url.Values{}
	params.Add("limit", strconv.Itoa(limit))
	params.Add("offset", strconv.Itoa(offset))
	params.Add("published_only", strconv.FormatBool(publishedOnly))
	
	endpoint := "/api/v1/posts?" + params.Encode()
	
	resp, err := c.makePublicRequest(ctx, "GET", endpoint)
	if err != nil {
		return nil, err
	}

	return c.parseListResponse(resp)
}

func (c *PostClient) GetUserPosts(ctx context.Context, userID string, limit, offset int) (*models.ListPostsResponse, error) {
	params := url.Values{}
	params.Add("limit", strconv.Itoa(limit))
	params.Add("offset", strconv.Itoa(offset))
	
	endpoint := fmt.Sprintf("/api/v1/posts/user/%s?%s", userID, params.Encode())
	
	resp, err := c.makePublicRequest(ctx, "GET", endpoint)
	if err != nil {
		return nil, err
	}

	return c.parseListResponse(resp)
}

func (c *PostClient) SearchPosts(ctx context.Context, query string, limit, offset int, publishedOnly bool) (*models.ListPostsResponse, error) {
	params := url.Values{}
	params.Add("q", query)
	params.Add("limit", strconv.Itoa(limit))
	params.Add("offset", strconv.Itoa(offset))
	params.Add("published_only", strconv.FormatBool(publishedOnly))
	
	endpoint := "/api/v1/posts/search?" + params.Encode()
	
	resp, err := c.makePublicRequest(ctx, "GET", endpoint)
	if err != nil {
		return nil, err
	}

	return c.parseListResponse(resp)
}

func (c *PostClient) GetStats(ctx context.Context, userID string) (*models.PostStatsResponse, error) {
	endpoint := "/api/v1/posts/stats"
	
	resp, err := c.makePublicRequest(ctx, "GET", endpoint)
	if err != nil {
		return nil, err
	}

	// Parse response into stats format
	dataBytes, err := json.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	var stats models.PostStatsResponse
	if err := json.Unmarshal(dataBytes, &stats); err != nil {
		return nil, fmt.Errorf("failed to parse stats response: %w", err)
	}

	return &stats, nil
}

func (c *PostClient) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	url := c.baseURL + "/health"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status %d", resp.StatusCode)
	}

	return nil
}

func (c *PostClient) makePostRequest(ctx context.Context, method, endpoint string, reqBody map[string]interface{}, userID string) (*models.PostResponse, error) {
	url := c.baseURL + endpoint
	
	var body io.Reader
	if reqBody != nil {
		jsonBody, err := json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}
		body = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	if userID != "" {
		req.Header.Set("X-User-ID", userID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var response models.APIResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !response.Success {
		return nil, fmt.Errorf("request failed: %s", response.Error.Message)
	}

	// Parse the data field for post response
	dataBytes, err := json.Marshal(response.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}

	var postResp models.PostResponse
	if err := json.Unmarshal(dataBytes, &postResp); err != nil {
		return nil, fmt.Errorf("failed to parse post data: %w", err)
	}

	return &postResp, nil
}

func (c *PostClient) makePublicRequest(ctx context.Context, method, endpoint string) (interface{}, error) {
	url := c.baseURL + endpoint

	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var response models.APIResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !response.Success {
		return nil, fmt.Errorf("request failed: %s", response.Error.Message)
	}

	return response.Data, nil
}

func (c *PostClient) parseListResponse(data interface{}) (*models.ListPostsResponse, error) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}

	var listResp models.ListPostsResponse
	if err := json.Unmarshal(dataBytes, &listResp); err != nil {
		return nil, fmt.Errorf("failed to parse list response: %w", err)
	}

	return &listResp, nil
}

func (c *PostClient) Close() {
	// Close any persistent connections if needed
}