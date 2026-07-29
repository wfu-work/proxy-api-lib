package auth

import (
	"context"
	"errors"
	"os"
	"strings"
)

// Credential 定义 OpenAI 官方请求所需的鉴权信息提供方式。
type Credential interface {
	AuthorizationHeader(ctx context.Context) (string, error)
}

// TokenSource 按请求解析由调用方管理的访问令牌。
// 令牌的刷新与持久化由调用方负责。
type TokenSource interface {
	Token(ctx context.Context) (string, error)
}

// APIKey 根据 OpenAI API Key 创建固定凭据。
func APIKey(key string) APIKeyCredential {
	return APIKeyCredential{Key: key}
}

// APIKeyCredential 使用 Authorization: Bearer 头发送 OpenAI API Key。
type APIKeyCredential struct {
	Key string
}

// AuthorizationHeader 校验 API Key，并返回可直接写入 HTTP 请求的鉴权头。
func (c APIKeyCredential) AuthorizationHeader(context.Context) (string, error) {
	key := strings.TrimSpace(c.Key)
	if key == "" {
		return "", errors.New("auth: api key is empty")
	}
	return "Bearer " + key, nil
}

// BearerToken 根据调用方提供的 Bearer Token 创建固定凭据。
func BearerToken(token string) BearerTokenCredential {
	return BearerTokenCredential{Token: token}
}

// FromTokenSource 创建按请求动态读取令牌的凭据。
func FromTokenSource(source TokenSource) TokenSourceCredential {
	return TokenSourceCredential{Source: source}
}

// TokenSourceCredential 在每次请求时通过 TokenSource 获取 Bearer Token。
type TokenSourceCredential struct {
	Source TokenSource
}

// AuthorizationHeader 从动态令牌源获取令牌，并生成 HTTP 鉴权头。
func (c TokenSourceCredential) AuthorizationHeader(ctx context.Context) (string, error) {
	if c.Source == nil {
		return "", errors.New("auth: token source is nil")
	}
	token, err := c.Source.Token(ctx)
	if err != nil {
		return "", err
	}
	return BearerToken(token).AuthorizationHeader(ctx)
}

// BearerTokenCredential 使用 Authorization: Bearer 头发送访问令牌。
type BearerTokenCredential struct {
	Token string
}

// AuthorizationHeader 校验访问令牌，并确保返回值包含 Bearer 前缀。
func (c BearerTokenCredential) AuthorizationHeader(context.Context) (string, error) {
	token := strings.TrimSpace(c.Token)
	if token == "" {
		return "", errors.New("auth: bearer token is empty")
	}
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		return token, nil
	}
	return "Bearer " + token, nil
}

// Env 创建从指定环境变量读取 API Key 的凭据。
func Env(name string) EnvCredential {
	return EnvCredential{Name: name}
}

// EnvCredential 从环境变量中解析 API Key 或 Bearer Token。
type EnvCredential struct {
	Name   string
	Bearer bool
}

// AuthorizationHeader 读取环境变量，并根据 Bearer 配置生成对应鉴权头。
func (c EnvCredential) AuthorizationHeader(ctx context.Context) (string, error) {
	value := strings.TrimSpace(os.Getenv(c.Name))
	if value == "" {
		return "", errors.New("auth: environment credential is empty")
	}
	if c.Bearer {
		return BearerToken(value).AuthorizationHeader(ctx)
	}
	return APIKey(value).AuthorizationHeader(ctx)
}
