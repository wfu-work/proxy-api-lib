package codexauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	// DefaultIssuer 是 OpenAI OAuth 的默认签发方地址。
	DefaultIssuer = "https://auth.openai.com"
	// DefaultClientID 是 Codex CLI 当前使用的 OAuth Client ID。
	// 该值属于 Codex 登录协议的一部分，OpenAI 可能在未来调整。
	DefaultClientID = "app_EMoamEEZ73f0CkXaXp7hrann"
	// DefaultRefreshScopes 是刷新 Codex 登录会话时请求的基础 OpenID scopes。
	DefaultRefreshScopes = "openid profile email"
)

// TokenSet 是 OpenAI OAuth Token 接口返回的令牌集合。
type TokenSet struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	TokenType    string `json:"token_type,omitempty"`
	Scope        string `json:"scope,omitempty"`
	ExpiresIn    int64  `json:"expires_in,omitempty"`
}

// EffectiveRefreshToken 返回服务端轮换后的 Refresh Token；响应未返回新值时保留旧值。
func (t TokenSet) EffectiveRefreshToken(previous string) string {
	if value := strings.TrimSpace(t.RefreshToken); value != "" {
		return value
	}
	return strings.TrimSpace(previous)
}

// OAuthError 描述 OpenAI OAuth Token 接口返回的错误。
type OAuthError struct {
	StatusCode  int
	Code        string
	Description string
	RequestID   string
	Cause       error
}

// Error 返回 OAuth 错误的可读文本。
func (e *OAuthError) Error() string {
	if e == nil {
		return ""
	}
	message := strings.TrimSpace(e.Description)
	if message == "" {
		message = strings.TrimSpace(e.Code)
	}
	if message == "" {
		message = fmt.Sprintf("status %d", e.StatusCode)
	}
	if e.Code != "" && e.Code != message {
		return fmt.Sprintf("codexauth: %s (%s)", message, e.Code)
	}
	return "codexauth: " + message
}

// Unwrap 返回 OAuth 请求的底层错误。
func (e *OAuthError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// OAuthClient 负责执行 Codex OAuth Token 刷新。
// Token 的加密存储、轮换持久化和刷新调度由调用方负责。
type OAuthClient struct {
	issuer     string
	clientID   string
	scopes     string
	httpClient *http.Client
}

// OAuthOption 配置 OAuthClient。
type OAuthOption func(*OAuthClient)

// WithIssuer 覆盖 OAuth 签发方，主要用于受控部署和测试。
func WithIssuer(issuer string) OAuthOption {
	return func(client *OAuthClient) { client.issuer = issuer }
}

// WithClientID 配置 OAuth Client ID。
func WithClientID(clientID string) OAuthOption {
	return func(client *OAuthClient) { client.clientID = clientID }
}

// WithScopes 配置刷新请求携带的 OAuth scopes。
func WithScopes(scopes string) OAuthOption {
	return func(client *OAuthClient) { client.scopes = scopes }
}

// WithHTTPClient 配置 OAuth 请求使用的 HTTP 客户端。
func WithHTTPClient(client *http.Client) OAuthOption {
	return func(oauth *OAuthClient) { oauth.httpClient = client }
}

// NewOAuthClient 创建 Codex OAuth Token 客户端。
func NewOAuthClient(opts ...OAuthOption) *OAuthClient {
	client := &OAuthClient{
		issuer:     DefaultIssuer,
		clientID:   DefaultClientID,
		scopes:     DefaultRefreshScopes,
		httpClient: http.DefaultClient,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(client)
		}
	}
	if client.httpClient == nil {
		client.httpClient = http.DefaultClient
	}
	return client
}

// TokenURL 返回当前 OAuth 签发方的 Token 地址。
func (c *OAuthClient) TokenURL() string {
	if c == nil {
		return ""
	}
	issuer := strings.TrimRight(strings.TrimSpace(c.issuer), "/")
	if issuer == "" {
		issuer = DefaultIssuer
	}
	return issuer + "/oauth/token"
}

// Refresh 使用 Refresh Token 获取新的 Access Token。
// 如果 OpenAI 返回新的 Refresh Token，调用方必须原子替换持久化的旧值。
func (c *OAuthClient) Refresh(ctx context.Context, refreshToken string) (*TokenSet, error) {
	if c == nil {
		return nil, errors.New("codexauth: OAuth client is nil")
	}
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil, errors.New("codexauth: refresh token is empty")
	}
	clientID := strings.TrimSpace(c.clientID)
	if clientID == "" {
		return nil, errors.New("codexauth: OAuth client ID is empty")
	}
	form := url.Values{
		"client_id":     {clientID},
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}
	if scopes := strings.TrimSpace(c.scopes); scopes != "" {
		form.Set("scope", scopes)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.TokenURL(), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &OAuthError{Cause: err, Description: err.Error()}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, &OAuthError{StatusCode: resp.StatusCode, Cause: err, Description: err.Error()}
	}
	requestID := responseRequestID(resp.Header)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, parseOAuthError(resp.StatusCode, requestID, body)
	}
	var tokens TokenSet
	if err := json.Unmarshal(body, &tokens); err != nil {
		return nil, &OAuthError{StatusCode: resp.StatusCode, RequestID: requestID, Cause: err, Description: "invalid token response"}
	}
	if strings.TrimSpace(tokens.AccessToken) == "" {
		return nil, &OAuthError{StatusCode: resp.StatusCode, RequestID: requestID, Description: "token response does not contain access_token"}
	}
	return &tokens, nil
}

// parseOAuthError 解析标准 OAuth 错误结构，并为非标准响应保留简短消息。
func parseOAuthError(statusCode int, requestID string, body []byte) error {
	var envelope struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
		Message          string `json:"message"`
	}
	_ = json.Unmarshal(body, &envelope)
	description := strings.TrimSpace(envelope.ErrorDescription)
	if description == "" {
		description = strings.TrimSpace(envelope.Message)
	}
	if description == "" {
		description = compactBody(body)
	}
	return &OAuthError{
		StatusCode:  statusCode,
		Code:        strings.TrimSpace(envelope.Error),
		Description: description,
		RequestID:   requestID,
	}
}

// responseRequestID 读取 OpenAI 常用的请求追踪头。
func responseRequestID(header http.Header) string {
	if value := strings.TrimSpace(header.Get("x-request-id")); value != "" {
		return value
	}
	return strings.TrimSpace(header.Get("x-oai-request-id"))
}

// compactBody 压缩并截断非标准错误响应，避免错误对象携带过大的 HTML 页面。
func compactBody(body []byte) string {
	message := strings.Join(strings.Fields(string(body)), " ")
	const maxLength = 512
	if len(message) > maxLength {
		message = message[:maxLength] + "..."
	}
	return message
}
