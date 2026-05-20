package aiok

import (
	"net/http"

	"github.com/wfu-work/proxy-api-lib/compatible"
)

const (
	Name    = "aiok"
	BaseURL = "https://aiok.club/v1"
)

// Option configures the Aiok provider preset.
type Option func(*compatible.Config)

// New returns an Aiok OpenAI-compatible Responses provider.
func New(opts ...Option) *compatible.ResponsesProvider {
	cfg := Config()
	for _, opt := range opts {
		opt(&cfg)
	}
	return compatible.OpenAIResponses(cfg)
}

// Config returns the default Aiok provider config.
func Config() compatible.Config {
	return compatible.Config{
		Name:    Name,
		BaseURL: BaseURL,
		WireAPI: compatible.WireAPIResponses,
	}
}

// WithHTTPClient overrides the HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(cfg *compatible.Config) {
		cfg.HTTPClient = client
	}
}

// WithProxyURL configures an HTTP or SOCKS-style proxy URL for upstream requests.
func WithProxyURL(proxyURL string) Option {
	return func(cfg *compatible.Config) {
		cfg.ProxyURL = proxyURL
	}
}

// WithBaseURL overrides the upstream base URL.
func WithBaseURL(baseURL string) Option {
	return func(cfg *compatible.Config) {
		cfg.BaseURL = baseURL
	}
}
