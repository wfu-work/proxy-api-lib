// Package codexauth 提供 ChatGPT/Codex OAuth Token 的低层解析与刷新能力。
//
// 该包不负责浏览器登录回调、Token 持久化或账号路由。调用方必须像保护密码一样
// 保护 Access Token、Refresh Token 和 ID Token。
package codexauth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Claims 是 ChatGPT/Codex JWT 中与账号展示和请求路由有关的字段。
// 未建模的 JWT 字段不会参与鉴权判断。
type Claims struct {
	Subject          string         `json:"sub,omitempty"`
	ClientID         string         `json:"client_id,omitempty"`
	Email            string         `json:"email,omitempty"`
	WorkspaceID      string         `json:"workspace_id,omitempty"`
	OrganizationID   string         `json:"organization_id,omitempty"`
	ChatGPTAccountID string         `json:"chatgpt_account_id,omitempty"`
	Expires          int64          `json:"exp,omitempty"`
	Auth             *AuthClaims    `json:"https://api.openai.com/auth,omitempty"`
	Profile          *ProfileClaims `json:"https://api.openai.com/profile,omitempty"`
}

// AuthClaims 是 OpenAI 自定义 auth claim 中常见的 ChatGPT 账号字段。
type AuthClaims struct {
	ChatGPTAccountID string               `json:"chatgpt_account_id,omitempty"`
	ChatGPTPlanType  string               `json:"chatgpt_plan_type,omitempty"`
	ChatGPTUserID    string               `json:"chatgpt_user_id,omitempty"`
	UserID           string               `json:"user_id,omitempty"`
	WorkspaceID      string               `json:"workspace_id,omitempty"`
	OrganizationID   string               `json:"organization_id,omitempty"`
	Organizations    []OrganizationClaims `json:"organizations,omitempty"`
}

// OrganizationClaims 描述 JWT 中的一个 OpenAI 组织或工作区。
type OrganizationClaims struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	IsDefault bool   `json:"is_default,omitempty"`
}

// ProfileClaims 是 OpenAI 自定义 profile claim 中的基础用户资料。
type ProfileClaims struct {
	Email string `json:"email,omitempty"`
	Name  string `json:"name,omitempty"`
}

// ParseUnverifiedClaims 仅对 JWT Payload 做 Base64URL 解码和 JSON 解析。
// 该方法不会验证签名、签发者或受众，不得将结果用于授权决策；它只适合从已经由
// OpenAI 服务端验证的调用方自有 Token 中提取显示和路由信息。
func ParseUnverifiedClaims(token string) (Claims, error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
		return Claims{}, errors.New("codexauth: invalid JWT")
	}
	payload, err := decodeJWTPart(parts[1])
	if err != nil {
		return Claims{}, fmt.Errorf("codexauth: decode JWT payload: %w", err)
	}
	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return Claims{}, fmt.Errorf("codexauth: decode JWT claims: %w", err)
	}
	return claims, nil
}

// ResolvedEmail 返回顶层 email；为空时回退到 OpenAI profile claim。
func (c Claims) ResolvedEmail() string {
	if email := strings.TrimSpace(c.Email); email != "" {
		return email
	}
	if c.Profile != nil {
		return strings.TrimSpace(c.Profile.Email)
	}
	return ""
}

// ResolvedName 返回 OpenAI profile claim 中的用户名称。
func (c Claims) ResolvedName() string {
	if c.Profile == nil {
		return ""
	}
	return strings.TrimSpace(c.Profile.Name)
}

// ResolvedAccountID 返回用于 ChatGPT-Account-ID 路由的账号标识。
func (c Claims) ResolvedAccountID() string {
	values := []string{c.ChatGPTAccountID}
	if c.Auth != nil {
		values = append(values, c.Auth.ChatGPTAccountID)
	}
	values = append(values, c.WorkspaceID)
	for _, value := range values {
		if normalized := normalizeScopedIdentity(value, "cgpt="); normalized != "" {
			return normalized
		}
	}
	return ""
}

// ResolvedWorkspaceID 返回 JWT 中的默认工作区或组织标识。
func (c Claims) ResolvedWorkspaceID() string {
	values := []string{c.WorkspaceID, c.OrganizationID}
	if c.Auth != nil {
		values = append(values, c.Auth.WorkspaceID, c.Auth.OrganizationID)
		for _, organization := range c.Auth.Organizations {
			if organization.IsDefault {
				values = append([]string{organization.ID}, values...)
				break
			}
		}
		for _, organization := range c.Auth.Organizations {
			values = append(values, organization.ID)
		}
		values = append(values, c.Auth.ChatGPTAccountID)
	}
	for _, value := range values {
		if normalized := normalizeScopedIdentity(value, "ws="); normalized != "" {
			return normalized
		}
	}
	return ""
}

// ResolvedUserID 返回 ChatGPT 用户标识，缺失时回退到 JWT subject。
func (c Claims) ResolvedUserID() string {
	if c.Auth != nil {
		for _, value := range []string{c.Auth.ChatGPTUserID, c.Auth.UserID} {
			if value = strings.TrimSpace(value); value != "" {
				return value
			}
		}
	}
	return strings.TrimSpace(c.Subject)
}

// ResolvedPlanType 返回 JWT 中的 ChatGPT 套餐类型。
func (c Claims) ResolvedPlanType() string {
	if c.Auth == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(c.Auth.ChatGPTPlanType))
}

// TokenExpiresAt 返回 JWT exp 对应的 Token 到期时间。
// 该时间是 Token 有效期，不是 ChatGPT 订阅到期时间。
func (c Claims) TokenExpiresAt() (time.Time, bool) {
	if c.Expires <= 0 {
		return time.Time{}, false
	}
	return time.Unix(c.Expires, 0), true
}

// decodeJWTPart 解码带或不带填充的 Base64URL JWT 段。
func decodeJWTPart(value string) ([]byte, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err == nil {
		return decoded, nil
	}
	return base64.URLEncoding.DecodeString(value)
}

// normalizeScopedIdentity 从 Codex 复合身份值中提取 cgpt= 或 ws= 标记。
func normalizeScopedIdentity(value, marker string) string {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return ""
	}
	scoped := raw
	if index := strings.LastIndex(raw, "::"); index >= 0 {
		scoped = raw[index+2:]
	}
	for _, segment := range strings.Split(scoped, "|") {
		if found, ok := strings.CutPrefix(strings.TrimSpace(segment), marker); ok {
			return strings.TrimSpace(found)
		}
	}
	if strings.ContainsAny(raw, "|=") || strings.Contains(raw, "::") {
		return ""
	}
	return raw
}
