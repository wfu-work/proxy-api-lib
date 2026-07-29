// Package proxyapi 为网关应用提供 OpenAI 官方 API 客户端能力。
package proxyapi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/wfu-work/proxy-api-lib/auth"
	"github.com/wfu-work/proxy-api-lib/transport"
)

// DefaultBaseURL 是 OpenAI 官方 API 的默认地址。
const DefaultBaseURL = "https://api.openai.com/v1"

// Client 调用 OpenAI 官方 API。
// 账号路由、故障转移、额度统计和持久化由上层应用负责。
type Client struct {
	baseURL    string
	credential auth.Credential
	httpClient *http.Client
	headers    map[string]string
	initErr    error

	Responses  *ResponsesService
	Embeddings *EmbeddingsService
	Models     *ModelsService
}

type clientConfig struct {
	baseURL    string
	credential auth.Credential
	httpClient *http.Client
	proxyURL   string
	headers    map[string]string
}

// Option 表示一项 OpenAI 客户端配置。
type Option func(*clientConfig)

// WithCredential 配置客户端默认使用的请求凭据。
func WithCredential(credential auth.Credential) Option {
	return func(cfg *clientConfig) {
		cfg.credential = credential
	}
}

// WithAPIKey 配置 OpenAI Platform API Key。
func WithAPIKey(key string) Option {
	return WithCredential(auth.APIKey(key))
}

// WithBearerToken 配置由调用方管理的访问令牌。
func WithBearerToken(token string) Option {
	return WithCredential(auth.BearerToken(token))
}

// WithBaseURL 覆盖 OpenAI 官方地址，主要用于单元测试或受控的 OpenAI 端点部署。
func WithBaseURL(baseURL string) Option {
	return func(cfg *clientConfig) {
		cfg.baseURL = baseURL
	}
}

// WithHTTPClient 配置自定义 HTTP 客户端。
func WithHTTPClient(client *http.Client) Option {
	return func(cfg *clientConfig) {
		cfg.httpClient = client
	}
}

// WithProxyURL 配置访问 OpenAI 官方 API 时使用的网络代理。
func WithProxyURL(proxyURL string) Option {
	return func(cfg *clientConfig) {
		cfg.proxyURL = proxyURL
	}
}

// WithHeader 添加每次请求都会携带的默认 HTTP 头。
func WithHeader(key, value string) Option {
	return func(cfg *clientConfig) {
		if cfg.headers == nil {
			cfg.headers = map[string]string{}
		}
		cfg.headers[key] = value
	}
}

// WithOrganization 配置 OpenAI-Organization 请求头。
func WithOrganization(organization string) Option {
	return WithHeader("OpenAI-Organization", organization)
}

// WithProject 配置 OpenAI-Project 请求头。
func WithProject(project string) Option {
	return WithHeader("OpenAI-Project", project)
}

// NewClient 创建只调用 OpenAI 官方 API 的客户端。
// 未指定 BaseURL 时固定使用 DefaultBaseURL。
func NewClient(opts ...Option) *Client {
	cfg := clientConfig{baseURL: DefaultBaseURL}
	for _, opt := range opts {
		opt(&cfg)
	}

	baseURL := strings.TrimRight(strings.TrimSpace(cfg.baseURL), "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	httpClient := cfg.httpClient
	var initErr error
	if httpClient == nil && strings.TrimSpace(cfg.proxyURL) != "" {
		httpClient, initErr = transport.NewHTTPClient(cfg.proxyURL)
		if initErr != nil {
			initErr = fmt.Errorf("proxyapi: invalid proxy URL: %w", initErr)
		}
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	c := &Client{
		baseURL:    baseURL,
		credential: cfg.credential,
		httpClient: httpClient,
		headers:    cloneHeaders(cfg.headers),
		initErr:    initErr,
	}
	c.Responses = &ResponsesService{client: c}
	c.Embeddings = &EmbeddingsService{client: c}
	c.Models = &ModelsService{client: c}
	return c
}

// BaseURL 返回客户端当前使用的 API 地址。
func (c *Client) BaseURL() string {
	if c == nil {
		return ""
	}
	return c.baseURL
}

// Credential 返回客户端默认请求凭据。
func (c *Client) Credential() auth.Credential {
	if c == nil {
		return nil
	}
	return c.credential
}

// newRequest 创建已注入鉴权信息和默认请求头的 OpenAI HTTP 请求。
// requestCredential 非空时优先于客户端默认凭据。
func (c *Client) newRequest(ctx context.Context, method, path string, body []byte, accept string, requestCredential auth.Credential) (*http.Request, error) {
	if c == nil {
		return nil, errors.New("proxyapi: client is nil")
	}
	if c.initErr != nil {
		return nil, c.initErr
	}
	credential := c.credential
	if requestCredential != nil {
		credential = requestCredential
	}
	if credential == nil {
		return nil, errors.New("proxyapi: OpenAI credential is required")
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	for key, value := range c.headers {
		req.Header.Set(key, value)
	}
	authorization, err := credential.AuthorizationHeader(ctx)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", authorization)
	return req, nil
}

// cloneHeaders 复制请求头配置，避免客户端与调用方共享可变 map。
func cloneHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	out := make(map[string]string, len(headers))
	for key, value := range headers {
		out[key] = value
	}
	return out
}
