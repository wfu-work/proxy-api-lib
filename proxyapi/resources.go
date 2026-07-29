package proxyapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/wfu-work/proxy-api-lib/openai"
)

// EmbeddingsService 提供 OpenAI 官方 Embeddings API。
type EmbeddingsService struct{ client *Client }

// EmbeddingRequest 是 OpenAI Embeddings 请求参数。
type EmbeddingRequest = openai.EmbeddingRequest

// Embedding 是单条向量结果。
type Embedding = openai.Embedding

// EmbeddingUsage 是 Embeddings 请求的 Token 用量。
type EmbeddingUsage = openai.EmbeddingUsage

// EmbeddingResponse 是 OpenAI Embeddings 响应。
type EmbeddingResponse = openai.EmbeddingResponse

// Create 创建文本向量，并保留 OpenAI 原始响应和请求 ID。
// 请求中的 Credential 非空时会覆盖客户端默认凭据。
func (s *EmbeddingsService) Create(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error) {
	payload := map[string]any{"model": req.Model, "input": req.Input}
	if req.EncodingFormat != "" {
		payload["encoding_format"] = req.EncodingFormat
	}
	if req.Dimensions != nil {
		payload["dimensions"] = *req.Dimensions
	}
	if req.User != "" {
		payload["user"] = req.User
	}
	for key, value := range req.Extra {
		payload[key] = value
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	httpReq, err := s.client.newRequest(ctx, http.MethodPost, "/embeddings", body, "application/json", req.Credential)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, parseAPIError(resp.StatusCode, resp.Header.Get("x-request-id"), respBody)
	}
	var out EmbeddingResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, err
	}
	out.Raw = append(out.Raw[:0], respBody...)
	out.RequestID = resp.Header.Get("x-request-id")
	return &out, nil
}

// ModelsService 提供 OpenAI 官方 Models API。
type ModelsService struct{ client *Client }

// Model 是 OpenAI 模型元数据。
type Model = openai.Model

// ModelList 是 OpenAI 模型列表响应。
type ModelList = openai.ModelList

// List 获取当前凭据可访问的 OpenAI 模型列表。
func (s *ModelsService) List(ctx context.Context) (*ModelList, error) {
	httpReq, err := s.client.newRequest(ctx, http.MethodGet, "/models", nil, "application/json", nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, parseAPIError(resp.StatusCode, resp.Header.Get("x-request-id"), body)
	}
	var out ModelList
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
