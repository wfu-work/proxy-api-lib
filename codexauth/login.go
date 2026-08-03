package codexauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultAuthorizeScopes 是 Codex CLI 浏览器授权使用的基础 Scope 集合。
	DefaultAuthorizeScopes = "openid profile email offline_access api.connectors.read api.connectors.invoke"
	// DefaultOriginator 标识请求来自兼容 Codex CLI 的本地客户端。
	DefaultOriginator = "codex_cli_rs"
)

// PKCECodes 保存 OAuth Authorization Code 流程的一次性校验材料。
type PKCECodes struct {
	CodeVerifier  string `json:"-"`
	CodeChallenge string `json:"codeChallenge"`
}

// AuthorizationRequest 描述生成 OpenAI 浏览器授权地址所需的参数。
type AuthorizationRequest struct {
	RedirectURI   string
	CodeChallenge string
	State         string
	Originator    string
	WorkspaceID   string
}

// DeviceAuthorization 是设备码登录开始后返回给用户的展示信息。
type DeviceAuthorization struct {
	VerificationURL string `json:"verificationUrl"`
	UserCode        string `json:"userCode"`
	DeviceAuthID    string `json:"-"`
	IntervalSeconds int64  `json:"intervalSeconds"`
}

type deviceTokenResponse struct {
	AuthorizationCode string `json:"authorization_code"`
	CodeVerifier      string `json:"code_verifier"`
}

// GeneratePKCE 生成符合 RFC 7636 的 S256 PKCE 校验材料。
func GeneratePKCE() (PKCECodes, error) {
	raw := make([]byte, 64)
	if _, err := rand.Read(raw); err != nil {
		return PKCECodes{}, fmt.Errorf("codexauth: generate PKCE verifier: %w", err)
	}
	verifier := base64.RawURLEncoding.EncodeToString(raw)
	digest := sha256.Sum256([]byte(verifier))
	return PKCECodes{
		CodeVerifier: verifier, CodeChallenge: base64.RawURLEncoding.EncodeToString(digest[:]),
	}, nil
}

// GenerateState 生成浏览器授权使用的高强度一次性 State。
func GenerateState() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("codexauth: generate OAuth state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// AuthorizeURL 构造兼容 Codex CLI 的 OpenAI 浏览器授权地址。
func (c *OAuthClient) AuthorizeURL(input AuthorizationRequest) (string, error) {
	if c == nil {
		return "", errors.New("codexauth: OAuth client is nil")
	}
	redirectURI := strings.TrimSpace(input.RedirectURI)
	challenge := strings.TrimSpace(input.CodeChallenge)
	state := strings.TrimSpace(input.State)
	if redirectURI == "" || challenge == "" || state == "" {
		return "", errors.New("codexauth: redirect URI, PKCE challenge and state are required")
	}
	issuer := strings.TrimRight(strings.TrimSpace(c.issuer), "/")
	if issuer == "" {
		issuer = DefaultIssuer
	}
	originator := strings.TrimSpace(input.Originator)
	if originator == "" {
		originator = DefaultOriginator
	}
	query := url.Values{
		"response_type":              {"code"},
		"client_id":                  {c.clientID},
		"redirect_uri":               {redirectURI},
		"scope":                      {DefaultAuthorizeScopes},
		"code_challenge":             {challenge},
		"code_challenge_method":      {"S256"},
		"id_token_add_organizations": {"true"},
		"codex_cli_simplified_flow":  {"true"},
		"state":                      {state},
		"originator":                 {originator},
	}
	if workspaceID := strings.TrimSpace(input.WorkspaceID); workspaceID != "" {
		query.Set("allowed_workspace_id", workspaceID)
	}
	return issuer + "/oauth/authorize?" + query.Encode(), nil
}

// ExchangeAuthorizationCode 使用授权码和 PKCE Verifier 换取 OAuth Token。
func (c *OAuthClient) ExchangeAuthorizationCode(ctx context.Context, code, redirectURI, codeVerifier string) (*TokenSet, error) {
	if c == nil {
		return nil, errors.New("codexauth: OAuth client is nil")
	}
	code = strings.TrimSpace(code)
	redirectURI = strings.TrimSpace(redirectURI)
	codeVerifier = strings.TrimSpace(codeVerifier)
	if code == "" || redirectURI == "" || codeVerifier == "" {
		return nil, errors.New("codexauth: authorization code, redirect URI and PKCE verifier are required")
	}
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {c.clientID},
		"code_verifier": {codeVerifier},
	}
	return c.exchangeToken(ctx, form)
}

// StartDeviceAuthorization 请求 OpenAI 设备码登录信息。
func (c *OAuthClient) StartDeviceAuthorization(ctx context.Context) (*DeviceAuthorization, error) {
	if c == nil {
		return nil, errors.New("codexauth: OAuth client is nil")
	}
	issuer := strings.TrimRight(strings.TrimSpace(c.issuer), "/")
	if issuer == "" {
		issuer = DefaultIssuer
	}
	body, err := json.Marshal(map[string]string{"client_id": c.clientID})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, issuer+"/api/accounts/deviceauth/usercode", strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &OAuthError{Cause: err, Description: err.Error()}
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, parseOAuthError(resp.StatusCode, responseRequestID(resp.Header), responseBody)
	}
	var payload struct {
		DeviceAuthID string `json:"device_auth_id"`
		UserCode     string `json:"user_code"`
		Interval     int64  `json:"interval"`
	}
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return nil, fmt.Errorf("codexauth: invalid device authorization response: %w", err)
	}
	if strings.TrimSpace(payload.DeviceAuthID) == "" || strings.TrimSpace(payload.UserCode) == "" {
		return nil, errors.New("codexauth: device authorization response is incomplete")
	}
	if payload.Interval <= 0 {
		payload.Interval = 5
	}
	return &DeviceAuthorization{
		VerificationURL: issuer + "/codex/device", UserCode: payload.UserCode,
		DeviceAuthID: payload.DeviceAuthID, IntervalSeconds: payload.Interval,
	}, nil
}

// CompleteDeviceAuthorization 轮询设备授权结果，并在成功后换取 OAuth Token。
func (c *OAuthClient) CompleteDeviceAuthorization(ctx context.Context, device DeviceAuthorization) (*TokenSet, error) {
	if c == nil {
		return nil, errors.New("codexauth: OAuth client is nil")
	}
	if strings.TrimSpace(device.DeviceAuthID) == "" || strings.TrimSpace(device.UserCode) == "" {
		return nil, errors.New("codexauth: device authorization identity is required")
	}
	interval := time.Duration(device.IntervalSeconds) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}
	for {
		result, pending, err := c.pollDeviceAuthorization(ctx, device)
		if err != nil {
			return nil, err
		}
		if !pending {
			return c.ExchangeAuthorizationCode(ctx, result.AuthorizationCode, c.DeviceRedirectURI(), result.CodeVerifier)
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

// DeviceRedirectURI 返回设备码授权完成时使用的固定回调地址。
func (c *OAuthClient) DeviceRedirectURI() string {
	issuer := DefaultIssuer
	if c != nil && strings.TrimSpace(c.issuer) != "" {
		issuer = strings.TrimRight(strings.TrimSpace(c.issuer), "/")
	}
	return issuer + "/deviceauth/callback"
}

func (c *OAuthClient) pollDeviceAuthorization(ctx context.Context, device DeviceAuthorization) (deviceTokenResponse, bool, error) {
	issuer := strings.TrimRight(strings.TrimSpace(c.issuer), "/")
	body, err := json.Marshal(map[string]string{"device_auth_id": device.DeviceAuthID, "user_code": device.UserCode})
	if err != nil {
		return deviceTokenResponse{}, false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, issuer+"/api/accounts/deviceauth/token", strings.NewReader(string(body)))
	if err != nil {
		return deviceTokenResponse{}, false, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return deviceTokenResponse{}, false, &OAuthError{Cause: err, Description: err.Error()}
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return deviceTokenResponse{}, false, err
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound {
		return deviceTokenResponse{}, true, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return deviceTokenResponse{}, false, parseOAuthError(resp.StatusCode, responseRequestID(resp.Header), responseBody)
	}
	var result deviceTokenResponse
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return deviceTokenResponse{}, false, fmt.Errorf("codexauth: invalid device token response: %w", err)
	}
	if strings.TrimSpace(result.AuthorizationCode) == "" || strings.TrimSpace(result.CodeVerifier) == "" {
		return deviceTokenResponse{}, false, errors.New("codexauth: device token response is incomplete")
	}
	return result, false, nil
}

func (c *OAuthClient) exchangeToken(ctx context.Context, form url.Values) (*TokenSet, error) {
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

func retryAfterSeconds(header http.Header) int64 {
	value := strings.TrimSpace(header.Get("Retry-After"))
	seconds, _ := strconv.ParseInt(value, 10, 64)
	return seconds
}
