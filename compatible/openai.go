package compatible

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/free-model/proxy-api-lib/transport"
)

const (
	DefaultOpenAIName    = "openai"
	DefaultOpenAIBaseURL = "https://api.openai.com/v1"
	WireAPIResponses     = "responses"
)

// Config describes an OpenAI-compatible upstream.
type Config struct {
	Name       string
	BaseURL    string
	WireAPI    string
	ProxyURL   string
	HTTPClient *http.Client
	Headers    map[string]string
}

// OpenAIResponses creates an OpenAI-compatible Responses provider.
func OpenAIResponses(cfg Config) *ResponsesProvider {
	if cfg.Name == "" {
		cfg.Name = DefaultOpenAIName
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultOpenAIBaseURL
	}
	if cfg.WireAPI == "" {
		cfg.WireAPI = WireAPIResponses
	}
	var initErr error
	if cfg.HTTPClient == nil && cfg.ProxyURL != "" {
		cfg.HTTPClient, initErr = transport.NewHTTPClient(cfg.ProxyURL)
		if initErr != nil {
			initErr = fmt.Errorf("compatible: invalid proxy_url %q: %w", cfg.ProxyURL, initErr)
		}
	}
	if cfg.HTTPClient == nil && initErr == nil {
		cfg.HTTPClient = http.DefaultClient
	}
	return &ResponsesProvider{
		name:       cfg.Name,
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		wireAPI:    cfg.WireAPI,
		httpClient: cfg.HTTPClient,
		headers:    cfg.Headers,
		initErr:    initErr,
	}
}
