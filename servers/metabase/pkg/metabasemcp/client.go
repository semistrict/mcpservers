package metabasemcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var globalClient *Client

// Client represents a Metabase API client
type Client struct {
	baseURL    string
	auther     Auther
	httpClient *http.Client
}

// InitializeClient creates and sets the global client with the provided configuration
func InitializeClient(serverURL, cookiesFile string) error {
	if serverURL == "" {
		return fmt.Errorf("server URL is required")
	}

	auther, err := NewCookieAuth(cookiesFile)
	if err != nil {
		return fmt.Errorf("failed to initialize cookie auth: %w", err)
	}

	globalClient = &Client{
		baseURL: normalizeBaseURL(serverURL),
		auther:  auther,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	return nil
}

func getClient() *Client {
	if globalClient == nil {
		panic("globalClient not initialized")
	}
	return globalClient
}

// MetabaseTool is the base struct for all Metabase tools
type MetabaseTool struct {
}

// DatabaseTool is for tools that operate on specific databases
type DatabaseTool struct {
	MetabaseTool
	DatabaseID int `json:"database_id" description:"Database ID in Metabase"`
}

// Request makes an API request to Metabase
func (c *Client) Request(ctx context.Context, method, endpoint string, body []byte, contentType string) ([]byte, error) {
	var requestBody io.Reader
	if body != nil {
		requestBody = bytes.NewReader(body)
	}

	url := fmt.Sprintf("%s%s", c.baseURL, endpoint)
	req, err := http.NewRequestWithContext(ctx, method, url, requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	err = c.auther.Auth(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("auth failed: %w", err)
	}
	if body != nil && contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(responseBody))
	}

	return responseBody, nil
}

func Get[ResponseType any](ctx context.Context, path string) (response ResponseType, err error) {
	respBody, err := getClient().Request(ctx, "GET", path, nil, "")
	if err != nil {
		err = fmt.Errorf("GET %s failed: %w", path, err)
		return
	}

	if err = json.Unmarshal(respBody, &response); err != nil {
		err = fmt.Errorf("failed to parse response: %w", err)
		return
	}

	return
}

func Post[ResponseType any](ctx context.Context, path string, body interface{}) (response ResponseType, err error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		err = fmt.Errorf("failed to marshal request body: %w", err)
		return
	}

	respBody, err := getClient().Request(ctx, "POST", path, jsonBody, "application/json")
	if err != nil {
		err = fmt.Errorf("POST %s failed: %w", path, err)
		return
	}

	if err = json.Unmarshal(respBody, &response); err != nil {
		err = fmt.Errorf("failed to parse response: %w", err)
		return
	}

	return
}

// normalizeBaseURL ensures the base URL doesn't have a trailing slash
func normalizeBaseURL(baseURL string) string {
	return strings.TrimRight(baseURL, "/")
}
