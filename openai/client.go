package openai

import (
	"net/http"

	"github.com/wfu-work/proxy-api-lib/compatible"
)

// Option configures the OpenAI provider.
type Option func(*compatible.Config)

// New returns an OpenAI official API provider.
func New(opts ...Option) *compatible.ResponsesProvider {
	cfg := compatible.Config{
		Name:    compatible.DefaultOpenAIName,
		BaseURL: compatible.DefaultOpenAIBaseURL,
		WireAPI: compatible.WireAPIResponses,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return compatible.OpenAIResponses(cfg)
}

// WithBaseURL overrides the upstream base URL.
func WithBaseURL(baseURL string) Option {
	return func(cfg *compatible.Config) {
		cfg.BaseURL = baseURL
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

// WithHeader adds a default header to upstream requests.
func WithHeader(key, value string) Option {
	return func(cfg *compatible.Config) {
		if cfg.Headers == nil {
			cfg.Headers = map[string]string{}
		}
		cfg.Headers[key] = value
	}
}
