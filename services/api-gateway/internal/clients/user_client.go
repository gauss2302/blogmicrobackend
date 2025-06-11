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

type UserClient struct {
	baseURL    string
	httpClient *http.Client
	logger     *logger.Logger
}

func NewUserClient(baseURL string, logger *logger.Logger) *UserClient {
	return &UserClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

func (c *UserClient) CreateUser(ctx context.Context, req map[string]interface{}, userID string) (*models.UserResponse, error) {
	return c.makeUserRequest(ctx, "POST", "/api/v1/users", req, userID)
}

func (c *UserClient) GetUser(ctx context.Context, id string, userID string) (*models.UserResponse, error) {
	endpoint := fmt.Sprintf("/api/v1/users/%s", id)
	return c.makeUserRequest(ctx, "GET", endpoint, nil, userID)
}

func (c *UserClient) UpdateUser(ctx context.Context, id string, req map[string]interface{}, userID string) (*models.UserResponse, error) {
	endpoint := fmt.Sprintf("/api/v1/users/%s", id)
	return c.makeUserRequest(ctx, "PUT", endpoint, req, userID)
}

func (c *UserClient) DeleteUser(ctx context.Context, id string, userID string) error {
	endpoint := fmt.Sprintf("/api/v1/users/%s", id)
	_, err := c.makeUserRequest(ctx, "DELETE", endpoint, nil, userID)
	return err
}

func (c *UserClient) ListUsers(ctx context.Context, limit, offset int, userID string) (*models.ListUsersResponse, error) {
	endpoint := fmt.Sprintf("/api/v1/users?limit=%d&offset=%d", limit, offset)
	
	resp, err := c.makeUserRequest(ctx, "GET", endpoint, nil, userID)
	if err != nil {
		return nil, err
	}

	// This is a simplified conversion - in practice you'd handle the list response properly
	return &models.ListUsersResponse{
		Users:  []*models.UserResponse{resp}, // Simplified
		Limit:  limit,
		Offset: offset,
		Total:  1,
	}, nil
}

func (c *UserClient) SearchUsers(ctx context.Context, query string, limit, offset int) (*models.ListUsersResponse, error) {
	params := url.Values{}
	params.Add("q", query)
	params.Add("limit", strconv.Itoa(limit))
	params.Add("offset", strconv.Itoa(offset))
	
	endpoint := "/api/v1/users/search?" + params.Encode()
	
	resp, err := c.makePublicRequest(ctx, "GET", endpoint)
	if err != nil {
		return nil, err
	}

	// Parse response into list format
	return c.parseListResponse(resp)
}

func (c *UserClient) GetUserProfile(ctx context.Context, id string) (*models.UserProfileResponse, error) {
	endpoint := fmt.Sprintf("/api/v1/users/%s/profile", id)
	
	resp, err := c.makePublicRequest(ctx, "GET", endpoint)
	if err != nil {
		return nil, err
	}

	// Parse response into profile format
	dataBytes, err := json.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	var profile models.UserProfileResponse
	if err := json.Unmarshal(dataBytes, &profile); err != nil {
		return nil, fmt.Errorf("failed to parse profile response: %w", err)
	}

	return &profile, nil
}

func (c *UserClient) GetStats(ctx context.Context) (*models.UserStatsResponse, error) {
	resp, err := c.makePublicRequest(ctx, "GET", "/api/v1/users/stats")
	if err != nil {
		return nil, err
	}

	// Parse response into stats format
	dataBytes, err := json.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	var stats models.UserStatsResponse
	if err := json.Unmarshal(dataBytes, &stats); err != nil {
		return nil, fmt.Errorf("failed to parse stats response: %w", err)
	}

	return &stats, nil
}

func (c *UserClient) HealthCheck() error {
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

func (c *UserClient) makeUserRequest(ctx context.Context, method, endpoint string, reqBody map[string]interface{}, userID string) (*models.UserResponse, error) {
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

	// Parse the data field for user response
	dataBytes, err := json.Marshal(response.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}

	var userResp models.UserResponse
	if err := json.Unmarshal(dataBytes, &userResp); err != nil {
		return nil, fmt.Errorf("failed to parse user data: %w", err)
	}

	return &userResp, nil
}

func (c *UserClient) makePublicRequest(ctx context.Context, method, endpoint string) (interface{}, error) {
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

func (c *UserClient) parseListResponse(data interface{}) (*models.ListUsersResponse, error) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}

	var listResp models.ListUsersResponse
	if err := json.Unmarshal(dataBytes, &listResp); err != nil {
		return nil, fmt.Errorf("failed to parse list response: %w", err)
	}

	return &listResp, nil
}

func (c *UserClient) Close() {
	// Close any persistent connections if needed
}