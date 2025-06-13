// Fix 7: Update services/api-gateway/internal/clients/auth_client.go
// Remove GoogleLogin and makeAuthRequestWithModel methods

package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"api-gateway/internal/models"
	"api-gateway/pkg/logger"
)

type AuthClient struct {
	baseURL    string
	httpClient *http.Client
	logger     *logger.Logger
}

func NewAuthClient(baseURL string, logger *logger.Logger) *AuthClient {
	return &AuthClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

func (c *AuthClient) GetGoogleAuthURL(ctx context.Context) (map[string]interface{}, error) {
	url := c.baseURL + "/api/v1/auth/google"
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get Google auth URL: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get Google auth URL failed with status %d: %s", resp.StatusCode, string(body))
	}

	var response models.APIResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !response.Success {
		return nil, fmt.Errorf("request failed: %s", response.Error.Message)
	}

	if data, ok := response.Data.(map[string]interface{}); ok {
		return data, nil
	}

	return nil, fmt.Errorf("unexpected response format")
}

func (c *AuthClient) GoogleCallback(ctx context.Context, queryParams url.Values) (map[string]interface{}, error) {
	callbackURL := c.baseURL + "/api/v1/auth/google/callback?" + queryParams.Encode()
	
	req, err := http.NewRequestWithContext(ctx, "GET", callbackURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create callback request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to process Google callback: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTemporaryRedirect || resp.StatusCode == http.StatusFound {
		location := resp.Header.Get("Location")
		return map[string]interface{}{
			"redirect_url": location,
		}, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read callback response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google callback failed with status %d: %s", resp.StatusCode, string(body))
	}

	var response models.APIResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse callback response: %w", err)
	}

	if !response.Success {
		return nil, fmt.Errorf("callback failed: %s", response.Error.Message)
	}

	if data, ok := response.Data.(map[string]interface{}); ok {
		return data, nil
	}

	return nil, fmt.Errorf("unexpected callback response format")
}

func (c *AuthClient) ExchangeAuthCode(ctx context.Context, req map[string]interface{}) (map[string]interface{}, error) {
	return c.makeAuthRequest(ctx, "POST", "/api/v1/auth/exchange", req, "")
}

func (c *AuthClient) RefreshToken(ctx context.Context, req map[string]interface{}) (map[string]interface{}, error) {
	return c.makeAuthRequest(ctx, "POST", "/api/v1/auth/refresh", req, "")
}

func (c *AuthClient) Logout(ctx context.Context, req map[string]interface{}, token string) error {
	_, err := c.makeAuthRequest(ctx, "POST", "/api/v1/auth/logout", req, token)
	return err
}

func (c *AuthClient) ValidateToken(ctx context.Context, token string) (*models.TokenValidationResponse, error) {
	url := c.baseURL + "/api/v1/auth/validate"
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to validate token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token validation failed with status %d: %s", resp.StatusCode, string(body))
	}

	var response models.APIResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !response.Success {
		return nil, fmt.Errorf("token validation failed: %s", response.Error.Message)
	}

	dataBytes, err := json.Marshal(response.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}

	var tokenResp models.TokenValidationResponse
	if err := json.Unmarshal(dataBytes, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token validation data: %w", err)
	}

	return &tokenResp, nil
}

func (c *AuthClient) HealthCheck() error {
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

// Helper method for OAuth endpoints that return generic data
func (c *AuthClient) makeAuthRequest(ctx context.Context, method, endpoint string, reqBody map[string]interface{}, token string) (map[string]interface{}, error) {
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
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
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

	if data, ok := response.Data.(map[string]interface{}); ok {
		return data, nil
	}

	return nil, fmt.Errorf("unexpected response format")
}

func (c *AuthClient) Close() {
	// Close any persistent connections if needed
}