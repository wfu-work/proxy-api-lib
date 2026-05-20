package proxyapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/wfu-work/proxy-api-lib/auth"
	"github.com/wfu-work/proxy-api-lib/domains"
)

// Provider sends normalized requests to an upstream AI provider.
type Provider interface {
	Name() string
	CreateResponse(ctx context.Context, req domains.ResponseRequest) (*domains.Response, error)
}

// StreamProvider is implemented by providers that support streaming.
type StreamProvider interface {
	StreamResponse(ctx context.Context, req domains.ResponseRequest) (*domains.ResponseStream, error)
}

// Client is the top-level entry point for model APIs.
type Client struct {
	provider   Provider
	credential domains.Credential

	Responses *ResponsesService
}

type clientConfig struct {
	provider   Provider
	credential domains.Credential
	httpClient *http.Client
}

// Option configures a Client.
type Option func(*clientConfig)

// WithProvider configures the upstream provider.
func WithProvider(provider Provider) Option {
	return func(cfg *clientConfig) {
		cfg.provider = provider
	}
}

// WithCredential configures the client-level credential.
func WithCredential(credential domains.Credential) Option {
	return func(cfg *clientConfig) {
		cfg.credential = credential
	}
}

// WithAPIKey configures an API key credential sent as Authorization: Bearer.
func WithAPIKey(key string) Option {
	return WithCredential(auth.APIKey(key))
}

// WithBearerToken configures a bearer token credential.
func WithBearerToken(token string) Option {
	return WithCredential(auth.BearerToken(token))
}

// NewClient builds a client from functional options.
func NewClient(opts ...Option) *Client {
	cfg := clientConfig{
		httpClient: http.DefaultClient,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	c := &Client{
		provider:   cfg.provider,
		credential: cfg.credential,
	}
	c.Responses = &ResponsesService{client: c}
	return c
}

// Provider returns the configured upstream provider.
func (c *Client) Provider() Provider {
	return c.provider
}

// Credential returns the configured client-level credential.
func (c *Client) Credential() domains.Credential {
	return c.credential
}

func (c *Client) withCredential(req domains.ResponseRequest) domains.ResponseRequest {
	if req.Credential == nil {
		req.Credential = c.credential
	}
	return req
}

func (c *Client) requireProvider() (Provider, error) {
	if c.provider == nil {
		return nil, errors.New("proxyapi: provider is required")
	}
	return c.provider, nil
}

// ResponsesService exposes the Responses API surface.
type ResponsesService struct {
	client *Client
}

// Create sends a non-streaming Responses API request.
func (s *ResponsesService) Create(ctx context.Context, req domains.ResponseRequest) (*domains.Response, error) {
	provider, err := s.client.requireProvider()
	if err != nil {
		return nil, err
	}
	return provider.CreateResponse(ctx, s.client.withCredential(req))
}

// Stream sends a streaming Responses API request when supported by the provider.
func (s *ResponsesService) Stream(ctx context.Context, req domains.ResponseRequest) (*domains.ResponseStream, error) {
	provider, err := s.client.requireProvider()
	if err != nil {
		return nil, err
	}
	streamProvider, ok := provider.(StreamProvider)
	if !ok {
		return nil, errors.New("proxyapi: provider does not support streaming responses")
	}
	return streamProvider.StreamResponse(ctx, s.client.withCredential(req))
}
