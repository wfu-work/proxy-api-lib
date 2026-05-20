package codexzh

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const UsageURL = "https://codexzh.com/api/v1/usage/stats"

// UsageStats describes CodexZH usage and quota details.
type UsageStats struct {
	DailyQuota        float64 `json:"dailyQuota"`
	WeeklyQuota       float64 `json:"weeklyQuota"`
	TodayUsed         float64 `json:"todayUsed"`
	WeekUsed          float64 `json:"weekUsed"`
	TodayCalls        int64   `json:"todayCalls"`
	TotalCalls        int64   `json:"totalCalls"`
	RPM               int64   `json:"rpm"`
	TPM               int64   `json:"tpm"`
	SubscriptionStart string  `json:"subscriptionStart"`
	SubscriptionEnd   string  `json:"subscriptionEnd"`
}

// UsageClient fetches CodexZH usage data.
type UsageClient struct {
	BaseURL    string
	HTTPClient *http.Client
	Timeout    time.Duration
}

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

// UsageOption configures a UsageClient.
type UsageOption func(*UsageClient)

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

// WithUsageTimeout overrides the request timeout.
func WithUsageTimeout(timeout time.Duration) UsageOption {
	return func(c *UsageClient) {
		c.Timeout = timeout
	}
}

// Fetch retrieves usage stats for the provided key.
func (c *UsageClient) Fetch(ctx context.Context, key string) (UsageStats, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return UsageStats{}, errors.New("codexzh: key is required")
	}
	reqURL := appendQueryParam(c.BaseURL, "key", key)
	reqCtx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, reqURL, nil)
	if err != nil {
		return UsageStats{}, err
	}
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
		return UsageStats{}, fmt.Errorf("codexzh: upstream returned %d", resp.StatusCode)
	}
	return ParseUsageResponse(body)
}

// ParseUsageResponse decodes CodexZH usage responses.
func ParseUsageResponse(body []byte) (UsageStats, error) {
	var direct UsageStats
	if err := json.Unmarshal(body, &direct); err == nil && hasCodexZHUsage(direct) {
		return direct, nil
	}
	var wrapped struct {
		Data UsageStats `json:"data"`
	}
	if err := json.Unmarshal(body, &wrapped); err != nil {
		return UsageStats{}, err
	}
	if !hasCodexZHUsage(wrapped.Data) {
		return UsageStats{}, errors.New("codexzh: invalid usage response")
	}
	return wrapped.Data, nil
}

func hasCodexZHUsage(stats UsageStats) bool {
	return stats.DailyQuota > 0 || stats.WeeklyQuota > 0 || stats.TodayUsed > 0 || stats.WeekUsed > 0
}

func appendQueryParam(rawURL, key, value string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		separator := "?"
		if strings.Contains(rawURL, "?") {
			separator = "&"
		}
		return rawURL + separator + url.QueryEscape(key) + "=" + url.QueryEscape(value)
	}
	query := parsed.Query()
	query.Set(key, value)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

// QuotaToUSD converts CodexZH quota units to USD.
func QuotaToUSD(value float64) float64 {
	return value / 500000
}

// ParseTime parses CodexZH date fields into unix milliseconds.
func ParseTime(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t.UnixMilli()
		}
	}
	return 0
}
