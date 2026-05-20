package aiok

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/wfu-work/proxy-api-lib/compatible"
)

var UsageURL = compatible.DefaultUsageURL(BaseURL)

// UsageStats describes Aiok usage and balance details.
type UsageStats struct {
	Balance float64 `json:"balance"`
}

// UsageClient fetches Aiok usage data from the conventional /v1/usage endpoint.
type UsageClient struct {
	BaseURL    string
	HTTPClient *http.Client
	Timeout    time.Duration
}

// UsageOption configures a UsageClient.
type UsageOption func(*UsageClient)

// NewUsageClient returns a usage client with defaults.
func NewUsageClient(opts ...UsageOption) *UsageClient {
	c := &UsageClient{BaseURL: UsageURL}
	for _, opt := range opts {
		opt(c)
	}
	if c.HTTPClient == nil {
		c.HTTPClient = http.DefaultClient
	}
	if c.Timeout <= 0 {
		c.Timeout = 30 * time.Second
	}
	if strings.TrimSpace(c.BaseURL) == "" {
		c.BaseURL = UsageURL
	}
	return c
}

// WithUsageHTTPClient overrides the HTTP client.
func WithUsageHTTPClient(client *http.Client) UsageOption {
	return func(c *UsageClient) {
		c.HTTPClient = client
	}
}

// WithUsageBaseURL overrides the usage endpoint.
func WithUsageBaseURL(baseURL string) UsageOption {
	return func(c *UsageClient) {
		c.BaseURL = baseURL
	}
}

// WithUsageProviderBaseURL builds the usage endpoint from a provider base URL.
func WithUsageProviderBaseURL(baseURL string) UsageOption {
	return func(c *UsageClient) {
		c.BaseURL = compatible.DefaultUsageURL(baseURL)
	}
}

// WithUsageTimeout overrides the request timeout.
func WithUsageTimeout(timeout time.Duration) UsageOption {
	return func(c *UsageClient) {
		c.Timeout = timeout
	}
}

// Fetch retrieves usage stats for the provided token.
func (c *UsageClient) Fetch(ctx context.Context, token string) (UsageStats, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return UsageStats{}, errors.New("aiok: token is required")
	}
	reqCtx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, c.BaseURL, nil)
	if err != nil {
		return UsageStats{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return UsageStats{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return UsageStats{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return UsageStats{}, fmt.Errorf("aiok: upstream returned %d", resp.StatusCode)
	}
	return ParseUsageResponse(body)
}

// ParseUsageResponse decodes Aiok usage responses.
func ParseUsageResponse(body []byte) (UsageStats, error) {
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return UsageStats{}, err
	}
	balance, ok := findFirstNumber(payload, "balance")
	if !ok {
		return UsageStats{}, errors.New("aiok: invalid usage response")
	}
	return UsageStats{Balance: balance}, nil
}

func findFirstNumber(value any, keys ...string) (float64, bool) {
	switch item := value.(type) {
	case map[string]any:
		for _, key := range keys {
			if number, ok := numberFromAny(item[key]); ok {
				return number, true
			}
		}
		for _, child := range item {
			if number, ok := findFirstNumber(child, keys...); ok {
				return number, true
			}
		}
	case []any:
		for _, child := range item {
			if number, ok := findFirstNumber(child, keys...); ok {
				return number, true
			}
		}
	}
	return 0, false
}

func numberFromAny(value any) (float64, bool) {
	switch item := value.(type) {
	case float64:
		return item, true
	case string:
		if item = strings.TrimSpace(item); item != "" {
			number, err := strconv.ParseFloat(item, 64)
			return number, err == nil
		}
	}
	return 0, false
}
