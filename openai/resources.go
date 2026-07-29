package openai

import (
	"encoding/json"
)

// EmbeddingRequest 描述 OpenAI 官方 Embeddings API 请求。
// Extra 用于透传扩展字段，Credential 可覆盖客户端默认凭据。
type EmbeddingRequest struct {
	Model          string         `json:"model"`
	Input          any            `json:"input"`
	EncodingFormat string         `json:"encoding_format,omitempty"`
	Dimensions     *int           `json:"dimensions,omitempty"`
	User           string         `json:"user,omitempty"`
	Extra          map[string]any `json:"-"`
	Credential     Credential     `json:"-"`
}

// Embedding 表示单条向量结果。
type Embedding struct {
	Object    string    `json:"object,omitempty"`
	Embedding []float64 `json:"embedding,omitempty"`
	Index     int       `json:"index,omitempty"`
}

// EmbeddingUsage 记录 Embeddings 请求消耗的 Token。
type EmbeddingUsage struct {
	PromptTokens int `json:"prompt_tokens,omitempty"`
	TotalTokens  int `json:"total_tokens,omitempty"`
}

// EmbeddingResponse 描述 OpenAI Embeddings API 响应。
// Raw 保留原始 JSON，RequestID 保存响应头中的 OpenAI 请求 ID。
type EmbeddingResponse struct {
	Object    string          `json:"object,omitempty"`
	Data      []Embedding     `json:"data,omitempty"`
	Model     string          `json:"model,omitempty"`
	Usage     EmbeddingUsage  `json:"usage,omitempty"`
	Raw       json.RawMessage `json:"-"`
	RequestID string          `json:"-"`
}

// Model 描述 OpenAI 模型元数据。
type Model struct {
	ID      string `json:"id,omitempty"`
	Object  string `json:"object,omitempty"`
	Created int64  `json:"created,omitempty"`
	OwnedBy string `json:"owned_by,omitempty"`
}

// ModelList 描述 OpenAI Models API 的列表响应。
type ModelList struct {
	Object string  `json:"object,omitempty"`
	Data   []Model `json:"data,omitempty"`
}
