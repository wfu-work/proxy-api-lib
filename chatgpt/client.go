// Package chatgpt 封装 ChatGPT/Codex 账号用量与订阅查询协议。
//
// 这些接口由 OpenAI 自有服务提供，但不属于公开稳定的 OpenAI Platform API。
// 调用方应将本包作为可独立升级的账号协议适配层，不要依赖它完成授权判断或账务结算。
package chatgpt

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/wfu-work/proxy-api-lib/auth"
	"github.com/wfu-work/proxy-api-lib/transport"
)

const (
	// DefaultBaseURL 是 ChatGPT 官方站点地址。
	DefaultBaseURL = "https://chatgpt.com"
	// DefaultOriginator 是 Codex CLI 请求使用的默认来源标识。
	DefaultOriginator = "codex_cli_rs"
	// DefaultCodexClientVersion 是模型目录兼容性判断使用的官方 Codex 客户端版本。
	// GPT-5.6 系列要求客户端版本不低于 0.144.0。
	DefaultCodexClientVersion = "0.144.0"
	// DefaultUserAgent 是官方 Codex 请求使用的默认客户端标识。
	DefaultUserAgent = DefaultOriginator + "/" + DefaultCodexClientVersion
)

// Client 调用 ChatGPT/Codex 账号内部接口。
// Token 刷新、加密存储、账号轮询和故障切换由调用方负责。
type Client struct {
	baseURL    string
	credential auth.Credential
	httpClient *http.Client
	headers    map[string]string
	initErr    error

	Usage    *UsageService
	Accounts *AccountsService
	Codex    *CodexService
	Resets   *RateLimitResetService
}

type clientConfig struct {
	baseURL    string
	credential auth.Credential
	httpClient *http.Client
	proxyURL   string
	headers    map[string]string
}

// Option 表示一项 ChatGPT 账号客户端配置。
type Option func(*clientConfig)

// WithCredential 配置 ChatGPT OAuth 请求凭据。
// 该凭据应产生 Bearer Access Token，而不是 OpenAI Platform API Key。
func WithCredential(credential auth.Credential) Option {
	return func(cfg *clientConfig) { cfg.credential = credential }
}

// WithAccessToken 配置固定的 ChatGPT OAuth Access Token。
func WithAccessToken(token string) Option {
	return WithCredential(auth.BearerToken(token))
}

// WithTokenSource 配置按请求读取最新 Access Token 的动态令牌源。
// 调用方可在 TokenSource 中实现安全的刷新和原子持久化。
func WithTokenSource(source auth.TokenSource) Option {
	return WithCredential(auth.FromTokenSource(source))
}

// WithBaseURL 覆盖 ChatGPT 官方地址，主要用于测试或受控部署。
func WithBaseURL(baseURL string) Option {
	return func(cfg *clientConfig) { cfg.baseURL = baseURL }
}

// WithHTTPClient 配置账号请求使用的 HTTP 客户端。
func WithHTTPClient(client *http.Client) Option {
	return func(cfg *clientConfig) { cfg.httpClient = client }
}

// WithProxyURL 配置访问 ChatGPT 官方服务时使用的网络代理。
func WithProxyURL(proxyURL string) Option {
	return func(cfg *clientConfig) { cfg.proxyURL = proxyURL }
}

// WithHeader 添加每次账号请求都会携带的默认 HTTP 头。
func WithHeader(key, value string) Option {
	return func(cfg *clientConfig) {
		if cfg.headers == nil {
			cfg.headers = map[string]string{}
		}
		cfg.headers[key] = value
	}
}

// WithOriginator 配置 Codex 请求来源标识。
func WithOriginator(originator string) Option {
	return WithHeader("originator", originator)
}

// WithUserAgent 配置账号请求使用的 User-Agent。
func WithUserAgent(userAgent string) Option {
	return WithHeader("User-Agent", userAgent)
}

// NewClient 创建 ChatGPT/Codex 账号客户端。
func NewClient(opts ...Option) *Client {
	cfg := clientConfig{
		baseURL: DefaultBaseURL,
		headers: map[string]string{"originator": DefaultOriginator, "User-Agent": DefaultUserAgent},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.baseURL), "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if !strings.HasSuffix(baseURL, "/backend-api") {
		baseURL += "/backend-api"
	}
	httpClient := cfg.httpClient
	var initErr error
	if httpClient == nil && strings.TrimSpace(cfg.proxyURL) != "" {
		httpClient, initErr = transport.NewHTTPClient(cfg.proxyURL)
		if initErr != nil {
			initErr = fmt.Errorf("chatgpt: invalid proxy URL: %w", initErr)
		}
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	client := &Client{
		baseURL:    baseURL,
		credential: cfg.credential,
		httpClient: httpClient,
		headers:    cloneHeaders(cfg.headers),
		initErr:    initErr,
	}
	client.Usage = &UsageService{client: client}
	client.Accounts = &AccountsService{client: client}
	client.Codex = &CodexService{client: client}
	client.Resets = &RateLimitResetService{client: client}
	return client
}

// BaseURL 返回当前使用的 ChatGPT backend-api 地址。
func (c *Client) BaseURL() string {
	if c == nil {
		return ""
	}
	return c.baseURL
}

// Credential 返回客户端使用的 ChatGPT OAuth 凭据。
func (c *Client) Credential() auth.Credential {
	if c == nil {
		return nil
	}
	return c.credential
}

// newRequest 创建带 OAuth 鉴权和账号路由头的 ChatGPT 请求。
func (c *Client) newRequest(ctx context.Context, method, path, accountID string, extraHeaders map[string]string) (*http.Request, error) {
	return c.newRequestBody(ctx, method, path, accountID, nil, extraHeaders)
}

// newRequestBody 创建带 OAuth 鉴权、账号路由头和可选请求体的 ChatGPT 请求。
func (c *Client) newRequestBody(ctx context.Context, method, path, accountID string, body []byte, extraHeaders map[string]string) (*http.Request, error) {
	return c.newRequestBodyURL(ctx, method, c.baseURL+path, accountID, body, extraHeaders)
}

// newRootRequest 创建相对于 ChatGPT 站点根路径的请求。
// 少量 Codex 账号接口同时存在 /backend-api 与 /api/codex 两套路由，
// 路由兼容必须留在协议库中，不能由业务应用重复拼接官方 URL 和请求头。
func (c *Client) newRootRequest(ctx context.Context, method, path, accountID string, body []byte, extraHeaders map[string]string) (*http.Request, error) {
	rootURL := strings.TrimSuffix(c.baseURL, "/backend-api")
	return c.newRequestBodyURL(ctx, method, rootURL+path, accountID, body, extraHeaders)
}

func (c *Client) newRequestBodyURL(ctx context.Context, method, requestURL, accountID string, body []byte, extraHeaders map[string]string) (*http.Request, error) {
	if c == nil {
		return nil, errors.New("chatgpt: client is nil")
	}
	if c.initErr != nil {
		return nil, c.initErr
	}
	if c.credential == nil {
		return nil, errors.New("chatgpt: OAuth access token is required")
	}
	req, err := http.NewRequestWithContext(ctx, method, requestURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	for key, value := range c.headers {
		req.Header.Set(key, value)
	}
	for key, value := range extraHeaders {
		req.Header.Set(key, value)
	}
	if accountID = strings.TrimSpace(accountID); accountID != "" {
		req.Header.Set("ChatGPT-Account-ID", accountID)
	}
	authorization, err := c.credential.AuthorizationHeader(ctx)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", authorization)
	return req, nil
}

// doJSON 执行请求并读取 JSON 响应，同时统一转换非成功状态。
func (c *Client) doJSON(req *http.Request) ([]byte, string, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, responseRequestID(resp.Header), err
	}
	requestID := responseRequestID(resp.Header)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, requestID, parseAPIError(resp.StatusCode, req.URL.Path, requestID, body)
	}
	return body, requestID, nil
}

// cloneHeaders 复制请求头，避免客户端与调用方共享可变 map。
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
