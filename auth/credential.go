package auth

import (
	"context"
	"errors"
	"os"
	"strings"
)

// APIKey returns a credential for OpenAI-style API keys.
func APIKey(key string) APIKeyCredential {
	return APIKeyCredential{Key: key}
}

// APIKeyCredential sends an API key as Authorization: Bearer.
type APIKeyCredential struct {
	Key string
}

func (c APIKeyCredential) AuthorizationHeader(context.Context) (string, error) {
	key := strings.TrimSpace(c.Key)
	if key == "" {
		return "", errors.New("auth: api key is empty")
	}
	return "Bearer " + key, nil
}

// BearerToken returns a credential for an explicit bearer token.
func BearerToken(token string) BearerTokenCredential {
	return BearerTokenCredential{Token: token}
}

// BearerTokenCredential sends a bearer token as Authorization: Bearer.
type BearerTokenCredential struct {
	Token string
}

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

// Env returns a credential that reads its value from an environment variable.
func Env(name string) EnvCredential {
	return EnvCredential{Name: name}
}

// EnvCredential resolves API keys or tokens from environment variables.
type EnvCredential struct {
	Name   string
	Bearer bool
}

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
